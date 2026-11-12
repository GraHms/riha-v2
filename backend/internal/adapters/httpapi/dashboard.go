package httpapi

import (
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

func (srv *Server) dashboardRoutes(api *gin.RouterGroup) {
	api.POST("/auth/signup", srv.throttle(srv.authLimiter, "signup"), srv.signup)
	api.POST("/auth/login", srv.throttle(srv.authLimiter, "login"), srv.login)
	api.POST("/auth/logout", srv.logout)
	api.POST("/auth/forgot", srv.throttle(srv.authLimiter, "forgot"), srv.forgotPassword)
	api.POST("/auth/reset", srv.throttle(srv.authLimiter, "reset"), srv.resetPassword)
	api.GET("/auth/me", srv.auth(), srv.me)
	api.POST("/auth/password", srv.auth(), srv.changePassword)

	srv.publicRoutes(api.Group("/public"))

	m := api.Group("", srv.auth(), srv.merchantOnly())
	m.GET("/merchant", func(c *gin.Context) { c.JSON(200, srv.merchantJSON(ctx(c), merchant(c))) })
	m.PATCH("/merchant", srv.updateMerchant)
	m.GET("/merchant/kyc", srv.getKyc)
	m.POST("/merchant/kyc", srv.saveKyc)
	m.POST("/merchant/kyc/documents", srv.uploadDoc)
	m.POST("/merchant/kyc/submit", srv.submitKyc)
	m.GET("/overview", srv.overview)
	m.GET("/payments", srv.listPayments)
	m.GET("/payments/export.csv", srv.exportPayments)
	m.GET("/payments/:id", srv.getPayment)
	m.POST("/payments/:id/refund", srv.refundPayment)
	m.POST("/payments/:id/receipt", srv.throttle(srv.authLimiter, "receipt"), srv.sendReceipt)
	m.GET("/links", srv.listLinks)
	m.POST("/links", srv.createLink)
	m.GET("/links/:id", srv.getLink)
	m.PATCH("/links/:id", srv.updateLink)
	m.POST("/links/:id/image", srv.putLinkImage)
	m.DELETE("/links/:id/image", srv.deleteLinkImage)
	m.GET("/balance", srv.balance)
	m.GET("/payouts", srv.listPayouts)
	m.POST("/payouts", srv.createPayout)
	m.GET("/payouts/:id/statement.pdf", srv.payoutStatement)
	m.GET("/payouts/:id/receipt", srv.payoutReceipt)
	m.GET("/bank-account", srv.getBank)
	m.PUT("/bank-account", srv.putBank)
	m.GET("/keys", srv.getKeys)
	m.POST("/keys/roll", srv.rollKey)
	m.GET("/api-logs", srv.apiLogs)
	m.GET("/developers/settings", srv.devSettings)
	m.PUT("/developers/redirects", srv.putRedirects)
	m.POST("/developers/endpoints", srv.createEndpoint)
	m.PATCH("/developers/endpoints/:id", srv.updateEndpoint)
	m.DELETE("/developers/endpoints/:id", srv.deleteEndpoint)
	m.POST("/developers/domains", srv.addDomain)
	m.DELETE("/developers/domains/:id", srv.deleteDomain)
	m.POST("/developers/domains/:id/verify", srv.verifyDomain)
	m.GET("/developers/deliveries", srv.listDeliveries)
	m.POST("/developers/deliveries/:id/resend", srv.resendDelivery)
	m.POST("/developers/test-event", srv.testEvent)
	m.GET("/developers/secret", func(c *gin.Context) { c.JSON(200, gin.H{"secret": merchant(c).SigningSecret(live(c))}) })
	m.POST("/developers/secret/roll", func(c *gin.Context) {
		s, err := srv.A.RollSigningSecret(ctx(c), merchant(c).ID, live(c), actor(c))
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(200, gin.H{"secret": s})
	})

	srv.subscriptionRoutes(m)

	srv.adminRoutes(api.Group("/admin", srv.auth(), adminOnly()))
	srv.partnerRoutes(api.Group("/partner", srv.auth(), partnerOnly()))
}

func (srv *Server) setCookie(c *gin.Context, tok string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(cookieName, tok, maxAge, "/", "", srv.cfg.CookieSecure, true)
}

func (srv *Server) signup(c *gin.Context) {
	var in app.SignupInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	u, m, err := srv.A.Signup(ctx(c), in)
	if err != nil {
		fail(c, err)
		return
	}
	tok, err := srv.A.NewSession(ctx(c), u, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		fail(c, err)
		return
	}
	srv.A.Record(ctx(c), m.ID, app.Actor{ID: u.ID, Role: u.Role, IP: c.ClientIP()}, "account.created", u.ID, "")
	srv.setCookie(c, tok, int(app.SessionTTL.Seconds()))
	c.Set("user", u)
	srv.me(c)
}

func (srv *Server) forgotPassword(c *gin.Context) {
	var in struct{ Email string }
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if err := srv.A.RequestPasswordReset(ctx(c), in.Email); err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (srv *Server) resetPassword(c *gin.Context) {
	var in struct{ Token, Password string }
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if err := srv.A.ResetPassword(ctx(c), in.Token, in.Password); err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (srv *Server) sendReceipt(c *gin.Context) {
	var in struct{ Email string }
	if err := c.ShouldBindJSON(&in); err != nil && !errors.Is(err, io.EOF) {
		bad(c, "Pedido inválido.")
		return
	}
	to, err := srv.A.SendReceipt(ctx(c), merchant(c).ID, live(c), c.Param("id"), in.Email)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true, "message": "Recibo enviado para " + to + "."})
}

// deviceCookie remembers a device that entered a login code.
const deviceCookie = "riha_device"

func (srv *Server) login(c *gin.Context) {
	var in struct{ Email, Password, Code string }
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	device, _ := c.Cookie(deviceCookie)
	r, err := srv.A.SignIn(ctx(c), app.LoginInput{Email: in.Email, Password: in.Password, Code: in.Code, Device: device, IP: c.ClientIP(), UA: c.GetHeader("User-Agent")})
	if err != nil {
		fail(c, err)
		return
	}
	if r.Device != "" {
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(deviceCookie, r.Device, int(app.DeviceTTL.Seconds()), "/api/auth", "", srv.cfg.CookieSecure, true)
	}
	srv.setCookie(c, r.Token, int(app.SessionTTL.Seconds()))
	c.Set("user", r.User)
	srv.me(c)
}

func (srv *Server) logout(c *gin.Context) {
	if tok, err := c.Cookie(cookieName); err == nil {
		srv.A.Logout(ctx(c), tok)
	}
	srv.setCookie(c, "", -1)
	c.JSON(200, gin.H{"ok": true})
}

func (srv *Server) me(c *gin.Context) {
	u := user(c)
	perms := u.Permissions
	if perms == nil {
		perms = []string{}
	}
	out := gin.H{"user": gin.H{"id": u.ID, "email": u.Email, "name": u.Name, "phone": u.Phone, "role": u.Role, "title": u.AdminTitle, "permissions": perms}}
	if u.Role == domain.RolePartner {
		if p, err := srv.A.Integrations.Partner(ctx(c), u.PartnerCode); err == nil {
			out["partner"] = gin.H{"code": p.Code, "name": p.Name}
		}
	}
	if m, err := srv.A.MerchantForUser(ctx(c), u); err == nil {
		out["merchant"] = srv.merchantJSON(ctx(c), m)
	}
	out["default_fee_bps"] = srv.A.DefaultFeeBps(ctx(c))
	c.JSON(200, out)
}

func (srv *Server) changePassword(c *gin.Context) {
	var in struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	u := user(c)
	tok, _ := c.Cookie(cookieName)
	if err := srv.A.ChangePassword(ctx(c), u, in.Current, in.New, tok); err != nil {
		fail(c, err)
		return
	}
	mid := ""
	if u.MerchantID != nil {
		mid = *u.MerchantID
	}
	srv.A.Record(ctx(c), mid, actor(c), "password.changed", u.ID, "")
	c.JSON(200, gin.H{"ok": true})
}

func (srv *Server) updateMerchant(c *gin.Context) {
	var in app.ProfileInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	if err := srv.A.UpdateProfile(ctx(c), m, in); err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.merchantJSON(ctx(c), m))
}

