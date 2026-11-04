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
	g.GET("/links/:merchant/:slug", srv.publicLink)
	g.POST("/links/:merchant/:slug/sessions", srv.throttle(srv.payLimiter, "link"), srv.publicLinkSession)
	g.GET("/pricing", func(c *gin.Context) { c.JSON(200, gin.H{"default_fee_bps": srv.A.DefaultFeeBps(ctx(c))}) })
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
	c.JSON(200, gin.H{"name": l.Name, "amount": l.Amount, "custom_amount": l.CustomAmount, "active": active, "type": l.Kind,
		"payment_method_types": l.Methods, "livemode": l.Livemode, "merchant": publicMerchant(m)})
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
