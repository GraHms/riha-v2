package service

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"riha/internal/ids"
	"riha/internal/models"
	"riha/internal/money"
	"riha/internal/providers"
)

const (
	MinAmount = 1000        // 10,00 MT
	MaxAmount = 100_000_000 // 1 000 000,00 MT
)

type Customer struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// SessionParams are the inputs to create a checkout session from any source.
type SessionParams struct {
	Amount             int64             `json:"amount"`
	Currency           string            `json:"currency"`
	Description        string            `json:"description"`
	SuccessURL         string            `json:"success_url"`
	CancelURL          string            `json:"cancel_url"`
	CallbackURL        string            `json:"callback_url"`
	ClientReferenceID  string            `json:"client_reference_id"`
	Customer           *Customer         `json:"customer"`
	LineItems          []models.LineItem `json:"line_items"`
	PaymentMethodTypes []string          `json:"payment_method_types"`
	Metadata           map[string]any    `json:"metadata"`
	ExpiresAt          *int64            `json:"expires_at"`
	Locale             string            `json:"locale"`

	// Set by the server, never by clients.
	SourceType    string  `json:"-"`
	PaymentLinkID *string `json:"-"`
	APIKeyID      *string `json:"-"`
	OriginDomain  string  `json:"-"`
	OriginIP      string  `json:"-"`
	OriginUA      string  `json:"-"`
	Channel       string  `json:"-"`
	CreatedBy     string  `json:"-"`
}

var e164MZ = regexp.MustCompile(`^\+258[0-9]{9}$`)

// CanAcceptPayments reports whether a merchant may take money in this mode.
func CanAcceptPayments(m *models.Merchant, live bool) *Error {
	switch m.Status {
	case models.MerchantSuspended:
		return Forbidden("account_suspended", "Esta conta está suspensa. Contacte o suporte da Riha.")
	case models.MerchantRejected:
		if live {
			return Forbidden("account_rejected", "A verificação desta conta foi rejeitada.")
		}
	}
	return nil
}

func validURL(raw, param string, live, required bool) (string, *Error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if required {
			return "", Invalid("parameter_missing", param, fmt.Sprintf("Falta o parâmetro %s.", param))
		}
		return "", nil
	}
	u, err := url.Parse(strings.ReplaceAll(raw, "{CHECKOUT_SESSION_ID}", "x"))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", Invalid("url_invalid", param, fmt.Sprintf("%s tem de ser um URL http(s) válido.", param))
	}
	if live && u.Scheme != "https" && !isLocal(u.Hostname()) {
		return "", Invalid("url_invalid", param, fmt.Sprintf("Em produção %s tem de usar https.", param))
	}
	if len(raw) > 1000 {
		return "", Invalid("url_invalid", param, param+" é demasiado longo.")
	}
	return raw, nil
}

func isLocal(h string) bool { return h == "localhost" || h == "127.0.0.1" || h == "::1" }

// DomainAllowed checks host against the merchant's verified domains
// (exact match or subdomain).
func (s *Service) DomainAllowed(merchantID, host string, live bool) bool {
	host = strings.ToLower(host)
	if !live && isLocal(host) {
		return true
	}
	var doms []models.AllowedDomain
	s.DB.Where("merchant_id = ? AND verified_at IS NOT NULL", merchantID).Find(&doms)
	for _, d := range doms {
		if host == d.Domain || strings.HasSuffix(host, "."+d.Domain) {
			return true
		}
	}
	return false
}

