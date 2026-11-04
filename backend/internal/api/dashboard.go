package api

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"gorm.io/gorm"
	"riha/internal/ids"
	"riha/internal/models"
	"riha/internal/money"
	"riha/internal/service"
)

type gormQuery = gorm.DB

var gormSession = gorm.Session{}

// ---------------------------------------------------------------- auth

func (srv *Server) setCookie(c *gin.Context, tok string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(cookieName, tok, maxAge, "/", "", srv.S.Cfg.CookieSecure, true)
}

func (srv *Server) signup(c *gin.Context) {
	var in service.SignupInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	u, _, err := srv.S.Signup(in)
	if err != nil {
		apiErr(c, err)
		return
	}
	tok, err := srv.S.NewSession(u, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		apiErr(c, err)
		return
	}
	srv.S.Audit(*u.MerchantID, u.ID, u.Role, "account.created", u.ID, "", c.ClientIP())
	srv.setCookie(c, tok, int(service.SessionTTL.Seconds()))
	c.Set("user", u)
	srv.me(c)
}

func (srv *Server) login(c *gin.Context) {
	var in struct{ Email, Password string }
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	tok, u, err := srv.S.Login(in.Email, in.Password, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		apiErr(c, err)
		return
	}
	srv.setCookie(c, tok, int(service.SessionTTL.Seconds()))
	c.Set("user", u)
	srv.me(c)
}

func (srv *Server) logout(c *gin.Context) {
	if tok, err := c.Cookie(cookieName); err == nil {
		srv.S.Logout(tok)
	}
	srv.setCookie(c, "", -1)
	c.JSON(200, gin.H{"ok": true})
}

