package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func (a *App) PublishableKey(ctx context.Context, merchantID string, live bool) string {
	k, err := a.Keys.Active(ctx, merchantID, live, domain.KeyPublishable)
	if err != nil {
		return ""
	}
	return k.Token
}

func (a *App) SecretKey(ctx context.Context, merchantID string, live bool) *domain.APIKey {
	k, err := a.Keys.Active(ctx, merchantID, live, domain.KeySecret)
	if err != nil {
		return nil
	}
	return k
}

func (a *App) APIKey(ctx context.Context, id string) *domain.APIKey {
	k, err := a.Keys.ByID(ctx, id)
	if err != nil {
		return nil
	}
	return k
}

func (a *App) RollSecretKey(ctx context.Context, m *domain.Merchant, live bool, actor Actor) (string, *domain.APIKey, error) {
	if e := domain.RequireApproved(m); e != nil {
		return "", nil, e
	}
	secret := domain.Token("sk_"+domain.Mode(live)+"_", 32)
	k := &domain.APIKey{MerchantID: m.ID, Livemode: live, Kind: domain.KeySecret, Hash: domain.Hash(secret), Last4: secret[len(secret)-4:], CreatedBy: actor.ID, CreatedAt: a.Now()}
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		if err := a.Keys.RevokeActive(ctx, m.ID, live, domain.KeySecret, a.Now()); err != nil {
			return err
		}
		for {
			k.ID = "key_" + strings.ToLower(domain.ShortCode(4))
			if _, err := a.Keys.ByID(ctx, k.ID); err != nil {
				break
			}
		}
		return a.Keys.Create(ctx, k)
	})
	if err != nil {
		return "", nil, err
	}
	a.Record(ctx, m.ID, actor, "api_key.rolled", k.ID, domain.Mode(live))
	if live {
		a.mailKeyRolled(ctx, m, k)
	}
	return secret, k, nil
}

func (a *App) AuthenticateSecret(ctx context.Context, secret string) (*domain.APIKey, *domain.Merchant, error) {
	if strings.HasPrefix(secret, "pk_") {
		return nil, nil, domain.Unauthorized("publishable_key_not_allowed", "A chave pública não pode criar pagamentos. Use a chave secreta no servidor.")
	}
	if !strings.HasPrefix(secret, "sk_test_") && !strings.HasPrefix(secret, "sk_live_") {
		return nil, nil, domain.ErrBadKey
	}
	k, err := a.Keys.BySecretHash(ctx, domain.Hash(secret))
	if err != nil {
		return nil, nil, domain.ErrBadKey
	}
	m, err := a.Merchants.ByID(ctx, k.MerchantID)
	if err != nil {
		return nil, nil, domain.ErrBadKey
	}
	if e := domain.RequireApproved(m); e != nil {
		return nil, nil, e
	}
	_ = a.Keys.Touch(ctx, k.ID, a.Now())
	return k, m, nil
}

func (a *App) LogRequest(ctx context.Context, l *domain.RequestLog) {
	l.ID, l.CreatedAt = domain.NewID("rlog"), a.Now()
	_ = a.RequestLogs.Create(ctx, l)
}

func (a *App) RecentRequests(ctx context.Context, merchantID string, live bool) []domain.RequestLog {
	r, _ := a.RequestLogs.Recent(ctx, merchantID, live, 20)
	return r
}

func (a *App) IdempotentResponse(ctx context.Context, merchantID string, live bool, key string) *domain.IdempotencyRecord {
	r, err := a.Idempotency.Get(ctx, merchantID, live, key)
	if err != nil {
		return nil
	}
	return r
}

func (a *App) StoreIdempotent(ctx context.Context, r *domain.IdempotencyRecord) {
	r.ID, r.CreatedAt = domain.NewID("idem"), a.Now()
	_ = a.Idempotency.Save(ctx, r)
}

func (a *App) DomainList(ctx context.Context, merchantID string) []domain.AllowedDomain {
	d, _ := a.Domains.List(ctx, merchantID)
	return d
}

