package api

import (
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/models"
	"riha/internal/money"
	"riha/internal/service"
)

var successStatuses = []string{models.PaymentSucceeded, models.PaymentRefunded, models.PaymentPartiallyRefunded}

func (srv *Server) adminOverview(c *gin.Context) {
	db := srv.S.DB
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	weekAgo := today.AddDate(0, 0, -7)
	var volToday, volWeekAgo, txToday, okToday int64
	db.Model(&models.Payment{}).Where("livemode AND status IN ? AND created_at >= ?", successStatuses, today).Select("COALESCE(SUM(amount),0)").Scan(&volToday)
	db.Model(&models.Payment{}).Where("livemode AND status IN ? AND created_at >= ? AND created_at < ?", successStatuses, weekAgo, weekAgo.Add(time.Since(today))).Select("COALESCE(SUM(amount),0)").Scan(&volWeekAgo)
	db.Model(&models.Payment{}).Where("livemode AND created_at >= ?", today).Count(&txToday)
	db.Model(&models.Payment{}).Where("livemode AND status IN ? AND created_at >= ?", successStatuses, today).Count(&okToday)

	type srow struct {
		Day    time.Time
		Method string
		Sum    int64
	}
	var rows []srow
	start := today.AddDate(0, 0, -13)
	db.Raw(`SELECT date_trunc('day', created_at) AS day, method, SUM(amount) AS sum FROM payments WHERE livemode AND status IN ? AND created_at >= ? GROUP BY 1,2`, successStatuses, start).Scan(&rows)
	byDay := map[string]gin.H{}
	var total14 int64
	for _, r := range rows {
		k := r.Day.UTC().Format("2006-01-02")
		if byDay[k] == nil {
			byDay[k] = gin.H{"mpesa": int64(0), "emola": int64(0), "card": int64(0)}
		}
		byDay[k][r.Method] = r.Sum
		total14 += r.Sum
	}
	stack := []gin.H{}
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		k := d.Format("2006-01-02")
		v := byDay[k]
		if v == nil {
			v = gin.H{"mpesa": int64(0), "emola": int64(0), "card": int64(0)}
		}
		v["date"] = k
		stack = append(stack, v)
	}
	// Operator health from the last 24h of attempts (mock providers).
	type hrow struct {
		Method string
		Total  int64
		Ok     int64
		Op     int64
	}
	var hrows []hrow
	db.Raw(`SELECT method, COUNT(*) AS total, SUM(CASE WHEN status IN ? THEN 1 ELSE 0 END) AS ok,
		SUM(CASE WHEN failure_code = 'operator_unavailable' THEN 1 ELSE 0 END) AS op
		FROM payments WHERE created_at >= ? AND status <> 'processing' GROUP BY method`, successStatuses, now.Add(-24*time.Hour)).Scan(&hrows)
	health := gin.H{}
	for _, m := range []string{"mpesa", "emola", "card"} {
		health[m] = gin.H{"total": 0, "success_rate": nil, "status": "operational"}
	}
	for _, h := range hrows {
		rate := float64(h.Ok) / float64(h.Total)
		st := "operational"
		if h.Total >= 5 && float64(h.Op)/float64(h.Total) > 0.2 {
			st = "down"
		} else if h.Total >= 5 && rate < 0.9 {
			st = "degraded"
		}
		health[h.Method] = gin.H{"total": h.Total, "success_rate": rate, "status": st}
	}
	c.JSON(200, gin.H{
		"volume_today": volToday, "volume_week_ago": volWeekAgo, "tx_today": txToday, "tx_ok_today": okToday,
		"stack": stack, "volume_14d": total14, "health": health,
		"approvals": srv.approvalQueue(4), "pending_approvals": srv.countApprovals(),
		"payouts_due":  srv.payoutRows(srv.S.DB.Where("livemode AND status IN ?", []string{models.PayoutNew, models.PayoutReview}).Order("due_at").Limit(4)),
		"payout_stats": srv.payoutStats(),
	})
}

