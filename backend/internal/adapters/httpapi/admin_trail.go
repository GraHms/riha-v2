package httpapi

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func trailJSON(ev domain.TrailEvent) gin.H {
	return gin.H{"id": ev.ID, "kind": ev.Kind, "source": ev.Source, "level": ev.Level, "title": ev.Title, "method": ev.Method, "path": ev.Path,
		"status": ev.Status, "duration_ms": ev.DurationMs, "ip": ev.IP, "user_agent": ev.UserAgent, "network": ev.Network, "request": ev.Request,
		"response": ev.Response, "data": ev.Data, "created_at": isoT(ev.CreatedAt), "request_id": ev.RequestID, "payment_id": ev.PaymentID,
		"session_id": ev.SessionID, "merchant_id": ev.MerchantID, "livemode": ev.Livemode}
}

func (srv *Server) adminTrailSearch(c *gin.Context) {
	hits, err := srv.A.FindThreads(ctx(c), c.Query("q"))
	if err != nil {
		fail(c, err)
		return
	}
	if hits == nil {
		hits = []ports.ThreadHit{}
	}
	c.JSON(200, gin.H{"data": hits})
}

func (srv *Server) adminTrailThread(c *gin.Context) {
	t, err := srv.A.Thread(ctx(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	cs := t.Session
	session := gin.H{"id": cs.ID, "status": cs.Status, "amount": cs.Amount, "description": cs.Description, "source_type": cs.SourceType, "livemode": cs.Livemode,
		"client_reference_id": cs.ClientReferenceID, "created_at": isoT(cs.CreatedAt), "expires_at": isoT(cs.ExpiresAt), "channel": cs.Channel,
		"return_host": hostOf(domain.FirstNonEmpty(cs.SuccessURL, cs.CancelURL)), "callback_host": hostOf(cs.CallbackURL), "origin_domain": cs.Domain}
	merchant := gin.H{}
	if t.Merchant != nil {
		merchant = gin.H{"id": t.Merchant.ID, "name": t.Merchant.Name, "slug": t.Merchant.Slug}
	}
	pays := []gin.H{}
	for _, p := range t.Payments {
		pays = append(pays, gin.H{"id": p.ID, "reference": p.Reference, "method": p.Method, "status": p.Status, "amount": p.Amount, "fee": p.Fee,
			"failure_code": p.FailureCode, "failure_message": p.FailureMessage, "phone": strings.TrimSpace(p.PhonePrefix + " ••• " + p.PhoneLast4),
			"operation": p.RailReference, "rail_id": p.RailID, "operator_reference": p.OperatorReference, "attempt_id": p.AttemptID,
			"created_at": isoT(p.CreatedAt), "succeeded_at": iso(p.SucceededAt), "failed_at": iso(p.FailedAt), "payer_ip": p.PayerIP, "payer_user_agent": p.PayerUserAgent,
			"paid_late": p.PaidLate, "duplicate_of": p.DuplicateOf, "recheck_at": iso(p.RecheckAt), "rechecks": p.Rechecks, "livemode": p.Livemode})
	}
	evs := []gin.H{}
	for _, ev := range t.Events {
		evs = append(evs, trailJSON(ev))
	}
	c.JSON(200, gin.H{"session": session, "merchant": merchant, "payments": pays, "events": evs})
}

func (srv *Server) adminTrailEvents(c *gin.Context) {
	q := ports.TrailQuery{MerchantID: c.Query("merchant"), Level: c.Query("level"), Search: domain.Trim(c.Query("q"), 80), Limit: 200}
	if k := c.Query("kind"); k != "" {
		q.Kinds = strings.Split(k, ",")
	}
	if h, err := strconv.Atoi(c.Query("hours")); err == nil && h > 0 {
		since := time.Now().Add(-time.Duration(h) * time.Hour)
		q.Since = &since
	}
	evs, err := srv.A.TrailEvents(ctx(c), q)
	if err != nil {
		fail(c, err)
		return
	}
	out := []gin.H{}
	for _, ev := range evs {
		j := trailJSON(ev)
		j["request"], j["response"] = "", ""
		out = append(out, j)
	}
	c.JSON(200, gin.H{"data": out})
}

func (srv *Server) adminMetrics(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days <= 0 || days > 365 {
		days = 7
	}
	to := time.Now()
	q := ports.MetricsQuery{From: to.AddDate(0, 0, -days), To: to, Live: c.DefaultQuery("live", "true") == "true", MerchantID: c.Query("merchant")}
	m, err := srv.A.AdminMetrics(ctx(c), q)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"metrics": m, "days": days, "live": q.Live})
}
