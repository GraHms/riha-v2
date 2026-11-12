package httpapi

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func (srv *Server) adminRoutes(a *gin.RouterGroup) {
	// The overview is every staff member's; each other area needs its permission.
	a.GET("/overview", srv.adminOverview)
	a.GET("/badges", srv.adminBadges)
	a.GET("/settings", srv.adminSettings)

	ap := a.Group("", can(domain.PermApprovals))
	ap.GET("/approvals", srv.adminApprovals)
	ap.GET("/documents/:id", srv.adminDocument)

	mr := a.Group("", can(domain.PermMerchants, domain.PermApprovals, domain.PermPricing))
	mr.GET("/merchants", srv.adminMerchants)
	mr.GET("/merchants/:id", srv.adminMerchant)
	mr.GET("/merchants/:id/routing", srv.adminMerchantRouting)
	a.PATCH("/merchants/:id", srv.adminUpdateMerchant)
	a.POST("/merchants/:id/decision", srv.adminDecision)

	po := a.Group("", can(domain.PermPayouts))
	po.GET("/payouts", srv.adminPayouts)
	po.POST("/payouts/action", srv.adminPayoutAction)
	po.GET("/payouts/export.csv", srv.adminPayoutExport)
	po.GET("/payouts/instructions.pdf", srv.adminPayoutInstructions)
	po.POST("/payouts/complete", srv.adminPayoutComplete)
	po.GET("/payouts/:id/statement.pdf", srv.adminPayoutStatement)
	po.GET("/payouts/:id/receipt", srv.adminPayoutReceipt)
	po.POST("/payouts/:id/email", srv.adminPayoutEmail)
	po.POST("/funding/:id/paid", srv.adminPayFunding)
	po.POST("/funding/confirm", srv.adminConfirmFunding)
	po.POST("/funding/:id/reject", srv.adminRejectClaim)
	po.GET("/funding/:id/receipt", srv.adminFundingReceipt)

	pr := a.Group("", can(domain.PermPricing))
	pr.PUT("/settings", srv.adminPutSettings)
	pr.PATCH("/integrations/:code", srv.adminEditIntegration)
	pr.PUT("/merchants/:id/routes/:method", srv.adminSetRoute)
	pr.PUT("/merchants/:id/deals/:code", srv.adminSetDeal)

	tx := a.Group("", can(domain.PermTransactions))
	tx.GET("/payments", srv.adminPayments)
	tx.POST("/payments/:id/check", srv.adminPaymentCheck)
	tx.GET("/trail/search", srv.adminTrailSearch)
	tx.GET("/trail/thread/:id", srv.adminTrailThread)
	tx.GET("/trail/events", srv.adminTrailEvents)

	a.GET("/metrics", can(domain.PermMetrics), srv.adminMetrics)

	st := a.Group("", can(domain.PermSettlement))
	st.GET("/settlement", srv.adminSettlement)
	st.GET("/settlement/export.csv", srv.adminSettlementExport)
	st.POST("/settlements", srv.adminRecordSettlement)
	st.DELETE("/settlements/:id", srv.adminDeleteSettlement)
	a.GET("/integrations", can(domain.PermSettlement, domain.PermPricing, domain.PermMerchants, domain.PermTeam), srv.adminIntegrations)

	a.GET("/partners", can(domain.PermTeam, domain.PermPricing), srv.adminPartners)
	tm := a.Group("", can(domain.PermTeam))
	tm.GET("/team", srv.adminTeam)
	tm.POST("/team", srv.adminInvite)

	tm.POST("/partners", srv.adminCreatePartner)
	tm.PATCH("/partners/:code", srv.adminRenamePartner)
	tm.POST("/partners/:code/users", srv.adminInvitePartnerUser)
	tm.PATCH("/users/:id", srv.adminSetAccess)
	tm.POST("/users/:id/invite", srv.adminResendInvite)
}

// can lets through staff with any of these permissions.
func can(perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := user(c)
		for _, p := range perms {
			if u.Can(p) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(403, gin.H{"error": gin.H{"code": "forbidden", "message": "Não tem permissão para esta área. Peça a quem gere a equipa."}})
	}
}

func refuse(c *gin.Context) {
	c.AbortWithStatusJSON(403, gin.H{"error": gin.H{"code": "forbidden", "message": "Não tem permissão para esta área. Peça a quem gere a equipa."}})
}

