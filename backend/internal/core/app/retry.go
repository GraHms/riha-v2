package app

import (
	"context"
	"fmt"
	"log"

	"riha/internal/core/domain"
)

// M-Pesa often gives up on a request after about a minute without the
// customer's phone ever showing it: a known M-Pesa problem. So when it fails
// the transaction that way, RIHA sends a new request by itself while the
// customer waits on the page, up to MpesaTries in all, and then one more
// through MpesaFallback. Each try is its own payment in the same session, so
// every one is checked for a late answer like any other, and a second success
// is caught as a duplicate.
const (
	defaultMpesaTries    = 3
	defaultMpesaFallback = "mtoca"
)

// fallbackFor is the integration the last try goes through, or "" when there
// is none for this payment.
func (a *App) fallbackFor(p *domain.Payment) string {
	f := a.MpesaFallback
	r := a.routing()
	if f == "" || r == nil || r.RouteFor(f, p.Method, p.Livemode) != f {
		return ""
	}
	return f
}

// TriesFor is how many tries a payment's customer can be sent in all.
func (a *App) TriesFor(p *domain.Payment) int {
	if p.Method != domain.MethodMpesa || a.MpesaTries < 1 {
		return 1
	}
	if f := a.fallbackFor(p); f != "" && p.Retry > 0 && p.Integration == f {
		return p.Retry + 1
	}
	n := a.MpesaTries
	if f := a.fallbackFor(p); f != "" && f != p.Integration {
		n++
	}
	return n
}

// nextTry creates the next try of a payment the operator failed, in the
// caller's transaction, so the session is never without a payment in progress
// in between. It returns nil when there is none to make.
func (a *App) nextTry(ctx context.Context, p *domain.Payment, cs *domain.CheckoutSession, o domain.Outcome) (*domain.Payment, error) {
	if o.Success || o.FailureCode != domain.FailOperatorTimeout || p.Method != domain.MethodMpesa || p.Status != domain.PaymentProcessing ||
		len(p.ContactPhone) != 9 || !cs.IsOpen(a.Now()) || a.MpesaTries < 1 {
		return nil, nil
	}
	made := p.Retry + 1
	via := ""
	switch f := a.fallbackFor(p); {
	case p.Integration == f && p.Retry > 0:
		return nil, nil
	case made < a.MpesaTries:
		via = p.Integration
	case made == a.MpesaTries && f != "" && f != p.Integration:
		via = f
	default:
		return nil, nil
	}
	next := *p
	next.ID, next.Reference, next.Status = domain.NewID("pay"), domain.PaymentReference(), domain.PaymentProcessing
	next.FailureCode, next.FailureMessage, next.OperatorReference, next.RailID, next.AttemptID = "", "", "", "", ""
	next.SucceededAt, next.FailedAt, next.RecheckAt, next.Rechecks = nil, nil, nil, 0
	next.PaidLate, next.DuplicateOf, next.AmountRefunded = false, "", 0
	next.Retry, next.Integration, next.CreatedAt = made, via, a.Now()
	next.PlatformFee, next.PartnerFee, next.Priced = 0, 0, false
	if price, err := a.priceFor(ctx, p.MerchantID, via, p.Method, p.Amount, p.Fee); err == nil {
		next.PlatformFee, next.PartnerFee, next.Priced = price.PlatformFee, price.PartnerFee, true
	} else {
		log.Printf("pricing payment %s: %v; the backlog prices it later", next.ID, err)
	}
	next.RailReference = a.railReference(via, p.Method, p.Livemode)
	if err := a.Payments.Create(ctx, &next); err != nil {
		return nil, err
	}
	of := a.TriesFor(&next)
	a.trace(ctx, domain.TrailEvent{MerchantID: p.MerchantID, Livemode: p.Livemode, SessionID: cs.ID, PaymentID: p.ID, Kind: domain.TrailLog, Source: "riha",
		Title: fmt.Sprintf("O M-Pesa falhou a transação: a RIHA tenta de novo sozinha (tentativa %d de %d), em %s", made+1, of, next.ID),
		Data:  map[string]any{"retried_by": next.ID, "try": made + 1, "of": of, "integration": via}})
	a.trace(ctx, domain.TrailEvent{MerchantID: p.MerchantID, Livemode: p.Livemode, SessionID: cs.ID, PaymentID: next.ID, Kind: domain.TrailSubmitted, Source: "riha",
		Title: fmt.Sprintf("Nova tentativa automática %d de %d, depois de o M-Pesa falhar %s: %s MT com o número %s", made+1, of, p.ID, domain.FormatShort(p.Amount), domain.MaskPhones(p.ContactPhone)),
		Data:  map[string]any{"retry_of": p.ID, "try": made + 1, "of": of, "integration": via, "reference": next.Reference, "operation": next.RailReference}})
	return &next, nil
}

// startTry sends a try made by nextTry to the operator, once its transaction
// is done. A try that can't start fails, which ends the tries.
func (a *App) startTry(p *domain.Payment, phone, descriptor string) {
	ctx := context.Background()
	res, err := a.Rail.Initiate(ctx, domain.ChargeRequest{
		PaymentID: p.ID, Description: p.Description, Method: p.Method, Amount: p.Amount, Phone: phone, Descriptor: descriptor,
		Live: p.Livemode, Reference: p.RailReference, OnRailID: func(id string) { a.keepRailID(p.ID, id) }, Integration: p.Integration,
	}, func(o domain.Outcome) {
		if err := a.CompletePayment(context.Background(), p.ID, o); err != nil {
			log.Printf("complete payment %s: %v", p.ID, err)
		}
	})
	switch {
	case err != nil:
		a.logPayment(ctx, p.ID, domain.TrailWarn, "payment %s: the automatic try didn't start: %v", p.ID, err)
		if err := a.CompletePayment(ctx, p.ID, domain.Outcome{FailureCode: domain.FailOperatorUnavailable}); err != nil {
			log.Printf("complete payment %s: %v", p.ID, err)
		}
	case res.Async:
		a.emitProcessing(ctx, p.ID)
	default:
		if err := a.CompletePayment(ctx, p.ID, res.Outcome); err != nil {
			log.Printf("complete payment %s: %v", p.ID, err)
		}
	}
}

// emitProcessing says a payment is processing, if it still is: the operator
// may have answered already.
func (a *App) emitProcessing(ctx context.Context, paymentID string) {
	_ = a.Tx.Do(ctx, func(ctx context.Context) error {
		now, err := a.Payments.Lock(ctx, paymentID)
		if err != nil || now.Status != domain.PaymentProcessing {
			return err
		}
		return a.emit(ctx, now.MerchantID, now.Livemode, "payment.processing", now.ID, domain.PaymentResource(now), "")
	})
}
