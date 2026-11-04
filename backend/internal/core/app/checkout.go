package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"riha/internal/core/domain"
)

type Customer struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

type SessionParams struct {
	Amount             int64             `json:"amount"`
	Currency           string            `json:"currency"`
	Description        string            `json:"description"`
	SuccessURL         string            `json:"success_url"`
	CancelURL          string            `json:"cancel_url"`
	CallbackURL        string            `json:"callback_url"`
	ClientReferenceID  string            `json:"client_reference_id"`
	Customer           *Customer         `json:"customer"`
	LineItems          []domain.LineItem `json:"line_items"`
	PaymentMethodTypes []string          `json:"payment_method_types"`
	Metadata           map[string]any    `json:"metadata"`
	ExpiresAt          *int64            `json:"expires_at"`
	Locale             string            `json:"locale"`
	Origin             domain.Origin     `json:"-"`
}

var e164MZ = regexp.MustCompile(`^\+258[0-9]{9}$`)

const MaxAttemptsPerSession = 10

func isLocal(h string) bool { return h == "localhost" || h == "127.0.0.1" || h == "::1" }

func validURL(raw, param string, live, required bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if required {
			return "", domain.Invalid("parameter_missing", param, fmt.Sprintf("Falta o parâmetro %s.", param))
		}
		return "", nil
	}
	u, err := url.Parse(strings.ReplaceAll(raw, "{CHECKOUT_SESSION_ID}", "x"))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", domain.Invalid("url_invalid", param, fmt.Sprintf("%s tem de ser um URL http(s) válido.", param))
	}
	if live && u.Scheme != "https" && !isLocal(u.Hostname()) {
		return "", domain.Invalid("url_invalid", param, fmt.Sprintf("Em produção %s tem de usar https.", param))
	}
	if len(raw) > 1000 {
		return "", domain.Invalid("url_invalid", param, param+" é demasiado longo.")
	}
	return raw, nil
}

func (a *App) DomainAllowed(ctx context.Context, merchantID, host string, live bool) bool {
	host = strings.ToLower(host)
	if !live && a.AllowPrivateNetworks && isLocal(host) {
		return true
	}
	doms, _ := a.Domains.List(ctx, merchantID)
	for _, d := range doms {
		if d.VerifiedAt != nil && (host == d.Domain || strings.HasSuffix(host, "."+d.Domain)) {
			return true
		}
	}
	return false
}