func (srv *Server) merchantJSON(m *models.Merchant) gin.H {
	bps := srv.S.FeeBpsFor(m)
	return gin.H{
		"id": m.ID, "slug": m.Slug, "name": m.Name, "legal_name": m.LegalName, "business_type": m.BusinessType, "nuit": m.NUIT,
		"phone": m.Phone, "email": m.Email, "city": m.City, "owner_name": m.OwnerName, "expected_volume": m.ExpectedVolume,
		"statement_descriptor": m.StatementDescriptor, "avatar_color": m.AvatarColor, "status": m.Status,
		"fee_bps": bps, "fee_negotiated": m.FeeBps != nil, "fee_label": money.FormatBps(bps),
		"success_url": m.SuccessURL, "cancel_url": m.CancelURL, "sms_notifications": m.SMSNotifications,
		"kyc_submitted_at": iso(m.KycSubmittedAt), "approved_at": iso(m.ApprovedAt), "review_note": m.ReviewNote,
		"created_at": m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (srv *Server) me(c *gin.Context) {
	u := user(c)
	out := gin.H{"user": gin.H{"id": u.ID, "email": u.Email, "name": u.Name, "phone": u.Phone, "role": u.Role, "title": u.AdminTitle}}
	if u.MerchantID != nil {
		var m models.Merchant
		if srv.S.DB.First(&m, "id = ?", *u.MerchantID).Error == nil {
			out["merchant"] = srv.merchantJSON(&m)
		}
	}
	out["default_fee_bps"] = srv.S.DefaultFeeBps()
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
	if err := srv.S.ChangePassword(u, in.Current, in.New); err != nil {
		apiErr(c, err)
		return
	}
	srv.S.Audit(strPtr(u.MerchantID), u.ID, u.Role, "password.changed", u.ID, "", c.ClientIP())
	c.JSON(200, gin.H{"ok": true})
}

func strPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ---------------------------------------------------------------- merchant

func (srv *Server) getMerchant(c *gin.Context) { c.JSON(200, srv.merchantJSON(merchant(c))) }

func (srv *Server) updateMerchant(c *gin.Context) {
	var in struct {
		Name                *string `json:"name"`
		LegalName           *string `json:"legal_name"`
		Phone               *string `json:"phone"`
		City                *string `json:"city"`
		OwnerName           *string `json:"owner_name"`
		StatementDescriptor *string `json:"statement_descriptor"`
		SMSNotifications    *bool   `json:"sms_notifications"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	set := func(dst *string, v *string, n int) {
		if v != nil {
			s := strings.TrimSpace(*v)
			if len(s) > n {
				s = s[:n]
			}
			*dst = s
		}
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		bad(c, "O nome comercial não pode ficar vazio.")
		return
	}
	set(&m.Name, in.Name, 200)
	set(&m.LegalName, in.LegalName, 200)
	set(&m.Phone, in.Phone, 40)
	set(&m.City, in.City, 80)
	set(&m.OwnerName, in.OwnerName, 200)
	if in.StatementDescriptor != nil {
		d := strings.ToUpper(strings.TrimSpace(*in.StatementDescriptor))
		if len(d) > 22 {
			bad(c, "O nome no extrato tem no máximo 22 caracteres.")
			return
		}
		m.StatementDescriptor = d
	}
	if in.SMSNotifications != nil {
		m.SMSNotifications = *in.SMSNotifications
	}
	if err := srv.S.DB.Save(m).Error; err != nil {
		apiErr(c, err)
		return
	}
	c.JSON(200, srv.merchantJSON(m))
}

func (srv *Server) kycJSON(m *models.Merchant) gin.H {
	var docs []models.KycDocument
	srv.S.DB.Where("merchant_id = ?", m.ID).Order("created_at").Find(&docs)
	d := []gin.H{}
	for _, x := range docs {
		d = append(d, gin.H{"id": x.ID, "kind": x.Kind, "file_name": x.FileName, "size": x.Size, "created_at": x.CreatedAt})
	}
	return gin.H{"status": m.Status, "business_type": m.BusinessType, "nuit": m.NUIT, "legal_name": m.LegalName, "city": m.City,
		"expected_volume": m.ExpectedVolume, "owner_name": m.OwnerName, "documents": d, "review_note": m.ReviewNote}
}

func (srv *Server) getKyc(c *gin.Context) { c.JSON(200, srv.kycJSON(merchant(c))) }

func (srv *Server) saveKyc(c *gin.Context) {
	var in struct {
		BusinessType   string `json:"business_type"`
		NUIT           string `json:"nuit"`
		LegalName      string `json:"legal_name"`
		City           string `json:"city"`
		ExpectedVolume string `json:"expected_volume"`
		OwnerName      string `json:"owner_name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	if m.Status == models.MerchantActive || m.Status == models.MerchantSuspended {
		apiErr(c, service.Forbidden("kyc_locked", "A verificação já está concluída. Contacte o suporte para alterar."))
		return
	}
	if in.BusinessType != "individual" && in.BusinessType != "company" {
		bad(c, "Escolha o tipo de negócio.")
		return
	}
	nuit := strings.Join(strings.Fields(in.NUIT), "")
	if len(nuit) != 9 || strings.Trim(nuit, "0123456789") != "" {
		apiErr(c, service.Invalid("nuit_invalid", "nuit", "O NUIT tem 9 dígitos."))
		return
	}
	m.BusinessType, m.NUIT = in.BusinessType, nuit
	if in.LegalName != "" {
		m.LegalName = strings.TrimSpace(in.LegalName)
	}
	if m.LegalName == "" {
		m.LegalName = m.Name
	}
	if in.City != "" {
		m.City = strings.TrimSpace(in.City)
	}
	if in.ExpectedVolume != "" {
		m.ExpectedVolume = in.ExpectedVolume
	}
	if in.OwnerName != "" {
		m.OwnerName = strings.TrimSpace(in.OwnerName)
	}
	srv.S.DB.Save(m)
	c.JSON(200, srv.kycJSON(m))
}

var docKinds = map[string]bool{"id_front": true, "id_back": true, "registration": true, "nib_proof": true}

func (srv *Server) uploadDoc(c *gin.Context) {
	m := merchant(c)
	kind := c.PostForm("kind")
	if !docKinds[kind] {
		bad(c, "Tipo de documento inválido.")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		bad(c, "Escolha um ficheiro.")
		return
	}
	if fh.Size > 8<<20 {
		bad(c, "O ficheiro tem de ter menos de 8 MB.")
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	allowed := map[string]string{".pdf": "application/pdf", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png"}
	ct, ok := allowed[ext]
	if !ok {
		bad(c, "Use PDF, JPG ou PNG.")
		return
	}
	f, err := fh.Open()
	if err != nil {
		apiErr(c, err)
		return
	}
	defer f.Close()
	head := make([]byte, 8)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	isPDF := strings.HasPrefix(string(head), "%PDF")
	isJPG := len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF
	isPNG := strings.HasPrefix(string(head), "\x89PNG")
	if (ct == "application/pdf" && !isPDF) || (ct == "image/jpeg" && !isJPG) || (ct == "image/png" && !isPNG) {
		bad(c, "O conteúdo do ficheiro não corresponde à extensão.")
		return
	}
	id := ids.New("doc")
	dir := filepath.Join(srv.S.Cfg.UploadDir, m.ID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		apiErr(c, err)
		return
	}
	path := filepath.Join(dir, id+ext)
	if err := c.SaveUploadedFile(fh, path); err != nil {
		apiErr(c, err)
		return
	}
	name := filepath.Base(fh.Filename)
	if len(name) > 200 {
		name = name[:200]
	}
	doc := &models.KycDocument{ID: id, MerchantID: m.ID, Kind: kind, FileName: name, StoredPath: path, Size: fh.Size, ContentType: ct}
	// Replace the previous document of the same kind.
	var old []models.KycDocument
	srv.S.DB.Where("merchant_id = ? AND kind = ?", m.ID, kind).Find(&old)
	for _, o := range old {
		os.Remove(o.StoredPath)
		srv.S.DB.Delete(&o)
	}
	if err := srv.S.DB.Create(doc).Error; err != nil {
		apiErr(c, err)
		return
	}
	if kind == "nib_proof" {
		srv.S.DB.Model(&models.BankAccount{}).Where("merchant_id = ?", m.ID).Update("document_id", id)
	}
	c.JSON(200, gin.H{"id": doc.ID, "kind": kind, "file_name": name, "size": fh.Size})
}

func (srv *Server) submitKyc(c *gin.Context) {
	m := merchant(c)
	if m.NUIT == "" || m.BusinessType == "" {
		apiErr(c, service.Invalid("kyc_incomplete", "nuit", "Preencha o tipo de negócio e o NUIT."))
		return
	}
	var docs int64
	srv.S.DB.Model(&models.KycDocument{}).Where("merchant_id = ? AND kind IN ?", m.ID, []string{"id_front", "registration"}).Count(&docs)
	if docs == 0 {
		apiErr(c, service.Invalid("kyc_incomplete", "documents", "Carregue pelo menos o Bilhete de Identidade."))
		return
	}
	if m.Status == models.MerchantOnboarding || m.Status == models.MerchantInfoRequested || m.Status == models.MerchantRejected {
		now := time.Now().UTC()
		m.Status, m.KycSubmittedAt = models.MerchantInReview, &now
		srv.S.DB.Save(m)
		srv.S.Audit(m.ID, user(c).ID, user(c).Role, "kyc.submitted", m.ID, "", c.ClientIP())
	}
	c.JSON(200, srv.kycJSON(m))
}

// ---------------------------------------------------------------- overview

func (srv *Server) overview(c *gin.Context) {
	m := merchant(c)
	lv := live(c)
	days := qInt(c, "days", 30)
	if days != 7 && days != 30 && days != 365 {
		days = 30
	}
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, -(days - 1))
	prevStart := start.AddDate(0, 0, -days)
	db := srv.S.DB
	succ := []string{models.PaymentSucceeded, models.PaymentRefunded, models.PaymentPartiallyRefunded}

	var total, prev int64
	db.Model(&models.Payment{}).Where("merchant_id = ? AND livemode = ? AND status IN ? AND created_at >= ?", m.ID, lv, succ, start).Select("COALESCE(SUM(amount),0)").Scan(&total)
	db.Model(&models.Payment{}).Where("merchant_id = ? AND livemode = ? AND status IN ? AND created_at >= ? AND created_at < ?", m.ID, lv, succ, prevStart, start).Select("COALESCE(SUM(amount),0)").Scan(&prev)

	type row struct {
		Bucket time.Time
		Sum    int64
	}
	var rows []row
	trunc := "day"
	if days == 365 {
		trunc = "month"
	}
	db.Raw(fmt.Sprintf(`SELECT date_trunc('%s', created_at) AS bucket, SUM(amount) AS sum FROM payments
		WHERE merchant_id = ? AND livemode = ? AND status IN ? AND created_at >= ? GROUP BY 1 ORDER BY 1`, trunc), m.ID, lv, succ, start).Scan(&rows)
	byB := map[string]int64{}
	for _, r := range rows {
		byB[r.Bucket.UTC().Format("2006-01-02")] = r.Sum
	}
	bars := []gin.H{}
	if days == 365 {
		for i := 11; i >= 0; i-- {
			t := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -i, 0)
			bars = append(bars, gin.H{"date": t.Format("2006-01-02"), "amount": byB[t.Format("2006-01-02")]})
		}
	} else {
		for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
			bars = append(bars, gin.H{"date": d.Format("2006-01-02"), "amount": byB[d.Format("2006-01-02")]})
		}
	}
	type mrow struct {
		Method string
		Sum    int64
	}
	var mrows []mrow
	db.Raw(`SELECT method, SUM(amount) AS sum FROM payments WHERE merchant_id = ? AND livemode = ? AND status IN ? AND created_at >= ? GROUP BY method`, m.ID, lv, succ, start).Scan(&mrows)
	byMethod := gin.H{"mpesa": int64(0), "emola": int64(0), "card": int64(0)}
	for _, r := range mrows {
		byMethod[r.Method] = r.Sum
	}
	var recent []models.Payment
	db.Where("merchant_id = ? AND livemode = ?", m.ID, lv).Order("created_at DESC").Limit(5).Find(&recent)
	rec := []any{}
	for i := range recent {
		rec = append(rec, srv.paymentRow(&recent[i]))
	}
	var pending models.Payout
	var pendingJSON any
	if db.Where("merchant_id = ? AND livemode = ? AND status IN ?", m.ID, lv, []string{models.PayoutNew, models.PayoutReview, models.PayoutSent}).Order("requested_at").First(&pending).Error == nil {
		pendingJSON = service.PayoutObject(&pending)
	}
	var bank models.BankAccount
	var bankJSON any
	if db.First(&bank, "merchant_id = ?", m.ID).Error == nil {
		bankJSON = bankObj(&bank)
	}
	c.JSON(200, gin.H{
		"available": srv.S.Balance(m.ID, lv), "in_transit": srv.S.InTransit(m.ID, lv), "received": total, "previous": prev,
		"days": days, "bars": bars, "by_method": byMethod, "recent": rec, "pending_payout": pendingJSON, "bank_account": bankJSON,
	})
}

// paymentRow is the compact dashboard representation of a payment.
func (srv *Server) paymentRow(p *models.Payment) gin.H {
	o := service.PaymentObject(p)
	o["created_at"] = p.CreatedAt.UTC().Format(time.RFC3339)
	o["method"] = p.Method
	o["customer_name"] = p.CustomerName
	o["phone_prefix"] = p.PhonePrefix
	o["phone_last4"] = p.PhoneLast4
	o["card_brand"] = p.CardBrand
	o["card_last4"] = p.CardLast4
	if p.PaymentLinkID != nil {
		var l models.PaymentLink
		if srv.S.DB.Select("name", "slug").First(&l, "id = ?", *p.PaymentLinkID).Error == nil {
			o["link_name"] = l.Name
			o["link_slug"] = l.Slug
		}
	}
	return o
}

// ---------------------------------------------------------------- payments

func (srv *Server) paymentsQuery(c *gin.Context) *gormQuery {
	m := merchant(c)
	q := srv.S.DB.Model(&models.Payment{}).Where("merchant_id = ? AND livemode = ?", m.ID, live(c))
	switch st := c.Query("status"); st {
	case "paid":
		q = q.Where("status IN ?", []string{models.PaymentSucceeded, models.PaymentRefunded, models.PaymentPartiallyRefunded})
	case "pending":
		q = q.Where("status = ?", models.PaymentProcessing)
	case "failed":
		q = q.Where("status = ?", models.PaymentFailed)
	}
	if src := c.Query("source"); src != "" {
		q = q.Where("source_type = ?", src)
	}
	if s := strings.TrimSpace(c.Query("q")); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("LOWER(customer_name) LIKE ? OR LOWER(reference) LIKE ? OR LOWER(client_reference_id) LIKE ? OR id = ? OR phone_last4 = ?", like, like, like, s, s)
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			q = q.Where("created_at >= ?", t)
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			q = q.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}
	return q
}

