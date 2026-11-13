package httpapi

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

// requireMicrocredit closes every Microcrédito route to accounts a RIHA super admin
// has not turned the service on for.
func (srv *Server) requireMicrocredit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if e := domain.RequireMicrocredit(merchant(c)); e != nil {
			fail(c, e)
			c.Abort()
			return
		}
		c.Next()
	}
}

func (srv *Server) microcreditRoutes(g *gin.RouterGroup) {
	g.GET("", func(c *gin.Context) {
		m := merchant(c)
		c.JSON(200, gin.H{"enabled": true, "since": iso(m.MicrocreditAt)})
	})
	g.GET("/overview", srv.mcOverview)
	g.GET("/settings", srv.mcSettings)
	g.PUT("/settings", srv.mcSaveSettings)
	g.GET("/products", srv.mcProducts)
	g.POST("/products", srv.mcCreateProduct)
	g.GET("/products/:id", srv.mcProduct)
	g.PUT("/products/:id", srv.mcUpdateProduct)
	g.GET("/requests", srv.mcRequests)
	g.GET("/requests/export.csv", srv.mcExportRequests)
	g.GET("/requests/:id", srv.mcRequest)
	g.POST("/requests/:id/review", srv.mcReviewRequest)
	g.POST("/requests/:id/decline", srv.mcDeclineRequest)
	g.POST("/requests/:id/approve", srv.mcApproveRequest)
	g.GET("/documents/:id", srv.mcDocument)
	g.GET("/loans", srv.mcLoans)
	g.GET("/loans/export.csv", srv.mcExportLoans)
	g.POST("/loans", srv.mcCreateLoan)
	g.GET("/loans/:id", srv.mcLoan)
	g.GET("/loans/:id/agreement.pdf", srv.mcAgreementPDF)
	g.POST("/loans/:id/:action", srv.mcLoanAction)
	g.POST("/loans/:id/instalments/:iid/paid", srv.mcMarkPaid)
	g.POST("/offers/resend", srv.mcResendOffers)
}

func (srv *Server) publicMicrocreditRoutes(g *gin.RouterGroup) {
	g.GET("/credit/:merchant", srv.throttle(srv.viewLimiter, "credit-view"), srv.pubCreditProducts)
	g.POST("/credit/:merchant/requests", srv.throttle(srv.payLimiter, "credit-request"), srv.pubCreditRequest)
	g.POST("/credit-r/:token/code", srv.throttle(srv.payLimiter, "credit-code"), srv.pubRequestCode)
	g.POST("/credit-r/:token/confirm", srv.throttle(srv.authLimiter, "credit-confirm"), srv.pubConfirmRequest)
	g.POST("/credit-r/:token/documents", srv.throttle(srv.payLimiter, "credit-doc"), srv.pubRequestDocument)
	g.GET("/credit-c/:token", srv.throttle(srv.viewLimiter, "credit-page"), srv.pubCreditPage)
	g.POST("/credit-c/:token/code", srv.throttle(srv.payLimiter, "credit-code"), srv.pubAgreementCode)
	g.POST("/credit-c/:token/accept", srv.throttle(srv.authLimiter, "credit-accept"), srv.pubAcceptAgreement)
	g.POST("/credit-c/:token/pay", srv.throttle(srv.payLimiter, "credit-pay"), srv.pubPayLoan)
	g.POST("/credit-c/:token/ask", srv.throttle(srv.payLimiter, "credit-ask"), srv.pubAskAgreement)
	g.GET("/credit-c/:token/agreement.pdf", srv.throttle(srv.viewLimiter, "credit-pdf"), srv.pubAgreementPDF)
}

func termRange(p *domain.CreditProduct) string {
	if p.TermMin == p.TermMax {
		return domain.TermLabel(p.Freq, p.TermMax)
	}
	return fmt.Sprintf("%d a %s", p.TermMin, domain.TermLabel(p.Freq, p.TermMax))
}