func (s *Service) CreateSession(m *models.Merchant, live bool, p SessionParams) (*models.CheckoutSession, error) {
	if e := CanAcceptPayments(m, live); e != nil {
		return nil, e
	}
	if p.Currency == "" {
		p.Currency = "MZN"
	}
	if strings.ToUpper(p.Currency) != "MZN" {
		return nil, Invalid("currency_unsupported", "currency", "Só aceitamos MZN.")
	}
	if p.Amount < MinAmount {
		return nil, Invalid("amount_too_small", "amount", "O valor mínimo é 1000 centavos (10,00 MT).")
	}
	if p.Amount > MaxAmount {
		return nil, Invalid("amount_too_large", "amount", "O valor máximo por pagamento é 1 000 000,00 MT.")
	}
	var e *Error
	if p.SuccessURL, e = validURL(p.SuccessURL, "success_url", live, p.SourceType == models.SourceAPI); e != nil {
		return nil, e
	}
	if p.CancelURL, e = validURL(p.CancelURL, "cancel_url", live, false); e != nil {
		return nil, e
	}
	if p.CallbackURL, e = validURL(p.CallbackURL, "callback_url", live, false); e != nil {
		return nil, e
	}
	if p.CallbackURL != "" {
		u, _ := url.Parse(p.CallbackURL)
		if !s.DomainAllowed(m.ID, u.Hostname(), live) {
			return nil, Invalid("callback_url_not_allowed", "callback_url", fmt.Sprintf("O domínio %s não está verificado.", u.Hostname()))
		}
	}
	if len(p.LineItems) > 0 {
		var sum int64
		for i, li := range p.LineItems {
			if strings.TrimSpace(li.Name) == "" || li.Quantity <= 0 || li.UnitAmount < 0 {
				return nil, Invalid("line_items_invalid", fmt.Sprintf("line_items[%d]", i), "Cada item precisa de name, quantity > 0 e unit_amount.")
			}
			sum += li.Quantity * li.UnitAmount
		}
		if sum != p.Amount {
			return nil, Invalid("line_items_mismatch", "line_items", fmt.Sprintf("A soma dos itens (%d) tem de ser igual a amount (%d).", sum, p.Amount))
		}
	}
	meta := models.JSONMap{}
	if len(p.Metadata) > 20 {
		return nil, Invalid("metadata_invalid", "metadata", "Máximo de 20 pares em metadata.")
	}
	for k, v := range p.Metadata {
		vs := fmt.Sprint(v)
		if len(k) > 40 || len(vs) > 500 {
			return nil, Invalid("metadata_invalid", "metadata", "Chaves até 40 caracteres e valores até 500.")
		}
		meta[k] = vs
	}
	methods := models.StringList{}
	for _, mt := range p.PaymentMethodTypes {
		mt = strings.ToLower(strings.TrimSpace(mt))
		ok := false
		for _, a := range providers.AllMethods {
			if a == mt {
				ok = true
			}
		}
		if !ok {
			return nil, Invalid("payment_method_invalid", "payment_method_types", fmt.Sprintf("Método %q desconhecido. Use mpesa, emola ou card.", mt))
		}
		if !methods.Has(mt) {
			methods = append(methods, mt)
		}
	}
	if len(methods) == 0 {
		methods = append(methods, providers.AllMethods...)
	}
	now := s.Now()
	exp := now.Add(time.Hour)
	if p.ExpiresAt != nil {
		exp = time.Unix(*p.ExpiresAt, 0).UTC()
		if exp.Before(now.Add(15*time.Minute-time.Second)) || exp.After(now.Add(24*time.Hour+time.Second)) {
			return nil, Invalid("expires_at_invalid", "expires_at", "expires_at tem de estar entre 15 minutos e 24 horas.")
		}
	}
	cs := &models.CheckoutSession{
		ID: ids.Moded("cs", live), MerchantID: m.ID, Livemode: live, Status: models.SessionOpen,
		Amount: p.Amount, Currency: "MZN", Description: trim(p.Description, 500),
		SuccessURL: p.SuccessURL, CancelURL: p.CancelURL, CallbackURL: p.CallbackURL,
		ClientReferenceID: trim(p.ClientReferenceID, 200), LineItems: p.LineItems, Metadata: meta,
		PaymentMethodTypes: methods, Locale: orDefault(p.Locale, "pt-MZ"), ExpiresAt: exp,
		SourceType: p.SourceType, PaymentLinkID: p.PaymentLinkID, APIKeyID: p.APIKeyID,
		OriginDomain: p.OriginDomain, OriginIP: p.OriginIP, OriginUserAgent: trim(p.OriginUA, 300),
		Channel: p.Channel, CreatedBy: p.CreatedBy,
	}
	if p.Customer != nil {
		cs.CustomerName = trim(p.Customer.Name, 200)
		cs.CustomerEmail = trim(p.Customer.Email, 200)
		if ph := strings.TrimSpace(p.Customer.Phone); ph != "" {
			// Accept +258XXXXXXXXX, 258XXXXXXXXX (a form-encoded "+" arrives as a space) or 9 local digits.
			d := providers.NormalizeMZPhone(ph)
			ph = "+258" + d
			if !e164MZ.MatchString(ph) {
				return nil, Invalid("customer_phone_invalid", "customer.phone", "Use o formato E.164, por exemplo +258841234567.")
			}
			cs.CustomerPhone = ph
		}
	}
	if err := s.DB.Create(cs).Error; err != nil {
		return nil, err
	}
	return cs, nil
}

