package httpapi

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func (srv *Server) adminRoutes(a *gin.RouterGroup) {
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
	a.GET("/payouts/instructions.pdf", srv.adminPayoutInstructions)
	a.POST("/payouts/complete", srv.adminPayoutComplete)
	a.GET("/payouts/:id/statement.pdf", srv.adminPayoutStatement)
	a.GET("/payouts/:id/receipt", srv.adminPayoutReceipt)
	a.POST("/payouts/:id/email", srv.adminPayoutEmail)
	a.GET("/settings", srv.adminSettings)
	a.PUT("/settings", srv.adminPutSettings)
	a.GET("/payments", srv.adminPayments)
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
		"volume_today": o.VolumeToday, "volume_week_ago": o.VolumeWeekAgo, "tx_today": o.TxToday, "tx_ok_today": o.TxOkToday,
		"stack": o.Stack, "volume_14d": o.Volume14d, "health": health,
		"approvals": applicationsJSON(o.Approvals, false), "pending_approvals": o.PendingApprovals,
		"payouts_due": payoutRowsJSON(o.PayoutsDue), "payout_stats": statsJSON(o.PayoutStats),
	})
}

func (srv *Server) adminApprovals(c *gin.Context) {
	statuses := []string{domain.MerchantInReview, domain.MerchantInfoRequested}
	if c.Query("tab") == "leads" {
		statuses = []string{domain.MerchantLead}
	}
	q := srv.A.ApprovalQueue(ctx(c), statuses, 200)
	leads, _ := srv.A.Merchants.Count(ctx(c), []string{domain.MerchantLead})
	c.JSON(200, gin.H{"data": applicationsJSON(q, true), "pending": srv.A.PendingApprovals(ctx(c)), "leads": leads})
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
		FeeBps   *int `json:"fee_bps"`
		ResetFee bool `json:"reset_fee"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if !in.ResetFee && in.FeeBps == nil {
		bad(c, "Nada para alterar.")
		return
	}
	if in.ResetFee {
		in.FeeBps = nil
	}
	if err := srv.A.SetMerchantFee(ctx(c), c.Param("id"), in.FeeBps, actor(c)); err != nil {
		fail(c, err)
		return
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
	c.JSON(200, gin.H{"data": payoutRowsJSON(srv.A.PayoutQueue(ctx(c), c.Query("tab"))), "stats": statsJSON(srv.A.PayoutStats(ctx(c)))})
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
	_ = w.Write([]string{"referencia", "banco", "nib", "titular", "valor_mzn", "descritivo", "prazo"})
	for _, p := range ps {
		_ = w.Write([]string{p.Ref(), p.BankName, p.NIB, csvSafe(p.Holder), centsCSV(p.Amount), "RIHA " + p.Ref(), isoT(p.DueAt)})
	}
	w.Flush()
}

func (srv *Server) adminSettings(c *gin.Context) {
	c.JSON(200, gin.H{"default_fee_bps": srv.A.DefaultFeeBps(ctx(c))})
}

func (srv *Server) adminPutSettings(c *gin.Context) {
	var in struct {
		DefaultFeeBps int `json:"default_fee_bps"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	old := srv.A.DefaultFeeBps(ctx(c))
	if err := srv.A.SetDefaultFeeBps(ctx(c), in.DefaultFeeBps); err != nil {
		fail(c, err)
		return
	}
	srv.A.Record(ctx(c), "", actor(c), "platform.default_fee", "", domain.FormatBps(old)+" → "+domain.FormatBps(in.DefaultFeeBps))
	srv.adminSettings(c)
}

func (srv *Server) adminPayments(c *gin.Context) {
	vs := srv.A.PlatformPayments(ctx(c), c.Query("q"))
	out := []any{}
	for i := range vs {
		o := paymentRow(&vs[i])
		o["merchant_id"] = vs[i].MerchantID
		out = append(out, o)
	}
	c.JSON(200, gin.H{"data": out})
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
