package httpapi

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func (srv *Server) subscriptionRoutes(m *gin.RouterGroup) {
	g := m.Group("/subscriptions")
	g.GET("/overview", srv.subsOverview)
	g.GET("/settings", srv.subsSettings)
	g.PUT("/settings", srv.putSubsSettings)
	g.GET("/plans", srv.listPlans)
	g.POST("/plans", srv.createPlan)
	g.GET("/plans/:id", srv.getPlan)
	g.PATCH("/plans/:id", srv.updatePlan)
	g.POST("/plans/:id/price", srv.changePrice)
	g.POST("/plans/:id/archive", srv.archivePlan)
	g.GET("/catalog", srv.getCatalog)
	g.PUT("/catalog", srv.putCatalog)
	g.GET("/catalog/slug", srv.checkCatalogSlug)
	g.POST("/catalog/cover", srv.putCatalogCover)
	g.DELETE("/catalog/cover", srv.deleteCatalogCover)
	g.GET("/members", srv.listMembers)
	g.GET("/members/export.csv", srv.exportMembers)
	g.POST("/members", srv.inviteMember)
	g.POST("/members/import", srv.importMembers)
	g.POST("/members/resend-invites", srv.resendInvites)
	g.GET("/members/:id", srv.getMember)
	g.PATCH("/members/:id", srv.updateMember)
	g.POST("/members/:id/resend", srv.resendMember)
	g.POST("/members/:id/paid-outside", srv.paidOutside)
	g.GET("/charges/:id/proof", srv.outsideProof)
	g.POST("/subs/:id/:action", srv.subAction)
}

func (srv *Server) publicSubscriptionRoutes(g *gin.RouterGroup) {
	g.GET("/subscriptions/plans/:merchant/:slug", srv.publicPlan)
	g.GET("/subscriptions/catalog/:merchant", srv.publicCatalog)
	g.GET("/subscriptions/cover/:slug", srv.throttle(srv.viewLimiter, "cover"), srv.publicCover)
	g.GET("/subscriptions/agreement/:merchant", srv.publicAgreement)
	g.POST("/subscriptions/subscribe/:merchant", srv.throttle(srv.payLimiter, "subscribe"), srv.publicSubscribe)
	g.GET("/invoices/:token", srv.throttle(srv.viewLimiter, "invoice-view"), srv.publicInvoice)
	g.POST("/invoices/:token/pay", srv.throttle(srv.payLimiter, "invoice"), srv.payInvoice)
	g.GET("/members/:token", srv.throttle(srv.viewLimiter, "member-view"), srv.publicMember)
	g.POST("/members/:token/cancel", srv.throttle(srv.payLimiter, "member-cancel"), srv.cancelByMember)
}

func short(t *time.Time) string {
	if t == nil {
		return ""
	}
	return domain.ShortDate(*t)
}