func (srv *Server) listPayments(c *gin.Context) {
	m := merchant(c)
	lim := qInt(c, "limit", 50)
	if lim > 200 {
		lim = 200
	}
	off := qInt(c, "offset", 0)
	var rows []models.Payment
	var total int64
	srv.paymentsQuery(c).Count(&total)
	srv.paymentsQuery(c).Order("created_at DESC").Offset(off).Limit(lim).Find(&rows)
	out := []any{}
	for i := range rows {
		out = append(out, srv.paymentRow(&rows[i]))
	}
	counts := gin.H{}
	base := srv.S.DB.Model(&models.Payment{}).Where("merchant_id = ? AND livemode = ?", m.ID, live(c))
	var all, paid, pend, failed int64
	base.Session(&gormSession).Count(&all)
	base.Session(&gormSession).Where("status IN ?", []string{models.PaymentSucceeded, models.PaymentRefunded, models.PaymentPartiallyRefunded}).Count(&paid)
	base.Session(&gormSession).Where("status = ?", models.PaymentProcessing).Count(&pend)
	base.Session(&gormSession).Where("status = ?", models.PaymentFailed).Count(&failed)
	counts["all"], counts["paid"], counts["pending"], counts["failed"] = all, paid, pend, failed
	c.JSON(200, gin.H{"data": out, "total": total, "counts": counts})
}

