package httpapi

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

func (srv *Server) merchantJSON(c context.Context, m *domain.Merchant) gin.H {
	bps := srv.A.FeeBpsFor(c, m)
	return gin.H{
		"id": m.ID, "slug": m.Slug, "name": m.Name, "legal_name": m.LegalName, "business_type": m.BusinessType, "nuit": m.NUIT,
		"phone": m.Phone, "email": m.Email, "city": m.City, "owner_name": m.OwnerName, "expected_volume": m.ExpectedVolume,
		"statement_descriptor": m.StatementDescriptor, "avatar_color": m.AvatarColor, "status": m.Status,
		"fee_bps": bps, "fee_negotiated": m.FeeBps != nil, "fee_label": domain.FormatBps(bps),
		"success_url": m.SuccessURL, "cancel_url": m.CancelURL, "sms_notifications": m.SMSNotifications, "email_notifications": m.EmailNotifications,
		"coming_soon": srv.A.ComingSoon, "live_refunds": !srv.A.NoLiveRefunds,
		"beta": m.Beta, "min_amount": domain.MinAmountFor(m), "plan_min_amount": domain.PlanMinFor(m),
		"kyc_submitted_at": iso(m.KycSubmittedAt), "approved_at": iso(m.ApprovedAt), "review_note": m.ReviewNote,
		"created_at": isoT(m.CreatedAt),
	}
}

func paymentRow(v *app.PaymentView) gin.H {
	o := domain.PaymentResource(&v.Payment)
	o["method"] = v.Method
	o["created_at"] = isoT(v.CreatedAt)
	o["customer_name"] = v.CustomerName
	o["phone_prefix"] = v.PhonePrefix
	o["phone_last4"] = v.PhoneLast4
	o["card_brand"] = v.CardBrand
	o["card_last4"] = v.CardLast4
	if v.LinkName != "" {
		o["link_name"], o["link_slug"] = v.LinkName, v.LinkSlug
	}
	return o
}

func paymentRows(vs []app.PaymentView) []any {
	out := make([]any, 0, len(vs))
	for i := range vs {
		out = append(out, paymentRow(&vs[i]))
	}
	return out
}

func deliveryJSON(x *domain.Delivery) gin.H {
	return gin.H{"id": x.ID, "event_id": x.EventID, "event_type": x.EventType, "target": x.Target, "url": x.URL, "status": x.Status,
		"attempts": x.Attempts, "last_status_code": x.LastStatusCode, "last_error": x.LastError, "duration_ms": x.LastDurationMs,
		"next_attempt_at": iso(x.NextAttemptAt), "created_at": isoT(x.CreatedAt), "updated_at": isoT(x.UpdatedAt)}
}

func bankJSON(b *domain.BankAccount) gin.H {
	if b == nil {
		return nil
	}
	return gin.H{"bank_name": b.BankName, "holder": b.Holder, "nib": b.NIB, "nib_last5": b.NIB[len(b.NIB)-5:], "verified": b.Verified, "document_id": b.DocumentID, "updated_at": b.UpdatedAt}
}

func (srv *Server) linkJSON(v app.LinkView, merchantSlug string) gin.H {
	o := domain.LinkResource(&v.PaymentLink, srv.A.PayURL, merchantSlug)
	o["payments_count"] = v.Count
	o["total_received"] = v.Total
	o["location"] = v.Location
	o["last_payment_at"] = iso(v.Last)
	o["image_url"] = linkImageURL(&v.PaymentLink, merchantSlug)
	return o
}

func linkImageURL(l *domain.PaymentLink, merchantSlug string) string {
	if l.Image == "" {
		return ""
	}
	return "/api/public/links/" + merchantSlug + "/" + l.Slug + "/image?v=" + version(l.Image)
}

func payoutJSON(p *domain.Payout) gin.H {
	o := domain.PayoutResource(p)
	o["requested_at_iso"] = isoT(p.RequestedAt)
	o["has_receipt"] = p.HasReceipt()
	o["receipt_name"] = p.ReceiptName
	o["emailed_at"] = iso(p.EmailedAt)
	o["paid_at_iso"] = iso(p.PaidAt)
	return o
}

func payoutRowJSON(r *app.PayoutRow) gin.H {
	o := payoutJSON(&r.Payout)
	o["merchant_id"] = r.MerchantID
	o["merchant_name"] = r.MerchantName
	o["due_at_iso"] = isoT(r.DueAt)
	o["nib_last4"] = r.NIB[len(r.NIB)-4:]
	return o
}

func payoutRowsJSON(rs []app.PayoutRow) []gin.H {
	out := make([]gin.H, 0, len(rs))
	for i := range rs {
		out = append(out, payoutRowJSON(&rs[i]))
	}
	return out
}

func docsJSON(ds []domain.Document) []gin.H {
	out := []gin.H{}
	for _, d := range ds {
		out = append(out, gin.H{"id": d.ID, "kind": d.Kind, "file_name": d.FileName, "size": d.Size, "content_type": d.ContentType, "created_at": d.CreatedAt})
	}
	return out
}