func productJSON(p *domain.CreditProduct) gin.H {
	fields := p.Fields
	if fields == nil {
		fields = []string{}
	}
	return gin.H{"id": p.ID, "name": p.Name, "min": p.Min, "max": p.Max, "freq": p.Freq, "freq_label": domain.FreqLabel(p.Freq), "term_min": p.TermMin,
		"term_max": p.TermMax, "term_label": termRange(p), "rate_bp": p.RateBP, "rate_label": domain.RateLabel(p.RateBP, p.Freq), "day_mode": p.DayMode,
		"client_day": p.ClientPicksDay(), "mora": p.Mora, "mora_bp": p.MoraBP, "grace": p.Grace, "mora_max_weeks": p.MoraMaxWeeks, "effort_pct": p.EffortPct,
		"fields": fields, "status": p.Status}
}

func datesJSON(ts []time.Time) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.UTC().Format(time.RFC3339)
	}
	return out
}

func agreementJSON(g *domain.LoanAgreement) gin.H {
	if g == nil {
		return nil
	}
	return gin.H{"id": g.ID, "version": g.Version, "status": g.Status, "total": g.Total, "dates": datesJSON(g.Dates), "amounts": g.Amounts,
		"mora_waived": g.MoraWaived, "sent_at": iso(&g.SentAt), "accepted_at": iso(g.AcceptedAt), "phone": domain.FormatPhone(g.Phone), "text": g.Text}
}

func (srv *Server) mcOverview(c *gin.Context) {
	o, err := srv.A.CreditOverview(ctx(c), merchant(c), live(c))
	if err != nil {
		fail(c, err)
		return
	}
	up := []gin.H{}
	for _, g := range o.Upcoming {
		up = append(up, gin.H{"date": iso(&g.Date), "product": g.Product, "links": g.Links, "total": g.Total})
	}
	att := []gin.H{}
	for _, x := range o.Attention {
		att = append(att, gin.H{"kind": x.Kind, "title": x.Title, "detail": x.Detail, "tone": x.Tone, "loan_id": x.LoanID, "request_id": x.RequestID, "action": x.Action})
	}
	c.JSON(200, gin.H{"book": o.Book, "book_loans": o.BookLoans, "book_products": o.BookProducts, "pending": o.Pending, "oldest_wait": iso(o.OldestWait),
		"overdue": o.Overdue, "overdue_loans": o.OverdueLoans, "received_riha": o.ReceivedRiha,
		"month": gin.H{"label": o.Month.Label, "expected": o.Month.Expected, "riha": o.Month.Riha, "outside": o.Month.Outside, "late": o.Month.Late,
			"to_come": o.Month.ToCome, "riha_n": o.Month.RihaN, "outside_n": o.Month.OutsideN, "late_n": o.Month.LateN, "to_come_n": o.Month.ToComeN},
		"upcoming": up, "attention": att, "aging": o.Aging, "par30": o.PAR30, "can_disburse": srv.A.CanDisburse(live(c)),
		"balance": srv.A.Balance(ctx(c), merchant(c).ID, live(c)), "request_url": srv.requestURL(merchant(c))})
}

func (srv *Server) requestURL(m *domain.Merchant) string {
	return strings.TrimRight(srv.A.PayURL, "/") + "/credito/" + m.Slug
}

func (srv *Server) mcSettings(c *gin.Context) {
	m := merchant(c)
	n, _ := srv.A.Users.CountByMerchant(ctx(c), m.ID)
	c.JSON(200, gin.H{"settings": srv.A.CreditSettings(ctx(c), m.ID), "can_disburse": srv.A.CanDisburse(live(c)), "people": n})
}

func (srv *Server) mcSaveSettings(c *gin.Context) {
	var in domain.CreditSettings
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	s, err := srv.A.SaveCreditSettings(ctx(c), merchant(c), in, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"settings": s})
}

func (srv *Server) mcProducts(c *gin.Context) {
	m := merchant(c)
	ps, err := srv.A.CreditProducts(ctx(c), m, live(c))
	if err != nil {
		fail(c, err)
		return
	}
	views, _, _ := srv.A.CreditLoans(ctx(c), m, live(c), "ativos", "", "")
	n, book := map[string]int{}, map[string]int64{}
	for _, v := range views {
		n[v.Loan.ProductID]++
		book[v.Loan.ProductID] += v.Debt - v.Mora
	}
	out := []gin.H{}
	for i := range ps {
		j := productJSON(&ps[i])
		j["loans"], j["book"] = n[ps[i].ID], book[ps[i].ID]
		out = append(out, j)
	}
	c.JSON(200, gin.H{"data": out, "request_url": srv.requestURL(m)})
}

