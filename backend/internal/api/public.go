package api

import (
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/models"
	"riha/internal/service"
)

func initials(name string) string {
	name = strings.SplitN(name, ",", 2)[0]
	out := []rune{}
	for _, w := range strings.Fields(name) {
		r := []rune(w)
		if len(r) > 0 && strings.ToUpper(string(r[0])) == string(r[0]) && len(out) < 2 {
			out = append(out, r[0])
		}
	}
	if len(out) == 0 && len(name) > 0 {
		out = append(out, []rune(strings.ToUpper(name))[0])
	}
	return string(out)
}

func publicMerchant(m *models.Merchant) gin.H {
	return gin.H{"name": m.Name, "initials": initials(m.Name), "avatar_color": m.AvatarColor, "descriptor": m.StatementDescriptor, "slug": m.Slug}
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		return strings.TrimPrefix(u.Hostname(), "www.")
	}
	return ""
}

func (srv *Server) sessionView(cs *models.CheckoutSession) gin.H {
	var m models.Merchant
	srv.S.DB.First(&m, "id = ?", cs.MerchantID)
	open := cs.Status == models.SessionOpen && cs.ExpiresAt.After(time.Now())
	status := cs.Status
	if cs.Status == models.SessionOpen && !open {
		status = models.SessionExpired
	}
	success := strings.ReplaceAll(cs.SuccessURL, "{CHECKOUT_SESSION_ID}", cs.ID)
	if success != "" && !strings.Contains(cs.SuccessURL, "{CHECKOUT_SESSION_ID}") {
		sep := "?"
		if strings.Contains(success, "?") {
			sep = "&"
		}
		success += sep + "session_id=" + cs.ID
	}
	out := gin.H{
		"id": cs.ID, "status": status, "amount": cs.Amount, "currency": cs.Currency, "description": cs.Description,
		"line_items": cs.LineItems, "payment_method_types": cs.PaymentMethodTypes, "livemode": cs.Livemode,
		"success_url": success, "cancel_url": cs.CancelURL, "return_host": hostOf(firstNonEmpty(cs.SuccessURL, cs.CancelURL)),
		"client_reference_id": cs.ClientReferenceID, "customer_phone": strings.TrimPrefix(cs.CustomerPhone, "+258"),
		"customer_name": cs.CustomerName, "expires_at": cs.ExpiresAt.UTC().Format(time.RFC3339), "merchant": publicMerchant(&m),
		"payment": nil,
	}
	if out["line_items"] == nil || len(cs.LineItems) == 0 {
		out["line_items"] = []any{}
	}
	var p models.Payment
	if srv.S.DB.Where("session_id = ?", cs.ID).Order("created_at DESC").First(&p).Error == nil {
		out["payment"] = gin.H{"id": p.ID, "reference": p.Reference, "status": p.Status, "method": p.Method, "phone_prefix": p.PhonePrefix,
			"phone_last4": p.PhoneLast4, "card_brand": p.CardBrand, "card_last4": p.CardLast4, "failure_code": p.FailureCode,
			"failure_message": p.FailureMessage, "created_at": p.CreatedAt.UTC().Format(time.RFC3339), "succeeded_at": iso(p.SucceededAt)}
	}
	return out
}

func firstNonEmpty(v ...string) string {
	for _, x := range v {
		if x != "" {
			return x
		}
	}
	return ""
}

func (srv *Server) publicSession(c *gin.Context) {
	var cs models.CheckoutSession
	if err := srv.S.DB.First(&cs, "id = ?", c.Param("id")).Error; err != nil {
		apiErr(c, service.NotFound("A sessão"))
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, srv.sessionView(&cs))
}

func (srv *Server) publicPay(c *gin.Context) {
	var in service.PayInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	in.PayerIP, in.PayerUA = c.ClientIP(), c.GetHeader("User-Agent")
	if _, err := srv.S.Pay(c.Param("id"), in); err != nil {
		apiErr(c, err)
		return
	}
	var cs models.CheckoutSession
	srv.S.DB.First(&cs, "id = ?", c.Param("id"))
	c.JSON(200, srv.sessionView(&cs))
}

func (srv *Server) publicCancel(c *gin.Context) {
	var cs models.CheckoutSession
	if err := srv.S.DB.First(&cs, "id = ?", c.Param("id")).Error; err != nil {
		apiErr(c, service.NotFound("A sessão"))
		return
	}
	c.JSON(200, gin.H{"cancel_url": cs.CancelURL})
}

func (srv *Server) findLink(c *gin.Context) (*models.PaymentLink, *models.Merchant, bool) {
	var m models.Merchant
	if err := srv.S.DB.First(&m, "slug = ?", c.Param("merchant")).Error; err != nil {
		apiErr(c, service.NotFound("O link"))
		return nil, nil, false
	}
	var l models.PaymentLink
	if err := srv.S.DB.First(&l, "merchant_id = ? AND slug = ?", m.ID, c.Param("slug")).Error; err != nil {
		apiErr(c, service.NotFound("O link"))
		return nil, nil, false
	}
	return &l, &m, true
}

func (srv *Server) publicLink(c *gin.Context) {
	l, m, ok := srv.findLink(c)
	if !ok {
		return
	}
	active := l.Active && (l.ExpiresAt == nil || l.ExpiresAt.After(time.Now()))
	c.JSON(200, gin.H{"name": l.Name, "amount": l.Amount, "custom_amount": l.CustomAmount, "active": active, "type": l.Kind,
		"payment_method_types": l.Methods, "livemode": l.Livemode, "merchant": publicMerchant(m)})
}

func (srv *Server) publicLinkSession(c *gin.Context) {
	l, _, ok := srv.findLink(c)
	if !ok {
		return
	}
	var in struct {
		Amount  int64  `json:"amount"`
		Channel string `json:"channel"`
	}
	c.ShouldBindJSON(&in)
	ch := map[string]bool{"whatsapp": true, "sms": true, "qr": true, "outro": true}
	if !ch[in.Channel] {
		in.Channel = "outro"
	}
	cs, err := srv.S.CreateLinkSession(l, in.Amount, in.Channel, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		apiErr(c, err)
		return
	}
	c.JSON(200, gin.H{"id": cs.ID})
}

func (srv *Server) publicReceipt(c *gin.Context) {
	var p models.Payment
	if err := srv.S.DB.First(&p, "reference = ? AND status IN ?", c.Param("ref"), []string{models.PaymentSucceeded, models.PaymentRefunded, models.PaymentPartiallyRefunded}).Error; err != nil {
		apiErr(c, service.NotFound("O recibo"))
		return
	}
	var m models.Merchant
	srv.S.DB.First(&m, "id = ?", p.MerchantID)
	c.JSON(200, gin.H{"reference": p.Reference, "amount": p.Amount, "method": p.Method, "phone_last4": p.PhoneLast4, "card_last4": p.CardLast4,
		"description": p.Description, "merchant": publicMerchant(&m), "paid_at": iso(p.SucceededAt), "status": p.Status})
}
