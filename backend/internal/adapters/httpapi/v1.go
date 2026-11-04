package httpapi

import (
	"encoding/json"
	"net/url"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

func (srv *Server) v1Routes(v1 *gin.RouterGroup) {
	v1.POST("/checkout", srv.v1CreateSession)
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

func list(items any, hasMore bool) gin.H {
	return gin.H{"object": "list", "data": items, "has_more": hasMore}
}

func limit(c *gin.Context) int { return max(1, min(100, qInt(c, "limit", 10))) }

func originDomain(c *gin.Context, successURL string) string {
	for _, h := range []string{c.GetHeader("Origin"), c.GetHeader("Referer"), successURL} {
		if u, err := url.Parse(h); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}
	return ""
}

func (srv *Server) v1CreateSession(c *gin.Context) {
	var p app.SessionParams
	if err := bind(c, &p); err != nil {
		fail(c, err)
		return
	}
	k := apiKey(c)
	p.Origin = domain.Origin{SourceType: domain.SourceAPI, APIKeyID: &k.ID, IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"), Domain: originDomain(c, p.SuccessURL)}
	cs, err := srv.A.CreateSession(ctx(c), merchant(c), k.Livemode, p)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, domain.SessionResource(cs, srv.A.PayURL))
}

func (srv *Server) v1GetSession(c *gin.Context) {
	cs, err := srv.A.MerchantSession(ctx(c), merchant(c).ID, live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, domain.SessionResource(cs, srv.A.PayURL))
}

func (srv *Server) v1ListSessions(c *gin.Context) {
	n := limit(c)
	rows, err := srv.A.ListSessions(ctx(c), merchant(c).ID, live(c), c.Query("client_reference_id"), n+1)
	if err != nil {
		fail(c, err)
		return
	}
	out := []any{}
	for i := range rows {
		if i < n {
			out = append(out, domain.SessionResource(&rows[i], srv.A.PayURL))
		}
	}
	c.JSON(200, list(out, len(rows) > n))
}

func (srv *Server) v1ExpireSession(c *gin.Context) {
	cs, err := srv.A.ExpireSession(ctx(c), merchant(c).ID, live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, domain.SessionResource(cs, srv.A.PayURL))
}

func (srv *Server) v1ListPayments(c *gin.Context) {
	n := limit(c)
	rows, err := srv.A.APIPayments(ctx(c), merchant(c).ID, live(c), c.Query("status"), c.Query("starting_after"), n+1)
	if err != nil {
		fail(c, err)
		return
	}
	out := []any{}
	for i := range rows {
		if i < n {
			out = append(out, domain.PaymentResource(&rows[i]))
		}
	}
	c.JSON(200, list(out, len(rows) > n))
}

func (srv *Server) v1GetPayment(c *gin.Context) {
	p, err := srv.A.MerchantPayment(ctx(c), merchant(c).ID, live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, domain.PaymentResource(p))
}

func (srv *Server) v1CreateRefund(c *gin.Context) {
	var in struct {
		Payment string `json:"payment"`
		Amount  int64  `json:"amount"`
		Reason  string `json:"reason"`
	}
	if err := bind(c, &in); err != nil {
		fail(c, err)
		return
	}
	if in.Payment == "" {
		fail(c, domain.Invalid("parameter_missing", "payment", "Falta o parâmetro payment."))
		return
	}
	r, err := srv.A.Refund(ctx(c), merchant(c).ID, live(c), in.Payment, in.Amount, in.Reason, apiKey(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, domain.RefundResource(r))
}

func (srv *Server) v1ListLinks(c *gin.Context) {
	m := merchant(c)
	vs, err := srv.A.ListLinks(ctx(c), m.ID, live(c), limit(c))
	if err != nil {
		fail(c, err)
		return
	}
	out := []any{}
	for i := range vs {
		out = append(out, domain.LinkResource(&vs[i].PaymentLink, srv.A.PayURL, m.Slug))
	}
	c.JSON(200, list(out, false))
}

func (srv *Server) v1CreateLink(c *gin.Context) {
	var in linkInput
	if err := bind(c, &in); err != nil {
		fail(c, err)
		return
	}
	m := merchant(c)
	l, err := srv.A.CreateLink(ctx(c), m, live(c), in.params(), apiKey(c).ID, "api")
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, domain.LinkResource(l, srv.A.PayURL, m.Slug))
}

func (srv *Server) v1UpdateLink(c *gin.Context) {
	var in linkInput
	if err := bind(c, &in); err != nil {
		fail(c, err)
		return
	}
	m := merchant(c)
	l, err := srv.A.UpdateLink(ctx(c), m, live(c), c.Param("id"), in.params())
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, domain.LinkResource(l, srv.A.PayURL, m.Slug))
}

func (srv *Server) v1Balance(c *gin.Context) {
	m, lv := merchant(c), live(c)
	c.JSON(200, gin.H{"object": "balance", "available": srv.A.Balance(ctx(c), m.ID, lv), "in_transit": srv.A.InTransit(ctx(c), m.ID, lv), "currency": "MZN", "livemode": lv})
}

func (srv *Server) v1ListPayouts(c *gin.Context) {
	ps, err := srv.A.ListPayouts(ctx(c), payoutQuery(merchant(c).ID, live(c), limit(c)))
	if err != nil {
		fail(c, err)
		return
	}
	out := []any{}
	for i := range ps {
		out = append(out, domain.PayoutResource(&ps[i]))
	}
	c.JSON(200, list(out, false))
}

func (srv *Server) v1CreatePayout(c *gin.Context) {
	var in struct {
		Amount int64 `json:"amount"`
	}
	if err := bind(c, &in); err != nil {
		fail(c, err)
		return
	}
	p, err := srv.A.RequestPayout(ctx(c), merchant(c).ID, live(c), in.Amount, apiKey(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, domain.PayoutResource(p))
}

func (srv *Server) v1ListEndpoints(c *gin.Context) {
	eps, err := srv.A.ListEndpoints(ctx(c), merchant(c).ID, live(c))
	if err != nil {
		fail(c, err)
		return
	}
	out := []any{}
	for i := range eps {
		out = append(out, domain.EndpointResource(&eps[i]))
	}
	c.JSON(200, list(out, false))
}

func (srv *Server) v1CreateEndpoint(c *gin.Context) {
	var in struct {
		URL           string   `json:"url"`
		EnabledEvents []string `json:"enabled_events"`
	}
	if err := bind(c, &in); err != nil {
		fail(c, err)
		return
	}
	m := merchant(c)
	e, err := srv.A.CreateEndpoint(ctx(c), m.ID, live(c), in.URL, in.EnabledEvents)
	if err != nil {
		fail(c, err)
		return
	}
	o := domain.EndpointResource(e)
	o["secret"] = m.SigningSecret(live(c))
	c.JSON(200, o)
}

func (srv *Server) v1DeleteEndpoint(c *gin.Context) {
	lv := live(c)
	ok, err := srv.A.DeleteEndpoint(ctx(c), merchant(c).ID, &lv, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	if !ok {
		fail(c, domain.NotFound("O endpoint"))
		return
	}
	c.JSON(200, gin.H{"id": c.Param("id"), "object": "webhook_endpoint", "deleted": true})
}

func (srv *Server) v1GetEvent(c *gin.Context) {
	ev, err := srv.A.Event(ctx(c), merchant(c).ID, live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"id": ev.ID, "object": "event", "type": ev.Type, "created": ev.CreatedAt.Unix(), "livemode": ev.Livemode, "data": gin.H{"object": json.RawMessage(ev.Data)}})
}
