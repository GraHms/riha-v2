package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"regexp"
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
		var body []byte
		if c.Request.Body != nil {
			body, _ = io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
		w := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = w
		c.Next()
		k := apiKey(c)
		took := time.Since(start).Milliseconds()
		srv.A.LogRequest(ctx(c), &domain.RequestLog{MerchantID: k.MerchantID, Livemode: k.Livemode, Method: c.Request.Method, Path: c.Request.URL.Path,
			Status: c.Writer.Status(), APIKeyID: k.ID, IP: c.ClientIP(), DurationMs: took, RequestID: c.GetString("request_id")})
		out := w.buf.Bytes()
		if c.Request.URL.RawQuery != "" && len(body) == 0 {
			body = []byte(c.Request.URL.RawQuery)
		}
		session, payment := threadOf(c.Request.URL.Path, out)
		srv.A.TraceAPI(ctx(c), domain.TrailEvent{MerchantID: k.MerchantID, Livemode: k.Livemode, SessionID: session, PaymentID: payment,
			Method: c.Request.Method, Path: c.Request.URL.Path, Status: c.Writer.Status(), DurationMs: took,
			IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"), Request: domain.RedactBody(body), Response: domain.RedactBody(out),
			Data: map[string]any{"api_key": "…" + k.Last4, "idempotency_key": c.GetHeader("Idempotency-Key")}})
	}
}

var (
	sessionInPath = regexp.MustCompile(`/(cs_(?:test|live)_[A-Za-z0-9]+)`)
	paymentInPath = regexp.MustCompile(`/(pay_[A-Za-z0-9]+)`)
)

// threadOf finds the checkout and payment an API call is about, in its path or its answer.
func threadOf(path string, response []byte) (string, string) {
	session, payment := "", ""
	if m := sessionInPath.FindStringSubmatch(path); m != nil {
		session = m[1]
	}
	if m := paymentInPath.FindStringSubmatch(path); m != nil {
		payment = m[1]
	}
	var r map[string]any
	if json.Unmarshal(response, &r) == nil {
		str := func(k string) string { v, _ := r[k].(string); return v }
		if id := str("id"); session == "" && strings.HasPrefix(id, "cs_") {
			session = id
		} else if payment == "" && strings.HasPrefix(id, "pay_") {
			payment = id
		}
		if v := str("payment"); payment == "" && strings.HasPrefix(v, "pay_") {
			payment = v
		}
		if v := str("checkout_session"); session == "" && strings.HasPrefix(v, "cs_") {
			session = v
		}
		if obj, ok := r["payment"].(map[string]any); ok && payment == "" {
			if id, _ := obj["id"].(string); strings.HasPrefix(id, "pay_") {
				payment = id
			}
		}
	}
	return session, payment
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
		claim := &domain.IdempotencyRecord{MerchantID: k.MerchantID, Livemode: k.Livemode, Key: key, Path: c.Request.URL.Path, RequestHash: hash}
		mine, rec, err := srv.A.ClaimIdempotent(ctx(c), claim)
		if err != nil {
			fail(c, err)
			return
		}
		if !mine {
			if rec.RequestHash != hash {
				fail(c, domain.Conflict("idempotency_key_reused", "Esta Idempotency-Key foi usada com outro pedido."))
				return
			}
			if rec.StatusCode == 0 {
				fail(c, domain.Conflict("idempotency_in_progress", "Um pedido com esta Idempotency-Key ainda está em curso. Repita daqui a pouco."))
				return
			}
			c.Header("Idempotent-Replayed", "true")
			c.Data(rec.StatusCode, "application/json; charset=utf-8", []byte(rec.ResponseBody))
			c.Abort()
			return
		}
		w := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = w
		done := false
		defer func() {
			// A request that panicked or failed on RIHA's side frees its key.
			if !done {
				srv.A.ReleaseIdempotent(context.WithoutCancel(ctx(c)), claim.ID)
			}
		}()
		c.Next()
		if st := c.Writer.Status(); st < 500 {
			srv.A.FinishIdempotent(context.WithoutCancel(ctx(c)), claim.ID, st, w.buf.String())
			done = true
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

// prune makes room without forgetting who is being held back: buckets that
// have filled up again are the same as new ones and go first; then any not
// spent. A flood of new keys never frees an exhausted one.
func (r *rateLimiter) prune(now time.Time) {
	for k, b := range r.b {
		if b.tokens+now.Sub(b.last).Seconds()*r.rate >= r.capacity {
			delete(r.b, k)
		}
	}
	if len(r.b) <= 50000 {
		return
	}
	for k, b := range r.b {
		if b.tokens+now.Sub(b.last).Seconds()*r.rate >= 1 {
			delete(r.b, k)
		}
	}
}

func (r *rateLimiter) allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	b, ok := r.b[key]
	if !ok {
		if len(r.b) > 100000 {
			r.prune(now)
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
