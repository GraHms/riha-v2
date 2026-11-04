// Package api wires HTTP routes: the dashboard API (/api, cookie sessions),
// the hosted checkout (/api/public), the admin API (/api/admin) and the
// public REST API (/v1, secret keys).
package api

import (
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"riha/internal/ids"
	"riha/internal/models"
	"riha/internal/service"
)

type Server struct {
	S       *service.Service
	limiter *rateLimiter
}

const cookieName = "riha_session"

func New(s *service.Service) *gin.Engine {
	srv := &Server{S: s, limiter: newRateLimiter(100)}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), requestID(), accessLog())
	r.MaxMultipartMemory = 8 << 20

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	api := r.Group("/api", csrf())
	{
		api.POST("/auth/signup", srv.signup)
		api.POST("/auth/login", srv.login)
		api.POST("/auth/logout", srv.logout)
		api.GET("/auth/me", srv.auth(), srv.me)
		api.POST("/auth/password", srv.auth(), srv.changePassword)

		pub := api.Group("/public")
		pub.GET("/checkout/:id", srv.publicSession)
		pub.POST("/checkout/:id/pay", srv.publicPay)
		pub.POST("/checkout/:id/cancel", srv.publicCancel)
		pub.GET("/links/:merchant/:slug", srv.publicLink)
		pub.POST("/links/:merchant/:slug/sessions", srv.publicLinkSession)
		pub.GET("/receipts/:ref", srv.publicReceipt)
		pub.GET("/pricing", func(c *gin.Context) { c.JSON(200, gin.H{"default_fee_bps": srv.S.DefaultFeeBps()}) })

		m := api.Group("", srv.auth(), srv.merchantOnly())
		m.GET("/merchant", srv.getMerchant)
		m.PATCH("/merchant", srv.updateMerchant)
		m.GET("/merchant/kyc", srv.getKyc)
		m.POST("/merchant/kyc", srv.saveKyc)
		m.POST("/merchant/kyc/documents", srv.uploadDoc)
		m.POST("/merchant/kyc/submit", srv.submitKyc)

		m.GET("/overview", srv.overview)
		m.GET("/payments", srv.listPayments)
		m.GET("/payments/export.csv", srv.exportPayments)
		m.GET("/payments/:id", srv.getPayment)
		m.POST("/payments/:id/refund", srv.refundPayment)
		m.POST("/payments/:id/receipt", srv.sendReceipt)

		m.GET("/links", srv.listLinks)
		m.POST("/links", srv.createLink)
		m.PATCH("/links/:id", srv.updateLink)

		m.GET("/balance", srv.balance)
		m.GET("/payouts", srv.listPayouts)
		m.POST("/payouts", srv.createPayout)
		m.GET("/bank-account", srv.getBank)
		m.PUT("/bank-account", srv.putBank)

		m.GET("/keys", srv.getKeys)
		m.POST("/keys/roll", srv.rollKey)
		m.GET("/api-logs", srv.apiLogs)

		m.GET("/developers/settings", srv.devSettings)
		m.PUT("/developers/redirects", srv.putRedirects)
		m.POST("/developers/endpoints", srv.createEndpoint)
		m.PATCH("/developers/endpoints/:id", srv.updateEndpoint)
		m.DELETE("/developers/endpoints/:id", srv.deleteEndpoint)
		m.POST("/developers/domains", srv.addDomain)
		m.DELETE("/developers/domains/:id", srv.deleteDomain)
		m.GET("/developers/deliveries", srv.listDeliveries)
		m.POST("/developers/deliveries/:id/resend", srv.resendDelivery)
		m.POST("/developers/test-event", srv.testEvent)
		m.GET("/developers/secret", srv.revealSecret)

		a := api.Group("/admin", srv.auth(), srv.adminOnly())
		a.GET("/overview", srv.adminOverview)
		a.GET("/approvals", srv.adminApprovals)
		a.GET("/merchants", srv.adminMerchants)
		a.GET("/merchants/:id", srv.adminMerchant)
		a.PATCH("/merchants/:id", srv.adminUpdateMerchant)
		a.POST("/merchants/:id/decision", srv.adminDecision)
		a.GET("/documents/:id", srv.adminDocument)
		a.GET("/payouts", srv.adminPayouts)
		a.POST("/payouts/action", srv.adminPayoutAction)
		a.GET("/payouts/export.csv", srv.adminPayoutExport)
		a.GET("/settings", srv.adminSettings)
		a.PUT("/settings", srv.adminPutSettings)
		a.GET("/payments", srv.adminPayments)
	}

	v1 := r.Group("/v1", srv.apiKeyAuth(), srv.apiLog(), srv.idempotency())
	{
		v1.POST("/checkout", srv.v1CreateSession) // short alias used on the landing page
		v1.POST("/checkout/sessions", srv.v1CreateSession)
		v1.GET("/checkout/sessions", srv.v1ListSessions)
		v1.GET("/checkout/sessions/:id", srv.v1GetSession)
		v1.POST("/checkout/sessions/:id/expire", srv.v1ExpireSession)
		v1.GET("/payments", srv.v1ListPayments)
		v1.GET("/payments/:id", srv.v1GetPayment)
		v1.POST("/refunds", srv.v1CreateRefund)
		v1.GET("/payment_links", srv.v1ListLinks)
		v1.POST("/payment_links", srv.v1CreateLink)
		v1.PATCH("/payment_links/:id", srv.v1UpdateLink)
		v1.GET("/balance", srv.v1Balance)
		v1.GET("/payouts", srv.v1ListPayouts)
		v1.POST("/payouts", srv.v1CreatePayout)
		v1.GET("/webhook_endpoints", srv.v1ListEndpoints)
		v1.POST("/webhook_endpoints", srv.v1CreateEndpoint)
		v1.DELETE("/webhook_endpoints/:id", srv.v1DeleteEndpoint)
		v1.GET("/events/:id", srv.v1GetEvent)
	}
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/v1") {
			apiErr(c, &service.Error{Status: 404, Type: "invalid_request_error", Code: "route_not_found", Message: "Rota desconhecida: " + c.Request.Method + " " + c.Request.URL.Path})
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(404, gin.H{"error": gin.H{"code": "route_not_found", "message": "Rota desconhecida."}})
			return
		}
		srv.static(c)
	})
	return r
}

