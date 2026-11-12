package app

import (
	"context"
	"fmt"
	"strings"

	"riha/internal/core/domain"
)

// AbroadResult is what U2opia says a payment came to, in its callback.
type AbroadResult struct {
	TransactionID string
	// Status is 2 for paid and 3 for failed; anything else isn't final.
	Status    int
	Reference string
	Amount    string
	Channel   string
	Raw       []byte
}

// AbroadCallback settles a payment from abroad on U2opia's word. Its callbacks
// can't be checked yet (no signature, no status call), so this only settles
// test-mode payments: anyone could claim a live one was paid.
func (a *App) AbroadCallback(ctx context.Context, r AbroadResult) {
	ev := domain.TrailEvent{Kind: domain.TrailRailCallback, Source: "operator", Method: "POST", Path: "/callbacks/u2opia", Response: domain.RedactBody(r.Raw),
		Data: map[string]any{"transaction": r.TransactionID, "status": r.Status, "amount": r.Amount, "channel": r.Channel}}
	p, err := a.Payments.ByRailID(ctx, r.TransactionID)
	if err != nil || !domain.Contains(a.AbroadMethods, p.Method) {
		ev.Level, ev.Title = domain.TrailWarn, fmt.Sprintf("Aviso da U2opia sobre a transação %q, que não é de nenhum pagamento", r.TransactionID)
		a.trace(ctx, ev)
		return
	}
	ev.PaymentID, ev.MerchantID, ev.SessionID, ev.Livemode = p.ID, p.MerchantID, p.SessionID, p.Livemode
	switch r.Status {
	case 2:
		ev.Title = "Aviso da U2opia: pago"
	case 3:
		ev.Title = "Aviso da U2opia: falhou"
	default:
		ev.Title = fmt.Sprintf("Aviso da U2opia com o estado %d, que não é final", r.Status)
	}
	if p.Livemode {
		ev.Level, ev.Title = domain.TrailError, ev.Title+". Ignorado: os avisos da U2opia ainda não podem ser verificados, por isso não contam com dinheiro real."
	}
	a.trace(ctx, ev)
	if p.Livemode || (r.Status != 2 && r.Status != 3) {
		return
	}
	o := domain.Outcome{Success: r.Status == 2, OperatorReference: domain.FirstNonEmpty(r.Reference, r.TransactionID)}
	if !o.Success {
		o.FailureCode = domain.FailRejected
	}
	if err := a.CompletePayment(ctx, p.ID, o); err != nil {
		a.logPayment(ctx, p.ID, domain.TrailError, "u2opia callback: payment %s: %v", p.ID, err)
	}
}

// AbroadReturn is where a customer back from U2opia's page goes: their
// checkout, which shows how the payment stands. U2opia's own status in the
// address is only shown on the trail, never trusted.
func (a *App) AbroadReturn(ctx context.Context, transactionID, status string) string {
	p, err := a.Payments.ByRailID(ctx, transactionID)
	if err != nil {
		return strings.TrimRight(a.PayURL, "/") + "/"
	}
	a.trace(ctx, domain.TrailEvent{MerchantID: p.MerchantID, Livemode: p.Livemode, SessionID: p.SessionID, PaymentID: p.ID, Kind: domain.TrailClient, Source: "customer",
		Title: "O cliente voltou da página da U2opia, que lhe mostrou: " + domain.FirstNonEmpty(status, "sem estado")})
	return strings.TrimRight(a.PayURL, "/") + "/c/" + p.SessionID
}
