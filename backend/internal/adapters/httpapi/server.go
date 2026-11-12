package httpapi

import (
	"context"
	"encoding/csv"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

type Config struct {
	StaticDir      string
	CookieSecure   bool
	TrustedProxies []string
	// CatalogSubdomains gives each catalog its own subdomain. Only once the
	// proxy in front answers for them.
	CatalogSubdomains bool
	// SMSWebhookSecret is the secret MozeSMS gave for its delivery reports. Their
	// signature isn't documented, so it's only used to log whether one matches.
	SMSWebhookSecret string
}

type Server struct {
	A           *app.App
	cfg         Config
	limiter     *rateLimiter
	authLimiter *rateLimiter
	payLimiter  *rateLimiter
	shellOnce   sync.Once
	shell       string
	cards       cardCache
	viewLimiter *rateLimiter
}

const cookieName = "riha_session"

func New(a *app.App, cfg Config) *gin.Engine {
	srv := &Server{A: a, cfg: cfg, limiter: newRateLimiter(100, 100), authLimiter: newRateLimiter(10, 10.0/60), payLimiter: newRateLimiter(20, 20.0/60), viewLimiter: newRateLimiter(120, 2)}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	_ = r.SetTrustedProxies(cfg.TrustedProxies)
	r.Use(gin.Recovery(), requestID(), securityHeaders(cfg.CookieSecure), bodyLimit(), accessLog())
	r.MaxMultipartMemory = 8 << 20
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.Any("/callbacks/bim", srv.throttle(srv.limiter, "callback"), srv.railCallback)
	r.POST("/callbacks/mozesms", srv.throttle(srv.limiter, "callback"), srv.smsReport)
	r.GET("/robots.txt", srv.robots)
	r.GET("/sitemap.xml", srv.sitemap)
	r.GET("/og/*rest", srv.throttle(srv.limiter, "og"), srv.ogImage)
	r.GET("/internal/tls-allowed", srv.throttle(srv.limiter, "tls"), srv.tlsAllowed)
	srv.dashboardRoutes(r.Group("/api", csrf()))
	srv.v1Routes(r.Group("/v1", srv.apiKeyAuth(), srv.apiLog(), srv.idempotency()))
	r.NoRoute(srv.noRoute)
	return r
}

func (srv *Server) noRoute(c *gin.Context) {
	p := c.Request.URL.Path
	switch {
	case strings.HasPrefix(p, "/v1"):
		fail(c, &domain.Error{Status: 404, Type: "invalid_request_error", Code: "route_not_found", Message: "Rota desconhecida: " + c.Request.Method + " " + p})
	case strings.HasPrefix(p, "/api"):
		c.JSON(404, gin.H{"error": gin.H{"code": "route_not_found", "message": "Rota desconhecida."}})
	case srv.cfg.StaticDir == "":
		c.String(404, "RIHA API. O frontend corre em "+srv.A.PublicURL)
	default:
		f := filepath.Join(srv.cfg.StaticDir, filepath.Clean("/"+p))
		if st, err := os.Stat(f); err == nil && !st.IsDir() {
			c.File(f)
			return
		}
		srv.servePage(c)
	}
}

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := "req_" + domain.Token("", 10)
		c.Set("request_id", id)
		c.Header("Request-Id", id)
		c.Request = c.Request.WithContext(domain.WithRequest(c.Request.Context(), domain.RequestInfo{ID: id, IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")}))
		c.Next()
	}
}

const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'"

func securityHeaders(https bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Content-Security-Policy", csp)
		if https {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api") || strings.HasPrefix(c.Request.URL.Path, "/v1") {
			h.Set("Cache-Control", "no-store")
		}
		c.Next()
	}
}

func bodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := int64(1 << 20)
		if strings.HasPrefix(c.ContentType(), "multipart/") {
			limit = 9 << 20
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

func (srv *Server) throttle(l *rateLimiter, name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.allow(name + "|" + c.ClientIP()) {
			fail(c, &domain.Error{Status: 429, Type: "rate_limit_error", Code: "rate_limited", Message: "Demasiadas tentativas. Aguarde um minuto e tente de novo."})
			return
		}
		c.Next()
	}
}

func accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/v1") {
			log.Printf("%s %s %d %s", c.Request.Method, p, c.Writer.Status(), time.Since(start).Round(time.Millisecond))
		}
	}
}