// static serves the built frontend with SPA fallback (production mode).
func (srv *Server) static(c *gin.Context) {
	dir := srv.S.Cfg.StaticDir
	if dir == "" {
		c.String(404, "Riha API. O frontend corre em "+srv.S.Cfg.PublicURL)
		return
	}
	p := filepath.Join(dir, filepath.Clean("/"+c.Request.URL.Path))
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		c.File(p)
		return
	}
	c.File(filepath.Join(dir, "index.html"))
}

// ---------------------------------------------------------------- helpers

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := "req_" + ids.Token("", 10)
		c.Set("request_id", id)
		c.Header("Request-Id", id)
		c.Next()
	}
}

func accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if c.Request.URL.Path == "/healthz" || !(strings.HasPrefix(c.Request.URL.Path, "/api") || strings.HasPrefix(c.Request.URL.Path, "/v1")) {
			return
		}
		log.Printf("%s %s %d %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start).Round(time.Millisecond))
	}
}

// csrf requires a custom header on state-changing dashboard requests. Browsers
// cannot send it cross-site without a CORS preflight, which we never allow.
func csrf() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.GetHeader("X-Requested-With") != "riha" {
			c.AbortWithStatusJSON(403, gin.H{"error": gin.H{"code": "csrf", "message": "Pedido recusado."}})
			return
		}
		c.Next()
	}
}

func apiErr(c *gin.Context, err error) {
	var e *service.Error
	if errors.As(err, &e) {
		body := gin.H{"type": e.Type, "code": e.Code, "message": e.Message, "request_id": c.GetString("request_id")}
		if e.Param != "" {
			body["param"] = e.Param
		}
		if strings.HasPrefix(c.Request.URL.Path, "/v1") {
			body["doc_url"] = "https://docs.riha.co.mz/erros#" + e.Code
		}
		c.AbortWithStatusJSON(e.Status, gin.H{"error": body})
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatusJSON(404, gin.H{"error": gin.H{"type": "invalid_request_error", "code": "resource_missing", "message": "Recurso não encontrado.", "request_id": c.GetString("request_id")}})
		return
	}
	log.Printf("internal error %s: %v", c.GetString("request_id"), err)
	c.AbortWithStatusJSON(500, gin.H{"error": gin.H{"type": "api_error", "code": "internal_error", "message": "Erro interno da Riha. Repita com a mesma Idempotency-Key.", "request_id": c.GetString("request_id")}})
}

func bad(c *gin.Context, msg string) {
	apiErr(c, service.Invalid("parameter_invalid", "", msg))
}

func (srv *Server) auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, err := c.Cookie(cookieName)
		if err != nil || tok == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": gin.H{"code": "unauthenticated", "message": "Inicie sessão."}})
			return
		}
		u, err := srv.S.UserForSession(tok)
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
		u := user(c)
		if u.MerchantID == nil {
			c.AbortWithStatusJSON(403, gin.H{"error": gin.H{"code": "forbidden", "message": "Esta conta não tem negócio associado."}})
			return
		}
		var m models.Merchant
		if err := srv.S.DB.First(&m, "id = ?", *u.MerchantID).Error; err != nil {
			apiErr(c, err)
			return
		}
		c.Set("merchant", &m)
		c.Next()
	}
}

func (srv *Server) adminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if user(c).Role != models.RoleAdmin {
			c.AbortWithStatusJSON(403, gin.H{"error": gin.H{"code": "forbidden", "message": "Só administradores."}})
			return
		}
		c.Next()
	}
}

func user(c *gin.Context) *models.User         { return c.MustGet("user").(*models.User) }
func merchant(c *gin.Context) *models.Merchant { return c.MustGet("merchant").(*models.Merchant) }

// live reports the dashboard mode (X-Riha-Mode header, default live).
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