func (srv *Server) mcProduct(c *gin.Context) {
	p, err := srv.A.CreditProduct(ctx(c), merchant(c), live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, productJSON(p))
}

func (srv *Server) mcCreateProduct(c *gin.Context) {
	var in app.ProductInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	p, err := srv.A.CreateCreditProduct(ctx(c), merchant(c), live(c), in, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, productJSON(p))
}

func (srv *Server) mcUpdateProduct(c *gin.Context) {
	var in app.ProductInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	p, err := srv.A.UpdateCreditProduct(ctx(c), merchant(c), live(c), c.Param("id"), in, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, productJSON(p))
}

func (srv *Server) productNames(c *gin.Context) map[string]*domain.CreditProduct {
	ps, _ := srv.A.CreditProducts(ctx(c), merchant(c), live(c))
	out := map[string]*domain.CreditProduct{}
	for i := range ps {
		out[ps[i].ID] = &ps[i]
	}
	return out
}

var requestStatusPT = map[string]string{domain.RequestNew: "Novo", domain.RequestReview: "Em análise", domain.RequestOffered: "Acordo enviado",
	domain.RequestDeclined: "Recusado", domain.RequestCanceled: "Cancelado"}

func requestRowJSON(r *domain.CreditRequest, p *domain.CreditProduct) gin.H {
	j := gin.H{"id": r.ID, "name": r.Name, "phone": domain.FormatPhone(r.Phone), "activity": r.Activity, "amount": r.Amount, "term": r.Term,
		"instalment": r.Instalment, "effort": r.EffortPct, "status": r.Status, "status_label": requestStatusPT[r.Status], "created_at": iso(&r.CreatedAt), "income": r.Income}
	if p != nil {
		j["product"], j["product_id"], j["term_label"], j["per"], j["effort_limit"] = p.Name, p.ID, domain.TermLabel(p.Freq, r.Term), domain.PerLabel(p.Freq), p.EffortPct
	}
	return j
}

func (srv *Server) mcRequests(c *gin.Context) {
	rs, counts, err := srv.A.CreditRequests(ctx(c), merchant(c), live(c), c.Query("filter"), c.Query("product"), c.Query("q"))
	if err != nil {
		fail(c, err)
		return
	}
	ps := srv.productNames(c)
	out := []gin.H{}
	for i := range rs {
		out = append(out, requestRowJSON(&rs[i], ps[rs[i].ProductID]))
	}
	c.JSON(200, gin.H{"data": out, "counts": gin.H{"decidir": counts[domain.RequestNew] + counts[domain.RequestReview], "enviado": counts[domain.RequestOffered],
		"todos": counts[domain.RequestNew] + counts[domain.RequestReview] + counts[domain.RequestOffered] + counts[domain.RequestDeclined] + counts[domain.RequestCanceled]}})
}

