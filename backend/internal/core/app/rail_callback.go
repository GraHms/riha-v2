package app

import (
	"context"
	"errors"
	"log"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// RailCallback is told that something happened to an operation. Anyone can reach the
// address it comes from, so the callback's own words are never believed: this only
// asks the operator straight away, and its answer decides. Polling and the sweep still
// run, so a callback that never comes costs only time. The id it brings is what finds
// a payment whose create answer was lost.
func (a *App) RailCallback(ctx context.Context, operation, railID string) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	p, err := a.Payments.ByRailReference(ctx, operation)
	if err != nil {
		log.Printf("rail callback: no payment for operation %q", operation)
		return
	}
	rail, ok := a.Rail.(ports.RailQuery)
	if !ok {
		return
	}
	id := domain.FirstNonEmpty(p.RailID, railID)
	o, known, err := rail.Query(ctx, domain.StatusQuery{PaymentID: p.ID, Live: p.Livemode, Reference: p.RailReference, RailID: id, Method: p.Method, Urgent: true})
	switch {
	case errors.Is(err, ports.ErrStillPending):
		log.Printf("rail callback: payment %s (%s) is still pending at the operator", p.ID, operation)
	case err != nil:
		log.Printf("rail callback: payment %s (%s): %v", p.ID, operation, err)
	case !known:
		log.Printf("rail callback: the operator has no answer for payment %s (%s)", p.ID, operation)
	case p.Status != domain.PaymentProcessing:
		if o.Success && p.Status != domain.PaymentSucceeded {
			log.Printf("payment %s (%s): PAID AFTER IT WAS CLOSED, %d cents from a customer, operator reference %s. CHECK BY HAND and credit or refund it", p.ID, operation, p.Amount, o.OperatorReference)
			return
		}
		log.Printf("rail callback: payment %s (%s) has already ended as %s", p.ID, operation, p.Status)
	default:
		if p.RailID == "" && id != "" {
			a.keepRailID(p.ID, id)
		}
		if err := a.CompletePayment(ctx, p.ID, o); err != nil {
			log.Printf("rail callback: payment %s (%s): %v", p.ID, operation, err)
			return
		}
		log.Printf("rail callback: payment %s (%s) ended, success %v, code %q", p.ID, operation, o.Success, o.FailureCode)
	}
}
