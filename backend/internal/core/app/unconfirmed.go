package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// UnconfirmedNotice is what a customer reads when they try to pay again while
// their last payment is still being confirmed.
const UnconfirmedNotice = "O pagamento anterior ainda está a ser confirmado com o operador. Se o aprovou no telemóvel, não pague de novo: avisamos por SMS assim que for confirmado."

// How long after it goes unconfirmed a payment is asked about again, one step
// per check, the last one repeating. Most late answers come within minutes.
var recheckAfter = []time.Duration{2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour, 4 * time.Hour}

// After this long from the request with no final answer, nobody will pay it:
// it fails as never confirmed and is left for a person to look at.
const unconfirmedFor = 24 * time.Hour

// markUnconfirmed takes a payment the operator didn't settle in time out of
// processing, so the customer isn't held up, without calling it failed.
func (a *App) markUnconfirmed(ctx context.Context, paymentID, why string) error {
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		p, err := a.Payments.Lock(ctx, paymentID)
		if err != nil || p.Status != domain.PaymentProcessing {
			return err
		}
		cs, err := a.Sessions.Lock(ctx, p.SessionID)
		if err != nil {
			return err
		}
		now := a.Now()
		next := now.Add(recheckAfter[0])
		p.Status, p.RecheckAt, p.Rechecks = domain.PaymentUnconfirmed, &next, 0
		if err := a.Payments.Save(ctx, p); err != nil {
			return err
		}
		a.trace(ctx, domain.TrailEvent{MerchantID: p.MerchantID, Livemode: p.Livemode, SessionID: cs.ID, PaymentID: p.ID, Kind: domain.TrailPaymentStatus, Source: "riha", Level: domain.TrailWarn,
			Title: "Fica a confirmar: " + why + ". A RIHA continua a perguntar ao operador e conclui o pagamento quando ele responder.",
			Data:  map[string]any{"from": domain.PaymentProcessing, "to": domain.PaymentUnconfirmed}})
		return a.emit(ctx, p.MerchantID, p.Livemode, "payment.unconfirmed", p.ID, domain.PaymentResource(p), cs.CallbackURL)
	})
}

// RecheckUnconfirmed asks the operator again about the unconfirmed payments
// that are due, and settles each one it gets a final answer for.
func (a *App) RecheckUnconfirmed(ctx context.Context) {
	rail, ok := a.Rail.(ports.RailQuery)
	due, err := a.Payments.DueRechecks(ctx, a.Now(), 50)
	if err != nil {
		log.Printf("unconfirmed payments: %v", err)
		return
	}
	for i := range due {
		p := &due[i]
		if !ok || p.RailReference == "" {
			a.endUnconfirmed(ctx, p, "não há forma de perguntar ao operador")
			continue
		}
		o, known, err := rail.Query(ctx, domain.StatusQuery{PaymentID: p.ID, Live: p.Livemode, Reference: p.RailReference, RailID: p.RailID, Method: p.Method, Integration: p.Integration, Amount: p.Amount})
		switch {
		case err == nil && known:
			_ = a.CompletePayment(ctx, p.ID, o)
		case err == nil, errors.Is(err, ports.ErrNoStatusCheck):
			// mtoca has no record of it: the request never reached the customer.
			a.endUnconfirmed(ctx, p, "o operador não tem registo dele")
		case a.Now().Sub(p.CreatedAt) >= unconfirmedFor:
			a.endUnconfirmed(ctx, p, "o operador não deu resposta final em "+domain.Ago(unconfirmedFor))
		default:
			step := min(p.Rechecks+1, len(recheckAfter)-1)
			next := a.Now().Add(recheckAfter[step])
			if err := a.Payments.SetRecheck(ctx, p.ID, next, p.Rechecks+1); err != nil {
				log.Printf("unconfirmed payment %s: %v", p.ID, err)
			}
		}
	}
}

func (a *App) endUnconfirmed(ctx context.Context, p *domain.Payment, why string) {
	a.logPayment(ctx, p.ID, domain.TrailWarn, "payment %s (%s): closed as never confirmed, %s. If the customer says they paid, check it with the operator by hand.", p.ID, p.RailReference, why)
	_ = a.CompletePayment(ctx, p.ID, domain.Outcome{FailureCode: domain.FailNotConfirmed})
}

// textCustomer tells the customer of an unconfirmed payment how it ended:
// they were asked to wait for this message instead of paying again.
func (a *App) textCustomer(ctx context.Context, p *domain.Payment, m *domain.Merchant) {
	if !p.Livemode || p.ContactPhone == "" {
		return
	}
	what := m.Name
	if d := strings.TrimSpace(p.Description); d != "" && len(d) <= 40 {
		what += " (" + d + ")"
	}
	amount := domain.FormatShort(p.Amount) + " MT"
	var body string
	switch {
	case p.Status == domain.PaymentSucceeded && p.DuplicateOf != "":
		body = fmt.Sprintf("RIHA: confirmámos o seu pagamento de %s a %s, ref. %s. Parece ter pago duas vezes: a loja vai ver consigo a devolução.", amount, what, p.Reference)
	case p.Status == domain.PaymentSucceeded:
		body = fmt.Sprintf("RIHA: confirmámos o seu pagamento de %s a %s, ref. %s. Não precisa de pagar de novo.", amount, what, p.Reference)
	default:
		body = fmt.Sprintf("RIHA: o pagamento de %s a %s não foi concluído e nada foi cobrado. Pode tentar de novo pelo link da loja.", amount, what)
	}
	err := a.Messages.Create(ctx, &domain.Message{ID: domain.NewID("msg"), MerchantID: p.MerchantID, Livemode: p.Livemode, Channel: domain.ChannelSMS,
		To: p.ContactPhone, Kind: "payment_result", Body: body, Status: domain.MessageQueued, CreatedAt: a.Now()})
	if err != nil {
		log.Printf("payment %s: SMS to the customer not queued: %v", p.ID, err)
		return
	}
	a.trace(ctx, domain.TrailEvent{MerchantID: p.MerchantID, Livemode: p.Livemode, SessionID: p.SessionID, PaymentID: p.ID, Kind: domain.TrailLog, Source: "riha",
		Title: "SMS ao cliente " + domain.MaskPhones(p.ContactPhone) + ": " + body})
	a.SendSoon(ctx)
}