func (srv *Server) mcExportRequests(c *gin.Context) {
	rs, _, err := srv.A.CreditRequests(ctx(c), merchant(c), live(c), "todos", "", "")
	if err != nil {
		fail(c, err)
		return
	}
	ps := srv.productNames(c)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="riha-pedidos-credito.csv"`)
	w := csv.NewWriter(c.Writer)
	_ = writeSafe(w, []string{"Recebido", "Cliente", "Número", "Atividade", "Produto", "Pede (MT)", "Prazo", "Prestação (MT)", "Rendimento (MT)", "Esforço", "Estado"})
	for _, r := range rs {
		pn, term := "", fmt.Sprint(r.Term)
		if p := ps[r.ProductID]; p != nil {
			pn, term = p.Name, domain.TermLabel(p.Freq, r.Term)
		}
		_ = writeSafe(w, []string{r.CreatedAt.In(domain.Maputo).Format("2006-01-02 15:04"), csvSafe(r.Name), domain.FormatPhone(r.Phone), csvSafe(r.Activity), csvSafe(pn),
			domain.Format(r.Amount), term, domain.Format(r.Instalment), domain.Format(r.Income), fmt.Sprintf("%d%%", r.EffortPct), requestStatusPT[r.Status]})
	}
	w.Flush()
}

func (srv *Server) mcRequest(c *gin.Context) {
	d, err := srv.A.CreditRequestDetail(ctx(c), merchant(c), live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	r := d.Request
	j := requestRowJSON(r, d.Product)
	j["id_number"], j["nuit"], j["address"], j["purpose"], j["wallet"], j["day"] = r.IDNumber, r.NUIT, r.Address, r.Purpose, domain.MethodNames[r.Wallet], r.Day
	j["extra"], j["decline_note"], j["decided_at"], j["phone_ok"] = r.Extra, r.DeclineNote, iso(r.DecidedAt), r.PhoneOKAt != nil
	docs := []gin.H{}
	for _, x := range d.Documents {
		docs = append(docs, gin.H{"id": x.ID, "kind": x.Kind, "label": app.CreditDocLabel(x.Kind), "content_type": x.ContentType, "name": x.FileName})
	}
	hist := []gin.H{}
	for _, l := range d.History {
		hist = append(hist, gin.H{"id": l.ID, "amount": l.Amount, "status": l.Status, "created_at": iso(&l.CreatedAt), "closed_at": iso(l.ClosedAt)})
	}
	j["documents"], j["history"], j["checks"], j["product_full"] = docs, hist, d.Checks, productJSON(d.Product)
	j["quote"] = gin.H{"instalment": d.Quote.Instalment, "total": d.Quote.Total, "interest": d.Quote.Interest, "dates": datesJSON(d.Quote.Dates)}
	if d.Loan != nil {
		j["loan"] = gin.H{"id": d.Loan.ID, "status": d.Loan.Status, "amount": d.Loan.Amount, "term": d.Loan.Term, "day": d.Loan.Day, "delivery": d.Loan.Delivery, "created_at": iso(&d.Loan.CreatedAt)}
	}
	c.JSON(200, j)
}

func (srv *Server) mcReviewRequest(c *gin.Context) {
	if err := srv.A.ReviewCreditRequest(ctx(c), merchant(c), live(c), c.Param("id"), actor(c)); err != nil {
		fail(c, err)
		return
	}
	srv.mcRequest(c)
}

func (srv *Server) mcDeclineRequest(c *gin.Context) {
	var in struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&in)
	if _, err := srv.A.DeclineCreditRequest(ctx(c), merchant(c), live(c), c.Param("id"), in.Note, actor(c)); err != nil {
		fail(c, err)
		return
	}
	srv.mcRequest(c)
}

func (srv *Server) mcApproveRequest(c *gin.Context) {
	var in app.OfferInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if _, err := srv.A.ApproveCreditRequest(ctx(c), merchant(c), live(c), c.Param("id"), in, user(c), actor(c)); err != nil {
		fail(c, err)
		return
	}
	srv.mcRequest(c)
}

func (srv *Server) mcDocument(c *gin.Context) {
	d, b, err := srv.A.CreditDocumentContent(ctx(c), merchant(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(d.FileName)))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, d.ContentType, b)
}

func pips(v *app.LoanView) []string {
	now := time.Now()
	out := []string{}
	for _, x := range v.Current {
		switch {
		case x.Status == domain.InstPaid:
			out = append(out, "paid")
		case x.Status == domain.InstPaidOut:
			out = append(out, "outside")
		case x.Open() && domain.DaysLate(x.DueAt, now) > 0:
			out = append(out, "late")
		default:
			out = append(out, "todo")
		}
	}
	if len(out) == 0 {
		for i := 0; i < v.Loan.Term; i++ {
			out = append(out, "todo")
		}
	}
	return out
}

func lastPaidLabel(v *app.LoanView) string {
	x := v.LastPaid
	if x == nil || x.PaidAt == nil {
		return ""
	}
	how := domain.MethodNames[x.Method]
	if x.Status == domain.InstPaidOut {
		how = "Fora da RIHA"
	}
	return domain.FirstNonEmpty(how, "Pago") + " · " + domain.ShortDate(*x.PaidAt)
}

func loanRowJSON(v *app.LoanView, p *domain.CreditProduct) gin.H {
	l := v.Loan
	j := gin.H{"id": l.ID, "name": l.Name, "phone": domain.FormatPhone(l.Phone), "reference": l.Reference, "amount": l.Amount, "debt": v.Debt, "paid": v.Paid,
		"n": len(pips(v)), "pips": pips(v), "state": v.State, "tone": v.Tone, "status": l.Status, "last": lastPaidLabel(v), "days_late": v.DaysLate, "next": nil}
	if v.Next != nil {
		j["next"] = iso(&v.Next.DueAt)
	}
	if p != nil {
		j["product"] = p.Name
	}
	return j
}

func (srv *Server) mcLoans(c *gin.Context) {
	vs, counts, err := srv.A.CreditLoans(ctx(c), merchant(c), live(c), c.Query("tab"), c.Query("product"), c.Query("q"))
	if err != nil {
		fail(c, err)
		return
	}
	ps := srv.productNames(c)
	out := []gin.H{}
	for _, v := range vs {
		out = append(out, loanRowJSON(v, ps[v.Loan.ProductID]))
	}
	c.JSON(200, gin.H{"data": out, "counts": counts})
}

func (srv *Server) mcExportLoans(c *gin.Context) {
	ps := srv.productNames(c)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="riha-emprestimos.csv"`)
	w := csv.NewWriter(c.Writer)
	_ = writeSafe(w, []string{"Referência", "Cliente", "Número", "Produto", "Emprestado (MT)", "Em dívida (MT)", "Prestações pagas", "Próxima", "Estado", "Último pagamento"})
	for _, tab := range []string{"ativos", "aceitar", "liquidados", "perdas"} {
		vs, _, err := srv.A.CreditLoans(ctx(c), merchant(c), live(c), tab, "", "")
		if err != nil {
			continue
		}
		for _, v := range vs {
			pn, next := "", ""
			if p := ps[v.Loan.ProductID]; p != nil {
				pn = p.Name
			}
			if v.Next != nil {
				next = domain.ShortDate(v.Next.DueAt)
			}
			_ = writeSafe(w, []string{v.Loan.Reference, csvSafe(v.Loan.Name), domain.FormatPhone(v.Loan.Phone), csvSafe(pn), domain.Format(v.Loan.Amount), domain.Format(v.Debt),
				fmt.Sprintf("%d/%d", v.Paid, len(pips(v))), next, v.State, lastPaidLabel(v)})
		}
	}
	w.Flush()
}