func (srv *Server) countApprovals() int64 {
	var n int64
	srv.S.DB.Model(&models.Merchant{}).Where("status = ?", models.MerchantInReview).Count(&n)
	return n
}

func (srv *Server) applicationJSON(m *models.Merchant, full bool) gin.H {
	checks := srv.S.KycChecks(m)
	o := gin.H{"id": m.ID, "name": m.Name, "legal_name": m.LegalName, "owner_name": m.OwnerName, "business_type": m.BusinessType,
		"nuit": m.NUIT, "city": m.City, "phone": m.Phone, "email": m.Email, "expected_volume": m.ExpectedVolume,
		"avatar_color": m.AvatarColor, "initials": initials(m.Name), "status": m.Status, "risk": service.Risk(checks),
		"submitted_at": iso(m.KycSubmittedAt), "created_at": m.CreatedAt.UTC().Format(time.RFC3339), "review_note": m.ReviewNote}
	if full {
		o["checks"] = checks
		var docs []models.KycDocument
		srv.S.DB.Where("merchant_id = ?", m.ID).Order("created_at").Find(&docs)
		d := []gin.H{}
		for _, x := range docs {
			d = append(d, gin.H{"id": x.ID, "kind": x.Kind, "file_name": x.FileName, "size": x.Size, "content_type": x.ContentType})
		}
		o["documents"] = d
		var b models.BankAccount
		if srv.S.DB.First(&b, "merchant_id = ?", m.ID).Error == nil {
			o["bank_account"] = bankObj(&b)
		}
	}
	return o
}

func (srv *Server) approvalQueue(limit int) []gin.H {
	var ms []models.Merchant
	srv.S.DB.Where("status IN ?", []string{models.MerchantInReview}).Order("kyc_submitted_at").Limit(limit).Find(&ms)
	out := []gin.H{}
	for i := range ms {
		out = append(out, srv.applicationJSON(&ms[i], false))
	}
	return out
}

func (srv *Server) adminApprovals(c *gin.Context) {
	var ms []models.Merchant
	srv.S.DB.Where("status IN ?", []string{models.MerchantInReview, models.MerchantInfoRequested}).Order("kyc_submitted_at").Limit(200).Find(&ms)
	out := []gin.H{}
	for i := range ms {
		out = append(out, srv.applicationJSON(&ms[i], true))
	}
	c.JSON(200, gin.H{"data": out, "pending": srv.countApprovals()})
}

func (srv *Server) merchantRow(m *models.Merchant) gin.H {
	since := time.Now().UTC().AddDate(0, 0, -30)
	var vol int64
	srv.S.DB.Model(&models.Payment{}).Where("merchant_id = ? AND livemode AND status IN ? AND created_at >= ?", m.ID, successStatuses, since).Select("COALESCE(SUM(amount),0)").Scan(&vol)
	var main struct{ Method string }
	srv.S.DB.Raw(`SELECT method FROM payments WHERE merchant_id = ? AND livemode AND status IN ? GROUP BY method ORDER BY SUM(amount) DESC LIMIT 1`, m.ID, successStatuses).Scan(&main)
	bps := srv.S.FeeBpsFor(m)
	return gin.H{"id": m.ID, "name": m.Name, "city": m.City, "status": m.Status, "initials": initials(m.Name), "avatar_color": m.AvatarColor,
		"volume_30d": vol, "balance": srv.S.Balance(m.ID, true), "main_method": main.Method, "created_at": m.CreatedAt.UTC().Format(time.RFC3339),
		"fee_bps": bps, "fee_negotiated": m.FeeBps != nil, "email": m.Email}
}