func (a *App) CreateSession(ctx context.Context, m *domain.Merchant, live bool, p SessionParams) (*domain.CheckoutSession, error) {
	if e := domain.RequireApproved(m); e != nil {
		return nil, e
	}
	if p.Currency == "" {
		p.Currency = "MZN"
	}
	if strings.ToUpper(p.Currency) != "MZN" {
		return nil, domain.Invalid("currency_unsupported", "currency", "Só aceitamos MZN.")
	}
	if p.Amount < domain.MinAmount {
		return nil, domain.Invalid("amount_too_small", "amount", "O valor mínimo é 1000 centavos (10,00 MT).")
	}
	if p.Amount > domain.MaxAmount {
		return nil, domain.Invalid("amount_too_large", "amount", "O valor máximo por pagamento é 1 000 000,00 MT.")
	}
	var err error
	if p.SuccessURL, err = validURL(p.SuccessURL, "success_url", live, p.Origin.SourceType == domain.SourceAPI); err != nil {
		return nil, err
	}
	if p.CancelURL, err = validURL(p.CancelURL, "cancel_url", live, false); err != nil {
		return nil, err
	}
	if p.CallbackURL, err = validURL(p.CallbackURL, "callback_url", live, false); err != nil {
		return nil, err
	}
	if p.CallbackURL != "" {
		u, _ := url.Parse(p.CallbackURL)
		if !a.DomainAllowed(ctx, m.ID, u.Hostname(), live) {
			return nil, domain.Invalid("callback_url_not_allowed", "callback_url", fmt.Sprintf("O domínio %s não está verificado.", u.Hostname()))
		}
	}
	if len(p.LineItems) > 100 {
		return nil, domain.Invalid("line_items_invalid", "line_items", "Máximo de 100 itens.")
	}
	if len(p.LineItems) > 0 {
		var sum int64
		for i, li := range p.LineItems {
			if strings.TrimSpace(li.Name) == "" || li.Quantity <= 0 || li.Quantity > 10000 || li.UnitAmount < 0 || li.UnitAmount > domain.MaxAmount {
				return nil, domain.Invalid("line_items_invalid", fmt.Sprintf("line_items[%d]", i), "Cada item precisa de name, quantity > 0 e unit_amount.")
			}
			sum += li.Quantity * li.UnitAmount
		}
		if sum != p.Amount {
			return nil, domain.Invalid("line_items_mismatch", "line_items", fmt.Sprintf("A soma dos itens (%d) tem de ser igual a amount (%d).", sum, p.Amount))
		}
	}
	if len(p.Metadata) > 20 {
		return nil, domain.Invalid("metadata_invalid", "metadata", "Máximo de 20 pares em metadata.")
	}
	meta := map[string]string{}
	for k, v := range p.Metadata {
		vs := fmt.Sprint(v)
		if len(k) > 40 || len(vs) > 500 {
			return nil, domain.Invalid("metadata_invalid", "metadata", "Chaves até 40 caracteres e valores até 500.")
		}
		meta[k] = vs
	}
	methods, err := a.cleanMethods(p.PaymentMethodTypes)
	if err != nil {
		return nil, err
	}
	now := a.Now()
	exp := now.Add(time.Hour)
	if p.ExpiresAt != nil {
		exp = time.Unix(*p.ExpiresAt, 0).UTC()
		if exp.Before(now.Add(15*time.Minute-time.Second)) || exp.After(now.Add(24*time.Hour+time.Second)) {
			return nil, domain.Invalid("expires_at_invalid", "expires_at", "expires_at tem de estar entre 15 minutos e 24 horas.")
		}
	}
	p.Origin.UserAgent = domain.Trim(p.Origin.UserAgent, 300)
	cs := &domain.CheckoutSession{
		ID: domain.ModeID("cs", live), MerchantID: m.ID, Livemode: live, Status: domain.SessionOpen,
		Amount: p.Amount, Currency: "MZN", Description: domain.Trim(p.Description, 500),
		SuccessURL: p.SuccessURL, CancelURL: p.CancelURL, CallbackURL: p.CallbackURL,
		ClientReferenceID: domain.Trim(p.ClientReferenceID, 200), LineItems: p.LineItems, Metadata: meta,
		PaymentMethodTypes: methods, Locale: domain.FirstNonEmpty(p.Locale, "pt-MZ"), ExpiresAt: exp, Origin: p.Origin, CreatedAt: now,
	}
	if p.Customer != nil {
		cs.CustomerName = domain.Trim(p.Customer.Name, 200)
		cs.CustomerEmail = domain.Trim(p.Customer.Email, 200)
		if ph := strings.TrimSpace(p.Customer.Phone); ph != "" {
			ph = "+258" + domain.NormalizeMZPhone(ph)
			if !e164MZ.MatchString(ph) {
				return nil, domain.Invalid("customer_phone_invalid", "customer.phone", "Use o formato E.164, por exemplo +258841234567.")
			}
			cs.CustomerPhone = ph
		}
	}
	return cs, a.Sessions.Create(ctx, cs)
}

func (a *App) Session(ctx context.Context, id string) (*domain.CheckoutSession, error) {
	cs, err := a.Sessions.ByID(ctx, id)
	return cs, notFound(err, "A sessão")
}

func (a *App) MerchantSession(ctx context.Context, merchantID string, live bool, id string) (*domain.CheckoutSession, error) {
	cs, err := a.Sessions.ByID(ctx, id)
	if err != nil || cs.MerchantID != merchantID || cs.Livemode != live {
		return nil, domain.NotFound("A sessão " + id)
	}
	return cs, nil
}

func (a *App) ListSessions(ctx context.Context, merchantID string, live bool, clientRef string, limit int) ([]domain.CheckoutSession, error) {
	return a.Sessions.List(ctx, merchantID, live, clientRef, limit)
}

func (a *App) CreateLinkSession(ctx context.Context, link *domain.PaymentLink, amount int64, channel, ip, ua string) (*domain.CheckoutSession, error) {
	m, err := a.Merchants.ByID(ctx, link.MerchantID)
	if err != nil {
		return nil, err
	}
	if !link.Active || (link.ExpiresAt != nil && link.ExpiresAt.Before(a.Now())) {
		return nil, domain.Invalid("link_inactive", "link", "Este link já não está ativo.")
	}
	if !link.Reusable {
		if n, _ := a.Payments.CountSettledByLink(ctx, link.ID); n > 0 {
			return nil, domain.Invalid("link_used", "link", "Este link já foi pago.")
		}
	}
	if !link.CustomAmount {
		amount = link.Amount
	}
	src := domain.SourceLink
	if link.Kind == "qr" {
		src, channel = domain.SourceQR, "qr"
	}
	lid := link.ID
	return a.CreateSession(ctx, m, link.Livemode, SessionParams{
		Amount: amount, Description: link.Name, PaymentMethodTypes: link.Methods, SuccessURL: m.SuccessURL, CancelURL: m.CancelURL,
		Origin: domain.Origin{SourceType: src, PaymentLinkID: &lid, IP: ip, UserAgent: ua, Channel: channel, CreatedBy: link.CreatedBy},
	})
}