func (srv *Server) mcCreateLoan(c *gin.Context) {
	var in app.OfferInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	l, err := srv.A.CreateLoan(ctx(c), merchant(c), live(c), in, user(c), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"id": l.ID})
}

func instalmentJSON(l *domain.Loan, x *domain.Instalment, now time.Time) gin.H {
	st := x.Status
	days := 0
	if x.Open() {
		st = "scheduled"
		if d := domain.DaysLate(x.DueAt, now); d > 0 {
			st, days = "late", d
		}
	}
	return gin.H{"id": x.ID, "seq": x.Seq, "version": x.Version, "due": iso(&x.DueAt), "amount": x.Amount, "mora": l.MoraOn(x, now), "mora_paid": x.MoraPaid,
		"mora_waived": x.MoraWaived, "status": st, "days_late": days, "method": domain.FirstNonEmpty(domain.MethodNames[x.Method], app.OutsideMethodLabel(x.Method)),
		"paid_at": iso(x.PaidAt), "marked_by": x.MarkedBy, "note": x.Note, "link_sent": iso(x.LinkSentAt), "opened": x.OpenedAt != nil, "steps": x.Steps}
}

func (srv *Server) loanJSON(v *app.LoanView) gin.H {
	l, now := v.Loan, time.Now()
	j := loanRowJSON(v, v.Product)
	cur, old := []gin.H{}, []gin.H{}
	for i := range v.Current {
		cur = append(cur, instalmentJSON(l, &v.Current[i], now))
	}
	for i := range v.Old {
		old = append(old, instalmentJSON(l, &v.Old[i], now))
	}
	gs := []gin.H{}
	for i := range v.Agreements {
		gs = append(gs, agreementJSON(&v.Agreements[i]))
	}
	j["plan"], j["old"], j["agreements"], j["proposal"] = cur, old, gs, agreementJSON(v.Proposal)
	j["term"], j["day"], j["freq"], j["rate_label"], j["rate_bp"] = l.Term, l.Day, l.Freq, domain.RateLabel(l.RateBP, l.Freq), l.RateBP
	j["mora_bp"], j["grace"], j["mora_max"], j["version"], j["proposed"] = l.MoraBP, l.Grace, l.MoraMax, l.Version, l.Proposed
	j["delivery"], j["delivery_status"], j["delivery_error"], j["delivered_at"], j["wallet"] = l.Delivery, l.DeliveryStatus, l.DeliveryError, iso(l.DeliveredAt), domain.MethodNames[l.Wallet]
	j["accepted_at"], j["created_at"], j["closed_at"], j["close_note"], j["asked_at"] = iso(l.AcceptedAt), iso(&l.CreatedAt), iso(l.ClosedAt), l.CloseNote, iso(l.AskedAt)
	j["manual"], j["settle_offer"], j["approved_by"], j["second_by"] = l.Manual, l.SettleOffer, l.ApprovedBy, l.SecondBy
	j["interest"], j["paid_riha"], j["paid_outside"], j["mora"], j["mora_paid"] = v.Interest, v.PaidRiha, v.PaidOutside, v.Mora, v.MoraPaid
	j["overdue"], j["to_come"], j["request_id"] = v.Overdue, v.ToCome, l.RequestID
	if v.Product != nil {
		j["product_full"] = productJSON(v.Product)
	}
	return j
}