func (srv *Server) getPayment(c *gin.Context) {
	m := merchant(c)
	var p models.Payment
	if err := srv.S.DB.First(&p, "id = ? AND merchant_id = ? AND livemode = ?", c.Param("id"), m.ID, live(c)).Error; err != nil {
		apiErr(c, service.NotFound("O pagamento"))
		return
	}
	o := srv.paymentRow(&p)
	var cs models.CheckoutSession
	if srv.S.DB.First(&cs, "id = ?", p.SessionID).Error == nil {
		o["session"] = gin.H{"id": cs.ID, "success_url": cs.SuccessURL, "callback_url": cs.CallbackURL, "status": cs.Status, "expires_at": cs.ExpiresAt, "created_at": cs.CreatedAt}
	}
	if p.APIKeyID != nil {
		var k models.APIKey
		if srv.S.DB.First(&k, "id = ?", *p.APIKeyID).Error == nil {
			o["api_key_label"] = map[bool]string{true: "sk_live_…", false: "sk_test_…"}[k.Livemode] + k.Last4
		}
	}
	if p.CreatedBy != "" {
		var u models.User
		if srv.S.DB.First(&u, "id = ?", p.CreatedBy).Error == nil {
			o["created_by_name"] = u.Name
		}
	}
	if p.PaymentLinkID != nil {
		var l models.PaymentLink
		if srv.S.DB.First(&l, "id = ?", *p.PaymentLinkID).Error == nil {
			o["link"] = gin.H{"id": l.ID, "name": l.Name, "custom_amount": l.CustomAmount, "reusable": l.Reusable, "type": l.Kind, "location": l.Location}
		}
	}
	// Webhook deliveries for this payment's events.
	var evs []models.Event
	srv.S.DB.Where("merchant_id = ? AND object_id IN ?", m.ID, []string{p.ID, p.SessionID}).Order("created_at").Find(&evs)
	evIDs := []string{}
	for _, e := range evs {
		evIDs = append(evIDs, e.ID)
	}
	var dels []models.WebhookDelivery
	if len(evIDs) > 0 {
		srv.S.DB.Where("event_id IN ?", evIDs).Order("created_at").Find(&dels)
	}
	d := []gin.H{}
	for _, x := range dels {
		d = append(d, deliveryJSON(&x))
	}
	o["deliveries"] = d
	var refunds []models.Refund
	srv.S.DB.Where("payment_id = ?", p.ID).Order("created_at").Find(&refunds)
	r := []any{}
	for i := range refunds {
		r = append(r, service.RefundObject(&refunds[i]))
	}
	o["refunds"] = r
	o["succeeded_at"] = iso(p.SucceededAt)
	o["failed_at"] = iso(p.FailedAt)
	o["payer_ip"] = p.PayerIP
	o["payer_user_agent"] = p.PayerUserAgent
	c.JSON(200, o)
}

