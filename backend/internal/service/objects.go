package service

import (
	"strings"

	"riha/internal/models"
	"riha/internal/providers"
)

var timeoutOutcome = providers.Outcome{FailureCode: providers.FailCustomerTimeout}

// CheckoutURL is where customers are sent for a session.
func CheckoutURL(publicURL, id string) string { return strings.TrimRight(publicURL, "/") + "/c/" + id }

func SessionObject(cs *models.CheckoutSession, publicURL string) map[string]any {
	var customer any
	if cs.CustomerName != "" || cs.CustomerPhone != "" || cs.CustomerEmail != "" {
		customer = map[string]any{"name": cs.CustomerName, "phone": cs.CustomerPhone, "email": cs.CustomerEmail}
	}
	o := map[string]any{
		"id": cs.ID, "object": "checkout.session", "status": cs.Status, "amount": cs.Amount, "currency": cs.Currency,
		"description": cs.Description, "url": CheckoutURL(publicURL, cs.ID), "client_reference_id": nullable(cs.ClientReferenceID),
		"success_url": nullable(cs.SuccessURL), "cancel_url": nullable(cs.CancelURL), "callback_url": nullable(cs.CallbackURL),
		"customer": customer, "line_items": cs.LineItems, "metadata": cs.Metadata, "payment_method_types": cs.PaymentMethodTypes,
		"payment": cs.PaymentID, "expires_at": cs.ExpiresAt.Unix(), "created": cs.CreatedAt.Unix(), "livemode": cs.Livemode,
		"locale": cs.Locale,
	}
	if o["line_items"] == nil || len(cs.LineItems) == 0 {
		o["line_items"] = []any{}
	}
	return o
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func PaymentSource(p *models.Payment) map[string]any {
	src := map[string]any{"type": p.SourceType}
	switch p.SourceType {
	case models.SourceAPI:
		src["checkout_session"] = p.SessionID
		src["api_key"] = p.APIKeyID
		src["origin"] = map[string]any{"domain": nullable(p.OriginDomain), "ip": nullable(p.OriginIP), "user_agent": nullable(p.OriginUserAgent)}
	case models.SourceLink, models.SourceQR:
		src["payment_link"] = p.PaymentLinkID
		src["created_by"] = nullable(p.CreatedBy)
		src["created_via"] = "dashboard"
		src["channel"] = nullable(p.Channel)
	case models.SourceDashboard:
		src["created_by"] = nullable(p.CreatedBy)
	}
	return src
}

func PaymentObject(p *models.Payment) map[string]any {
	pm := map[string]any{"type": p.Method}
	switch p.Method {
	case providers.MethodCard:
		pm["card"] = map[string]any{"brand": p.CardBrand, "last4": p.CardLast4}
	default:
		pm[p.Method] = map[string]any{"phone_last4": p.PhoneLast4, "operator_reference": nullable(p.OperatorReference)}
	}
	return map[string]any{
		"id": p.ID, "object": "payment", "reference": p.Reference, "status": p.Status, "amount": p.Amount,
		"fee": p.Fee, "fee_bps": p.FeeBps, "net": p.Net, "amount_refunded": p.AmountRefunded, "currency": "MZN",
		"payment_method": pm, "source": PaymentSource(p), "checkout_session": p.SessionID,
		"client_reference_id": nullable(p.ClientReferenceID), "description": nullable(p.Description),
		"failure_code": nullable(p.FailureCode), "failure_message": nullable(p.FailureMessage),
		"metadata": p.Metadata, "created": p.CreatedAt.Unix(), "livemode": p.Livemode,
	}
}

func PayoutObject(p *models.Payout) map[string]any {
	nib := p.NIB
	if len(nib) > 5 {
		nib = "•••• " + nib[len(nib)-5:]
	}
	o := map[string]any{
		"id": p.ID, "object": "payout", "reference": PayoutRef(p), "status": p.Status, "amount": p.Amount, "currency": "MZN",
		"bank_account": map[string]any{"bank": p.BankName, "holder": p.Holder, "nib": nib},
		"requested_at": p.RequestedAt.Unix(), "due_at": p.DueAt.Unix(), "failure_reason": nullable(p.FailureReason), "livemode": p.Livemode,
	}
	if p.PaidAt != nil {
		o["paid_at"] = p.PaidAt.Unix()
	} else {
		o["paid_at"] = nil
	}
	return o
}

func RefundObject(r *models.Refund) map[string]any {
	return map[string]any{"id": r.ID, "object": "refund", "payment": r.PaymentID, "amount": r.Amount, "currency": "MZN",
		"reason": nullable(r.Reason), "status": r.Status, "created": r.CreatedAt.Unix(), "livemode": r.Livemode}
}

func LinkObject(l *models.PaymentLink, publicURL, merchantSlug string) map[string]any {
	var amount any = l.Amount
	if l.CustomAmount {
		amount = nil
	}
	var exp any
	if l.ExpiresAt != nil {
		exp = l.ExpiresAt.Unix()
	}
	return map[string]any{
		"id": l.ID, "object": "payment_link", "name": l.Name, "slug": l.Slug, "amount": amount, "custom_amount": l.CustomAmount,
		"reusable": l.Reusable, "payment_method_types": l.Methods, "active": l.Active, "expires_at": exp, "type": l.Kind,
		"url": strings.TrimRight(publicURL, "/") + "/p/" + merchantSlug + "/" + l.Slug, "livemode": l.Livemode, "created": l.CreatedAt.Unix(),
	}
}

func EndpointObject(e *models.WebhookEndpoint) map[string]any {
	return map[string]any{"id": e.ID, "object": "webhook_endpoint", "url": e.URL, "enabled_events": e.Events, "status": map[bool]string{true: "enabled", false: "disabled"}[e.Active], "livemode": e.Livemode, "created": e.CreatedAt.Unix()}
}