func (a *App) RemoveDomain(ctx context.Context, merchantID, id string) error {
	return a.Domains.Delete(ctx, merchantID, id)
}

func (a *App) SetRedirects(ctx context.Context, m *domain.Merchant, success, cancel string) error {
	for _, u := range []string{success, cancel} {
		if u != "" && !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
			return domain.Invalid("url_invalid", "success_url", "Os URLs têm de começar por https://")
		}
	}
	m.SuccessURL, m.CancelURL = strings.TrimSpace(success), strings.TrimSpace(cancel)
	return a.Merchants.Save(ctx, m)
}

func (a *App) RecentDeliveries(ctx context.Context, merchantID string, live bool) []domain.Delivery {
	d, _ := a.Deliveries.Recent(ctx, merchantID, live, 30)
	return d
}

func (a *App) Resend(ctx context.Context, merchantID, id string) error {
	ok, err := a.Deliveries.Requeue(ctx, merchantID, id, a.Now())
	if err != nil {
		return err
	}
	if !ok {
		return domain.NotFound("A entrega")
	}
	return nil
}

func (a *App) SendTestEvent(ctx context.Context, merchantID string, live bool) error {
	eps, _ := a.Endpoints.List(ctx, merchantID, live)
	active := false
	for _, e := range eps {
		active = active || e.Active
	}
	if !active {
		return domain.Invalid("parameter_invalid", "", "Adicione um endpoint de webhook primeiro.")
	}
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		return a.emit(ctx, merchantID, live, "payment.succeeded", "pay_test_event", map[string]any{
			"id": "pay_test_event", "object": "payment", "status": "succeeded", "amount": 125000, "livemode": live, "test_event": true,
		}, "")
	})
}

func (a *App) Event(ctx context.Context, merchantID string, live bool, id string) (*domain.Event, error) {
	ev, err := a.Events.ByID(ctx, id)
	if err != nil || ev.MerchantID != merchantID || ev.Livemode != live {
		return nil, domain.NotFound("O evento")
	}
	return ev, nil
}

func (a *App) DeliverDue(ctx context.Context, n int) int {
	now := a.Now()
	batch, err := a.Deliveries.ClaimDue(ctx, now, now.Add(2*time.Minute), n)
	if err != nil {
		return 0
	}
	for i := range batch {
		a.deliver(ctx, &batch[i])
	}
	return len(batch)
}

func (a *App) deliver(ctx context.Context, d *domain.Delivery) {
	ev, err := a.Events.ByID(ctx, d.EventID)
	if err != nil {
		return
	}
	m, err := a.Merchants.ByID(ctx, d.MerchantID)
	if err != nil {
		return
	}
	attempt := d.Attempts + 1
	body := domain.EventPayload(ev, d.Target, attempt)
	start := time.Now()
	headers := map[string]string{
		"Content-Type": "application/json", "User-Agent": "Riha-Webhooks/1.0",
		"Riha-Signature": domain.Sign(m.SigningSecret(d.Livemode), a.Now().Unix(), body), "Riha-Event-Id": ev.ID,
	}
	var code int
	var reply []byte
	if r, ok := a.Sender.(ports.WebhookReplier); ok {
		code, reply, err = r.SendReply(ctx, d.URL, headers, body)
	} else {
		code, err = a.Sender.Send(ctx, d.URL, headers, body)
	}
	d.Attempts, d.LastStatusCode, d.LastDurationMs, d.LastError = attempt, code, time.Since(start).Milliseconds(), ""
	if err != nil {
		d.LastError = domain.Trim(err.Error(), 500)
	}
	a.traceDelivery(ctx, d, ev, body, reply, err)
	switch {
	case code >= 200 && code < 300:
		d.Status, d.NextAttemptAt = domain.DeliverySucceeded, nil
	case attempt > len(domain.RetrySchedule):
		d.Status, d.NextAttemptAt = domain.DeliveryFailed, nil
	default:
		next := a.Now().Add(domain.RetrySchedule[attempt-1])
		d.NextAttemptAt = &next
	}
	d.UpdatedAt = a.Now()
	_ = a.Deliveries.Save(ctx, d)
}