func (srv *Server) kycJSON(c *gin.Context, m *domain.Merchant) gin.H {
	return gin.H{"status": m.Status, "business_type": m.BusinessType, "nuit": m.NUIT, "legal_name": m.LegalName, "city": m.City,
		"expected_volume": m.ExpectedVolume, "owner_name": m.OwnerName, "documents": docsJSON(srv.A.DocumentsOf(ctx(c), m.ID)), "review_note": m.ReviewNote}
}

func (srv *Server) getKyc(c *gin.Context) { c.JSON(200, srv.kycJSON(c, merchant(c))) }

func (srv *Server) saveKyc(c *gin.Context) {
	var in app.KycInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	if err := srv.A.SaveKyc(ctx(c), m, in); err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.kycJSON(c, m))
}

func (srv *Server) uploadDoc(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		bad(c, "Escolha um ficheiro.")
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
	d, err := srv.A.UploadDocument(ctx(c), merchant(c), app.Upload{Kind: c.PostForm("kind"), FileName: fh.Filename, Size: fh.Size, Head: head[:n], Body: f})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"id": d.ID, "kind": d.Kind, "file_name": d.FileName, "size": d.Size})
}

func (srv *Server) submitKyc(c *gin.Context) {
	m := merchant(c)
	if err := srv.A.SubmitKyc(ctx(c), m, actor(c)); err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.kycJSON(c, m))
}

