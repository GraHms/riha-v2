package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// RailCallback is told that something happened to an operation. Anyone can reach the
// address it comes from, so the callback's own words are never believed: this only
// asks the operator straight away, about RIHA's own operation number and the ids
// the operator gave RIHA, and its answer decides. An id in the callback is never
// asked about nor kept: anyone could send the id of another, cheaper, paid
// transaction. Polling and the sweep still run, so a callback that never comes
// costs only time.
func (a *App) RailCallback(ctx context.Context, operation, railID string, raw []byte) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	p, err := a.Payments.ByRailReference(ctx, operation)
	ev := callbackEvent(ctx, operation, raw)
	if err != nil {
		ev.Level, ev.Title = domain.TrailWarn, ev.Title+". Não há pagamento com esta operação."
		a.trace(ctx, ev)
		log.Printf("rail callback: no payment for operation %q", operation)
		return
	}
	ev.PaymentID, ev.MerchantID, ev.SessionID, ev.Livemode = p.ID, p.MerchantID, p.SessionID, p.Livemode
	a.trace(ctx, ev)
	rail, ok := a.Rail.(ports.RailQuery)
	if !ok {
		return
	}
	if railID != "" && railID != p.RailID {
		a.logPayment(ctx, p.ID, domain.TrailInfo, "rail callback: payment %s (%s) brought transaction id %q; asking the operator by RIHA's own references", p.ID, operation, railID)
	}
	o, known, err := rail.Query(ctx, domain.StatusQuery{PaymentID: p.ID, Live: p.Livemode, Reference: p.RailReference, RailID: p.RailID, Method: p.Method, Integration: p.Integration, Urgent: true, Amount: p.Amount})
	switch {
	case errors.Is(err, ports.ErrStillPending):
		a.logPayment(ctx, p.ID, domain.TrailInfo, "rail callback: payment %s (%s) is still pending at the operator", p.ID, operation)
	case err != nil:
		a.logPayment(ctx, p.ID, domain.TrailWarn, "rail callback: payment %s (%s): %v", p.ID, operation, err)
	case !known:
		a.logPayment(ctx, p.ID, domain.TrailWarn, "rail callback: the operator has no answer for payment %s (%s)", p.ID, operation)
	case p.Status != domain.PaymentProcessing && p.Status != domain.PaymentUnconfirmed:
		if o.Success && p.Status != domain.PaymentSucceeded {
			a.logPayment(ctx, p.ID, domain.TrailError, "payment %s (%s): PAID AFTER IT WAS CLOSED, %d cents from a customer, operator reference %s. CHECK BY HAND: the operator check on its trail credits it", p.ID, operation, p.Amount, o.OperatorReference)
			return
		}
		a.logPayment(ctx, p.ID, domain.TrailInfo, "rail callback: payment %s (%s) has already ended as %s", p.ID, operation, p.Status)
	default:
		if err := a.CompletePayment(ctx, p.ID, o); err != nil {
			a.logPayment(ctx, p.ID, domain.TrailError, "rail callback: payment %s (%s): %v", p.ID, operation, err)
			return
		}
		log.Printf("rail callback: payment %s (%s) ended, success %v, code %q", p.ID, operation, o.Success, o.FailureCode)
	}
}

var bimTypes = map[string]string{"M": "M-Pesa", "E": "e-Mola", "I": "Bim IZI"}

// callbackEvent reads what the operator said in a callback, for the trail. What it
// says is kept as it was sent, numbers masked, and summed up in the title.
func callbackEvent(ctx context.Context, operation string, raw []byte) domain.TrailEvent {
	r := domain.RequestFrom(ctx)
	ev := domain.TrailEvent{Kind: domain.TrailRailCallback, Source: "operator", Method: "POST", Path: "/callbacks/bim", IP: r.IP, UserAgent: r.UserAgent,
		Request: domain.RedactBody(raw), Data: map[string]any{"operation": operation}}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	str := func(k string) string {
		switch v := m[k].(type) {
		case string:
			return v
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
		return ""
	}
	wallet := domain.FirstNonEmpty(bimTypes[str("paymentType")], str("paymentType"))
	if rej := str("rejection_reason"); rej != "" {
		ev.Title = "O bim avisou: recusado, código " + rej
		ev.Data["rejection_reason"] = rej
	} else {
		ev.Title = "O bim avisou: pago"
		if amt := str("amount"); amt != "" {
			ev.Title += ", " + amt + " MT"
		}
		if c := str("comissionAmount"); c != "" {
			ev.Title += ", comissão " + c + " MT"
			ev.Data["commission"] = c
		}
	}
	if wallet != "" {
		ev.Title += " (" + wallet + ")"
		ev.Data["wallet"] = wallet
	}
	for _, k := range []string{"amount", "paymentTimestamp", "rejectTimestamp", "transactionID_NEP"} {
		if v := str(k); v != "" {
			ev.Data[k] = v
		}
	}
	return ev
}

// TraceCallbackWithoutOperation keeps a callback nobody could match to an operation.
func (a *App) TraceCallbackWithoutOperation(ctx context.Context, method string, raw []byte) {
	r := domain.RequestFrom(ctx)
	a.trace(ctx, domain.TrailEvent{Kind: domain.TrailRailCallback, Source: "operator", Level: domain.TrailWarn, Method: method, Path: "/callbacks/bim", IP: r.IP, UserAgent: r.UserAgent,
		Title: "Aviso recebido sem número de operação", Request: domain.RedactBody(raw)})
}