func (srv *Server) adminMerchants(c *gin.Context) {
	q := srv.S.DB.Model(&models.Merchant{})
	switch c.Query("status") {
	case "active":
		q = q.Where("status = ?", models.MerchantActive)
	case "review":
		q = q.Where("status IN ?", []string{models.MerchantInReview, models.MerchantInfoRequested, models.MerchantOnboarding})
	case "suspended":
		q = q.Where("status IN ?", []string{models.MerchantSuspended, models.MerchantRejected})
	}
	if s := strings.TrimSpace(c.Query("q")); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("LOWER(name) LIKE ? OR nuit LIKE ? OR LOWER(email) LIKE ?", like, like, like)
	}
	var ms []models.Merchant
	q.Order("created_at DESC").Limit(200).Find(&ms)
	out := []gin.H{}
	for i := range ms {
		out = append(out, srv.merchantRow(&ms[i]))
	}
	var total, active int64
	srv.S.DB.Model(&models.Merchant{}).Count(&total)
	srv.S.DB.Model(&models.Merchant{}).Where("status = ?", models.MerchantActive).Count(&active)
	c.JSON(200, gin.H{"data": out, "total": total, "active": active})
}

func (srv *Server) adminMerchant(c *gin.Context) {
	var m models.Merchant
	if err := srv.S.DB.First(&m, "id = ?", c.Param("id")).Error; err != nil {
		apiErr(c, service.NotFound("O comerciante"))
		return
	}
	o := srv.applicationJSON(&m, true)
	for k, v := range srv.merchantRow(&m) {
		o[k] = v
	}
	o["status"] = m.Status
	o["in_transit"] = srv.S.InTransit(m.ID, true)
	o["default_fee_bps"] = srv.S.DefaultFeeBps()
	o["statement_descriptor"] = m.StatementDescriptor
	var pays []models.Payment
	srv.S.DB.Where("merchant_id = ? AND livemode", m.ID).Order("created_at DESC").Limit(10).Find(&pays)
	pp := []any{}
	for i := range pays {
		pp = append(pp, srv.paymentRow(&pays[i]))
	}
	o["recent_payments"] = pp
	o["payouts"] = srv.payoutRows(srv.S.DB.Where("merchant_id = ? AND livemode", m.ID).Order("requested_at DESC").Limit(10))
	var audit []models.AuditLog
	srv.S.DB.Where("merchant_id = ?", m.ID).Order("created_at DESC").Limit(20).Find(&audit)
	a := []gin.H{}
	for _, x := range audit {
		a = append(a, gin.H{"action": x.Action, "actor_role": x.ActorRole, "details": x.Details, "created_at": x.CreatedAt})
	}
	o["audit"] = a
	c.JSON(200, o)
}

