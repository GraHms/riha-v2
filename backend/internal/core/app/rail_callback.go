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
// asks the operator, and its answer decides.
func (a *App) RailCallback(ctx context.Context, operation string) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	p, err := a.Payments.ByRailReference(ctx, operation)
	if err != nil {
		log.Printf("rail callback: no payment for operation %q", operation)
		return
	}
	if p.Status != domain.PaymentProcessing {
		log.Printf("rail callback: payment %s (%s) has already ended as %s", p.ID, operation, p.Status)
		return
	}
	rail, ok := a.Rail.(ports.RailQuery)
	if !ok {
		return
	}
	o, known, err := rail.Query(ctx, domain.StatusQuery{PaymentID: p.ID, Live: p.Livemode, Reference: p.RailReference, RailID: p.RailID, Method: p.Method})
	switch {
	case errors.Is(err, ports.ErrStillPending):
		log.Printf("rail callback: payment %s (%s) is still pending at the operator", p.ID, operation)
	case err != nil:
		log.Printf("rail callback: payment %s (%s): %v", p.ID, operation, err)
	case !known:
		log.Printf("rail callback: the operator has no answer for payment %s (%s)", p.ID, operation)
	default:
		if err := a.CompletePayment(ctx, p.ID, o); err != nil {
			log.Printf("rail callback: payment %s (%s): %v", p.ID, operation, err)
			return
		}
		log.Printf("rail callback: payment %s (%s) ended, success %v, code %q", p.ID, operation, o.Success, o.FailureCode)
	}
}