func payoutQuery(merchantID string, live bool, limit int) ports.PayoutQuery {
	return ports.PayoutQuery{MerchantID: merchantID, Live: &live, OrderBy: "requested_at DESC", Limit: limit}
}

func statsJSON(s app.PayoutStats) gin.H {
	return gin.H{"pending_count": s.PendingCount, "pending_total": s.PendingTotal, "sent_count": s.SentCount, "sent_total": s.SentTotal,
		"urgent": s.Urgent, "paid_count": s.PaidCount, "all_count": s.AllCount}
}

func applicationsJSON(as []app.Application, full bool) []gin.H {
	out := []gin.H{}
	for i := range as {
		out = append(out, applicationJSON(&as[i], full))
	}
	return out
}

func (srv *Server) adminOverview(c *gin.Context) {
	o := srv.A.PlatformOverview(ctx(c))
	health := gin.H{}
	for _, m := range domain.AllMethods {
		h, ok := o.Health[m]
		if !ok || h.Total == 0 {
			health[m] = gin.H{"total": 0, "success_rate": nil, "status": "operational"}
			continue
		}
		rate := float64(h.Succeeded) / float64(h.Total)
		st := "operational"
		if h.Total >= 5 && float64(h.OperatorOff)/float64(h.Total) > 0.2 {
			st = "down"
		} else if h.Total >= 5 && rate < 0.9 {
			st = "degraded"
		}
		health[m] = gin.H{"total": h.Total, "success_rate": rate, "status": st}
	}
	c.JSON(200, gin.H{
		"alerts":       len(srv.A.ActiveAlerts(ctx(c))),
		"volume_today": o.VolumeToday, "volume_week_ago": o.VolumeWeekAgo, "tx_today": o.TxToday, "tx_ok_today": o.TxOkToday,
		"stack": o.Stack, "volume_14d": o.Volume14d, "health": health,
		"approvals": applicationsJSON(o.Approvals, false), "pending_approvals": o.PendingApprovals, "unconfirmed_payments": o.UnconfirmedPayments,
		"payouts_due": payoutRowsJSON(o.PayoutsDue), "payout_stats": statsJSON(o.PayoutStats),
	})
}

// adminBadges are the menu's counts, each one a small indexed read, shown
// only for what this person can open.
func (srv *Server) adminBadges(c *gin.Context) {
	u := user(c)
	out := gin.H{}
	if u.Can(domain.PermApprovals) {
		out["pending_approvals"] = srv.A.PendingApprovals(ctx(c))
	}
	if u.Can(domain.PermPayouts) {
		out["payout_stats"] = gin.H{"pending_count": srv.A.PayoutStats(ctx(c)).PendingCount}
	}
	if u.Can(domain.PermTransactions) {
		live := true
		n, _ := srv.A.Payments.Count(ctx(c), ports.PaymentQuery{Live: &live, Statuses: []string{domain.PaymentUnconfirmed}})
		out["unconfirmed_payments"] = n
	}
	if u.Can(domain.PermMetrics) {
		out["alerts"] = len(srv.A.ActiveAlerts(ctx(c)))
	}
	c.JSON(200, out)
}

func (srv *Server) adminApprovals(c *gin.Context) {
	statuses := []string{domain.MerchantInReview, domain.MerchantInfoRequested}
	if c.Query("tab") == "leads" {
		statuses = []string{domain.MerchantLead}
	}
	page, _ := strconv.Atoi(c.Query("page"))
	q := app.ApprovalQuery{Statuses: statuses, Query: c.Query("q"), Sort: c.Query("sort"), Page: max(page, 1), PerPage: 50}
	list, total := srv.A.ApprovalQueue(ctx(c), q)
	leads, _ := srv.A.Merchants.Count(ctx(c), []string{domain.MerchantLead})
	c.JSON(200, gin.H{"data": applicationsJSON(list, true), "pending": srv.A.PendingApprovals(ctx(c)), "leads": leads,
		"total": total, "page": q.Page, "per_page": q.PerPage})
}

func (srv *Server) adminMerchants(c *gin.Context) {
	rows, total, active := srv.A.MerchantDirectory(ctx(c), c.Query("status"), c.Query("q"))
	out := []gin.H{}
	for i := range rows {
		out = append(out, merchantRowJSON(&rows[i]))
	}
	c.JSON(200, gin.H{"data": out, "total": total, "active": active})
}