func (a *App) Sweep(ctx context.Context) {
	now := a.Now()
	stale, _ := a.Payments.StaleProcessing(ctx, now.Add(-3*time.Minute), 100)
	for i := range stale {
		a.settleStale(ctx, &stale[i])
	}
	expired, _ := a.Sessions.ExpiredOpen(ctx, now, 200)
	for _, cs := range expired {
		_, _ = a.ExpireSession(ctx, "", false, cs.ID)
	}
	tp, _ := a.Payouts.TestModeToSettle(ctx, now.Add(-20*time.Second), 50)
	for _, p := range tp {
		action := domain.ActPaid
		if p.Status == domain.PayoutNew {
			action = domain.ActApprove
		}
		_, _ = a.TransitionPayout(ctx, p.ID, action, "", "system")
	}
	_ = a.Idempotency.Purge(ctx, now.Add(-24*time.Hour))
	_ = a.AuthSessions.Purge(ctx, now)
	_ = a.Resets.Purge(ctx, now.Add(-24*time.Hour))
	_ = a.Emails.Purge(ctx, now.Add(-90*24*time.Hour))
}

const (
	// After this long with no record at the operator, nobody paid. bim holds a
	// payment for up to 5 minutes while the customer answers, so this waits longer.
	railNoRecordAfter = 7 * time.Minute
	// After this long without any answer from the operator, stop waiting.
	railGiveUpAfter = 30 * time.Minute
	// After this long still pending, the customer has moved on: an error on their
	// phone doesn't reach the status call, and they can't retry until this ends.
	railPendingGiveUpAfter = 10 * time.Minute
	// How long a payment closed as timed out is still asked about, in case it was paid late.
	railLateCheckFor = 24 * time.Hour
)

// settleStale ends a payment whose answer never came. Real money only ends on what
// the operator says: the customer may have paid while the answer was being lost.
func (a *App) settleStale(ctx context.Context, p *domain.Payment) {
	timedOut := domain.Outcome{FailureCode: domain.FailCustomerTimeout}
	rail, ok := a.Rail.(ports.RailQuery)
	if !ok || p.RailReference == "" {
		_ = a.CompletePayment(ctx, p.ID, timedOut)
		return
	}
	age := a.Now().Sub(p.CreatedAt)
	o, known, err := rail.Query(ctx, domain.StatusQuery{PaymentID: p.ID, Live: p.Livemode, Reference: p.RailReference, RailID: p.RailID, Method: p.Method, Integration: p.Integration})
	switch {
	case errors.Is(err, ports.ErrNoStatusCheck):
		_ = a.CompletePayment(ctx, p.ID, timedOut)
	case err == nil && known:
		_ = a.CompletePayment(ctx, p.ID, o)
	case err == nil && age >= railNoRecordAfter:
		a.logPayment(ctx, p.ID, domain.TrailWarn, "payment %s (%s): the operator has no answer for it, closed as timed out", p.ID, p.RailReference)
		_ = a.CompletePayment(ctx, p.ID, timedOut)
	case errors.Is(err, ports.ErrStillPending) && age >= railPendingGiveUpAfter:
		_ = a.markUnconfirmed(ctx, p.ID, "sem resposta final do operador ao fim de "+domain.Ago(age))
	case err != nil && age >= railGiveUpAfter:
		a.logPayment(ctx, p.ID, domain.TrailWarn, "payment %s (%s): no answer from the operator after %s: %v", p.ID, p.RailReference, age.Round(time.Minute), err)
		_ = a.markUnconfirmed(ctx, p.ID, "o operador não respondeu ao fim de "+domain.Ago(age))
	case errors.Is(err, ports.ErrStillPending):
	case err != nil:
		a.logPayment(ctx, p.ID, domain.TrailWarn, "payment %s (%s): query failed: %v", p.ID, p.RailReference, err)
	}
}