func (srv *Server) adminUpdateMerchant(c *gin.Context) {
	var in struct {
		// FeeBps: set to a number to negotiate, or null with reset_fee=true to
		// return to the platform default.
		FeeBps   *int `json:"fee_bps"`
		ResetFee bool `json:"reset_fee"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	var m models.Merchant
	if err := srv.S.DB.First(&m, "id = ?", c.Param("id")).Error; err != nil {
		apiErr(c, service.NotFound("O comerciante"))
		return
	}
	old := money.FormatBps(srv.S.FeeBpsFor(&m))
	switch {
	case in.ResetFee:
		m.FeeBps = nil
	case in.FeeBps != nil:
		if *in.FeeBps < 0 || *in.FeeBps > 2000 {
			apiErr(c, service.Invalid("fee_invalid", "fee_bps", "A taxa tem de estar entre 0% e 20%."))
			return
		}
		v := *in.FeeBps
		m.FeeBps = &v
	default:
		bad(c, "Nada para alterar.")
		return
	}
	if err := srv.S.DB.Model(&m).Update("fee_bps", m.FeeBps).Error; err != nil {
		apiErr(c, err)
		return
	}
	srv.S.Audit(m.ID, user(c).ID, user(c).Role, "fee.changed", m.ID, old+" → "+money.FormatBps(srv.S.FeeBpsFor(&m)), c.ClientIP())
	srv.adminMerchant(c)
}

func (srv *Server) adminDecision(c *gin.Context) {
	var in struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m, err := srv.S.Decide(c.Param("id"), in.Decision, strings.TrimSpace(in.Note), user(c).ID)
	if err != nil {
		apiErr(c, err)
		return
	}
	srv.S.Audit(m.ID, user(c).ID, user(c).Role, "merchant."+in.Decision, m.ID, in.Note, c.ClientIP())
	c.JSON(200, srv.applicationJSON(m, true))
}

func (srv *Server) adminDocument(c *gin.Context) {
	var d models.KycDocument
	if err := srv.S.DB.First(&d, "id = ?", c.Param("id")).Error; err != nil {
		apiErr(c, service.NotFound("O documento"))
		return
	}
	if d.StoredPath == "" {
		// Seeded demo documents have no file: render a labelled placeholder.
		c.Header("Content-Type", "image/svg+xml")
		c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		label := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(d.FileName)
		c.String(200, `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="420"><rect width="100%%" height="100%%" fill="#F0EEE8"/><text x="50%%" y="50%%" text-anchor="middle" font-family="monospace" font-size="20" fill="#5E6862">%s · documento de demonstração</text></svg>`, label)
		return
	}
	c.Header("Content-Type", d.ContentType)
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, strings.ReplaceAll(d.FileName, `"`, "")))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
	c.File(d.StoredPath)
}

// ---------------------------------------------------------------- payouts

func (srv *Server) payoutRows(q *gormQuery) []gin.H {
	var ps []models.Payout
	q.Find(&ps)
	names := map[string]string{}
	out := []gin.H{}
	for i := range ps {
		p := &ps[i]
		if _, ok := names[p.MerchantID]; !ok {
			var m models.Merchant
			srv.S.DB.Select("name").First(&m, "id = ?", p.MerchantID)
			names[p.MerchantID] = m.Name
		}
		o := service.PayoutObject(p)
		o["merchant_id"] = p.MerchantID
		o["merchant_name"] = names[p.MerchantID]
		o["requested_at_iso"] = p.RequestedAt.UTC().Format(time.RFC3339)
		o["due_at_iso"] = p.DueAt.UTC().Format(time.RFC3339)
		o["nib_last4"] = p.NIB[len(p.NIB)-4:]
		out = append(out, o)
	}
	return out
}

func (srv *Server) payoutStats() gin.H {
	var pend, sent struct {
		Count int64
		Total int64
	}
	srv.S.DB.Model(&models.Payout{}).Where("livemode AND status IN ?", []string{models.PayoutNew, models.PayoutReview}).Select("COUNT(*) AS count, COALESCE(SUM(amount),0) AS total").Scan(&pend)
	srv.S.DB.Model(&models.Payout{}).Where("livemode AND status = ?", models.PayoutSent).Select("COUNT(*) AS count, COALESCE(SUM(amount),0) AS total").Scan(&sent)
	var urgent int64
	srv.S.DB.Model(&models.Payout{}).Where("livemode AND status IN ? AND due_at < ?", []string{models.PayoutNew, models.PayoutReview}, time.Now().UTC().Add(8*time.Hour)).Count(&urgent)
	var paid, all int64
	srv.S.DB.Model(&models.Payout{}).Where("livemode AND status = ?", models.PayoutPaid).Count(&paid)
	srv.S.DB.Model(&models.Payout{}).Where("livemode").Count(&all)
	return gin.H{"pending_count": pend.Count, "pending_total": pend.Total, "sent_count": sent.Count, "sent_total": sent.Total, "urgent": urgent, "paid_count": paid, "all_count": all}
}

func (srv *Server) adminPayouts(c *gin.Context) {
	q := srv.S.DB.Where("livemode")
	switch c.Query("tab") {
	case "", "pending":
		q = q.Where("status IN ?", []string{models.PayoutNew, models.PayoutReview}).Order("due_at")
	case "sent":
		q = q.Where("status = ?", models.PayoutSent).Order("sent_at")
	case "paid":
		q = q.Where("status = ?", models.PayoutPaid).Order("paid_at DESC")
	default:
		q = q.Order("requested_at DESC")
	}
	c.JSON(200, gin.H{"data": srv.payoutRows(q.Limit(300)), "stats": srv.payoutStats()})
}

func (srv *Server) adminPayoutAction(c *gin.Context) {
	var in struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"`
		Reason string   `json:"reason"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || len(in.IDs) == 0 {
		bad(c, "Escolha pelo menos um levantamento.")
		return
	}
	done, failed := []string{}, []gin.H{}
	for _, id := range in.IDs {
		p, err := srv.S.TransitionPayout(id, in.Action, in.Reason, user(c).ID)
		if err != nil {
			failed = append(failed, gin.H{"id": id, "error": err.Error()})
			continue
		}
		done = append(done, service.PayoutRef(p))
		srv.S.Audit(p.MerchantID, user(c).ID, user(c).Role, "payout."+in.Action, p.ID, money.Format(p.Amount)+" MT", c.ClientIP())
	}
	c.JSON(200, gin.H{"done": done, "failed": failed, "stats": srv.payoutStats()})
}

// adminPayoutExport produces a simple bank transfer file (CSV).
func (srv *Server) adminPayoutExport(c *gin.Context) {
	idsParam := strings.Split(c.Query("ids"), ",")
	var ps []models.Payout
	q := srv.S.DB.Where("livemode")
	if c.Query("ids") != "" {
		q = q.Where("id IN ?", idsParam)
	} else {
		q = q.Where("status IN ?", []string{models.PayoutNew, models.PayoutReview, models.PayoutSent})
	}
	q.Order("due_at").Find(&ps)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="riha-transferencias-%s.csv"`, time.Now().Format("20060102-1504")))
	w := csv.NewWriter(c.Writer)
	w.Write([]string{"referencia", "banco", "nib", "titular", "valor_mzn", "descritivo", "prazo"})
	for _, p := range ps {
		w.Write([]string{service.PayoutRef(&p), p.BankName, p.NIB, csvSafe(p.Holder), fmt.Sprintf("%d.%02d", p.Amount/100, p.Amount%100), "RIHA " + service.PayoutRef(&p), p.DueAt.UTC().Format(time.RFC3339)})
	}
	w.Flush()
}