func (srv *Server) overview(c *gin.Context) {
	o, err := srv.A.Overview(ctx(c), merchant(c), live(c), qInt(c, "days", 30))
	if err != nil {
		fail(c, err)
		return
	}
	bars := make([]gin.H, 0, len(o.Bars))
	for _, b := range o.Bars {
		bars = append(bars, gin.H{"date": b.Date.Format("2006-01-02"), "amount": b.Amount})
	}
	var pending any
	if o.PendingPayout != nil {
		pending = domain.PayoutResource(o.PendingPayout)
	}
	var bank any
	if o.Bank != nil {
		bank = bankJSON(o.Bank)
	}
	c.JSON(200, gin.H{"available": o.Available, "in_transit": o.InTransit, "received": o.Received, "previous": o.Previous, "days": o.Days,
		"bars": bars, "by_method": o.ByMethod, "recent": paymentRows(o.Recent), "pending_payout": pending, "bank_account": bank})
}

func paymentFilter(c *gin.Context) app.PaymentFilter {
	f := app.PaymentFilter{Status: c.Query("status"), Source: c.Query("source"), LinkID: c.Query("link"), Search: strings.TrimSpace(c.Query("q")), Limit: max(1, min(200, qInt(c, "limit", 50))), Offset: max(0, qInt(c, "offset", 0))}
	if f.Limit > 200 {
		f.Limit = 200
	}
	if t, err := time.Parse("2006-01-02", c.Query("from")); err == nil {
		f.From = &t
	}
	if t, err := time.Parse("2006-01-02", c.Query("to")); err == nil {
		t = t.AddDate(0, 0, 1)
		f.To = &t
	}
	return f
}

func (srv *Server) listPayments(c *gin.Context) {
	p, err := srv.A.SearchPayments(ctx(c), merchant(c).ID, live(c), paymentFilter(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"data": paymentRows(p.Data), "total": p.Total, "counts": p.Counts})
}