func deliveryJSON(x *models.WebhookDelivery) gin.H {
	return gin.H{"id": x.ID, "event_id": x.EventID, "event_type": x.EventType, "target": x.Target, "url": x.URL, "status": x.Status,
		"attempts": x.Attempts, "last_status_code": x.LastStatusCode, "last_error": x.LastError, "duration_ms": x.LastDurationMs,
		"next_attempt_at": iso(x.NextAttemptAt), "created_at": x.CreatedAt.UTC().Format(time.RFC3339), "updated_at": x.UpdatedAt.UTC().Format(time.RFC3339)}
}

func (srv *Server) refundPayment(c *gin.Context) {
	var in struct {
		Amount int64  `json:"amount"`
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&in)
	m := merchant(c)
	r, err := srv.S.Refund(m.ID, live(c), c.Param("id"), in.Amount, in.Reason, user(c).ID)
	if err != nil {
		apiErr(c, err)
		return
	}
	srv.S.Audit(m.ID, user(c).ID, user(c).Role, "refund.created", r.ID, money.Format(r.Amount)+" MT", c.ClientIP())
	c.JSON(200, service.RefundObject(r))
}

func (srv *Server) sendReceipt(c *gin.Context) {
	// SMS / email delivery is mocked.
	c.JSON(200, gin.H{"ok": true, "message": "Recibo enviado ao cliente (simulado)."})
}