// CheckClosedPayments looks for customers who paid after their payment was
// closed as timed out, from before payments were left unconfirmed instead. It
// only says so: the console's operator check credits it.
func (a *App) CheckClosedPayments(ctx context.Context) {
	rail, ok := a.Rail.(ports.RailQuery)
	if !ok {
		return
	}
	closed, err := a.Payments.RecentlyTimedOut(ctx, a.Now().Add(-railLateCheckFor), 200)
	if err != nil {
		log.Printf("late payments: %v", err)
		return
	}
	for i := range closed {
		p := &closed[i]
		if o, known, err := rail.Query(ctx, domain.StatusQuery{PaymentID: p.ID, Live: p.Livemode, Reference: p.RailReference, RailID: p.RailID, Method: p.Method, Integration: p.Integration}); err == nil && known && o.Success {
			a.logPayment(ctx, p.ID, domain.TrailError, "payment %s (%s): PAID AFTER IT WAS CLOSED, %d cents from a customer, operator reference %s. CHECK BY HAND: the operator check on its trail credits it", p.ID, p.RailReference, p.Amount, o.OperatorReference)
		}
	}
}

func (a *App) RunWorkers(ctx context.Context) {
	tick := func(every time.Duration, f func()) {
		go func() {
			t := time.NewTicker(every)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					f()
				}
			}
		}()
	}
	tick(time.Second, func() {
		for a.DeliverDue(ctx, 20) == 20 {
		}
	})
	tick(2*time.Second, func() {
		for a.SendDueEmails(ctx, 20) == 20 {
		}
	})
	tick(10*time.Second, func() { a.Sweep(ctx) })
	tick(5*time.Minute, func() { a.CheckClosedPayments(ctx) })
	tick(time.Minute, func() { a.RecheckUnconfirmed(ctx) })
	tick(15*time.Second, func() { a.BillingTick(ctx) })
	tick(2*time.Second, func() { a.SendQueued(ctx) })
	tick(time.Minute, func() { a.Watchdog(ctx) })
	tick(6*time.Hour, func() { a.PruneTrail(ctx) })
}

// traceDelivery puts a webhook attempt on the trail of the payment or checkout it is about.
func (a *App) traceDelivery(ctx context.Context, d *domain.Delivery, ev *domain.Event, body, reply []byte, err error) {
	t := domain.TrailEvent{MerchantID: d.MerchantID, Livemode: d.Livemode, Kind: domain.TrailWebhook, Source: "riha", Method: "POST", Path: hostOf(d.URL),
		Status: d.LastStatusCode, DurationMs: d.LastDurationMs, Request: domain.RedactBody(body), Response: domain.RedactBody(reply),
		Data: map[string]any{"event": ev.Type, "event_id": ev.ID, "attempt": d.Attempts, "url": d.URL}}
	switch {
	case strings.HasPrefix(ev.ObjectID, "pay_"):
		t.PaymentID = ev.ObjectID
	case strings.HasPrefix(ev.ObjectID, "cs_"):
		t.SessionID = ev.ObjectID
	}
	switch {
	case err != nil:
		t.Level, t.Title = domain.TrailWarn, fmt.Sprintf("Webhook %s para %s falhou (tentativa %d): %v", ev.Type, hostOf(d.URL), d.Attempts, err)
	case d.LastStatusCode >= 200 && d.LastStatusCode < 300:
		t.Title = fmt.Sprintf("Webhook %s entregue a %s: HTTP %d em %d ms", ev.Type, hostOf(d.URL), d.LastStatusCode, d.LastDurationMs)
	default:
		t.Level, t.Title = domain.TrailWarn, fmt.Sprintf("Webhook %s para %s respondeu HTTP %d (tentativa %d)", ev.Type, hostOf(d.URL), d.LastStatusCode, d.Attempts)
	}
	a.trace(ctx, t)
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}