func (srv *Server) mcLoan(c *gin.Context) {
	v, err := srv.A.CreditLoan(ctx(c), merchant(c), live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.loanJSON(v))
}

func (srv *Server) mcLoanAction(c *gin.Context) {
	m, lv, id := merchant(c), live(c), c.Param("id")
	body, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<16))
	var err error
	bind := func(dst any) bool {
		if len(body) == 0 {
			return true
		}
		if e := json.Unmarshal(body, dst); e != nil {
			bad(c, "Pedido inválido.")
			return false
		}
		return true
	}
	switch c.Param("action") {
	case "resend":
		err = srv.A.ResendLoanLink(ctx(c), m, lv, id, actor(c))
	case "second":
		_, err = srv.A.ApproveLoanSecond(ctx(c), m, lv, id, user(c), actor(c))
	case "cancel":
		err = srv.A.CancelLoanOffer(ctx(c), m, lv, id, actor(c))
	case "retry":
		_, err = srv.A.RetryDelivery(ctx(c), m, lv, id, actor(c))
	case "delivered":
		var in struct {
			Note string `json:"note"`
		}
		if !bind(&in) {
			return
		}
		_, err = srv.A.MarkDelivered(ctx(c), m, lv, id, in.Note, actor(c))
	case "settle":
		var in app.SettleInput
		if !bind(&in) {
			return
		}
		_, err = srv.A.SettleEarly(ctx(c), m, lv, id, in, user(c), actor(c))
	case "write-off":
		var in struct {
			Note string `json:"note"`
		}
		if !bind(&in) {
			return
		}
		_, err = srv.A.WriteOffLoan(ctx(c), m, lv, id, in.Note, actor(c))
	case "agreement":
		var in app.AgreementInput
		if !bind(&in) {
			return
		}
		_, err = srv.A.ProposeAgreement(ctx(c), m, lv, id, in, user(c), actor(c))
	default:
		fail(c, domain.NotFound("A ação"))
		return
	}
	if err != nil {
		fail(c, err)
		return
	}
	srv.mcLoan(c)
}