// CreateLinkSession opens a checkout session for a payment link visit.
func (s *Service) CreateLinkSession(link *models.PaymentLink, amount int64, channel, ip, ua string) (*models.CheckoutSession, error) {
	var m models.Merchant
	if err := s.DB.First(&m, "id = ?", link.MerchantID).Error; err != nil {
		return nil, err
	}
	if !link.Active || (link.ExpiresAt != nil && link.ExpiresAt.Before(s.Now())) {
		return nil, Invalid("link_inactive", "link", "Este link já não está ativo.")
	}
	if !link.Reusable {
		var n int64
		s.DB.Model(&models.Payment{}).Where("payment_link_id = ? AND status IN ?", link.ID, []string{models.PaymentSucceeded, models.PaymentRefunded, models.PaymentPartiallyRefunded}).Count(&n)
		if n > 0 {
			return nil, Invalid("link_used", "link", "Este link já foi pago.")
		}
	}
	if !link.CustomAmount {
		amount = link.Amount
	}
	src := models.SourceLink
	if link.Kind == "qr" {
		src = models.SourceQR
		channel = "qr"
	}
	lid := link.ID
	return s.CreateSession(&m, link.Livemode, SessionParams{
		Amount: amount, Description: link.Name, PaymentMethodTypes: link.Methods,
		SuccessURL: m.SuccessURL, CancelURL: m.CancelURL,
		SourceType: src, PaymentLinkID: &lid, OriginIP: ip, OriginUA: ua, Channel: channel, CreatedBy: link.CreatedBy,
	})
}

// PayInput is what the payer submits on the hosted checkout.
type PayInput struct {
	Method     string `json:"method"`
	Phone      string `json:"phone"`
	CardNumber string `json:"card_number"`
	CardExpiry string `json:"card_expiry"`
	CardCVC    string `json:"card_cvc"`
	Name       string `json:"name"`
	PayerIP    string `json:"-"`
	PayerUA    string `json:"-"`
}

// Pay starts a payment attempt for an open session.
func (s *Service) Pay(sessionID string, in PayInput) (*models.Payment, error) {
	var cs models.CheckoutSession
	if err := s.DB.First(&cs, "id = ?", sessionID).Error; err != nil {
		return nil, NotFound("A sessão")
	}
	if cs.Status != models.SessionOpen || cs.ExpiresAt.Before(s.Now()) {
		return nil, Invalid("session_not_open", "session", "Esta sessão de pagamento já não está aberta.")
	}
	in.Method = strings.ToLower(in.Method)
	if !cs.PaymentMethodTypes.Has(in.Method) {
		return nil, Invalid("payment_method_not_allowed", "method", "Este método não está disponível neste pagamento.")
	}
	var m models.Merchant
	if err := s.DB.First(&m, "id = ?", cs.MerchantID).Error; err != nil {
		return nil, err
	}
	if e := CanAcceptPayments(&m, cs.Livemode); e != nil {
		return nil, e
	}
	var inflight int64
	s.DB.Model(&models.Payment{}).Where("session_id = ? AND status = ?", cs.ID, models.PaymentProcessing).Count(&inflight)
	if inflight > 0 {
		return nil, Conflict("payment_in_progress", "Já existe um pagamento em curso. Confirme no telemóvel ou aguarde.")
	}
	bps := s.FeeBpsFor(&m)
	fee, net := money.Split(cs.Amount, bps)
	p := &models.Payment{
		ID: ids.New("pay"), Reference: ids.PaymentReference(), MerchantID: m.ID, Livemode: cs.Livemode, SessionID: cs.ID,
		Status: models.PaymentProcessing, Amount: cs.Amount, FeeBps: bps, Fee: fee, Net: net, Method: in.Method,
		CustomerName: firstNonEmpty(trim(in.Name, 200), cs.CustomerName), CustomerEmail: cs.CustomerEmail,
		Description: cs.Description, ClientReferenceID: cs.ClientReferenceID, Metadata: cs.Metadata,
		SourceType: cs.SourceType, PaymentLinkID: cs.PaymentLinkID, APIKeyID: cs.APIKeyID, OriginDomain: cs.OriginDomain,
		OriginIP: cs.OriginIP, OriginUserAgent: cs.OriginUserAgent, Channel: cs.Channel, CreatedBy: cs.CreatedBy,
		PayerIP: in.PayerIP, PayerUserAgent: trim(in.PayerUA, 300),
	}
	if in.Method != providers.MethodCard {
		ph := providers.NormalizeMZPhone(in.Phone)
		if len(ph) == 9 {
			p.PhonePrefix, p.PhoneLast4 = ph[:2], ph[5:]
		}
	}
	// Validate input with the provider before persisting anything: create the
	// row first so the async callback can always find it.
	if err := s.DB.Create(p).Error; err != nil {
		return nil, err
	}
	res, err := s.Provider.Initiate(providers.ChargeRequest{
		PaymentID: p.ID, Method: in.Method, Amount: p.Amount, Phone: in.Phone,
		CardNumber: in.CardNumber, CardExpiry: in.CardExpiry, CardCVC: in.CardCVC, Descriptor: m.StatementDescriptor,
	}, func(o providers.Outcome) {
		if err := s.CompletePayment(p.ID, o); err != nil {
			log.Printf("complete payment %s: %v", p.ID, err)
		}
	})
	if err != nil {
		s.DB.Delete(p)
		var ve *providers.ValidationError
		if errors.As(err, &ve) {
			return nil, Invalid("payment_details_invalid", ve.Field, ve.Msg)
		}
		return nil, err
	}
	if in.Method == providers.MethodCard {
		s.DB.Model(p).Updates(map[string]any{"card_brand": res.Brand, "card_last4": res.Last4})
		p.CardBrand, p.CardLast4 = res.Brand, res.Last4
	}
	if res.Async {
		s.DB.Transaction(func(tx *gorm.DB) error {
			return s.emit(tx, m.ID, p.Livemode, "payment.processing", p.ID, PaymentObject(p), "")
		})
	} else {
		if err := s.CompletePayment(p.ID, res.Outcome); err != nil {
			return nil, err
		}
	}
	s.DB.First(p, "id = ?", p.ID)
	return p, nil
}

