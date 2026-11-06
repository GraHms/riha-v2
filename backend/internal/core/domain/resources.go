package domain

import (
	"strings"
	"time"
)

type M = map[string]any

func null(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func CheckoutURL(publicURL, id string) string { return strings.TrimRight(publicURL, "/") + "/c/" + id }

func LinkURL(publicURL, merchantSlug, slug string) string {
	return strings.TrimRight(publicURL, "/") + "/p/" + merchantSlug + "/" + slug
}

func meta(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func SessionResource(cs *CheckoutSession, publicURL string) M {
	var customer any
	if cs.CustomerName != "" || cs.CustomerPhone != "" || cs.CustomerEmail != "" {
		customer = M{"name": cs.CustomerName, "phone": cs.CustomerPhone, "email": cs.CustomerEmail}
	}
	items := cs.LineItems
	if items == nil {
		items = []LineItem{}
	}
	return M{
		"id": cs.ID, "object": "checkout.session", "status": cs.Status, "amount": cs.Amount, "currency": cs.Currency, "submit_type": FirstNonEmpty(cs.SubmitType, SubmitPay),
		"description": cs.Description, "url": CheckoutURL(publicURL, cs.ID), "client_reference_id": null(cs.ClientReferenceID),
		"success_url": null(cs.SuccessURL), "cancel_url": null(cs.CancelURL), "callback_url": null(cs.CallbackURL),
		"customer": customer, "line_items": items, "metadata": meta(cs.Metadata), "payment_method_types": cs.PaymentMethodTypes,
		"payment": cs.PaymentID, "expires_at": cs.ExpiresAt.Unix(), "created": cs.CreatedAt.Unix(), "livemode": cs.Livemode,
		"locale": cs.Locale,
	}
}

func PaymentSource(p *Payment) M {
	src := M{"type": p.SourceType}
	switch p.SourceType {
	case SourceAPI:
		src["checkout_session"] = p.SessionID
		src["api_key"] = p.APIKeyID
		src["origin"] = M{"domain": null(p.Origin.Domain), "ip": null(p.Origin.IP), "user_agent": null(p.Origin.UserAgent)}
	case SourceLink, SourceQR:
		src["payment_link"] = p.PaymentLinkID
		src["created_by"] = null(p.CreatedBy)
		src["created_via"] = "dashboard"
		src["channel"] = null(p.Channel)
	case SourceDashboard:
		src["created_by"] = null(p.CreatedBy)
	}
	return src
}

func PaymentResource(p *Payment) M {
	pm := M{"type": p.Method}
	if p.Method == MethodCard {
		pm["card"] = M{"brand": p.CardBrand, "last4": p.CardLast4}
	} else {
		pm[p.Method] = M{"phone_last4": p.PhoneLast4, "operator_reference": null(p.OperatorReference)}
	}
	return M{
		"id": p.ID, "object": "payment", "reference": p.Reference, "status": p.Status, "amount": p.Amount,
		"fee": p.Fee, "fee_bps": p.FeeBps, "net": p.Net, "amount_refunded": p.AmountRefunded, "currency": "MZN",
		"payment_method": pm, "source": PaymentSource(p), "checkout_session": p.SessionID,
		"client_reference_id": null(p.ClientReferenceID), "description": null(p.Description),
		"failure_code": null(p.FailureCode), "failure_message": null(p.FailureMessage),
		"metadata": meta(p.Metadata), "created": p.CreatedAt.Unix(), "livemode": p.Livemode,
	}
}

func PayoutResource(p *Payout) M {
	nib := p.NIB
	if len(nib) > 5 {
		nib = "•••• " + nib[len(nib)-5:]
	}
	var paid any
	if p.PaidAt != nil {
		paid = p.PaidAt.Unix()
	}
	return M{
		"id": p.ID, "object": "payout", "reference": p.Ref(), "status": p.Status, "amount": p.Amount, "currency": "MZN",
		"bank_account": M{"bank": p.BankName, "holder": p.Holder, "nib": nib},
		"requested_at": p.RequestedAt.Unix(), "due_at": p.DueAt.Unix(), "failure_reason": null(p.FailureReason),
		"livemode": p.Livemode, "paid_at": paid,
	}
}

func RefundResource(r *Refund) M {
	return M{"id": r.ID, "object": "refund", "payment": r.PaymentID, "amount": r.Amount, "currency": "MZN",
		"reason": null(r.Reason), "status": r.Status, "created": r.CreatedAt.Unix(), "livemode": r.Livemode}
}

func LinkResource(l *PaymentLink, publicURL, merchantSlug string) M {
	var amount any = l.Amount
	if l.CustomAmount {
		amount = nil
	}
	var exp any
	if l.ExpiresAt != nil {
		exp = l.ExpiresAt.Unix()
	}
	return M{
		"id": l.ID, "object": "payment_link", "name": l.Name, "slug": l.Slug, "amount": amount, "custom_amount": l.CustomAmount,
		"reusable": l.Reusable, "payment_method_types": l.Methods, "active": l.Active, "expires_at": exp, "type": l.Kind,
		"url": LinkURL(publicURL, merchantSlug, l.Slug), "livemode": l.Livemode, "created": l.CreatedAt.Unix(),
		"details": l.Details, "suggested_amount": nonZero(l.SuggestedAmount), "min_amount": nonZero(l.MinCustom), "max_amount": nonZero(l.MaxCustom),
		"after_message": l.AfterMessage, "redirect_url": l.RedirectURL, "archived": l.Archived, "submit_type": FirstNonEmpty(l.SubmitType, SubmitPay),
	}
}

func EndpointResource(e *WebhookEndpoint) M {
	status := "disabled"
	if e.Active {
		status = "enabled"
	}
	return M{"id": e.ID, "object": "webhook_endpoint", "url": e.URL, "enabled_events": e.Events, "status": status, "livemode": e.Livemode, "created": e.CreatedAt.Unix()}
}

func AccountResource(m *Merchant) M { return M{"id": m.ID, "object": "account", "status": m.Status} }

func unix(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Unix()
}

func PlanURL(publicURL, merchantSlug, slug string) string {
	return strings.TrimRight(publicURL, "/") + "/s/" + merchantSlug + "/" + slug
}

func PlanResource(p *Plan, publicURL, merchantSlug string) M {
	return M{"id": p.ID, "object": "plan", "livemode": p.Livemode, "name": p.Name, "status": p.Status, "amount": p.Amount, "currency": "MZN",
		"interval": p.Interval, "anchor": p.Anchor, "billing_day": p.BillingDay, "months": p.Months, "missed": p.Missed, "fields": p.Fields,
		"ends_at": unix(p.EndsAt), "version": p.Version, "in_catalog": p.InCatalog, "url": PlanURL(publicURL, merchantSlug, p.Slug), "created": p.CreatedAt.Unix()}
}

func SubscriptionResource(s *Subscription, m *Member) M {
	out := M{"id": s.ID, "object": "subscription", "livemode": s.Livemode, "plan": s.PlanID, "plan_version": s.PlanVersion, "amount": s.Amount,
		"currency": "MZN", "status": s.Status, "member": s.MemberID, "next_due": unix(s.NextDueAt), "paused_until": unix(s.PausedUntil),
		"canceled_at": unix(s.CanceledAt), "consent_at": unix(s.ConsentAt), "created": s.CreatedAt.Unix()}
	if s.PendingFrom != nil {
		out["pending_change"] = M{"amount": s.PendingAmount, "version": s.PendingVersion, "effective": s.PendingFrom.Unix()}
	}
	if m != nil {
		out["customer"] = M{"name": m.Name, "payer_name": null(m.PayerName), "phone": "+258" + m.Phone, "fields": meta(m.Fields), "channel": m.Channel}
	}
	return out
}

func ChargeResource(c *Charge, publicURL string) M {
	out := M{"id": c.ID, "object": "subscription_charge", "livemode": c.Livemode, "subscription": c.SubscriptionID, "member": c.MemberID,
		"plan": c.PlanID, "period": c.Period, "label": c.Label, "due": c.DueAt.Unix(), "amount": c.Amount, "currency": "MZN", "status": c.Status,
		"paid_at": unix(c.PaidAt), "payment": c.PaymentID, "method": null(c.Method), "created": c.CreatedAt.Unix()}
	if c.Unpaid() {
		out["url"] = strings.TrimRight(publicURL, "/") + "/i/" + c.Token
	}
	if c.Status == ChargePaidOutside {
		out["paid_outside"] = M{"method": c.OutsideMethod, "reference": null(c.OutsideRef), "marked_by": c.MarkedBy, "proof_sha256": null(c.OutsideProofHash)}
	}
	return out
}

func nonZero(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