func (srv *Server) adminMerchant(c *gin.Context) {
	d, err := srv.A.MerchantDossier(ctx(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	o := applicationJSON(&d.Application, true)
	for k, v := range merchantRowJSON(&d.Row) {
		o[k] = v
	}
	o["in_transit"] = d.InTransit
	o["default_fee_bps"] = d.DefaultFeeBps
	o["statement_descriptor"] = d.Merchant.StatementDescriptor
	o["recent_payments"] = paymentRows(d.RecentPayments)
	o["payouts"] = payoutRowsJSON(d.Payouts)
	audit := []gin.H{}
	for _, x := range d.Audit {
		audit = append(audit, gin.H{"action": x.Action, "actor_role": x.ActorRole, "details": x.Details, "created_at": x.CreatedAt})
	}
	o["audit"] = audit
	c.JSON(200, o)
}

func (srv *Server) adminUpdateMerchant(c *gin.Context) {
	var in struct {
		FeeBps   *int  `json:"fee_bps"`
		ResetFee bool  `json:"reset_fee"`
		Beta     *bool `json:"beta"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if !in.ResetFee && in.FeeBps == nil && in.Beta == nil {
		bad(c, "Nada para alterar.")
		return
	}
	if (in.Beta != nil && !user(c).Can(domain.PermMerchants)) || ((in.ResetFee || in.FeeBps != nil) && !user(c).Can(domain.PermPricing)) {
		refuse(c)
		return
	}
	if in.Beta != nil {
		if err := srv.A.SetMerchantBeta(ctx(c), c.Param("id"), *in.Beta, actor(c)); err != nil {
			fail(c, err)
			return
		}
	}
	if in.ResetFee || in.FeeBps != nil {
		if in.ResetFee {
			in.FeeBps = nil
		}
		if err := srv.A.SetMerchantFee(ctx(c), c.Param("id"), in.FeeBps, actor(c)); err != nil {
			fail(c, err)
			return
		}
	}
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
	need := domain.PermApprovals
	if in.Decision == "suspend" || in.Decision == "reactivate" {
		need = domain.PermMerchants
	}
	if !user(c).Can(need) {
		refuse(c)
		return
	}
	m, err := srv.A.Decide(ctx(c), c.Param("id"), in.Decision, strings.TrimSpace(in.Note), user(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	srv.A.Record(ctx(c), m.ID, actor(c), "merchant."+in.Decision, m.ID, in.Note)
	a := srv.A.Application(ctx(c), m)
	c.JSON(200, applicationJSON(&a, true))
}

func (srv *Server) adminDocument(c *gin.Context) {
	d, err := srv.A.Document(ctx(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	if d.StoredPath == "" {
		label := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(d.FileName)
		c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		c.Data(200, "image/svg+xml", []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="640" height="420"><rect width="100%%" height="100%%" fill="#F0EEE8"/><text x="50%%" y="50%%" text-anchor="middle" font-family="monospace" font-size="20" fill="#5E6862">%s · documento de demonstração</text></svg>`, label)))
		return
	}
	data, err := srv.A.DocumentContent(ctx(c), d)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, strings.ReplaceAll(d.FileName, `"`, "")))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, d.ContentType, data)
}

func (srv *Server) adminPayouts(c *gin.Context) {
	rows := srv.A.PayoutQueue(ctx(c), c.Query("tab"))
	out := payoutRowsJSON(rows)
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	// Who pays each withdrawal: staff only, never the merchant.
	parts := srv.A.PayoutFunding(ctx(c), ids)
	names := srv.A.SourceNames(ctx(c))
	for i := range rows {
		fs := []gin.H{}
		for j := range parts[rows[i].ID] {
			f := &parts[rows[i].ID][j]
			o := fundingJSON(f)
			o["source_name"] = names[f.Source]
			fs = append(fs, o)
		}
		out[i]["funding"] = fs
	}
	c.JSON(200, gin.H{"data": out, "stats": statsJSON(srv.A.PayoutStats(ctx(c)))})
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
		p, err := srv.A.TransitionPayout(ctx(c), id, in.Action, in.Reason, user(c).ID)
		if err != nil {
			failed = append(failed, gin.H{"id": id, "error": err.Error()})
			continue
		}
		done = append(done, p.Ref())
		srv.A.Record(ctx(c), p.MerchantID, actor(c), "payout."+in.Action, p.ID, domain.Format(p.Amount)+" MT")
	}
	c.JSON(200, gin.H{"done": done, "failed": failed, "stats": statsJSON(srv.A.PayoutStats(ctx(c)))})
}