func (srv *Server) planJSON(p *domain.Plan, links catalogLinks, st *app.PlanStats) gin.H {
	out := gin.H{"id": p.ID, "name": p.Name, "slug": p.Slug, "status": p.Status, "amount": p.Amount, "interval": p.Interval, "anchor": p.Anchor,
		"billing_day": p.BillingDay, "months": p.Months, "missed": p.Missed, "fields": nonNil(p.Fields), "ends_at": iso(p.EndsAt), "version": p.Version,
		"highlights": nonNil(p.Highlights), "cycles": p.Cycles, "duration_label": p.DurationLabel(), "customer_when": p.CustomerWhen(srv.A.Now()),
		"in_catalog": p.InCatalog, "catalog_order": p.CatalogOrder, "url": links.plan(p.Slug),
		"schedule_label": p.ScheduleLabel(), "when_label": p.WhenLabel(), "months_label": domain.MonthsLabel(p.Months),
		"interval_label": domain.IntervalLabel(p.Interval), "monthly": domain.MonthlyEquivalent(p.Amount, p.Interval)}
	if st != nil {
		out["stats"] = gin.H{"members": st.Members, "by_status": st.ByStatus, "monthly": st.Monthly}
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (srv *Server) subsOverview(c *gin.Context) {
	m := merchant(c)
	o := srv.A.SubscriptionOverview(ctx(c), m, live(c))
	stats := srv.A.PlanStatsFor(ctx(c), m.ID, live(c))
	links := srv.linksFor(ctx(c), m)
	plans := []gin.H{}
	for _, p := range srv.A.PlanList(ctx(c), m.ID, live(c)) {
		st := stats[p.ID]
		plans = append(plans, srv.planJSON(&p, links, &st))
	}
	c.JSON(200, gin.H{"overview": o, "plans": plans, "catalog_url": links.URL})
}

func settingsJSON(s domain.SubscriptionSettings) gin.H {
	steps := []gin.H{}
	for _, st := range domain.ReminderSteps(s.ToleranceDays) {
		steps = append(steps, gin.H{"key": st.Key, "days": st.Days, "hour": st.Hour, "title": st.Title, "hint": st.Hint, "locked": st.Locked, "on": s.StepOn(st.Key)})
	}
	return gin.H{"steps": steps, "channels": s.Channels, "sender_id": s.SenderID, "tolerance_days": s.ToleranceDays, "final_action": s.FinalAction,
		"require_proof": s.RequireProof, "receipt_sms": s.ReceiptSMS, "catalog_multi": s.CatalogMulti}
}

func (srv *Server) subsSettings(c *gin.Context) {
	c.JSON(200, settingsJSON(srv.A.SubSettingsFor(ctx(c), merchant(c))))
}

func (srv *Server) putSubsSettings(c *gin.Context) {
	var in struct {
		Disabled      []string `json:"disabled_steps"`
		Channels      []string `json:"channels"`
		SenderID      string   `json:"sender_id"`
		ToleranceDays int      `json:"tolerance_days"`
		FinalAction   string   `json:"final_action"`
		RequireProof  bool     `json:"require_proof"`
		ReceiptSMS    bool     `json:"receipt_sms"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	cur := srv.A.SubSettingsFor(ctx(c), m)
	cur.DisabledSteps, cur.Channels, cur.SenderID, cur.ToleranceDays, cur.FinalAction = in.Disabled, in.Channels, in.SenderID, in.ToleranceDays, in.FinalAction
	cur.RequireProof, cur.ReceiptSMS = in.RequireProof, in.ReceiptSMS
	s, err := srv.A.SaveSubSettings(ctx(c), m, cur, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, settingsJSON(s))
}

func (srv *Server) listPlans(c *gin.Context) {
	m := merchant(c)
	stats := srv.A.PlanStatsFor(ctx(c), m.ID, live(c))
	links := srv.linksFor(ctx(c), m)
	out := []gin.H{}
	for _, p := range srv.A.PlanList(ctx(c), m.ID, live(c)) {
		st := stats[p.ID]
		out = append(out, srv.planJSON(&p, links, &st))
	}
	c.JSON(200, gin.H{"data": out})
}

func (srv *Server) createPlan(c *gin.Context) {
	var in app.PlanInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	p, err := srv.A.CreatePlan(ctx(c), m, live(c), in, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.planJSON(p, srv.linksFor(ctx(c), m), &app.PlanStats{}))
}

func (srv *Server) getPlan(c *gin.Context) {
	m := merchant(c)
	p, err := srv.A.Plan(ctx(c), m.ID, live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	st := srv.A.PlanStatsFor(ctx(c), m.ID, live(c))[p.ID]
	out := srv.planJSON(p, srv.linksFor(ctx(c), m), &st)
	versions := []gin.H{}
	subs := srv.A.PlanMembers(ctx(c), m.ID, live(c), p.ID)
	byVersion := map[int]int{}
	for _, s := range subs {
		byVersion[s.PlanVersion]++
	}
	for _, v := range srv.A.PlanVersions(ctx(c), p.ID) {
		versions = append(versions, gin.H{"version": v.Version, "amount": v.Amount, "scope": v.Scope, "since": domain.ShortDate(v.CreatedAt), "members": byVersion[v.Version], "current": v.Version == p.Version})
	}
	out["versions"] = versions
	periods := []gin.H{}
	for _, d := range srv.A.FirstPeriods(p, 4) {
		periods = append(periods, gin.H{"key": domain.PeriodKey(d), "label": domain.PeriodLabel(p, d), "due": domain.ShortDate(d)})
	}
	out["first_periods"] = periods
	pending := 0
	for _, s := range subs {
		if s.PendingFrom != nil {
			pending++
		}
	}
	out["pending_changes"] = pending
	c.JSON(200, out)
}

func (srv *Server) updatePlan(c *gin.Context) {
	var in app.PlanInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	p, err := srv.A.UpdatePlan(ctx(c), m, live(c), c.Param("id"), in, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.planJSON(p, srv.linksFor(ctx(c), m), nil))
}

func (srv *Server) changePrice(c *gin.Context) {
	var in struct {
		Amount        int64    `json:"amount"`
		Scope         string   `json:"scope"`
		Subscriptions []string `json:"subscriptions"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	r, err := srv.A.ChangePrice(ctx(c), m, live(c), c.Param("id"), in.Amount, in.Scope, in.Subscriptions, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	effects := ""
	if r.EffectsAt != nil {
		effects = domain.LongDate(*r.EffectsAt)
	}
	c.JSON(200, gin.H{"plan": srv.planJSON(r.Plan, srv.linksFor(ctx(c), m), nil), "affected": r.Affected, "effective": effects})
}

func (srv *Server) archivePlan(c *gin.Context) {
	if err := srv.A.ArchivePlan(ctx(c), merchant(c), live(c), c.Param("id"), actor(c)); err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (srv *Server) putCatalog(c *gin.Context) {
	var in struct {
		Entries []app.CatalogEntry `json:"entries"`
		Multi   bool               `json:"multi"`
		Page    *app.CatalogPage   `json:"page"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || len(in.Entries) > 100 {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	if err := srv.A.SaveCatalog(ctx(c), m, live(c), in.Entries, in.Multi, in.Page, actor(c)); err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.catalogPageJSON(m, srv.A.SubSettingsFor(ctx(c), m)))
}

func memberRow(mem *domain.Member) gin.H {
	return gin.H{"id": mem.ID, "name": mem.Name, "payer_name": mem.PayerName, "phone": domain.FormatPhone(mem.Phone), "email": mem.Email,
		"channel": mem.Channel, "fields": mem.Fields, "status": mem.Status, "debt": mem.Debt, "debt_count": mem.DebtCount, "plans": mem.PlanNames,
		"next_due": short(mem.NextDueAt), "next_due_iso": iso(mem.NextDueAt), "last_paid": short(mem.LastPaidAt), "last_paid_via": mem.LastPaidVia,
		"created_at": mem.CreatedAt.UTC().Format(time.RFC3339)}
}

func (srv *Server) memberQuery(c *gin.Context) ports.MemberQuery {
	q := ports.MemberQuery{MerchantID: merchant(c).ID, Live: live(c), PlanID: c.Query("plan"), Search: domain.Trim(c.Query("q"), 80),
		FieldKey: domain.Trim(c.Query("field"), 40), FieldValue: domain.Trim(c.Query("value"), 120), Limit: 50}
	switch c.Query("status") {
	case "past_due":
		q.Statuses = []string{domain.SubPastDue, domain.SubSuspended}
	case "paused":
		q.Statuses = []string{domain.SubPaused}
	case "invited":
		q.Statuses = []string{domain.SubInvited}
	case "canceled":
		q.Statuses = []string{domain.SubCanceled, domain.SubEnded}
	default:
		q.Statuses = []string{domain.SubActive, domain.SubPastDue, domain.SubPaused, domain.SubSuspended, domain.SubInvited}
	}
	if n, err := strconv.Atoi(c.Query("limit")); err == nil && n > 0 && n <= 200 {
		q.Limit = n
	}
	q.Offset, _ = strconv.Atoi(c.Query("offset"))
	if q.Offset < 0 {
		q.Offset = 0
	}
	return q
}

func (srv *Server) listMembers(c *gin.Context) {
	m := merchant(c)
	q := srv.memberQuery(c)
	ms, total := srv.A.MemberList(ctx(c), q)
	rows := []gin.H{}
	for i := range ms {
		rows = append(rows, memberRow(&ms[i]))
	}
	counts := srv.A.MemberCounts(ctx(c), m.ID, live(c))
	all := counts[domain.SubActive] + counts[domain.SubPastDue] + counts[domain.SubPaused] + counts[domain.SubSuspended] + counts[domain.SubInvited]
	fields := []gin.H{}
	seen := map[string]bool{}
	for _, p := range srv.A.PlanList(ctx(c), m.ID, live(c)) {
		for i, f := range p.Fields {
			if i == 0 || seen[f] {
				continue
			}
			seen[f] = true
			fields = append(fields, gin.H{"key": f, "values": nonNil(srv.A.MemberFieldValues(ctx(c), m.ID, live(c), f))})
		}
	}
	c.JSON(200, gin.H{"data": rows, "total": total, "counts": gin.H{"all": all, "past_due": counts[domain.SubPastDue] + counts[domain.SubSuspended],
		"paused": counts[domain.SubPaused], "invited": counts[domain.SubInvited], "canceled": counts[domain.SubCanceled]}, "fields": fields})
}

func (srv *Server) exportMembers(c *gin.Context) {
	q := srv.memberQuery(c)
	q.Limit = 5000
	ms, _ := srv.A.MemberList(ctx(c), q)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="riha-membros.csv"`)
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"Membro", "Quem paga", "Número", "Planos", "Estado", "Próxima", "Em dívida (MT)", "Último pagamento", "Campos"})
	for _, mem := range ms {
		var fs []string
		for k, v := range mem.Fields {
			fs = append(fs, k+": "+v)
		}
		_ = w.Write([]string{csvSafe(mem.Name), csvSafe(mem.PayerName), domain.FormatPhone(mem.Phone), csvSafe(mem.PlanNames), mem.Status, short(mem.NextDueAt),
			domain.Format(mem.Debt), short(mem.LastPaidAt), csvSafe(strings.Join(fs, "; "))})
	}
	w.Flush()
}

func (srv *Server) inviteMember(c *gin.Context) {
	var in app.MemberInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	mem, err := srv.A.InviteMember(ctx(c), merchant(c), live(c), in, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, memberRow(mem))
}

func readUpload(c *gin.Context, field string, max int64) (string, []byte, bool) {
	fh, err := c.FormFile(field)
	if err != nil {
		bad(c, "Escolha um ficheiro.")
		return "", nil, false
	}
	if fh.Size > max {
		bad(c, "O ficheiro é demasiado grande.")
		return "", nil, false
	}
	f, err := fh.Open()
	if err != nil {
		fail(c, err)
		return "", nil, false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil || int64(len(data)) > max {
		bad(c, "O ficheiro é demasiado grande.")
		return "", nil, false
	}
	return fh.Filename, data, true
}

func (srv *Server) importMembers(c *gin.Context) {
	name, data, ok := readUpload(c, "file", 4<<20)
	if !ok {
		return
	}
	m := merchant(c)
	if c.PostForm("dry_run") == "true" {
		pv, err := srv.A.PreviewImport(ctx(c), m, live(c), name, data, c.PostForm("plan_id"))
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(200, pv)
		return
	}
	res, err := srv.A.ImportMembers(ctx(c), m, live(c), name, data, c.PostForm("plan_id"), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, res)
}

func (srv *Server) resendInvites(c *gin.Context) {
	n, err := srv.A.ResendInvites(ctx(c), merchant(c), live(c), 72*time.Hour, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"sent": n})
}

func chargeJSON(ch *domain.Charge, plan *domain.Plan) gin.H {
	return gin.H{"id": ch.ID, "subscription_id": ch.SubscriptionID, "plan_id": ch.PlanID, "plan_name": plan.Name, "label": ch.Label, "period": ch.Period,
		"amount": ch.Amount, "status": ch.Status, "due": domain.ShortDate(ch.DueAt), "due_iso": ch.DueAt.UTC().Format(time.RFC3339),
		"year": ch.DueAt.In(domain.Maputo).Year(), "month": int(ch.DueAt.In(domain.Maputo).Month()), "paid_at": short(ch.PaidAt), "paid_at_iso": iso(ch.PaidAt),
		"method": ch.Method, "outside_method": ch.OutsideMethod, "outside_label": domain.OutsideMethods[ch.OutsideMethod], "outside_ref": ch.OutsideRef,
		"marked_by": ch.MarkedBy, "has_proof": ch.OutsideProofPath != "", "last_failure": ch.LastFailure, "resends": ch.Resends,
		"link_sent_at": iso(ch.LinkSentAt), "opened_at": iso(ch.OpenedAt), "first": ch.First, "steps": nonNil(ch.Steps)}
}

func (srv *Server) getMember(c *gin.Context) {
	m := merchant(c)
	d, err := srv.A.MemberDetailFor(ctx(c), m.ID, live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	subs := []gin.H{}
	links := srv.linksFor(ctx(c), m)
	for _, s := range d.Subs {
		p := d.Plans[s.PlanID]
		row := gin.H{"id": s.ID, "plan": srv.planJSON(p, links, nil), "last_due": lastDue(p, s.AnchorAt), "plan_version": s.PlanVersion, "amount": s.Amount, "status": s.Status,
			"next_due": short(s.NextDueAt), "next_due_iso": iso(s.NextDueAt), "paused_until": short(s.PausedUntil), "since": domain.ShortDate(s.AnchorAt),
			"since_label": domain.PeriodLabel(p, s.AnchorAt), "consent_text": s.ConsentText, "consent_at": iso(s.ConsentAt), "skip_periods": nonNil(s.SkipPeriods)}
		if s.PendingFrom != nil {
			row["pending"] = gin.H{"amount": s.PendingAmount, "version": s.PendingVersion, "from": domain.LongDate(*s.PendingFrom)}
		}
		subs = append(subs, row)
	}
	charges := []gin.H{}
	for i := range d.Charges {
		charges = append(charges, chargeJSON(&d.Charges[i], d.Plans[d.Charges[i].PlanID]))
	}
	msgs := []gin.H{}
	for _, msg := range d.Messages {
		msgs = append(msgs, gin.H{"id": msg.ID, "kind": msg.Kind, "channel": msg.Channel, "to": domain.FormatPhone(msg.To), "body": msg.Body, "status": msg.Status,
			"error": msg.Error, "charge_id": msg.ChargeID, "created_at": msg.CreatedAt.UTC().Format(time.RFC3339)})
	}
	set := srv.A.SubSettingsFor(ctx(c), m)
	c.JSON(200, gin.H{"member": memberRow(d.Member), "subscriptions": subs, "charges": charges, "messages": msgs,
		"tolerance_days": set.ToleranceDays, "final_action": set.FinalAction, "require_proof": set.RequireProof, "receipt_sms": set.ReceiptSMS,
		"test_mode": !live(c)})
}

func (srv *Server) updateMember(c *gin.Context) {
	var in app.MemberInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	mem, err := srv.A.UpdateMember(ctx(c), merchant(c), live(c), c.Param("id"), in, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, memberRow(mem))
}

func (srv *Server) resendMember(c *gin.Context) {
	if err := srv.A.ResendLink(ctx(c), merchant(c), live(c), c.Param("id"), actor(c)); err != nil {
		fail(c, err)
		return
	}
	srv.A.SendSoon(ctx(c))
	c.JSON(200, gin.H{"ok": true})
}

func (srv *Server) paidOutside(c *gin.Context) {
	in := app.OutsideInput{Method: c.PostForm("method"), Ref: c.PostForm("reference")}
	for _, id := range strings.Split(c.PostForm("charge_ids"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			in.ChargeIDs = append(in.ChargeIDs, id)
		}
	}
	if d := c.PostForm("date"); d != "" {
		t, err := time.ParseInLocation("2006-01-02", d, domain.Maputo)
		if err != nil {
			bad(c, "Data inválida.")
			return
		}
		in.Date = t
	}
	if fh, err := c.FormFile("proof"); err == nil {
		f, err := fh.Open()
		if err != nil {
			fail(c, err)
			return
		}
		defer f.Close()
		head := make([]byte, 8)
		n, _ := io.ReadFull(f, head)
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			fail(c, err)
			return
		}
		in.Proof = &app.Upload{FileName: fh.Filename, Size: fh.Size, Head: head[:n], Body: f}
	}
	u := user(c)
	out, err := srv.A.MarkPaidOutside(ctx(c), merchant(c), live(c), c.Param("id"), in, actor(c), domain.FirstNonEmpty(u.Name, u.Email))
	if err != nil {
		fail(c, err)
		return
	}
	srv.A.SendSoon(ctx(c))
	c.JSON(200, gin.H{"marked": len(out)})
}

func (srv *Server) outsideProof(c *gin.Context) {
	att, err := srv.A.OutsideProof(ctx(c), merchant(c).ID, live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	sendAttachment(c, att)
}

func (srv *Server) subAction(c *gin.Context) {
	var in struct {
		Until  string `json:"until"`
		PlanID string `json:"plan_id"`
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&in)
	m, id, a := merchant(c), c.Param("id"), actor(c)
	var err error
	out := gin.H{"ok": true}
	switch c.Param("action") {
	case "skip":
		var label string
		label, err = srv.A.SkipNext(ctx(c), m, live(c), id, a)
		out["skipped"] = label
	case "pause":
		var t time.Time
		t, err = time.ParseInLocation("2006-01-02", in.Until, domain.Maputo)
		if err != nil {
			bad(c, "Escolha a data.")
			return
		}
		err = srv.A.PauseSubscription(ctx(c), m, live(c), id, t, a)
	case "resume":
		err = srv.A.ResumeSubscription(ctx(c), m, live(c), id, a)
	case "cancel":
		err = srv.A.CancelSubscription(ctx(c), m, live(c), id, domain.Trim(in.Reason, 200), a)
	case "change-plan":
		err = srv.A.ChangeSubscriptionPlan(ctx(c), m, live(c), id, in.PlanID, a)
	case "send-now":
		err = srv.A.SendNextNow(ctx(c), m, live(c), id, a)
	default:
		fail(c, domain.NotFound("A ação"))
		return
	}
	if err != nil {
		fail(c, err)
		return
	}
	srv.A.SendSoon(ctx(c))
	c.JSON(200, out)
}

func merchantBrand(m *domain.Merchant) gin.H {
	return gin.H{"name": m.Name, "slug": m.Slug, "initials": domain.Initials(m.Name), "color": m.AvatarColor}
}

func publicPlanJSON(p *domain.Plan) gin.H {
	return gin.H{"id": p.ID, "name": p.Name, "slug": p.Slug, "amount": p.Amount, "interval": p.Interval, "interval_label": domain.IntervalLabel(p.Interval),
		"months": p.Months, "months_label": domain.MonthsLabel(p.Months), "when_label": p.WhenLabel(), "anchor": p.Anchor, "billing_day": p.BillingDay,
		"fields": nonNil(p.Fields), "ends_at": iso(p.EndsAt), "missed": p.Missed}
}

func (srv *Server) firstDueText(p *domain.Plan) string {
	ds := srv.A.FirstPeriods(p, 1)
	if len(ds) == 0 {
		return ""
	}
	return strings.ToLower(domain.PeriodLabel(p, ds[0]))
}

func (srv *Server) publicPlan(c *gin.Context) {
	m, p, err := srv.A.PublicPlan(ctx(c), c.Param("merchant"), c.Param("slug"))
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"merchant": merchantBrand(m), "plan": srv.customerPlanJSON(p), "page": srv.catalogPageJSON(m, srv.A.SubSettingsFor(ctx(c), m)), "test_mode": !p.Livemode})
}

func (srv *Server) publicCatalog(c *gin.Context) {
	m, plans, set, err := srv.A.PublicCatalog(ctx(c), c.Param("merchant"))
	if err != nil {
		fail(c, err)
		return
	}
	out := []gin.H{}
	for i := range plans {
		out = append(out, srv.customerPlanJSON(&plans[i]))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"merchant": merchantBrand(m), "plans": out, "multi": set.CatalogMulti, "page": srv.catalogPageJSON(m, set), "test_mode": !plans[0].Livemode})
}

func (srv *Server) publicAgreement(c *gin.Context) {
	mer, err := srv.A.PublicMerchant(ctx(c), c.Param("merchant"))
	if err != nil {
		fail(c, err)
		return
	}
	var plans []*domain.Plan
	for _, id := range strings.Split(c.Query("plans"), ",") {
		if p := srv.A.PublicPlanByID(ctx(c), mer.ID, strings.TrimSpace(id)); p != nil {
			plans = append(plans, p)
		}
		if len(plans) >= 5 {
			break
		}
	}
	phone := c.Query("phone")
	if !domain.ValidMZMobile(phone) {
		phone = ""
	}
	c.JSON(200, gin.H{"texts": nonNil(srv.A.AgreementPreview(ctx(c), mer, plans, domain.FirstNonEmpty(phone, "8•••••••"), c.Query("channel")))})
}

func (srv *Server) publicSubscribe(c *gin.Context) {
	var in app.SubscribeInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	in.IP = c.ClientIP()
	tok, err := srv.A.Subscribe(ctx(c), c.Param("merchant"), in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"token": tok})
}

