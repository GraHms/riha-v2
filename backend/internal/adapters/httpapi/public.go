package httpapi

import (
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
)

func (srv *Server) publicRoutes(g *gin.RouterGroup) {
	g.GET("/checkout/:id", srv.publicSession)
	g.POST("/checkout/:id/pay", srv.throttle(srv.payLimiter, "pay"), srv.publicPay)
	g.POST("/checkout/:id/cancel", srv.publicCancel)
	g.POST("/checkout/:id/telemetry", srv.throttle(srv.viewLimiter, "telemetry"), srv.publicTelemetry)
	g.GET("/links/:merchant/:slug", srv.publicLink)
	g.POST("/links/:merchant/:slug/sessions", srv.throttle(srv.payLimiter, "link"), srv.publicLinkSession)
	g.GET("/links/:merchant/:slug/image", srv.throttle(srv.viewLimiter, "link-image"), srv.publicLinkImage)
	srv.publicSubscriptionRoutes(g)
	g.GET("/pricing", func(c *gin.Context) {
		c.JSON(200, gin.H{"default_fee_bps": srv.A.DefaultFeeBps(ctx(c)), "coming_soon": srv.A.ComingSoon})
	})
}

func (srv *Server) publicSession(c *gin.Context) {
	cs, err := srv.A.Session(ctx(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, srv.sessionView(ctx(c), cs))
}

func (srv *Server) publicPay(c *gin.Context) {
	var in app.PayInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	in.PayerIP, in.PayerUA = c.ClientIP(), c.GetHeader("User-Agent")
	if _, err := srv.A.Pay(ctx(c), c.Param("id"), in); err != nil {
		fail(c, err)
		return
	}
	srv.publicSession(c)
}

func (srv *Server) publicCancel(c *gin.Context) {
	cs, err := srv.A.Session(ctx(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"cancel_url": cs.CancelURL})
}

func (srv *Server) publicLink(c *gin.Context) {
	l, m, err := srv.A.PublicLink(ctx(c), c.Param("merchant"), c.Param("slug"))
	if err != nil {
		fail(c, err)
		return
	}
	active := l.Active && (l.ExpiresAt == nil || l.ExpiresAt.After(time.Now()))
	lo, hi := l.CustomRange()
	c.JSON(200, gin.H{"name": l.Name, "amount": l.Amount, "custom_amount": l.CustomAmount, "active": active, "type": l.Kind,
		"details": l.Details, "image_url": linkImageURL(l, m.Slug), "submit_type": l.SubmitTypeOrPay(), "suggested_amount": l.SuggestedAmount, "min_amount": lo, "max_amount": hi,
		"payment_method_types": srv.A.ActiveMethods(l.Methods), "coming_soon": srv.A.ComingSoon, "livemode": l.Livemode, "merchant": publicMerchant(m)})
}

var channels = map[string]bool{"whatsapp": true, "sms": true, "qr": true, "outro": true}

func (srv *Server) publicLinkSession(c *gin.Context) {
	l, _, err := srv.A.PublicLink(ctx(c), c.Param("merchant"), c.Param("slug"))
	if err != nil {
		fail(c, err)
		return
	}
	var in struct {
		Amount  int64  `json:"amount"`
		Channel string `json:"channel"`
	}
	_ = c.ShouldBindJSON(&in)
	if !channels[in.Channel] {
		in.Channel = "outro"
	}
	cs, err := srv.A.CreateLinkSession(ctx(c), l, in.Amount, in.Channel, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"id": cs.ID})
}

func (srv *Server) publicLinkImage(c *gin.Context) {
	b, err := srv.A.LinkImage(ctx(c), c.Param("merchant"), c.Param("slug"))
	if err != nil {
		c.Status(404)
		return
	}
	c.Header("Cache-Control", "public, max-age=2592000, immutable")
	c.Data(200, "image/jpeg", b)
}

func (srv *Server) publicTelemetry(c *gin.Context) {
	var in struct {
		Events []app.ClientEvent `json:"events"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.Status(204)
		return
	}
	_ = srv.A.ClientTelemetry(ctx(c), c.Param("id"), in.Events)
	c.Status(204)
}