type PayInput struct {
	Method     string `json:"method"`
	Phone      string `json:"phone"`
	CardNumber string `json:"card_number"`
	CardExpiry string `json:"card_expiry"`
	CardCVC    string `json:"card_cvc"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	PayerIP    string `json:"-"`
	PayerUA    string `json:"-"`
}

func (a *App) Pay(ctx context.Context, sessionID string, in PayInput) (*domain.Payment, error) {
	cs, err := a.Session(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if !cs.IsOpen(a.Now()) {
		return nil, domain.Invalid("session_not_open", "session", "Esta sessão de pagamento já não está aberta.")
	}
	in.Email = strings.TrimSpace(in.Email)
	if in.Email != "" {
		if _, err := mail.ParseAddress(in.Email); err != nil || len(in.Email) > 200 {
			return nil, domain.Invalid("email_invalid", "email", "Email inválido.")
		}
	}
	in.Method = strings.ToLower(in.Method)
	if !domain.Contains(cs.PaymentMethodTypes, in.Method) || domain.Contains(a.ComingSoon, in.Method) {
		return nil, domain.Invalid("payment_method_not_allowed", "method", "Este método não está disponível neste pagamento.")
	}
	m, err := a.Merchants.ByID(ctx, cs.MerchantID)
	if err != nil {
		return nil, err
	}
	if e := domain.RequireApproved(m); e != nil {
		return nil, e
	}
	bps := a.FeeBpsFor(ctx, m)
	fee, net := domain.Split(cs.Amount, bps)
	p := &domain.Payment{
		ID: domain.NewID("pay"), Reference: domain.PaymentReference(), MerchantID: m.ID, Livemode: cs.Livemode, SessionID: cs.ID,
		Status: domain.PaymentProcessing, Amount: cs.Amount, FeeBps: bps, Fee: fee, Net: net, Method: in.Method,
		CustomerName: domain.FirstNonEmpty(domain.Trim(in.Name, 200), cs.CustomerName), CustomerEmail: domain.FirstNonEmpty(in.Email, cs.CustomerEmail), RailReference: domain.RailReference(),
		Description: cs.Description, ClientReferenceID: cs.ClientReferenceID, Metadata: cs.Metadata, Origin: cs.Origin,
		PayerIP: in.PayerIP, PayerUserAgent: domain.Trim(in.PayerUA, 300), CreatedAt: a.Now(),
	}
	if in.Method != domain.MethodCard {
		if ph := domain.NormalizeMZPhone(in.Phone); len(ph) == 9 {
			p.PhonePrefix, p.PhoneLast4 = ph[:2], ph[5:]
		}
	}
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		locked, err := a.Sessions.Lock(ctx, cs.ID)
		if err != nil {
			return err
		}
		if !locked.IsOpen(a.Now()) {
			return domain.Invalid("session_not_open", "session", "Esta sessão de pagamento já não está aberta.")
		}
		if n, _ := a.Payments.CountBySession(ctx, cs.ID, domain.PaymentProcessing); n > 0 {
			return domain.Conflict("payment_in_progress", "Já existe um pagamento em curso. Confirme no telemóvel ou aguarde.")
		}
		if n, _ := a.Payments.CountBySession(ctx, cs.ID, domain.PaymentFailed); n >= MaxAttemptsPerSession {
			return domain.Forbidden("too_many_attempts", "Demasiadas tentativas falhadas neste pagamento. Peça um novo link à loja.")
		}
		return a.Payments.Create(ctx, p)
	})
	if err != nil {
		return nil, err
	}
	res, err := a.Rail.Initiate(ctx, domain.ChargeRequest{
		PaymentID: p.ID, Method: in.Method, Amount: p.Amount, Phone: in.Phone,
		CardNumber: in.CardNumber, CardExpiry: in.CardExpiry, CardCVC: in.CardCVC, Descriptor: m.StatementDescriptor,
		Live: p.Livemode, Reference: p.RailReference,
	}, func(o domain.Outcome) {
		if err := a.CompletePayment(context.Background(), p.ID, o); err != nil {
			log.Printf("complete payment %s: %v", p.ID, err)
		}
	})
	if err != nil {
		_ = a.Payments.Delete(ctx, p.ID)
		var ie *domain.InputError
		if errors.As(err, &ie) {
			return nil, domain.Invalid("payment_details_invalid", ie.Field, ie.Msg)
		}
		return nil, err
	}
	if in.Method == domain.MethodCard {
		p.CardBrand, p.CardLast4 = res.Brand, res.Last4
		if err := a.Payments.Save(ctx, p); err != nil {
			return nil, err
		}
	}
	if res.Async {
		_ = a.Tx.Do(ctx, func(ctx context.Context) error {
			return a.emit(ctx, m.ID, p.Livemode, "payment.processing", p.ID, domain.PaymentResource(p), "")
		})
	} else if err := a.CompletePayment(ctx, p.ID, res.Outcome); err != nil {
		return nil, err
	}
	return a.Payments.ByID(ctx, p.ID)
}

func (a *App) CompletePayment(ctx context.Context, paymentID string, o domain.Outcome) error {
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		p, err := a.Payments.Lock(ctx, paymentID)
		if err != nil {
			return err
		}
		if p.Status != domain.PaymentProcessing {
			return nil
		}
		m, err := a.Merchants.Lock(ctx, p.MerchantID)
		if err != nil {
			return err
		}
		cs, err := a.Sessions.Lock(ctx, p.SessionID)
		if err != nil {
			return err
		}
		now := a.Now()
		if o.Success {
			p.Status, p.OperatorReference, p.SucceededAt = domain.PaymentSucceeded, o.OperatorReference, &now
			if err := a.Payments.Save(ctx, p); err != nil {
				return err
			}
			if err := a.ledger(ctx, p.MerchantID, p.Livemode, domain.LedgerPayment, p.Net, p.ID, "Pagamento "+p.Reference); err != nil {
				return err
			}
			cs.Status, cs.PaymentID, cs.CompletedAt = domain.SessionComplete, &p.ID, &now
			if err := a.Sessions.Save(ctx, cs); err != nil {
				return err
			}
			if err := a.emit(ctx, p.MerchantID, p.Livemode, "payment.succeeded", p.ID, domain.PaymentResource(p), cs.CallbackURL); err != nil {
				return err
			}
			if err := a.emit(ctx, p.MerchantID, p.Livemode, "checkout.session.completed", cs.ID, domain.SessionResource(cs, a.PayURL), cs.CallbackURL); err != nil {
				return err
			}
			return a.mailPaymentSucceeded(ctx, p, m)
		}
		p.Status, p.FailureCode, p.FailedAt = domain.PaymentFailed, o.FailureCode, &now
		p.FailureMessage = domain.FailureMessages[o.FailureCode]
		if err := a.Payments.Save(ctx, p); err != nil {
			return err
		}
		return a.emit(ctx, p.MerchantID, p.Livemode, "payment.failed", p.ID, domain.PaymentResource(p), cs.CallbackURL)
	})
}

func (a *App) ExpireSession(ctx context.Context, merchantID string, live bool, id string) (*domain.CheckoutSession, error) {
	var out *domain.CheckoutSession
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		cs, err := a.Sessions.Lock(ctx, id)
		if err != nil || (merchantID != "" && (cs.MerchantID != merchantID || cs.Livemode != live)) {
			return domain.NotFound("A sessão")
		}
		if cs.Status != domain.SessionOpen {
			return domain.Invalid("session_not_open", "id", "Só sessões abertas podem expirar.")
		}
		if n, _ := a.Payments.CountBySession(ctx, cs.ID, domain.PaymentProcessing); n > 0 {
			return domain.Conflict("payment_in_progress", "Há um pagamento em curso nesta sessão.")
		}
		cs.Status = domain.SessionExpired
		if err := a.Sessions.Save(ctx, cs); err != nil {
			return err
		}
		out = cs
		return a.emit(ctx, cs.MerchantID, cs.Livemode, "checkout.session.expired", cs.ID, domain.SessionResource(cs, a.PayURL), cs.CallbackURL)
	})
	return out, err
}

func (a *App) LatestPayment(ctx context.Context, sessionID string) *domain.Payment {
	p, err := a.Payments.LatestBySession(ctx, sessionID)
	if err != nil {
		return nil
	}
	return p
}

func (a *App) Receipt(ctx context.Context, ref string) (*domain.Payment, *domain.Merchant, error) {
	p, err := a.Payments.ByReference(ctx, ref)
	if err != nil || !domain.Contains(domain.SettledStatuses, p.Status) {
		return nil, nil, domain.NotFound("O recibo")
	}
	m, err := a.Merchants.ByID(ctx, p.MerchantID)
	return p, m, err
}

// cleanMethods is CleanMethods without what can't take money yet.
func (a *App) cleanMethods(in []string) ([]string, error) {
	methods, err := domain.CleanMethods(in)
	if err != nil {
		return nil, err
	}
	out := a.ActiveMethods(methods)
	if len(out) == 0 {
		return nil, domain.Invalid("payment_method_unavailable", "payment_method_types", "Estes métodos chegam em breve. Por agora só aceitamos M-Pesa.")
	}
	return out, nil
}

func (a *App) ActiveMethods(methods []string) []string {
	out := make([]string, 0, len(methods))
	for _, m := range methods {
		if !domain.Contains(a.ComingSoon, m) {
			out = append(out, m)
		}
	}
	return out
}
