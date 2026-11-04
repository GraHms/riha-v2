package httpapi

import (
	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func (srv *Server) v1SubscriptionRoutes(v1 *gin.RouterGroup) {
	v1.GET("/plans", srv.v1ListPlans)
	v1.POST("/plans", srv.v1CreatePlan)
	v1.GET("/plans/:id", srv.v1GetPlan)
	v1.GET("/subscriptions", srv.v1ListSubscriptions)
	v1.POST("/subscriptions", srv.v1CreateSubscription)
	v1.GET("/subscriptions/:id", srv.v1GetSubscription)
	v1.POST("/subscriptions/:id/cancel", srv.v1CancelSubscription)
	v1.GET("/subscription_charges", srv.v1ListCharges)
	v1.GET("/subscription_charges/:id", srv.v1GetCharge)
}

func v1Actor(c *gin.Context) app.Actor {
	return app.Actor{ID: "api:" + apiKey(c).ID, Role: "api", IP: c.ClientIP()}
}

func (srv *Server) v1ListPlans(c *gin.Context) {
	m, lv := merchant(c), apiKey(c).Livemode
	out := []domain.M{}
	for _, p := range srv.A.PlanList(ctx(c), m.ID, lv) {
		out = append(out, domain.PlanResource(&p, srv.A.PublicURL, m.Slug))
	}
	c.JSON(200, list(out, false))
}

func (srv *Server) v1CreatePlan(c *gin.Context) {
	var in app.PlanInput
	if err := bind(c, &in); err != nil {
		fail(c, err)
		return
	}
	if in.Interval == "" {
		in.Interval = domain.IntervalMonth
	}
	if in.Anchor == "" {
		in.Anchor = domain.AnchorFixed
	}
	if in.Months == 0 {
		in.Months = domain.AllMonths
	}
	if in.Missed == "" {
		in.Missed = domain.MissedDebt
	}
	m := merchant(c)
	p, err := srv.A.CreatePlan(ctx(c), m, apiKey(c).Livemode, in, v1Actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, domain.PlanResource(p, srv.A.PublicURL, m.Slug))
}

func (srv *Server) v1GetPlan(c *gin.Context) {
	m := merchant(c)
	p, err := srv.A.Plan(ctx(c), m.ID, apiKey(c).Livemode, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, domain.PlanResource(p, srv.A.PublicURL, m.Slug))
}

func (srv *Server) v1ListSubscriptions(c *gin.Context) {
	m, lv := merchant(c), apiKey(c).Livemode
	var st []string
	if s := c.Query("status"); s != "" {
		st = []string{s}
	}
	n := limit(c)
	subs := srv.A.SubscriptionList(ctx(c), m.ID, lv, c.Query("member"), st, n+1)
	out := []domain.M{}
	for i := range subs {
		if i == n {
			break
		}
		out = append(out, domain.SubscriptionResource(&subs[i], srv.A.MemberByID(ctx(c), subs[i].MemberID)))
	}
	c.JSON(200, list(out, len(subs) > n))
}

func (srv *Server) v1CreateSubscription(c *gin.Context) {
	var in struct {
		Plan     string `json:"plan"`
		Customer struct {
			Phone     string `json:"phone"`
			Name      string `json:"name"`
			PayerName string `json:"payer_name"`
			Email     string `json:"email"`
			Channel   string `json:"channel"`
		} `json:"customer"`
		Fields      map[string]string `json:"fields"`
		FirstPeriod string            `json:"first_period"`
	}
	if err := bind(c, &in); err != nil {
		fail(c, err)
		return
	}
	m, lv := merchant(c), apiKey(c).Livemode
	mem, err := srv.A.InviteMember(ctx(c), m, lv, app.MemberInput{PlanIDs: []string{in.Plan}, Phone: in.Customer.Phone, Name: in.Customer.Name,
		PayerName: in.Customer.PayerName, Email: in.Customer.Email, Channel: in.Customer.Channel, Fields: in.Fields, FirstPeriod: in.FirstPeriod}, v1Actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	for _, s := range srv.A.SubscriptionList(ctx(c), m.ID, lv, mem.ID, nil, 20) {
		if s.PlanID == in.Plan {
			c.JSON(200, domain.SubscriptionResource(&s, mem))
			return
		}
	}
	fail(c, domain.NotFound("A assinatura"))
}

func (srv *Server) v1GetSubscription(c *gin.Context) {
	s, mem, err := srv.A.SubscriptionByID(ctx(c), merchant(c).ID, apiKey(c).Livemode, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, domain.SubscriptionResource(s, mem))
}

func (srv *Server) v1CancelSubscription(c *gin.Context) {
	m, lv := merchant(c), apiKey(c).Livemode
	if _, _, err := srv.A.SubscriptionByID(ctx(c), m.ID, lv, c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	if err := srv.A.CancelSubscription(ctx(c), m, lv, c.Param("id"), "api", v1Actor(c)); err != nil {
		fail(c, err)
		return
	}
	srv.v1GetSubscription(c)
}

func (srv *Server) v1ListCharges(c *gin.Context) {
	m, lv := merchant(c), apiKey(c).Livemode
	q := ports.ChargeQuery{MerchantID: m.ID, Live: &lv, OrderBy: "due_at DESC", Limit: limit(c) + 1}
	if s := c.Query("subscription"); s != "" {
		q.SubscriptionIDs = []string{s}
	}
	if s := c.Query("status"); s != "" {
		q.Statuses = []string{s}
	}
	cs := srv.A.ChargesFor(ctx(c), q)
	out := []domain.M{}
	for i := range cs {
		if i == q.Limit-1 {
			break
		}
		out = append(out, domain.ChargeResource(&cs[i], srv.A.PublicURL))
	}
	c.JSON(200, list(out, len(cs) >= q.Limit))
}

func (srv *Server) v1GetCharge(c *gin.Context) {
	lv := apiKey(c).Livemode
	cs := srv.A.ChargesFor(ctx(c), ports.ChargeQuery{MerchantID: merchant(c).ID, Live: &lv, IDs: []string{c.Param("id")}, Limit: 1})
	if len(cs) == 0 {
		fail(c, domain.NotFound("A cobrança "+c.Param("id")))
		return
	}
	c.JSON(200, domain.ChargeResource(&cs[0], srv.A.PublicURL))
}