func (srv *Server) adminPayoutExport(c *gin.Context) {
	var ids []string
	if q := c.Query("ids"); q != "" {
		ids = strings.Split(q, ",")
	}
	ps := srv.A.PayoutsForExport(ctx(c), ids)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="riha-transferencias-%s.csv"`, time.Now().Format("20060102-1504")))
	w := csv.NewWriter(c.Writer)
	_ = writeSafe(w, []string{"referencia", "banco", "nib", "titular", "valor_mzn", "descritivo", "prazo", "quem_paga"})
	ids = make([]string, len(ps))
	for i := range ps {
		ids[i] = ps[i].ID
	}
	parts := srv.A.PayoutFunding(ctx(c), ids)
	names := srv.A.SourceNames(ctx(c))
	for _, p := range ps {
		// The bank is sent what the merchant receives: the amount less the
		// transfer fee, split between whoever holds the merchant's money.
		fs := parts[p.ID]
		if len(fs) == 0 {
			_ = writeSafe(w, []string{p.Ref(), p.BankName, p.NIB, csvSafe(p.Holder), centsCSV(p.Net()), "RIHA " + p.Ref(), isoT(p.DueAt), "RIHA"})
			continue
		}
		for _, f := range fs {
			if f.Status == domain.FundingCancelled {
				continue
			}
			_ = writeSafe(w, []string{p.Ref(), p.BankName, p.NIB, csvSafe(p.Holder), centsCSV(f.Send), "RIHA " + p.Ref(), isoT(p.DueAt), csvSafe(names[f.Source])})
		}
	}
	w.Flush()
}

func (srv *Server) adminSettings(c *gin.Context) {
	t := srv.A.TransferPricing(ctx(c))
	c.JSON(200, gin.H{"default_fee_bps": srv.A.DefaultFeeBps(ctx(c)), "transfer_fee": t.Fee, "free_transfer_from": t.FreeFrom})
}

func (srv *Server) adminPutSettings(c *gin.Context) {
	var in struct {
		DefaultFeeBps    *int   `json:"default_fee_bps"`
		TransferFee      *int64 `json:"transfer_fee"`
		FreeTransferFrom *int64 `json:"free_transfer_from"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if in.TransferFee != nil || in.FreeTransferFrom != nil {
		t := srv.A.TransferPricing(ctx(c))
		old := t
		if in.TransferFee != nil {
			t.Fee = *in.TransferFee
		}
		if in.FreeTransferFrom != nil {
			t.FreeFrom = *in.FreeTransferFrom
		}
		if err := srv.A.SetTransferPricing(ctx(c), t); err != nil {
			fail(c, err)
			return
		}
		srv.A.Record(ctx(c), "", actor(c), "platform.transfer_fee", "", fmt.Sprintf("%s MT abaixo de %s MT → %s MT abaixo de %s MT",
			domain.FormatShort(old.Fee), domain.FormatShort(old.FreeFrom), domain.FormatShort(t.Fee), domain.FormatShort(t.FreeFrom)))
	}
	if in.DefaultFeeBps == nil {
		srv.adminSettings(c)
		return
	}
	old := srv.A.DefaultFeeBps(ctx(c))
	if err := srv.A.SetDefaultFeeBps(ctx(c), *in.DefaultFeeBps); err != nil {
		fail(c, err)
		return
	}
	srv.A.Record(ctx(c), "", actor(c), "platform.default_fee", "", domain.FormatBps(old)+" → "+domain.FormatBps(*in.DefaultFeeBps))
	srv.adminSettings(c)
}