var methodNames = domain.MethodNames

func (srv *Server) publicInvoice(c *gin.Context) {
	v, err := srv.A.Invoice(ctx(c), c.Param("token"), true)
	if err != nil {
		fail(c, err)
		return
	}
	lines := []gin.H{}
	for _, l := range v.Lines {
		lines = append(lines, gin.H{"id": l.Charge.ID, "label": l.Charge.Label, "plan_name": l.Plan.Name, "amount": l.Charge.Amount, "status": l.Charge.Status,
			"due": domain.ShortDate(l.Charge.DueAt), "subscription_id": l.Subscription.ID, "current": l.Charge.ID == v.Charge.ID, "past_due": l.Charge.Status == domain.ChargePastDue})
	}
	paid := []gin.H{}
	for _, l := range v.Paid {
		paid = append(paid, gin.H{"label": l.Charge.Label, "plan_name": l.Plan.Name, "amount": l.Charge.Amount, "paid_at": short(l.Charge.PaidAt),
			"method": methodNames[l.Charge.Method], "outside": l.Charge.Status == domain.ChargePaidOutside})
	}
	fields := gin.H{}
	for k, val := range v.Member.Fields {
		fields[k] = val
	}
	out := gin.H{"merchant": merchantBrand(v.Merchant), "member": gin.H{"name": v.Member.Name, "fields": fields},
		"status": v.Charge.Status, "label": v.Charge.Label, "lines": lines, "total": v.Total, "needs_consent": v.NeedsConsent, "agreements": nonNil(v.Agreements),
		"coming_soon": srv.A.ComingSoon,
		"last_method": domain.FirstNonEmpty(v.Member.LastMethod, domain.WalletFor(v.Member.Phone)), "has_last": v.Member.LastMethod != "",
		"phone_masked": domain.MaskPhone(domain.FirstNonEmpty(v.Member.LastPhone, v.Member.Phone)), "channel": v.Member.Channel,
		"paid": paid, "next_due": "", "next_amount": v.NextAmount, "skipped": nonNil(v.Skipped), "test_mode": !v.Member.Livemode}
	if v.NextDue != nil {
		out["next_due"] = domain.LongDate(*v.NextDue)
	}
	if v.Payment != nil {
		out["payment"] = gin.H{"status": v.Payment.Status, "reference": v.Payment.Reference, "method": methodNames[v.Payment.Method],
			"failure_message": v.Payment.FailureMessage, "amount": v.Payment.Amount, "paid_at": short(v.Payment.SucceededAt)}
	}
	plans := []gin.H{}
	seen := map[string]bool{}
	for _, s := range v.Subs {
		if s.Status == domain.SubCanceled || seen[s.PlanID] {
			continue
		}
		seen[s.PlanID] = true
		p := v.Plans[s.PlanID]
		if p == nil {
			continue
		}
		plans = append(plans, gin.H{"name": p.Name, "amount": s.Amount, "schedule": p.ScheduleLabel(), "next_due": short(s.NextDueAt), "status": s.Status, "months": p.Months})
	}
	out["subscriptions"] = plans
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.JSON(200, out)
}

