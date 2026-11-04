package api

import (
	"bytes"
	"encoding/base64"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/ids"
	"riha/internal/models"
	"riha/internal/service"
)

// apiKeyAuth accepts "Authorization: Bearer sk_…" or HTTP basic "-u sk_…:".
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
			apiErr(c, &service.Error{Status: 401, Type: "authentication_error", Code: "api_key_missing", Message: "Envie a chave secreta no cabeçalho Authorization: Bearer sk_…"})
			return
		}
		k, m, err := srv.S.AuthenticateSecret(secret)
		if err != nil {
			apiErr(c, err)
			return
		}
		if !srv.limiter.allow(m.ID) {
			apiErr(c, &service.Error{Status: 429, Type: "rate_limit_error", Code: "rate_limited", Message: "Demasiados pedidos (100 por segundo por conta)."})
			return
		}
		c.Set("merchant", m)
		c.Set("api_key", k)
		c.Set("livemode", k.Livemode)
		c.Next()
	}
}

func apiKey(c *gin.Context) *models.APIKey { return c.MustGet("api_key").(*models.APIKey) }

func (srv *Server) apiLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		m := merchant(c)
		k := apiKey(c)
		srv.S.DB.Create(&models.APIRequestLog{ID: ids.New("rlog"), MerchantID: m.ID, Livemode: k.Livemode, Method: c.Request.Method,
			Path: c.Request.URL.Path, Status: c.Writer.Status(), APIKeyID: k.ID, IP: c.ClientIP(),
			DurationMs: time.Since(start).Milliseconds(), RequestID: c.GetString("request_id")})
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

// idempotency replays the stored response for a repeated Idempotency-Key and
// rejects reuse of a key with a different body (409).
func (srv *Server) idempotency() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if c.Request.Method != "POST" || key == "" {
			c.Next()
			return
		}
		if len(key) > 255 {
			apiErr(c, service.Invalid("idempotency_key_invalid", "Idempotency-Key", "Idempotency-Key até 255 caracteres."))
			return
		}
		body, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		hash := ids.Hash(c.Request.URL.Path + "\n" + string(body))
		m, k := merchant(c), apiKey(c)
		var rec models.IdempotencyRecord
		if err := srv.S.DB.First(&rec, "merchant_id = ? AND livemode = ? AND key = ?", m.ID, k.Livemode, key).Error; err == nil {
			if rec.RequestHash != hash {
				apiErr(c, service.Conflict("idempotency_key_reused", "Esta Idempotency-Key foi usada com outro pedido."))
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
			srv.S.DB.Create(&models.IdempotencyRecord{ID: ids.New("idem"), MerchantID: m.ID, Livemode: k.Livemode, Key: key,
				Path: c.Request.URL.Path, RequestHash: hash, StatusCode: st, ResponseBody: w.buf.String()})
		}
	}
}

// rateLimiter is a per-account token bucket (in memory, per process).
type rateLimiter struct {
	mu   sync.Mutex
	rate float64
	b    map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newRateLimiter(perSecond float64) *rateLimiter {
	return &rateLimiter{rate: perSecond, b: map[string]*bucket{}}
}

func (r *rateLimiter) allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	b, ok := r.b[key]
	if !ok {
		b = &bucket{tokens: r.rate, last: now}
		r.b[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * r.rate
	if b.tokens > r.rate {
		b.tokens = r.rate
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