// CompletePayment records the provider's final answer. Idempotent.
func (s *Service) CompletePayment(paymentID string, o providers.Outcome) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var p models.Payment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&p, "id = ?", paymentID).Error; err != nil {
			return err
		}
		if p.Status != models.PaymentProcessing {
			return nil
		}
		if _, err := lockMerchant(tx, p.MerchantID); err != nil {
			return err
		}
		var cs models.CheckoutSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&cs, "id = ?", p.SessionID).Error; err != nil {
			return err
		}
		now := s.Now()
		if o.Success {
			p.Status, p.OperatorReference, p.SucceededAt = models.PaymentSucceeded, o.OperatorReference, &now
			if err := tx.Save(&p).Error; err != nil {
				return err
			}
			if err := ledger(tx, p.MerchantID, p.Livemode, models.LedgerPayment, p.Net, p.ID, "Pagamento "+p.Reference); err != nil {
				return err
			}
			cs.Status, cs.PaymentID, cs.CompletedAt = models.SessionComplete, &p.ID, &now
			if err := tx.Save(&cs).Error; err != nil {
				return err
			}
			if err := s.emit(tx, p.MerchantID, p.Livemode, "payment.succeeded", p.ID, PaymentObject(&p), cs.CallbackURL); err != nil {
				return err
			}
			return s.emit(tx, p.MerchantID, p.Livemode, "checkout.session.completed", cs.ID, SessionObject(&cs, s.Cfg.PublicURL), cs.CallbackURL)
		}
		p.Status, p.FailureCode, p.FailedAt = models.PaymentFailed, o.FailureCode, &now
		p.FailureMessage = providers.FailureMessages[o.FailureCode]
		if err := tx.Save(&p).Error; err != nil {
			return err
		}
		return s.emit(tx, p.MerchantID, p.Livemode, "payment.failed", p.ID, PaymentObject(&p), cs.CallbackURL)
	})
}

// ExpireSession closes an open session (API or sweeper).
func (s *Service) ExpireSession(merchantID string, live bool, id string) (*models.CheckoutSession, error) {
	var out models.CheckoutSession
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id)
		if merchantID != "" {
			q = q.Where("merchant_id = ? AND livemode = ?", merchantID, live)
		}
		if err := q.First(&out).Error; err != nil {
			return NotFound("A sessão")
		}
		if out.Status != models.SessionOpen {
			return Invalid("session_not_open", "id", "Só sessões abertas podem expirar.")
		}
		var inflight int64
		tx.Model(&models.Payment{}).Where("session_id = ? AND status = ?", out.ID, models.PaymentProcessing).Count(&inflight)
		if inflight > 0 {
			return Conflict("payment_in_progress", "Há um pagamento em curso nesta sessão.")
		}
		out.Status = models.SessionExpired
		if err := tx.Save(&out).Error; err != nil {
			return err
		}
		return s.emit(tx, out.MerchantID, out.Livemode, "checkout.session.expired", out.ID, SessionObject(&out, s.Cfg.PublicURL), out.CallbackURL)
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func trim(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func firstNonEmpty(v ...string) string {
	for _, x := range v {
		if x != "" {
			return x
		}
	}
	return ""
}
