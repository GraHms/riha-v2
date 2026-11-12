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
	"riha/internal/core/ports"
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
	SuccessMessage     string            `json:"-"`
	SubmitType         string            `json:"submit_type"`
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
	if min := domain.MinAmountFor(m); p.Amount < min {
		return nil, domain.Invalid("amount_too_small", "amount", fmt.Sprintf("O valor mínimo é %d centavos (%s MT).", min, domain.FormatShort(min)))
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
	methods, err := a.cleanMethods(p.PaymentMethodTypes, live)
	if err != nil {
		return nil, err
	}
	if p.SubmitType, err = domain.CleanSubmitType(p.SubmitType); err != nil {
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
		SuccessURL: p.SuccessURL, CancelURL: p.CancelURL, CallbackURL: p.CallbackURL, SuccessMessage: domain.Trim(p.SuccessMessage, 300), SubmitType: p.SubmitType,
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
	if !link.Active || link.Archived || (link.ExpiresAt != nil && link.ExpiresAt.Before(a.Now())) {
		return nil, domain.Invalid("link_inactive", "link", "Este link já não está ativo.")
	}
	if !link.Reusable {
		if n, _ := a.Payments.CountSettledByLink(ctx, link.ID); n > 0 {
			return nil, domain.Invalid("link_used", "link", "Este link já foi pago.")
		}
	}
	if !link.CustomAmount {
		amount = link.Amount
	} else if lo, hi := link.CustomRange(domain.MinAmountFor(m)); amount < lo || amount > hi {
		if amount < lo {
			return nil, domain.Invalid("amount_too_small", "amount", fmt.Sprintf("O valor mínimo é %s MT.", domain.FormatShort(lo)))
		}
		return nil, domain.Invalid("amount_too_large", "amount", fmt.Sprintf("O valor máximo é %s MT.", domain.FormatShort(hi)))
	}
	src := domain.SourceLink
	if link.Kind == "qr" {
		src, channel = domain.SourceQR, "qr"
	}
	lid := link.ID
	cs, err := a.CreateSession(ctx, m, link.Livemode, SessionParams{
		Amount: amount, Description: link.Name, PaymentMethodTypes: link.Methods, SuccessURL: domain.FirstNonEmpty(link.RedirectURL, m.SuccessURL), CancelURL: m.CancelURL,
		SuccessMessage: link.AfterMessage, SubmitType: link.SubmitType,
		Origin: domain.Origin{SourceType: src, PaymentLinkID: &lid, IP: ip, UserAgent: ua, Channel: channel, CreatedBy: link.CreatedBy},
	})
	if err == nil {
		a.trace(ctx, domain.TrailEvent{MerchantID: m.ID, Livemode: cs.Livemode, SessionID: cs.ID, Kind: domain.TrailCheckoutCreated, Source: "customer", IP: ip, UserAgent: ua,
			Title: fmt.Sprintf("O cliente abriu o link %q (%s MT), canal %s", link.Name, domain.FormatShort(cs.Amount), domain.FirstNonEmpty(channel, "outro")),
			Data:  map[string]any{"link": link.ID, "channel": channel, "amount": cs.Amount}})
	}
	return cs, err
}

type PayInput struct {
	Method     string `json:"method"`
	Phone      string `json:"phone"`
	CardNumber string `json:"card_number"`
	CardExpiry string `json:"card_expiry"`
	CardCVC    string `json:"card_cvc"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	// AttemptID is made by the page before it asks. Asking again with the same one
	// returns the same payment, so a repeated request can never charge twice.
	AttemptID string `json:"attempt_id"`
	// PayAgain: the customer saw that an earlier payment is still being
	// confirmed and chose to pay anyway.
	PayAgain bool `json:"pay_again"`
	// Country is where a customer paying from abroad has their wallet.
	Country string `json:"country"`
	PayerIP string `json:"-"`
	PayerUA string `json:"-"`
}

func (a *App) Pay(ctx context.Context, sessionID string, in PayInput) (*domain.Payment, error) {
	cs, err := a.Session(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	// A repeat comes first: the payment it asks about may have ended the session.
	attempt := domain.CleanAttemptID(in.AttemptID)
	if p, err := a.paymentForAttempt(ctx, cs.ID, attempt); err != nil || p != nil {
		return p, err
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
	if !domain.Contains(cs.PaymentMethodTypes, in.Method) || domain.Contains(a.ComingSoonFor(cs.Livemode), in.Method) {
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
		CustomerName: domain.FirstNonEmpty(domain.Trim(in.Name, 200), cs.CustomerName), CustomerEmail: domain.FirstNonEmpty(in.Email, cs.CustomerEmail), AttemptID: attempt,
		Description: cs.Description, ClientReferenceID: cs.ClientReferenceID, Metadata: cs.Metadata, Origin: cs.Origin,
		PayerIP: in.PayerIP, PayerUserAgent: domain.Trim(in.PayerUA, 300), CreatedAt: a.Now(),
	}
	p.Integration = a.routeFor(ctx, m.ID, in.Method, cs.Livemode)
	if price, err := a.priceFor(ctx, m.ID, p.Integration, in.Method, p.Amount, p.Fee); err == nil {
		p.PlatformFee, p.PartnerFee, p.Priced = price.PlatformFee, price.PartnerFee, true
	} else {
		log.Printf("pricing payment %s: %v; the backlog prices it later", p.ID, err)
	}
	p.RailReference = a.railReference(p.Integration, in.Method, cs.Livemode)
	abroad := domain.Contains(a.RedirectMethodsFor(cs.Livemode), in.Method)
	if domain.Contains(a.HostedMethodsFor(cs.Livemode), in.Method) {
		// Paid on U2opia's page: the number it asks for is the one the
		// merchant gave, if any; the card itself is never typed here.
		in.Phone = domain.FirstNonEmpty(in.Phone, cs.CustomerPhone)
		in.CardNumber, in.CardExpiry, in.CardCVC = "", "", ""
	}
	switch {
	case abroad:
		in.Country = strings.ToUpper(strings.TrimSpace(in.Country))
		if len(in.Country) != 2 {
			return nil, domain.Invalid("country_required", "country", "Escolha o país da sua carteira.")
		}
		if d := domain.Digits(in.Phone); len(d) >= 4 {
			p.PhonePrefix, p.PhoneLast4 = in.Country, d[len(d)-4:]
		}
		p.Country = in.Country
	case in.Method != domain.MethodCard:
		if ph := domain.NormalizeMZPhone(in.Phone); len(ph) == 9 {
			p.PhonePrefix, p.PhoneLast4, p.ContactPhone = ph[:2], ph[5:], ph
		}
	}
	var repeated *domain.Payment
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		locked, err := a.Sessions.Lock(ctx, cs.ID)
		if err != nil {
			return err
		}
		// Under the lock too: the same request can arrive twice at once.
		if repeated, err = a.paymentForAttempt(ctx, cs.ID, attempt); err != nil || repeated != nil {
			return err
		}
		if !locked.IsOpen(a.Now()) {
			return domain.Invalid("session_not_open", "session", "Esta sessão de pagamento já não está aberta.")
		}
		if n, _ := a.Payments.CountBySession(ctx, cs.ID, domain.PaymentProcessing); n > 0 {
			return domain.Conflict("payment_in_progress", "Já existe um pagamento em curso. Confirme no telemóvel ou aguarde.")
		}
		if n, _ := a.Payments.CountBySession(ctx, cs.ID, domain.PaymentUnconfirmed); n > 0 && !in.PayAgain {
			return domain.Conflict("payment_unconfirmed", UnconfirmedNotice)
		}
		if n, _ := a.Payments.CountBySession(ctx, cs.ID, domain.PaymentFailed); n >= MaxAttemptsPerSession {
			return domain.Forbidden("too_many_attempts", "Demasiadas tentativas falhadas neste pagamento. Peça um novo link à loja.")
		}
		return a.Payments.Create(ctx, p)
	})
	if err != nil {
		return nil, err
	}
	if repeated != nil {
		a.trace(ctx, domain.TrailEvent{MerchantID: m.ID, Livemode: repeated.Livemode, SessionID: cs.ID, PaymentID: repeated.ID, Kind: domain.TrailSubmitted, Source: "customer",
			IP: in.PayerIP, UserAgent: in.PayerUA, Title: "O cliente repetiu o pedido com o mesmo código: devolvido o mesmo pagamento, sem cobrar duas vezes", Data: map[string]any{"attempt_id": repeated.AttemptID}})
		return repeated, nil
	}
	phone := ""
	if in.Phone != "" {
		phone = domain.MaskPhones(domain.NormalizeMZPhone(in.Phone))
	}
	a.trace(ctx, domain.TrailEvent{MerchantID: m.ID, Livemode: p.Livemode, SessionID: cs.ID, PaymentID: p.ID, Kind: domain.TrailSubmitted, Source: "customer",
		IP: in.PayerIP, UserAgent: in.PayerUA, Title: fmt.Sprintf("O cliente pediu para pagar %s MT por %s%s", domain.FormatShort(p.Amount), domain.FirstNonEmpty(domain.MethodNames[in.Method], in.Method), map[bool]string{true: " com o número " + phone, false: ""}[phone != ""]),
		Data: map[string]any{"method": in.Method, "amount": p.Amount, "attempt_id": p.AttemptID, "phone": phone, "reference": p.Reference, "operation": p.RailReference}})
	res, err := a.Rail.Initiate(ctx, domain.ChargeRequest{
		PaymentID: p.ID, Description: p.Description, Method: in.Method, Amount: p.Amount, Phone: in.Phone,
		CardNumber: in.CardNumber, CardExpiry: in.CardExpiry, CardCVC: in.CardCVC, Descriptor: m.StatementDescriptor, Country: in.Country,
		Live: p.Livemode, Reference: p.RailReference, OnRailID: func(id string) { a.keepRailID(p.ID, id) }, Integration: p.Integration,
	}, func(o domain.Outcome) {
		if err := a.CompletePayment(context.Background(), p.ID, o); err != nil {
			log.Printf("complete payment %s: %v", p.ID, err)
		}
	})
	if err != nil {
		a.trace(ctx, domain.TrailEvent{MerchantID: m.ID, Livemode: p.Livemode, SessionID: cs.ID, PaymentID: p.ID, Kind: domain.TrailLog, Source: "riha", Level: domain.TrailWarn,
			Title: "O pagamento não começou: " + err.Error()})
		_ = a.Payments.Delete(ctx, p.ID)
		var ie *domain.InputError
		if errors.As(err, &ie) {
			return nil, domain.Invalid("payment_details_invalid", ie.Field, ie.Msg)
		}
		return nil, err
	}
	if in.Method == domain.MethodCard || res.RedirectURL != "" {
		if err := a.Tx.Do(ctx, func(ctx context.Context) error {
			cur, err := a.Payments.Lock(ctx, p.ID)
			if err != nil {
				return err
			}
			cur.CardBrand, cur.CardLast4, cur.RedirectURL = res.Brand, res.Last4, res.RedirectURL
			return a.Payments.Save(ctx, cur)
		}); err != nil {
			return nil, err
		}
	}
	if res.Async {
		// The operator may have answered already: "processing" is only said
		// of a payment still processing, never after it ended.
		_ = a.Tx.Do(ctx, func(ctx context.Context) error {
			now, err := a.Payments.Lock(ctx, p.ID)
			if err != nil || now.Status != domain.PaymentProcessing {
				return err
			}
			return a.emit(ctx, m.ID, now.Livemode, "payment.processing", now.ID, domain.PaymentResource(now), "")
		})
	} else if err := a.CompletePayment(ctx, p.ID, res.Outcome); err != nil {
		return nil, err
	}
	return a.Payments.ByID(ctx, p.ID)
}

func (a *App) paymentForAttempt(ctx context.Context, sessionID, attempt string) (*domain.Payment, error) {
	if attempt == "" {
		return nil, nil
	}
	p, err := a.Payments.BySessionAttempt(ctx, sessionID, attempt)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	return p, err
}

func (a *App) railReference(integration, method string, live bool) string {
	if r := a.routing(); r != nil {
		return r.ReferenceVia(integration, method, live)
	}
	if r, ok := a.Rail.(ports.RailReferencer); ok {
		return r.NewReference(method, live)
	}
	return domain.RailReference()
}

// keepRailID stores the rail's id for a payment, so the sweep can still ask about it
// after a restart. Losing it can leave a paid customer uncredited, so it's logged.
func (a *App) keepRailID(paymentID, id string) {
	err := a.Tx.Do(context.Background(), func(ctx context.Context) error {
		p, err := a.Payments.Lock(ctx, paymentID)
		if err != nil {
			return err
		}
		p.RailID = id
		return a.Payments.Save(ctx, p)
	})
	if err != nil {
		a.logPayment(context.Background(), paymentID, domain.TrailError, "payment %s: couldn't keep rail id %s: %v", paymentID, id, err)
	}
}

// CompletePayment ends a payment on the operator's answer. It takes one still
// processing, or one left unconfirmed when the answer was late: that one is paid
// late, or fails, and its customer is told by SMS, since they were asked to wait.
func (a *App) CompletePayment(ctx context.Context, paymentID string, o domain.Outcome) error {
	var notify *domain.Payment
	var merchant *domain.Merchant
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		p, err := a.Payments.Lock(ctx, paymentID)
		if err != nil {
			return err
		}
		if p.Status != domain.PaymentProcessing && p.Status != domain.PaymentUnconfirmed {
			return nil
		}
		late := p.Status == domain.PaymentUnconfirmed
		m, err := a.Merchants.Lock(ctx, p.MerchantID)
		if err != nil {
			return err
		}
		cs, err := a.Sessions.Lock(ctx, p.SessionID)
		if err != nil {
			return err
		}
		now := a.Now()
		took := now.Sub(p.CreatedAt)
		ev := domain.TrailEvent{MerchantID: p.MerchantID, Livemode: p.Livemode, SessionID: cs.ID, PaymentID: p.ID, Kind: domain.TrailPaymentStatus, Source: "riha",
			Data: map[string]any{"from": p.Status, "operator_reference": o.OperatorReference, "seconds": int(took.Seconds())}}
		switch {
		case o.Success && late:
			ev.Title = "Pago depois de ficar a confirmar: o operador confirmou " + domain.Ago(took) + " depois do pedido"
			ev.Data["to"] = domain.PaymentSucceeded
		case o.Success:
			ev.Title = fmt.Sprintf("Pagamento concluído, %d s depois do pedido", int(took.Seconds()))
			ev.Data["to"] = domain.PaymentSucceeded
		default:
			ev.Title, ev.Level = fmt.Sprintf("Pagamento falhou, %s depois do pedido: %s", domain.Ago(took), domain.FirstNonEmpty(domain.FailureMessages[o.FailureCode], o.FailureCode)), domain.TrailWarn
			ev.Data["to"], ev.Data["failure_code"] = domain.PaymentFailed, o.FailureCode
		}
		a.trace(ctx, ev)
		// Someone who never answered isn't texted a day later that nothing happened.
		if late && p.ContactPhone != "" && (o.Success || o.FailureCode != domain.FailNotConfirmed) {
			c := *p
			notify, merchant = &c, m
		}
		p.ContactPhone, p.RecheckAt, p.RedirectURL = "", nil, ""
		if !o.Success {
			p.Status, p.FailureCode, p.FailedAt = domain.PaymentFailed, o.FailureCode, &now
			p.FailureMessage = domain.FailureMessages[o.FailureCode]
			if err := a.Payments.Save(ctx, p); err != nil {
				return err
			}
			if notify != nil {
				notify.Status, notify.FailureCode = p.Status, p.FailureCode
			}
			if cs.SourceType == domain.SourceSubscription {
				if err := a.chargesFailed(ctx, cs, p); err != nil {
					return err
				}
			}
			return a.emit(ctx, p.MerchantID, p.Livemode, "payment.failed", p.ID, domain.PaymentResource(p), cs.CallbackURL)
		}
		p.Status, p.OperatorReference, p.SucceededAt, p.PaidLate = domain.PaymentSucceeded, o.OperatorReference, &now, late
		p.FailureCode, p.FailureMessage = "", ""
		// A late answer can land on a session the customer has paid since, or
		// on something they paid again in a new session.
		sessionPaid := cs.Status == domain.SessionComplete && cs.PaymentID != nil && *cs.PaymentID != p.ID
		if sessionPaid {
			p.DuplicateOf = *cs.PaymentID
		} else if late {
			if twin, err := a.Payments.LaterTwin(ctx, p); err == nil && twin != nil {
				p.DuplicateOf = twin.ID
			}
		}
		if err := a.Payments.Save(ctx, p); err != nil {
			return err
		}
		if notify != nil {
			notify.Status, notify.DuplicateOf = p.Status, p.DuplicateOf
		}
		if p.DuplicateOf != "" {
			a.trace(ctx, domain.TrailEvent{MerchantID: p.MerchantID, Livemode: p.Livemode, SessionID: cs.ID, PaymentID: p.ID, Kind: domain.TrailPaymentStatus, Source: "riha", Level: domain.TrailError,
				Title: "Possível pagamento em duplicado: o mesmo cliente já tinha pago " + domain.FormatShort(p.Amount) + " MT em " + p.DuplicateOf + ". Veja com o comerciante se é para reembolsar.",
				Data:  map[string]any{"duplicate_of": p.DuplicateOf}})
		}
		if err := a.ledger(ctx, p.MerchantID, p.Livemode, domain.LedgerPayment, p.Net, p.ID, "Pagamento "+p.Reference); err != nil {
			return err
		}
		if !sessionPaid {
			cs.Status, cs.PaymentID, cs.CompletedAt = domain.SessionComplete, &p.ID, &now
			if err := a.Sessions.Save(ctx, cs); err != nil {
				return err
			}
			if cs.SourceType == domain.SourceSubscription {
				if err := a.settleCharges(ctx, cs, p); err != nil {
					return err
				}
			}
		}
		if err := a.emit(ctx, p.MerchantID, p.Livemode, "payment.succeeded", p.ID, domain.PaymentResource(p), cs.CallbackURL); err != nil {
			return err
		}
		if !sessionPaid {
			if err := a.emit(ctx, p.MerchantID, p.Livemode, "checkout.session.completed", cs.ID, domain.SessionResource(cs, a.PayURL), cs.CallbackURL); err != nil {
				return err
			}
		}
		return a.mailPaymentSucceeded(ctx, p, m)
	})
	if err == nil && notify != nil {
		a.textCustomer(ctx, notify, merchant)
	}
	return err
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
func (a *App) cleanMethods(in []string, live bool) ([]string, error) {
	methods, err := domain.CleanMethods(in)
	if err != nil {
		return nil, err
	}
	out := a.ActiveMethodsFor(methods, live)
	if len(out) == 0 {
		return nil, domain.Invalid("payment_method_unavailable", "payment_method_types", "Estes métodos ainda não estão disponíveis.")
	}
	return out, nil
}

func (a *App) ActiveMethods(methods []string) []string { return a.ActiveMethodsFor(methods, true) }

// HostedMethodsFor lists the methods a mode takes wholly on the operator's
// page, the customer choosing nothing here.
func (a *App) HostedMethodsFor(live bool) []string {
	if live {
		return nil
	}
	return a.HostedMethods
}

// RedirectMethodsFor lists the methods a mode takes on the operator's own
// page: the customer picks their country, then pays there.
func (a *App) RedirectMethodsFor(live bool) []string {
	if live {
		return nil
	}
	return a.AbroadMethods
}

// ComingSoonShown is what a checkout shows as coming soon: never a method
// only offered when asked for, like a wallet from abroad.
func (a *App) ComingSoonShown(live bool) []string {
	out := []string{}
	for _, m := range a.ComingSoonFor(live) {
		if !domain.Contains(domain.OptInMethods, m) {
			out = append(out, m)
		}
	}
	return out
}

// ComingSoonFor lists the methods a mode can't take money with yet.
func (a *App) ComingSoonFor(live bool) []string {
	if live || a.TestComingSoon == nil {
		return a.ComingSoon
	}
	return a.TestComingSoon
}

// ActiveMethodsFor keeps the methods a mode can take money with.
func (a *App) ActiveMethodsFor(methods []string, live bool) []string {
	soon := a.ComingSoonFor(live)
	out := make([]string, 0, len(methods))
	for _, m := range methods {
		if !domain.Contains(soon, m) {
			out = append(out, m)
		}
	}
	return out
}