func (srv *Server) adminPayments(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	r := srv.A.PlatformPayments(ctx(c), app.PlatformPaymentsQuery{Query: strings.TrimSpace(c.Query("q")), View: c.Query("view"), Method: c.Query("method"),
		MerchantID: c.Query("merchant"), Test: c.Query("mode") == "test", Page: page})
	out := []any{}
	for i := range r.Data {
		o := paymentRow(&r.Data[i])
		o["merchant_id"], o["merchant_name"] = r.Data[i].MerchantID, r.Merchants[r.Data[i].MerchantID]
		o["rail_reference"], o["duplicate_of"] = r.Data[i].RailReference, r.Data[i].DuplicateOf
		o["integration"] = r.Data[i].Integration
		out = append(out, o)
	}
	c.JSON(200, gin.H{"data": out, "total": r.Total, "page": max(page, 1), "per_page": r.PerPage,
		"counts": gin.H{"unconfirmed": r.Unconfirmed, "duplicates": r.Duplicates, "paid_late": r.PaidLate}})
}

// adminPaymentCheck asks the operator about a payment now; with apply, it
// also acts on an answer RIHA doesn't have yet.
func (srv *Server) adminPaymentCheck(c *gin.Context) {
	var in struct {
		Apply bool `json:"apply"`
	}
	_ = c.ShouldBindJSON(&in)
	r, err := srv.A.CheckWithOperator(ctx(c), c.Param("id"), user(c).ID, in.Apply)
	if err != nil {
		fail(c, err)
		return
	}
	p := r.Payment
	c.JSON(200, gin.H{"answer": r.Answer, "failure_code": r.FailureCode, "failure_message": domain.FailureMessages[r.FailureCode], "operator_reference": r.OperatorReference,
		"error": r.Error, "fix": r.Fix, "applied": r.Applied,
		"payment": gin.H{"id": p.ID, "status": p.Status, "failure_code": p.FailureCode, "paid_late": p.PaidLate, "duplicate_of": p.DuplicateOf}})
}

func sendPDF(c *gin.Context, name string, data []byte, download bool) {
	disp := "inline"
	if download {
		disp = "attachment"
	}
	c.Writer.Header().Del("Content-Security-Policy")
	c.Header("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disp, name))
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(200, "application/pdf", data)
}

func sendAttachment(c *gin.Context, a *domain.Attachment) {
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(a.Name)))
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'")
	c.Data(200, a.ContentType, a.Data)
}

func idsParam(c *gin.Context) []string {
	var out []string
	for _, x := range strings.Split(c.Query("ids"), ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

func (srv *Server) adminPayoutInstructions(c *gin.Context) {
	pdf, err := srv.A.PayoutInstructionsPDF(ctx(c), idsParam(c))
	if err != nil {
		fail(c, err)
		return
	}
	sendPDF(c, "riha-ordens-"+time.Now().Format("20060102-1504")+".pdf", pdf, c.Query("download") == "1")
}

func (srv *Server) adminPayoutComplete(c *gin.Context) {
	fh, err := c.FormFile("receipt")
	if err != nil {
		bad(c, "Carregue o comprovativo bancário.")
		return
	}
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
	ids := []string{}
	for _, x := range strings.Split(c.PostForm("ids"), ",") {
		if x = strings.TrimSpace(x); x != "" {
			ids = append(ids, x)
		}
	}
	done, err := srv.A.CompletePayouts(ctx(c), ids, app.Upload{FileName: fh.Filename, Size: fh.Size, Head: head[:n], Body: f}, user(c).ID)
	refs := []string{}
	for _, p := range done {
		refs = append(refs, p.Ref())
		srv.A.Record(ctx(c), p.MerchantID, actor(c), "payout.paid", p.ID, domain.Format(p.Amount)+" MT · "+p.ReceiptName)
	}
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"done": refs, "stats": statsJSON(srv.A.PayoutStats(ctx(c)))})
}

func (srv *Server) adminPayoutStatement(c *gin.Context) {
	pdf, p, err := srv.A.PayoutStatementPDF(ctx(c), c.Param("id"), "")
	if err != nil {
		fail(c, err)
		return
	}
	sendPDF(c, "riha-"+strings.ToLower(p.Ref())+".pdf", pdf, c.Query("download") == "1")
}

func (srv *Server) adminPayoutReceipt(c *gin.Context) {
	a, err := srv.A.PayoutReceipt(ctx(c), c.Param("id"), "")
	if err != nil {
		fail(c, err)
		return
	}
	sendAttachment(c, a)
}

func (srv *Server) adminPayoutEmail(c *gin.Context) {
	to, err := srv.A.EmailPayoutStatement(ctx(c), c.Param("id"), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true, "to": to})
}