func applicationJSON(a *app.Application, full bool) gin.H {
	m := &a.Merchant
	o := gin.H{"id": m.ID, "name": m.Name, "legal_name": m.LegalName, "owner_name": m.OwnerName, "business_type": m.BusinessType,
		"nuit": m.NUIT, "city": m.City, "phone": m.Phone, "email": m.Email, "expected_volume": m.ExpectedVolume,
		"avatar_color": m.AvatarColor, "initials": domain.Initials(m.Name), "status": m.Status, "risk": a.Risk,
		"submitted_at": iso(m.KycSubmittedAt), "created_at": isoT(m.CreatedAt), "review_note": m.ReviewNote}
	if full {
		o["checks"] = a.Checks
		o["documents"] = docsJSON(a.Documents)
		if a.Bank != nil {
			o["bank_account"] = bankJSON(a.Bank)
		}
	}
	return o
}

func merchantRowJSON(r *app.MerchantRow) gin.H {
	m := &r.Merchant
	return gin.H{"id": m.ID, "name": m.Name, "city": m.City, "status": m.Status, "initials": domain.Initials(m.Name), "avatar_color": m.AvatarColor,
		"volume_30d": r.Volume30d, "balance": r.Balance, "main_method": r.MainMethod, "created_at": isoT(m.CreatedAt),
		"fee_bps": r.FeeBps, "fee_negotiated": m.FeeBps != nil, "email": m.Email, "beta": m.Beta, "imported_from": m.ImportedFrom}
}

func publicMerchant(m *domain.Merchant) gin.H {
	return gin.H{"name": m.Name, "initials": domain.Initials(m.Name), "avatar_color": m.AvatarColor, "descriptor": m.StatementDescriptor, "slug": m.Slug}
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		return strings.TrimPrefix(u.Hostname(), "www.")
	}
	return ""
}

func (srv *Server) sessionView(c context.Context, cs *domain.CheckoutSession) gin.H {
	m, _ := srv.A.Merchants.ByID(c, cs.MerchantID)
	if m == nil {
		m = &domain.Merchant{}
	}
	status := cs.Status
	if cs.Status == domain.SessionOpen && !cs.ExpiresAt.After(time.Now()) {
		status = domain.SessionExpired
	}
	success := strings.ReplaceAll(cs.SuccessURL, "{CHECKOUT_SESSION_ID}", cs.ID)
	if success != "" && !strings.Contains(cs.SuccessURL, "{CHECKOUT_SESSION_ID}") {
		sep := "?"
		if strings.Contains(success, "?") {
			sep = "&"
		}
		success += sep + "session_id=" + cs.ID
	}
	items := cs.LineItems
	if items == nil {
		items = []domain.LineItem{}
	}
	out := gin.H{
		"id": cs.ID, "status": status, "amount": cs.Amount, "currency": cs.Currency, "description": cs.Description,
		"line_items": items, "payment_method_types": srv.A.ActiveMethods(cs.PaymentMethodTypes), "coming_soon": srv.A.ComingSoon, "livemode": cs.Livemode,
		"success_url": success, "cancel_url": cs.CancelURL, "return_host": hostOf(domain.FirstNonEmpty(cs.SuccessURL, cs.CancelURL)),
		"client_reference_id": cs.ClientReferenceID, "customer_phone": strings.TrimPrefix(cs.CustomerPhone, "+258"),
		"customer_name": cs.CustomerName, "email_on_file": cs.CustomerEmail != "", "expires_at": isoT(cs.ExpiresAt), "merchant": publicMerchant(m), "payment": nil,
	}
	out["success_message"] = cs.SuccessMessage
	out["submit_type"] = domain.FirstNonEmpty(cs.SubmitType, domain.SubmitPay)
	if cs.PaymentLinkID != nil {
		if l, err := srv.A.Links.ByID(c, *cs.PaymentLinkID); err == nil && (l.Details != "" || l.Image != "") {
			out["link"] = gin.H{"details": l.Details, "image_url": linkImageURL(l, m.Slug)}
		}
	}
	if p := srv.A.LatestPayment(c, cs.ID); p != nil {
		out["payment"] = gin.H{"id": p.ID, "reference": p.Reference, "attempt_id": p.AttemptID, "status": p.Status, "method": p.Method, "phone_prefix": p.PhonePrefix,
			"phone_last4": p.PhoneLast4, "card_brand": p.CardBrand, "card_last4": p.CardLast4, "failure_code": p.FailureCode,
			"failure_message": p.FailureMessage, "created_at": isoT(p.CreatedAt), "succeeded_at": iso(p.SucceededAt), "stage": waitStage(p)}
	}
	return out
}

// waitStage is where a payment in progress is, for the checkout's waiting screen. bim
// holds the call to it while the customer answers on the phone, and answers the moment
// they do, so a rail id means the customer is done and the wallet is confirming.
func waitStage(p *domain.Payment) string {
	switch {
	case p.Status != domain.PaymentProcessing:
		return ""
	case p.RailID != "":
		return "answered"
	}
	return "sent"
}