func (srv *Server) exportPayments(c *gin.Context) {
	var rows []models.Payment
	srv.paymentsQuery(c).Order("created_at DESC").Limit(10000).Find(&rows)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="riha-transacoes.csv"`)
	w := csv.NewWriter(c.Writer)
	w.Write([]string{"id", "referencia", "data", "estado", "metodo", "origem", "cliente", "client_reference_id", "valor_mzn", "taxa_mzn", "liquido_mzn", "reembolsado_mzn"})
	f := func(v int64) string { return fmt.Sprintf("%d.%02d", v/100, v%100) }
	for _, p := range rows {
		w.Write([]string{p.ID, p.Reference, p.CreatedAt.UTC().Format(time.RFC3339), p.Status, p.Method, p.SourceType, csvSafe(p.CustomerName), csvSafe(p.ClientReferenceID), f(p.Amount), f(p.Fee), f(p.Net), f(p.AmountRefunded)})
	}
	w.Flush()
}

// csvSafe neutralises spreadsheet formula injection.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// ---------------------------------------------------------------- links

func (srv *Server) linkJSON(l *models.PaymentLink, slug string) gin.H {
	o := service.LinkObject(l, srv.S.Cfg.PublicURL, slug)
	var st struct {
		Count int64
		Total int64
	}
	srv.S.DB.Model(&models.Payment{}).Where("payment_link_id = ? AND status IN ?", l.ID, []string{models.PaymentSucceeded, models.PaymentRefunded, models.PaymentPartiallyRefunded}).
		Select("COUNT(*) AS count, COALESCE(SUM(amount),0) AS total").Scan(&st)
	o["payments_count"] = st.Count
	o["total_received"] = st.Total
	o["location"] = l.Location
	return o
}

func (srv *Server) listLinks(c *gin.Context) {
	m := merchant(c)
	var rows []models.PaymentLink
	srv.S.DB.Where("merchant_id = ? AND livemode = ?", m.ID, live(c)).Order("created_at DESC").Find(&rows)
	out := []any{}
	for i := range rows {
		out = append(out, srv.linkJSON(&rows[i], m.Slug))
	}
	c.JSON(200, gin.H{"data": out})
}

type dashLinkInput struct {
	Name         *string  `json:"name"`
	Amount       *int64   `json:"amount"`
	CustomAmount *bool    `json:"custom_amount"`
	Reusable     *bool    `json:"reusable"`
	Methods      []string `json:"payment_method_types"`
	Active       *bool    `json:"active"`
	ExpiresAt    *int64   `json:"expires_at"`
	Type         string   `json:"type"`
	Location     string   `json:"location"`
}

func (srv *Server) createLink(c *gin.Context) {
	var in dashLinkInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	l, err := srv.S.CreateLink(m, live(c), service.LinkParams{Name: in.Name, Amount: in.Amount, CustomAmount: in.CustomAmount, Reusable: in.Reusable,
		Methods: in.Methods, ExpiresAt: in.ExpiresAt, Kind: in.Type, Location: in.Location}, user(c).ID, "dashboard")
	if err != nil {
		apiErr(c, err)
		return
	}
	c.JSON(200, srv.linkJSON(l, m.Slug))
}

func (srv *Server) updateLink(c *gin.Context) {
	var in dashLinkInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	m := merchant(c)
	l, err := srv.S.UpdateLink(m, live(c), c.Param("id"), service.LinkParams{Name: in.Name, Amount: in.Amount, Reusable: in.Reusable, Methods: in.Methods, Active: in.Active, ExpiresAt: in.ExpiresAt})
	if err != nil {
		apiErr(c, err)
		return
	}
	c.JSON(200, srv.linkJSON(l, m.Slug))
}

// ---------------------------------------------------------------- money

func (srv *Server) balance(c *gin.Context) {
	m := merchant(c)
	lv := live(c)
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	prevMonth := monthStart.AddDate(0, -1, 0)
	var paidPrev struct {
		Count int64
		Total int64
	}
	srv.S.DB.Model(&models.Payout{}).Where("merchant_id = ? AND livemode = ? AND status = ? AND paid_at >= ? AND paid_at < ?", m.ID, lv, models.PayoutPaid, prevMonth, monthStart).
		Select("COUNT(*) AS count, COALESCE(SUM(amount),0) AS total").Scan(&paidPrev)
	var paidThis struct {
		Count int64
		Total int64
	}
	srv.S.DB.Model(&models.Payout{}).Where("merchant_id = ? AND livemode = ? AND status = ? AND paid_at >= ?", m.ID, lv, models.PayoutPaid, monthStart).
		Select("COUNT(*) AS count, COALESCE(SUM(amount),0) AS total").Scan(&paidThis)
	var next models.Payout
	var nextDue any
	if srv.S.DB.Where("merchant_id = ? AND livemode = ? AND status IN ?", m.ID, lv, []string{models.PayoutNew, models.PayoutReview, models.PayoutSent}).Order("due_at DESC").First(&next).Error == nil {
		nextDue = next.DueAt.UTC().Format(time.RFC3339)
	}
	c.JSON(200, gin.H{"available": srv.S.Balance(m.ID, lv), "in_transit": srv.S.InTransit(m.ID, lv), "in_transit_due": nextDue,
		"paid_previous_month": paidPrev, "paid_this_month": paidThis, "min_payout": service.MinPayout, "can_payout": !lv || m.Status == models.MerchantActive})
}

func (srv *Server) listPayouts(c *gin.Context) {
	var rows []models.Payout
	srv.S.DB.Where("merchant_id = ? AND livemode = ?", merchant(c).ID, live(c)).Order("requested_at DESC").Limit(100).Find(&rows)
	out := []any{}
	for i := range rows {
		o := service.PayoutObject(&rows[i])
		o["requested_at_iso"] = rows[i].RequestedAt.UTC().Format(time.RFC3339)
		out = append(out, o)
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
	p, err := srv.S.RequestPayout(m.ID, live(c), in.Amount, user(c).ID)
	if err != nil {
		apiErr(c, err)
		return
	}
	srv.S.Audit(m.ID, user(c).ID, user(c).Role, "payout.requested", p.ID, money.Format(p.Amount)+" MT", c.ClientIP())
	c.JSON(200, service.PayoutObject(p))
}

var Banks = []string{"BCI", "Millennium bim", "Standard Bank", "Absa", "Moza Banco", "FNB", "Access Bank", "Nedbank", "Ecobank", "Letshego", "Banco Único"}

func bankObj(b *models.BankAccount) gin.H {
	return gin.H{"bank_name": b.BankName, "holder": b.Holder, "nib": b.NIB, "nib_last5": b.NIB[len(b.NIB)-5:], "verified": b.Verified, "document_id": b.DocumentID, "updated_at": b.UpdatedAt}
}

func (srv *Server) getBank(c *gin.Context) {
	var b models.BankAccount
	out := gin.H{"banks": Banks, "account": nil}
	if srv.S.DB.First(&b, "merchant_id = ?", merchant(c).ID).Error == nil {
		ba := bankObj(&b)
		if b.DocumentID != nil {
			var d models.KycDocument
			if srv.S.DB.First(&d, "id = ?", *b.DocumentID).Error == nil {
				ba["document"] = gin.H{"file_name": d.FileName, "size": d.Size}
			}
		}
		out["account"] = ba
	}
	c.JSON(200, out)
}

func (srv *Server) putBank(c *gin.Context) {
	var in struct {
		BankName string `json:"bank_name"`
		Holder   string `json:"holder"`
		NIB      string `json:"nib"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	nib := strings.Join(strings.Fields(in.NIB), "")
	if !service.ValidNIB(nib) {
		apiErr(c, service.Invalid("nib_invalid", "nib", "O NIB tem 21 dígitos."))
		return
	}
	okBank := false
	for _, b := range Banks {
		if b == in.BankName {
			okBank = true
		}
	}
	if !okBank {
		apiErr(c, service.Invalid("bank_invalid", "bank_name", "Escolha um banco da lista."))
		return
	}
	if strings.TrimSpace(in.Holder) == "" {
		apiErr(c, service.Invalid("parameter_missing", "holder", "Indique o titular da conta."))
		return
	}
	m := merchant(c)
	var b models.BankAccount
	exists := srv.S.DB.First(&b, "merchant_id = ?", m.ID).Error == nil
	changed := !exists || b.NIB != nib
	if !exists {
		b = models.BankAccount{ID: ids.New("ba"), MerchantID: m.ID}
		var d models.KycDocument
		if srv.S.DB.First(&d, "merchant_id = ? AND kind = ?", m.ID, "nib_proof").Error == nil {
			b.DocumentID = &d.ID
		}
	}
	b.BankName, b.Holder, b.NIB = in.BankName, strings.TrimSpace(in.Holder), nib
	if changed {
		// A new NIB must be verified again by operations; active accounts keep
		// paying out (mock) but the change is audited.
		b.Verified = m.Status == models.MerchantActive
	}
	if err := srv.S.DB.Save(&b).Error; err != nil {
		apiErr(c, err)
		return
	}
	if changed {
		srv.S.Audit(m.ID, user(c).ID, user(c).Role, "bank_account.changed", b.ID, b.BankName+" •••• "+nib[16:], c.ClientIP())
	}
	c.JSON(200, gin.H{"account": bankObj(&b), "banks": Banks})
}