func (srv *Server) payInvoice(c *gin.Context) {
	var in app.InvoicePay
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	in.IP, in.UA = c.ClientIP(), c.GetHeader("User-Agent")
	if _, err := srv.A.PayInvoice(ctx(c), c.Param("token"), in); err != nil {
		fail(c, err)
		return
	}
	srv.publicInvoice(c)
}

// lastDue is the label of a member's last payment, on a plan where payments end.
func lastDue(p *domain.Plan, anchor time.Time) string {
	if !p.Limited() {
		return ""
	}
	n := domain.MaxCycles
	if p.Cycles > 0 {
		n = p.Cycles
	}
	ds := domain.Schedule(p, anchor, n)
	return domain.PeriodLabel(p, ds[len(ds)-1])
}

// publicMember is the member's own page: subscriptions, payments and what failed.
func (srv *Server) publicMember(c *gin.Context) {
	v, err := srv.A.MemberPortal(ctx(c), c.Param("token"))
	if err != nil {
		fail(c, err)
		return
	}
	subs := []gin.H{}
	for _, ps := range v.Subs {
		x := gin.H{"id": ps.Sub.ID, "plan_name": "", "schedule": "", "amount": ps.Sub.Amount, "status": ps.Sub.Status, "next_due": "", "debt": ps.Debt,
			"can_cancel": !domain.Closed(ps.Sub.Status), "canceled_at": short(ps.Sub.CanceledAt)}
		if ps.Plan != nil {
			x["plan_name"], x["schedule"] = ps.Plan.Name, ps.Plan.ScheduleLabel()
		}
		if ps.Sub.NextDueAt != nil && !domain.Closed(ps.Sub.Status) {
			x["next_due"] = domain.LongDate(*ps.Sub.NextDueAt)
		}
		if ps.Sub.PendingFrom != nil && !domain.Closed(ps.Sub.Status) {
			x["new_amount"], x["new_from"] = ps.Sub.PendingAmount, domain.LongDate(*ps.Sub.PendingFrom)
		}
		subs = append(subs, x)
	}
	pays := []gin.H{}
	for _, ch := range v.Charges {
		x := gin.H{"id": ch.ID, "label": ch.Label, "plan_name": "", "amount": ch.Amount, "status": ch.Status, "due": domain.ShortDate(ch.DueAt),
			"paid_at": short(ch.PaidAt), "method": methodNames[ch.Method], "outside": domain.OutsideMethods[ch.OutsideMethod], "failure": ""}
		if p := v.Plans[ch.PlanID]; p != nil {
			x["plan_name"] = p.Name
		}
		if ch.Unpaid() {
			x["failure"] = ch.LastFailure
		}
		pays = append(pays, x)
	}
	pay := ""
	if v.PayToken != "" {
		pay = "/i/" + v.PayToken
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.JSON(200, gin.H{"merchant": merchantBrand(v.Merchant), "member": gin.H{"name": v.Member.Name, "phone_masked": domain.MaskPhone(v.Member.Phone), "channel": v.Member.Channel},
		"subscriptions": subs, "payments": pays, "due": v.Due, "pay_url": pay, "test_mode": !v.Member.Livemode})
}

func (srv *Server) cancelByMember(c *gin.Context) {
	var in struct {
		SubscriptionID string `json:"subscription_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.SubscriptionID == "" {
		bad(c, "Pedido inválido.")
		return
	}
	if err := srv.A.CancelByMember(ctx(c), c.Param("token"), in.SubscriptionID, c.ClientIP()); err != nil {
		fail(c, err)
		return
	}
	srv.publicMember(c)
}