func (srv *Server) getPayment(c *gin.Context) {
	d, err := srv.A.PaymentDetail(ctx(c), merchant(c).ID, live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	o := paymentRow(&d.PaymentView)
	if cs := d.Session; cs != nil {
		o["session"] = gin.H{"id": cs.ID, "success_url": cs.SuccessURL, "callback_url": cs.CallbackURL, "status": cs.Status, "expires_at": cs.ExpiresAt, "created_at": cs.CreatedAt}
	}
	if d.APIKey != nil {
		o["api_key_label"] = "sk_" + domain.Mode(d.APIKey.Livemode) + "_…" + d.APIKey.Last4
	}
	if d.CreatedByName != "" {
		o["created_by_name"] = d.CreatedByName
	}
	if l := d.Link; l != nil {
		o["link"] = gin.H{"id": l.ID, "name": l.Name, "custom_amount": l.CustomAmount, "reusable": l.Reusable, "type": l.Kind, "location": l.Location}
	}
	dels := []gin.H{}
	for i := range d.Deliveries {
		dels = append(dels, deliveryJSON(&d.Deliveries[i]))
	}
	refunds := []any{}
	for i := range d.Refunds {
		refunds = append(refunds, domain.RefundResource(&d.Refunds[i]))
	}
	o["deliveries"], o["refunds"] = dels, refunds
	o["succeeded_at"], o["failed_at"] = iso(d.SucceededAt), iso(d.FailedAt)
	o["payer_ip"], o["payer_user_agent"] = d.PayerIP, d.PayerUserAgent
	c.JSON(200, o)
}

func (srv *Server) refundPayment(c *gin.Context) {
	var in struct {
		Amount int64  `json:"amount"`
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&in)
	m := merchant(c)
	r, err := srv.A.Refund(ctx(c), m.ID, live(c), c.Param("id"), in.Amount, in.Reason, user(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	srv.A.Record(ctx(c), m.ID, actor(c), "refund.created", r.ID, domain.Format(r.Amount)+" MT")
	c.JSON(200, domain.RefundResource(r))
}

func (srv *Server) exportPayments(c *gin.Context) {
	rows, err := srv.A.ExportPayments(ctx(c), merchant(c).ID, live(c), paymentFilter(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="riha-transacoes.csv"`)
	w := csv.NewWriter(c.Writer)
	_ = writeSafe(w, []string{"id", "referencia", "data", "estado", "metodo", "origem", "cliente", "client_reference_id", "valor_mzn", "taxa_mzn", "liquido_mzn", "reembolsado_mzn"})
	for _, p := range rows {
		_ = writeSafe(w, []string{p.ID, p.Reference, isoT(p.CreatedAt), p.Status, p.Method, p.SourceType, csvSafe(p.CustomerName), csvSafe(p.ClientReferenceID), centsCSV(p.Amount), centsCSV(p.Fee), centsCSV(p.Net), centsCSV(p.AmountRefunded)})
	}
	w.Flush()
}

type linkInput struct {
	Name         *string  `json:"name"`
	Amount       *int64   `json:"amount"`
	CustomAmount *bool    `json:"custom_amount"`
	Reusable     *bool    `json:"reusable"`
	Methods      []string `json:"payment_method_types"`
	Active       *bool    `json:"active"`
	ExpiresAt    *int64   `json:"expires_at"`
	Slug         *string  `json:"slug"`
	Type         string   `json:"type"`
	Location     string   `json:"location"`
	Details      *string  `json:"details"`
	Suggested    *int64   `json:"suggested_amount"`
	MinAmount    *int64   `json:"min_amount"`
	MaxAmount    *int64   `json:"max_amount"`
	AfterMessage *string  `json:"after_message"`
	RedirectURL  *string  `json:"redirect_url"`
	Archived     *bool    `json:"archived"`
	SubmitType   *string  `json:"submit_type"`
}

func (in linkInput) params() app.LinkParams {
	return app.LinkParams{Name: in.Name, Amount: in.Amount, CustomAmount: in.CustomAmount, Reusable: in.Reusable, Methods: in.Methods,
		Active: in.Active, ExpiresAt: in.ExpiresAt, Slug: in.Slug, Kind: in.Type, Location: in.Location,
		Details: in.Details, SuggestedAmount: in.Suggested, MinAmount: in.MinAmount, MaxAmount: in.MaxAmount, AfterMessage: in.AfterMessage, RedirectURL: in.RedirectURL, Archived: in.Archived, SubmitType: in.SubmitType}
}

func (srv *Server) putLinkImage(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		bad(c, "Escolha uma imagem.")
		return
	}
	f, err := fh.Open()
	if err != nil {
		bad(c, "Não foi possível ler a imagem.")
		return
	}
	defer f.Close()
	m := merchant(c)
	l, err := srv.A.SaveLinkImage(ctx(c), m, live(c), c.Param("id"), f)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.linkJSON(srv.A.LinkView(ctx(c), l), m.Slug))
}

func (srv *Server) deleteLinkImage(c *gin.Context) {
	m := merchant(c)
	l, err := srv.A.RemoveLinkImage(ctx(c), m, live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.linkJSON(srv.A.LinkView(ctx(c), l), m.Slug))
}

func (srv *Server) listLinks(c *gin.Context) {
	m := merchant(c)
	vs, err := srv.A.ListLinks(ctx(c), m.ID, live(c), 0)
	if err != nil {
		fail(c, err)
		return
	}
	out := []any{}
	for _, v := range vs {
		out = append(out, srv.linkJSON(v, m.Slug))
	}
	c.JSON(200, gin.H{"data": out})
}

func (srv *Server) getLink(c *gin.Context) {
	m := merchant(c)
	v, err := srv.A.MerchantLink(ctx(c), m.ID, live(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.linkJSON(v, m.Slug))
}

func (srv *Server) createLink(c *gin.Context) {
	var in linkInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	l, err := srv.A.CreateLink(ctx(c), m, live(c), in.params(), user(c).ID, "dashboard")
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.linkJSON(srv.A.LinkView(ctx(c), l), m.Slug))
}

func (srv *Server) updateLink(c *gin.Context) {
	var in linkInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	p := in.params()
	p.CustomAmount, p.Slug = nil, nil
	l, err := srv.A.UpdateLink(ctx(c), m, live(c), c.Param("id"), p)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.linkJSON(srv.A.LinkView(ctx(c), l), m.Slug))
}

func (srv *Server) balance(c *gin.Context) {
	s := srv.A.BalanceSummary(ctx(c), merchant(c), live(c))
	var due any
	if s.InTransitDue != nil {
		due = isoT(*s.InTransitDue)
	}
	c.JSON(200, gin.H{"available": s.Available, "in_transit": s.InTransit, "in_transit_due": due,
		"paid_previous_month": gin.H{"count": s.PaidPrevCount, "total": s.PaidPrev},
		"paid_this_month":     gin.H{"count": s.PaidThisCount, "total": s.PaidThis},
		"min_payout":          domain.MinPayout, "can_payout": s.CanPayout, "transfer": srv.transferJSON(c)})
}

// transferJSON says what a withdrawal to the merchant's bank costs, so the form can
// show it before they ask.
func (srv *Server) transferJSON(c *gin.Context) gin.H {
	t := srv.A.TransferPricing(ctx(c))
	bim := true
	bank := ""
	if b, err := srv.A.Banks.ByMerchant(ctx(c), merchant(c).ID); err == nil {
		bim, bank = domain.IsBimAccount(b.BankName, b.NIB), b.BankName
	}
	return gin.H{"fee": t.Fee, "free_from": t.FreeFrom, "bank": bank, "bank_is_bim": bim}
}

func (srv *Server) listPayouts(c *gin.Context) {
	lv := live(c)
	ps, err := srv.A.ListPayouts(ctx(c), payoutQuery(merchant(c).ID, lv, 100))
	if err != nil {
		fail(c, err)
		return
	}
	out := []any{}
	for i := range ps {
		out = append(out, payoutJSON(&ps[i]))
	}
	c.JSON(200, gin.H{"data": out})
}

func (srv *Server) createPayout(c *gin.Context) {
	var in struct {
		Amount int64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	p, err := srv.A.RequestPayout(ctx(c), m.ID, live(c), in.Amount, user(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	srv.A.Record(ctx(c), m.ID, actor(c), "payout.requested", p.ID, domain.Format(p.Amount)+" MT")
	c.JSON(200, domain.PayoutResource(p))
}

func (srv *Server) bankResponse(c *gin.Context, b *domain.BankAccount) gin.H {
	out := gin.H{"banks": app.Banks, "account": nil}
	if b != nil {
		ba := bankJSON(b)
		if b.DocumentID != nil {
			if d, err := srv.A.Document(ctx(c), *b.DocumentID); err == nil {
				ba["document"] = gin.H{"file_name": d.FileName, "size": d.Size}
			}
		}
		out["account"] = ba
	}
	return out
}

func (srv *Server) getBank(c *gin.Context) {
	c.JSON(200, srv.bankResponse(c, srv.A.BankAccount(ctx(c), merchant(c).ID)))
}

func (srv *Server) putBank(c *gin.Context) {
	var in app.BankInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	b, err := srv.A.SaveBankAccount(ctx(c), merchant(c), in, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.bankResponse(c, b))
}

func (srv *Server) getKeys(c *gin.Context) {
	m, lv := merchant(c), live(c)
	var sk any
	if k := srv.A.SecretKey(ctx(c), m.ID, lv); k != nil {
		sk = gin.H{"id": k.ID, "last4": k.Last4, "created_at": k.CreatedAt, "last_used_at": iso(k.LastUsedAt)}
	}
	c.JSON(200, gin.H{"publishable": srv.A.PublishableKey(ctx(c), m.ID, lv), "secret": sk, "livemode": lv})
}

func (srv *Server) rollKey(c *gin.Context) {
	secret, k, err := srv.A.RollSecretKey(ctx(c), merchant(c), live(c), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"secret_key": secret, "id": k.ID, "last4": k.Last4, "created_at": k.CreatedAt})
}

func (srv *Server) apiLogs(c *gin.Context) {
	out := []gin.H{}
	for _, r := range srv.A.RecentRequests(ctx(c), merchant(c).ID, live(c)) {
		out = append(out, gin.H{"method": r.Method, "path": r.Path, "status": r.Status, "created_at": r.CreatedAt, "duration_ms": r.DurationMs, "request_id": r.RequestID})
	}
	c.JSON(200, gin.H{"data": out})
}

func (srv *Server) devSettings(c *gin.Context) {
	m, lv := merchant(c), live(c)
	eps, _ := srv.A.ListEndpoints(ctx(c), m.ID, lv)
	e := []any{}
	for i := range eps {
		e = append(e, domain.EndpointResource(&eps[i]))
	}
	d := []gin.H{}
	for _, x := range srv.A.DomainList(ctx(c), m.ID) {
		d = append(d, gin.H{"id": x.ID, "domain": x.Domain, "verified": x.VerifiedAt != nil, "txt_name": "_riha." + x.Domain, "txt_value": x.Token})
	}
	sec := m.SigningSecret(lv)
	c.JSON(200, gin.H{"success_url": m.SuccessURL, "cancel_url": m.CancelURL, "endpoints": e, "domains": d,
		"secret_hint": "whsec_••••••••" + sec[len(sec)-4:], "event_types": domain.AllEventTypes})
}

func (srv *Server) putRedirects(c *gin.Context) {
	var in struct {
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if err := srv.A.SetRedirects(ctx(c), merchant(c), in.SuccessURL, in.CancelURL); err != nil {
		fail(c, err)
		return
	}
	srv.devSettings(c)
}

func (srv *Server) createEndpoint(c *gin.Context) {
	var in struct {
		URL    string   `json:"url"`
		Events []string `json:"enabled_events"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	ep, err := srv.A.CreateEndpoint(ctx(c), m.ID, live(c), in.URL, in.Events)
	if err != nil {
		fail(c, err)
		return
	}
	srv.A.Record(ctx(c), m.ID, actor(c), "webhook_endpoint.created", ep.ID, ep.URL)
	srv.devSettings(c)
}

func (srv *Server) updateEndpoint(c *gin.Context) {
	var in struct {
		Events []string `json:"enabled_events"`
		Active *bool    `json:"active"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if err := srv.A.UpdateEndpoint(ctx(c), merchant(c).ID, c.Param("id"), in.Events, in.Active); err != nil {
		fail(c, err)
		return
	}
	srv.devSettings(c)
}

func (srv *Server) deleteEndpoint(c *gin.Context) {
	_, _ = srv.A.DeleteEndpoint(ctx(c), merchant(c).ID, nil, c.Param("id"))
	srv.devSettings(c)
}

func (srv *Server) verifyDomain(c *gin.Context) {
	m := merchant(c)
	d, err := srv.A.VerifyDomain(ctx(c), m.ID, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	srv.A.Record(ctx(c), m.ID, actor(c), "domain.verified", d.Domain, "")
	srv.devSettings(c)
}

func (srv *Server) addDomain(c *gin.Context) {
	var in struct {
		Domain string `json:"domain"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	if _, err := srv.A.AddDomain(ctx(c), m.ID, in.Domain); err != nil {
		fail(c, err)
		return
	}
	srv.A.Record(ctx(c), m.ID, actor(c), "domain.added", in.Domain, "")
	srv.devSettings(c)
}

func (srv *Server) deleteDomain(c *gin.Context) {
	_ = srv.A.RemoveDomain(ctx(c), merchant(c).ID, c.Param("id"))
	srv.devSettings(c)
}

func (srv *Server) listDeliveries(c *gin.Context) {
	ds := srv.A.RecentDeliveries(ctx(c), merchant(c).ID, live(c))
	out := []gin.H{}
	for i := range ds {
		out = append(out, deliveryJSON(&ds[i]))
	}
	c.JSON(200, gin.H{"data": out})
}

func (srv *Server) resendDelivery(c *gin.Context) {
	if err := srv.A.Resend(ctx(c), merchant(c).ID, c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (srv *Server) testEvent(c *gin.Context) {
	if err := srv.A.SendTestEvent(ctx(c), merchant(c).ID, live(c)); err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (srv *Server) payoutStatement(c *gin.Context) {
	pdf, p, err := srv.A.PayoutStatementPDF(ctx(c), c.Param("id"), merchant(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	sendPDF(c, "riha-"+strings.ToLower(p.Ref())+".pdf", pdf, c.Query("download") == "1")
}

func (srv *Server) payoutReceipt(c *gin.Context) {
	a, err := srv.A.PayoutReceipt(ctx(c), c.Param("id"), merchant(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	sendAttachment(c, a)
}