// ---------------------------------------------------------------- developers

func (srv *Server) getKeys(c *gin.Context) {
	m := merchant(c)
	lv := live(c)
	var pk models.APIKey
	srv.S.DB.Where("merchant_id = ? AND livemode = ? AND kind = ? AND revoked_at IS NULL", m.ID, lv, models.KeyPublishable).First(&pk)
	var sk models.APIKey
	var skJSON any
	if srv.S.DB.Where("merchant_id = ? AND livemode = ? AND kind = ? AND revoked_at IS NULL", m.ID, lv, models.KeySecret).First(&sk).Error == nil {
		skJSON = gin.H{"id": sk.ID, "last4": sk.Last4, "created_at": sk.CreatedAt, "last_used_at": iso(sk.LastUsedAt)}
	}
	c.JSON(200, gin.H{"publishable": pk.Token, "secret": skJSON, "livemode": lv})
}

func (srv *Server) rollKey(c *gin.Context) {
	m := merchant(c)
	lv := live(c)
	if lv && m.Status != models.MerchantActive {
		apiErr(c, service.Forbidden("account_not_verified", "As chaves de produção ficam disponíveis depois da verificação da conta. Use o modo de teste."))
		return
	}
	secret, k, err := srv.S.RollSecretKey(m.ID, lv, user(c).ID)
	if err != nil {
		apiErr(c, err)
		return
	}
	srv.S.Audit(m.ID, user(c).ID, user(c).Role, "api_key.rolled", k.ID, map[bool]string{true: "live", false: "test"}[lv], c.ClientIP())
	c.JSON(200, gin.H{"secret_key": secret, "id": k.ID, "last4": k.Last4, "created_at": k.CreatedAt})
}