func csrf() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.GetHeader("X-Requested-With") != "riha" {
			c.AbortWithStatusJSON(403, gin.H{"error": gin.H{"code": "csrf", "message": "Pedido recusado."}})
			return
		}
		c.Next()
	}
}

func fail(c *gin.Context, err error) {
	var e *domain.Error
	if errors.As(err, &e) {
		body := gin.H{"type": e.Type, "code": e.Code, "message": e.Message, "request_id": c.GetString("request_id")}
		if e.Param != "" {
			body["param"] = e.Param
		}
		if strings.HasPrefix(c.Request.URL.Path, "/v1") {
			body["doc_url"] = "https://rihagateway.com/docs#erros"
		}
		c.AbortWithStatusJSON(e.Status, gin.H{"error": body})
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		c.AbortWithStatusJSON(404, gin.H{"error": gin.H{"type": "invalid_request_error", "code": "resource_missing", "message": "Recurso não encontrado.", "request_id": c.GetString("request_id")}})
		return
	}
	log.Printf("internal error %s: %v", c.GetString("request_id"), err)
	c.AbortWithStatusJSON(500, gin.H{"error": gin.H{"type": "api_error", "code": "internal_error", "message": "Erro interno da RIHA. Repita com a mesma Idempotency-Key.", "request_id": c.GetString("request_id")}})
}

func bad(c *gin.Context, msg string) { fail(c, domain.Invalid("parameter_invalid", "", msg)) }

func ctx(c *gin.Context) context.Context { return c.Request.Context() }

func (srv *Server) auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, err := c.Cookie(cookieName)
		if err != nil || tok == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": gin.H{"code": "unauthenticated", "message": "Inicie sessão."}})
			return
		}
		u, err := srv.A.UserForSession(ctx(c), tok)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": gin.H{"code": "unauthenticated", "message": "Sessão expirada. Inicie sessão de novo."}})
			return
		}
		c.Set("user", u)
		c.Next()
	}
}

func (srv *Server) merchantOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		m, err := srv.A.MerchantForUser(ctx(c), user(c))
		if err != nil {
			fail(c, err)
			return
		}
		c.Set("merchant", m)
		c.Next()
	}
}

func adminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if user(c).Role != domain.RoleAdmin {
			c.AbortWithStatusJSON(403, gin.H{"error": gin.H{"code": "forbidden", "message": "Só administradores."}})
			return
		}
		c.Next()
	}
}

func user(c *gin.Context) *domain.User         { return c.MustGet("user").(*domain.User) }
func merchant(c *gin.Context) *domain.Merchant { return c.MustGet("merchant").(*domain.Merchant) }

func actor(c *gin.Context) app.Actor {
	u := user(c)
	return app.Actor{ID: u.ID, Role: u.Role, IP: c.ClientIP()}
}

func live(c *gin.Context) bool {
	if v, ok := c.Get("livemode"); ok {
		return v.(bool)
	}
	return c.GetHeader("X-Riha-Mode") != "test"
}

func qInt(c *gin.Context, k string, def int) int {
	if v, err := strconv.Atoi(c.Query(k)); err == nil {
		return v
	}
	return def
}

func iso(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

func isoT(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// csvSafe keeps a spreadsheet from reading a cell as a formula: text that
// starts like one gets a leading quote. Numbers, negative ones too, stay.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		if _, err := strconv.ParseFloat(s, 64); err == nil {
			return s
		}
		return "'" + s
	}
	return s
}

// writeSafe writes a CSV row with every cell made safe.
func writeSafe(w *csv.Writer, row []string) error {
	for i := range row {
		row[i] = csvSafe(row[i])
	}
	return w.Write(row)
}

func centsCSV(v int64) string { return strconv.FormatInt(v/100, 10) + "." + pad2(v%100) }

func pad2(v int64) string {
	if v < 10 {
		return "0" + strconv.FormatInt(v, 10)
	}
	return strconv.FormatInt(v, 10)
}