// ---------------------------------------------------------------- settings / payments

func (srv *Server) adminSettings(c *gin.Context) {
	c.JSON(200, gin.H{"default_fee_bps": srv.S.DefaultFeeBps()})
}

func (srv *Server) adminPutSettings(c *gin.Context) {
	var in struct {
		DefaultFeeBps int `json:"default_fee_bps"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	old := srv.S.DefaultFeeBps()
	if err := srv.S.SetDefaultFeeBps(in.DefaultFeeBps); err != nil {
		apiErr(c, err)
		return
	}
	srv.S.Audit("", user(c).ID, user(c).Role, "platform.default_fee", "", money.FormatBps(old)+" → "+money.FormatBps(in.DefaultFeeBps), c.ClientIP())
	srv.adminSettings(c)
}

func (srv *Server) adminPayments(c *gin.Context) {
	var rows []models.Payment
	q := srv.S.DB.Where("livemode").Order("created_at DESC").Limit(100)
	if s := c.Query("q"); s != "" {
		q = q.Where("reference = ? OR id = ?", s, s)
	}
	q.Find(&rows)
	out := []any{}
	for i := range rows {
		o := srv.paymentRow(&rows[i])
		o["merchant_id"] = rows[i].MerchantID
		out = append(out, o)
	}
	c.JSON(200, gin.H{"data": out})
}