func (srv *Server) apiLogs(c *gin.Context) {
	var rows []models.APIRequestLog
	srv.S.DB.Where("merchant_id = ? AND livemode = ?", merchant(c).ID, live(c)).Order("created_at DESC").Limit(20).Find(&rows)
	out := []gin.H{}
	for _, r := range rows {
		out = append(out, gin.H{"method": r.Method, "path": r.Path, "status": r.Status, "created_at": r.CreatedAt, "duration_ms": r.DurationMs, "request_id": r.RequestID})
	}
	c.JSON(200, gin.H{"data": out})
}

func (srv *Server) devSettings(c *gin.Context) {
	m := merchant(c)
	lv := live(c)
	var eps []models.WebhookEndpoint
	srv.S.DB.Where("merchant_id = ? AND livemode = ?", m.ID, lv).Order("created_at").Find(&eps)
	e := []any{}
	for i := range eps {
		e = append(e, service.EndpointObject(&eps[i]))
	}
	var doms []models.AllowedDomain
	srv.S.DB.Where("merchant_id = ?", m.ID).Order("created_at").Find(&doms)
	d := []gin.H{}
	for _, x := range doms {
		d = append(d, gin.H{"id": x.ID, "domain": x.Domain, "verified": x.VerifiedAt != nil})
	}
	sec := m.SigningSecret(lv)
	c.JSON(200, gin.H{"success_url": m.SuccessURL, "cancel_url": m.CancelURL, "endpoints": e, "domains": d,
		"secret_hint": "whsec_••••••••" + sec[len(sec)-4:], "event_types": service.AllEventTypes})
}

func (srv *Server) revealSecret(c *gin.Context) {
	c.JSON(200, gin.H{"secret": merchant(c).SigningSecret(live(c))})
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
	for _, u := range []string{in.SuccessURL, in.CancelURL} {
		if u != "" && !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
			bad(c, "Os URLs têm de começar por https://")
			return
		}
	}
	m := merchant(c)
	m.SuccessURL, m.CancelURL = strings.TrimSpace(in.SuccessURL), strings.TrimSpace(in.CancelURL)
	srv.S.DB.Save(m)
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
	ep, err := srv.S.CreateEndpoint(m.ID, live(c), in.URL, in.Events)
	if err != nil {
		apiErr(c, err)
		return
	}
	srv.S.Audit(m.ID, user(c).ID, user(c).Role, "webhook_endpoint.created", ep.ID, ep.URL, c.ClientIP())
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
	var ep models.WebhookEndpoint
	if err := srv.S.DB.First(&ep, "id = ? AND merchant_id = ?", c.Param("id"), merchant(c).ID).Error; err != nil {
		apiErr(c, service.NotFound("O endpoint"))
		return
	}
	if in.Events != nil {
		evs := models.StringList{}
		for _, e := range in.Events {
			for _, a := range service.AllEventTypes {
				if a == e && !evs.Has(e) {
					evs = append(evs, e)
				}
			}
		}
		ep.Events = evs
	}
	if in.Active != nil {
		ep.Active = *in.Active
	}
	srv.S.DB.Save(&ep)
	srv.devSettings(c)
}

func (srv *Server) deleteEndpoint(c *gin.Context) {
	srv.S.DB.Where("id = ? AND merchant_id = ?", c.Param("id"), merchant(c).ID).Delete(&models.WebhookEndpoint{})
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
	if _, err := srv.S.AddDomain(m.ID, in.Domain); err != nil {
		apiErr(c, err)
		return
	}
	srv.S.Audit(m.ID, user(c).ID, user(c).Role, "domain.added", in.Domain, "", c.ClientIP())
	srv.devSettings(c)
}

func (srv *Server) deleteDomain(c *gin.Context) {
	srv.S.DB.Where("id = ? AND merchant_id = ?", c.Param("id"), merchant(c).ID).Delete(&models.AllowedDomain{})
	srv.devSettings(c)
}

func (srv *Server) listDeliveries(c *gin.Context) {
	var rows []models.WebhookDelivery
	srv.S.DB.Where("merchant_id = ? AND livemode = ?", merchant(c).ID, live(c)).Order("created_at DESC").Limit(30).Find(&rows)
	out := []gin.H{}
	for i := range rows {
		out = append(out, deliveryJSON(&rows[i]))
	}
	c.JSON(200, gin.H{"data": out})
}

func (srv *Server) resendDelivery(c *gin.Context) {
	if err := srv.S.Resend(merchant(c).ID, c.Param("id")); err != nil {
		apiErr(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (srv *Server) testEvent(c *gin.Context) {
	m := merchant(c)
	var n int64
	srv.S.DB.Model(&models.WebhookEndpoint{}).Where("merchant_id = ? AND livemode = ? AND active = ?", m.ID, live(c), true).Count(&n)
	if n == 0 {
		bad(c, "Adicione um endpoint de webhook primeiro.")
		return
	}
	if err := srv.S.EmitTest(m.ID, live(c)); err != nil {
		apiErr(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
