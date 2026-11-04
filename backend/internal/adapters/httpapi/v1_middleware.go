package httpapi

import (
	"bytes"
	"encoding/base64"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/domain"
)

func (srv *Server) apiKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		var secret string
		switch {
		case strings.HasPrefix(h, "Bearer "):
			secret = strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		case strings.HasPrefix(h, "Basic "):
			raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(h, "Basic "))
			secret = strings.SplitN(string(raw), ":", 2)[0]
		}
		if secret == "" {
			fail(c, domain.Unauthorized("api_key_missing", "Envie a chave secreta no cabeçalho Authorization: Bearer sk_…"))
			return
		}
		k, m, err := srv.A.AuthenticateSecret(ctx(c), secret)
		if err != nil {
			fail(c, err)
			return
		}
		if !srv.limiter.allow(m.ID) {
			fail(c, &domain.Error{Status: 429, Type: "rate_limit_error", Code: "rate_limited", Message: "Demasiados pedidos (100 por segundo por conta)."})
			return
		}
		c.Set("merchant", m)
		c.Set("api_key", k)
		c.Set("livemode", k.Livemode)
		c.Next()
	}
}

func apiKey(c *gin.Context) *domain.APIKey { return c.MustGet("api_key").(*domain.APIKey) }

func (srv *Server) apiLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		k := apiKey(c)
		srv.A.LogRequest(ctx(c), &domain.RequestLog{MerchantID: k.MerchantID, Livemode: k.Livemode, Method: c.Request.Method, Path: c.Request.URL.Path,
			Status: c.Writer.Status(), APIKeyID: k.ID, IP: c.ClientIP(), DurationMs: time.Since(start).Milliseconds(), RequestID: c.GetString("request_id")})
	}
}

type captureWriter struct {
	gin.ResponseWriter
	buf bytes.Buffer
}

func (w *captureWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

func (srv *Server) idempotency() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if c.Request.Method != "POST" || key == "" {
			c.Next()
			return
		}
		if len(key) > 255 {
			fail(c, domain.Invalid("idempotency_key_invalid", "Idempotency-Key", "Idempotency-Key até 255 caracteres."))
			return
		}
		body, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		hash := domain.Hash(c.Request.URL.Path + "\n" + string(body))
		k := apiKey(c)
		if rec := srv.A.IdempotentResponse(ctx(c), k.MerchantID, k.Livemode, key); rec != nil {
			if rec.RequestHash != hash {
				fail(c, domain.Conflict("idempotency_key_reused", "Esta Idempotency-Key foi usada com outro pedido."))
				return
			}
			c.Header("Idempotent-Replayed", "true")
			c.Data(rec.StatusCode, "application/json; charset=utf-8", []byte(rec.ResponseBody))
			c.Abort()
			return
		}
		w := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = w
		c.Next()
		if st := c.Writer.Status(); st < 500 {
			srv.A.StoreIdempotent(ctx(c), &domain.IdempotencyRecord{MerchantID: k.MerchantID, Livemode: k.Livemode, Key: key,
				Path: c.Request.URL.Path, RequestHash: hash, StatusCode: st, ResponseBody: w.buf.String()})
		}
	}
}

type rateLimiter struct {
	mu       sync.Mutex
	capacity float64
	rate     float64
	b        map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newRateLimiter(capacity, perSecond float64) *rateLimiter {
	return &rateLimiter{capacity: capacity, rate: perSecond, b: map[string]*bucket{}}
}

func (r *rateLimiter) allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	b, ok := r.b[key]
	if !ok {
		if len(r.b) > 100000 {
			r.b = map[string]*bucket{}
		}
		b = &bucket{tokens: r.capacity, last: now}
		r.b[key] = b
	}
	b.tokens = min(r.capacity, b.tokens+now.Sub(b.last).Seconds()*r.rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