func (srv *Server) mcMarkPaid(c *gin.Context) {
	var in app.MarkPaidInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if _, err := srv.A.MarkInstalmentPaid(ctx(c), merchant(c), live(c), c.Param("id"), c.Param("iid"), in, user(c), actor(c)); err != nil {
		fail(c, err)
		return
	}
	srv.mcLoan(c)
}

func (srv *Server) mcResendOffers(c *gin.Context) {
	n, err := srv.A.ResendOffers(ctx(c), merchant(c), live(c), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"sent": n})
}

func pdfOut(c *gin.Context, b []byte, l *domain.Loan) {
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="acordo-%s.pdf"`, l.Reference))
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(200, "application/pdf", b)
}

func (srv *Server) mcAgreementPDF(c *gin.Context) {
	b, l, err := srv.A.LoanAgreementPDF(ctx(c), merchant(c), live(c), c.Param("id"), qInt(c, "v", 0))
	if err != nil {
		fail(c, err)
		return
	}
	pdfOut(c, b, l)
}

// Public: the customer's pages.

func publicProductJSON(p *domain.CreditProduct) gin.H {
	j := productJSON(p)
	delete(j, "status")
	return j
}

func (srv *Server) pubCreditProducts(c *gin.Context) {
	m, ps, lv, err := srv.A.PublicCreditProducts(ctx(c), c.Param("merchant"))
	if err != nil {
		fail(c, err)
		return
	}
	out := []gin.H{}
	for i := range ps {
		out = append(out, publicProductJSON(&ps[i]))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"merchant": merchantBrand(m), "products": out, "test_mode": !lv, "contact": domain.NormalizeMZPhone(m.Phone)})
}

func (srv *Server) pubCreditRequest(c *gin.Context) {
	var in app.CreditRequestInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	in.IP = c.ClientIP()
	r, code, err := srv.A.StartCreditRequest(ctx(c), c.Param("merchant"), in)
	if err != nil {
		fail(c, err)
		return
	}
	out := gin.H{"token": r.Token, "phone": domain.MaskPhone(r.Phone)}
	if code != "" {
		out["test_code"] = code
	}
	c.JSON(200, out)
}

func (srv *Server) pubRequestCode(c *gin.Context) {
	code, err := srv.A.ResendRequestCode(ctx(c), c.Param("token"))
	if err != nil {
		fail(c, err)
		return
	}
	out := gin.H{"ok": true}
	if code != "" {
		out["test_code"] = code
	}
	c.JSON(200, out)
}

func (srv *Server) pubConfirmRequest(c *gin.Context) {
	var in struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if _, err := srv.A.ConfirmCreditRequest(ctx(c), c.Param("token"), in.Code); err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (srv *Server) pubRequestDocument(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		bad(c, "Escolha uma foto ou um PDF.")
		return
	}
	f, err := fh.Open()
	if err != nil {
		bad(c, "Não foi possível ler o ficheiro.")
		return
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		bad(c, "Não foi possível ler o ficheiro.")
		return
	}
	d, err := srv.A.AddCreditDocument(ctx(c), c.Param("token"), app.Upload{Kind: c.PostForm("kind"), FileName: fh.Filename, Size: fh.Size, Head: head[:n], Body: f})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"id": d.ID, "kind": d.Kind})
}

func (srv *Server) pubCreditPage(c *gin.Context) {
	v, err := srv.A.CreditPage(ctx(c), c.Param("token"), c.Query("opened") == "1")
	if err != nil {
		fail(c, err)
		return
	}
	out := gin.H{"merchant": merchantBrand(v.Merchant), "kind": "request", "test_mode": false, "contact": domain.NormalizeMZPhone(v.Merchant.Phone)}
	if v.Product != nil {
		out["product"] = publicProductJSON(v.Product)
	}
	if r := v.Request; r != nil {
		out["request"] = gin.H{"status": r.Status, "amount": r.Amount, "term": r.Term, "instalment": r.Instalment, "created_at": iso(&r.CreatedAt),
			"phone": domain.FormatPhone(r.Phone), "wallet": domain.MethodNames[r.Wallet], "name": r.Name, "phone_ok": r.PhoneOKAt != nil}
		out["test_mode"] = !r.Livemode
	}
	if lv := v.Loan; lv != nil {
		l := lv.Loan
		now := time.Now()
		plan := []gin.H{}
		for i := range lv.Current {
			x := instalmentJSON(l, &lv.Current[i], now)
			for _, k := range []string{"marked_by", "note", "steps", "link_sent", "opened"} {
				delete(x, k)
			}
			plan = append(plan, x)
		}
		loan := gin.H{"status": l.Status, "reference": l.Reference, "name": l.Name, "amount": l.Amount, "wallet": domain.MethodNames[l.Wallet],
			"phone": domain.FormatPhone(l.Phone), "delivery": l.Delivery, "delivery_status": l.DeliveryStatus, "accepted_at": iso(l.AcceptedAt),
			"debt": lv.Debt, "paid": lv.Paid, "plan": plan, "settle_offer": l.SettleOffer, "asked": l.AskedAt != nil, "mora": lv.Mora, "overdue": lv.Overdue,
			"days_late": lv.DaysLate, "freq": l.Freq, "rate_label": domain.RateLabel(l.RateBP, l.Freq), "mora_bp": l.MoraBP, "grace": l.Grace, "mora_max": l.MoraMax,
			"version": l.Version, "interest": lv.Interest, "term": l.Term, "day": l.Day, "closed_at": iso(l.ClosedAt)}
		if lv.Next != nil {
			loan["next"] = instalmentJSON(l, lv.Next, now)
		}
		for i := range lv.Agreements {
			if lv.Agreements[i].Status == domain.AgreementAccepted {
				loan["agreement"] = agreementJSON(&lv.Agreements[i])
			}
		}
		if p := lv.Proposal; p != nil {
			j := agreementJSON(p)
			if p.Version == 1 {
				j["dates"] = datesJSON(domain.DueDates(l.Freq, l.Day, l.Term, time.Now()))
			} else {
				j["dates"] = datesJSON(domain.NextDates(p.Dates[0].In(domain.Maputo).Day(), len(p.Amounts), time.Now()))
			}
			loan["proposal"] = j
		}
		out["kind"], out["loan"], out["test_mode"] = "loan", loan, !l.Livemode
		if p := v.Payment; p != nil {
			out["payment"] = gin.H{"status": p.Status, "reference": p.Reference, "method": methodNames[p.Method], "amount": p.Amount,
				"failure_message": p.FailureMessage, "paid_at": iso(p.SucceededAt)}
		}
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.JSON(200, out)
}

func (srv *Server) pubAgreementCode(c *gin.Context) {
	var in struct {
		Accept bool `json:"accept"`
	}
	_ = c.ShouldBindJSON(&in)
	code, err := srv.A.SendAgreementCode(ctx(c), c.Param("token"), in.Accept)
	if err != nil {
		fail(c, err)
		return
	}
	out := gin.H{"ok": true}
	if code != "" {
		out["test_code"] = code
	}
	c.JSON(200, out)
}

func (srv *Server) pubAcceptAgreement(c *gin.Context) {
	var in struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if _, err := srv.A.AcceptAgreement(ctx(c), c.Param("token"), in.Code, c.ClientIP()); err != nil {
		fail(c, err)
		return
	}
	srv.pubCreditPage(c)
}

func (srv *Server) pubPayLoan(c *gin.Context) {
	var in app.LoanPay
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	in.IP, in.UA = c.ClientIP(), c.GetHeader("User-Agent")
	if _, err := srv.A.PayLoan(ctx(c), c.Param("token"), in); err != nil {
		fail(c, err)
		return
	}
	srv.pubCreditPage(c)
}

func (srv *Server) pubAskAgreement(c *gin.Context) {
	if err := srv.A.AskNewAgreement(ctx(c), c.Param("token")); err != nil {
		fail(c, err)
		return
	}
	srv.pubCreditPage(c)
}

func (srv *Server) pubAgreementPDF(c *gin.Context) {
	b, l, err := srv.A.PublicAgreementPDF(ctx(c), c.Param("token"), qInt(c, "v", 0))
	if err != nil {
		fail(c, err)
		return
	}
	pdfOut(c, b, l)
}