// OperatorCheck is what the operator says about a payment right now, next to
// what RIHA has, for a person looking into it.
type OperatorCheck struct {
	Payment *domain.Payment
	// Answer: paid, failed, pending (no final answer yet), none (no record),
	// unreachable, or unsupported (this rail can't be asked).
	Answer            string
	FailureCode       string
	OperatorReference string
	Error             string
	// Fix is what applying the answer would do: credit, fail, or nothing.
	Fix     string
	Applied bool
}

// CheckWithOperator asks the operator about one payment now. With apply it
// also acts on a final answer RIHA doesn't have yet: settles a payment still
// open, and credits one closed as unanswered that the operator says was paid.
func (a *App) CheckWithOperator(ctx context.Context, paymentID, actor string, apply bool) (*OperatorCheck, error) {
	p, err := a.Payments.ByID(ctx, paymentID)
	if err != nil {
		return nil, notFound(err, "O pagamento")
	}
	out := &OperatorCheck{Payment: p}
	rail, ok := a.Rail.(ports.RailQuery)
	if !ok || p.RailReference == "" || p.Method == domain.MethodCard {
		out.Answer = "unsupported"
		return out, nil
	}
	o, known, err := rail.Query(ctx, domain.StatusQuery{PaymentID: p.ID, Live: p.Livemode, Reference: p.RailReference, RailID: p.RailID, Method: p.Method, Integration: p.Integration, Urgent: true, Amount: p.Amount})
	switch {
	case errors.Is(err, ports.ErrNoStatusCheck):
		out.Answer = "unsupported"
	case errors.Is(err, ports.ErrStillPending):
		out.Answer = "pending"
	case err != nil:
		out.Answer, out.Error = "unreachable", err.Error()
	case !known:
		out.Answer = "none"
	case o.Success:
		out.Answer, out.OperatorReference = "paid", o.OperatorReference
	default:
		out.Answer, out.FailureCode = "failed", o.FailureCode
	}
	open := p.Status == domain.PaymentProcessing || p.Status == domain.PaymentUnconfirmed
	// Only a payment closed without the operator's own final word is
	// reopened: for want of an answer, or on an error reaching it. One the
	// operator turned down was already settled on its word.
	unanswered := p.Status == domain.PaymentFailed && domain.Contains([]string{domain.FailCustomerTimeout, domain.FailNotCompleted, domain.FailNotConfirmed, domain.FailOperatorUnavailable}, p.FailureCode)
	switch {
	case out.Answer == "paid" && (open || unanswered):
		out.Fix = "credit"
	case out.Answer == "failed" && open:
		out.Fix = "fail"
	}
	if !apply || out.Fix == "" {
		return out, nil
	}
	if unanswered {
		if err := a.reopenUnanswered(ctx, p.ID, actor); err != nil {
			return out, err
		}
	}
	if err := a.CompletePayment(ctx, p.ID, o); err != nil {
		return out, err
	}
	out.Applied = true
	out.Payment, _ = a.Payments.ByID(ctx, p.ID)
	return out, nil
}

// reopenUnanswered puts a payment closed as unanswered back to unconfirmed,
// so the operator's late answer settles it the same way as any other.
func (a *App) reopenUnanswered(ctx context.Context, paymentID, actor string) error {
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		p, err := a.Payments.Lock(ctx, paymentID)
		if err != nil {
			return err
		}
		if p.Status != domain.PaymentFailed {
			return domain.Conflict("payment_state", "O pagamento mudou entretanto. Atualize a página.")
		}
		from := p.FailureCode
		p.Status, p.FailureCode, p.FailureMessage, p.FailedAt = domain.PaymentUnconfirmed, "", "", nil
		if err := a.Payments.Save(ctx, p); err != nil {
			return err
		}
		a.trace(ctx, domain.TrailEvent{MerchantID: p.MerchantID, Livemode: p.Livemode, SessionID: p.SessionID, PaymentID: p.ID, Kind: domain.TrailPaymentStatus, Source: "riha", Level: domain.TrailWarn,
			Title: "Reaberto por um administrador: estava fechado sem resposta (" + from + ") e o operador diz que foi pago",
			Data:  map[string]any{"from": domain.PaymentFailed, "to": domain.PaymentUnconfirmed, "actor": actor}})
		a.Record(ctx, p.MerchantID, Actor{ID: actor, Role: domain.RoleAdmin}, "payment.reopened", p.ID,
			fmt.Sprintf("%s: fechado como %s, o operador diz que foi pago (%s MT)", p.ID, from, domain.FormatShort(p.Amount)))
		return nil
	})
}
