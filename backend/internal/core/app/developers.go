package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
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

// RollSigningSecret replaces the secret that signs a mode's webhooks, for a
// merchant who fears it leaked. The old one stops signing at once.
func (a *App) RollSigningSecret(ctx context.Context, merchantID string, live bool, actor Actor) (string, error) {
	secret := domain.Token("whsec_", 32)
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		m, err := a.Merchants.Lock(ctx, merchantID)
		if err != nil {
			return err
		}
		if live {
			m.SigningSecretLive = secret
		} else {
			m.SigningSecretTest = secret
		}
		return a.Merchants.Save(ctx, m)
	})
	if err != nil {
		return "", err
	}
	a.Record(ctx, merchantID, actor, "signing_secret.rolled", "", domain.Mode(live))
	return secret, nil
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

// ClaimIdempotent takes an Idempotency-Key before its request runs. It gives
// the claim, or what holds the key: the stored answer to replay, or a request
// still running (StatusCode 0).
func (a *App) ClaimIdempotent(ctx context.Context, r *domain.IdempotencyRecord) (mine bool, held *domain.IdempotencyRecord, err error) {
	r.ID, r.CreatedAt, r.StatusCode = domain.NewID("idem"), a.Now(), 0
	ok, err := a.Idempotency.Claim(ctx, r)
	if err != nil || ok {
		return ok, nil, err
	}
	held, err = a.Idempotency.Get(ctx, r.MerchantID, r.Livemode, r.Key)
	if err != nil {
		return false, nil, err
	}
	return false, held, nil
}

var webhookSecretRe = regexp.MustCompile(`whsec_[A-Za-z0-9_\-]+`)

// FinishIdempotent stores the answer to replay. A webhook signing secret is
// shown once, on the first answer, and never kept.
func (a *App) FinishIdempotent(ctx context.Context, id string, status int, body string) {
	_ = a.Idempotency.Finish(ctx, id, status, webhookSecretRe.ReplaceAllString(body, "whsec_…"))
}

func (a *App) ReleaseIdempotent(ctx context.Context, id string) { _ = a.Idempotency.Release(ctx, id) }

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
		return a.emit(ctx, merchantID, live, domain.EventPing, "ping", map[string]any{
			"object": "ping", "livemode": live, "test_event": true, "message": "Evento de teste da RIHA: o seu endpoint recebe e verifica a assinatura.",
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

var ipInError = regexp.MustCompile(`\[?(?:[0-9a-fA-F]{0,4}:){2,}[0-9a-fA-F]{0,4}\]?(?::\d+)?|\b\d{1,3}(?:\.\d{1,3}){3}(?::\d+)?\b`)

// webhookHosts remembers merchants' hosts that keep failing, so a dead
// endpoint waits apart instead of slowing every merchant's webhooks.
type webhookHosts struct {
	mu    sync.Mutex
	fails map[string]int
	until map[string]time.Time
}

const (
	webhookWorkers   = 8
	webhookPerHost   = 2
	hostFailsToPause = 5
	hostPause        = 10 * time.Minute
)

func (h *webhookHosts) paused(host string, now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return now.Before(h.until[host])
}

func (h *webhookHosts) result(host string, ok bool, now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fails == nil {
		h.fails, h.until = map[string]int{}, map[string]time.Time{}
	}
	if ok {
		delete(h.fails, host)
		delete(h.until, host)
		return
	}
	h.fails[host]++
	if h.fails[host] >= hostFailsToPause {
		h.until[host], h.fails[host] = now.Add(hostPause), 0
	}
}

// DeliverDue sends a batch of due webhooks side by side: a few at a time,
// at most two to one host, and none to a host that keeps failing.
func (a *App) DeliverDue(ctx context.Context, n int) int {
	now := a.Now()
	batch, err := a.Deliveries.ClaimDue(ctx, now, now.Add(2*time.Minute), n)
	if err != nil {
		return 0
	}
	workers := make(chan struct{}, webhookWorkers)
	perHost := map[string]chan struct{}{}
	var wg sync.WaitGroup
	for i := range batch {
		d := &batch[i]
		host := hostOf(d.URL)
		if a.hosts.paused(host, now) {
			next := now.Add(hostPause)
			d.NextAttemptAt, d.LastError, d.UpdatedAt = &next, "O endereço falhou várias vezes seguidas: tentamos de novo daqui a pouco.", now
			_ = a.Deliveries.Save(ctx, d)
			continue
		}
		if perHost[host] == nil {
			perHost[host] = make(chan struct{}, webhookPerHost)
		}
		slot := perHost[host]
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("webhook %s: panic: %v", d.ID, r)
				}
			}()
			workers <- struct{}{}
			slot <- struct{}{}
			defer func() { <-slot; <-workers }()
			a.deliver(ctx, d)
			a.hosts.result(host, d.Status == domain.DeliverySucceeded, a.Now())
		}()
	}
	wg.Wait()
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
	// An endpoint deleted, or turned off, since the event takes nothing more.
	if d.EndpointID != nil {
		if e, err := a.Endpoints.ByID(ctx, d.MerchantID, *d.EndpointID); errors.Is(err, domain.ErrNotFound) || (err == nil && !e.Active) {
			d.Status, d.NextAttemptAt, d.LastError, d.UpdatedAt = domain.DeliveryFailed, nil, "Endpoint removido ou desativado.", a.Now()
			_ = a.Deliveries.Save(ctx, d)
			return
		}
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
		// The merchant reads this: never the addresses RIHA's network saw.
		d.LastError = domain.Trim(ipInError.ReplaceAllString(err.Error(), "<endereço>"), 500)
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
	o, known, err := rail.Query(ctx, domain.StatusQuery{PaymentID: p.ID, Live: p.Livemode, Reference: p.RailReference, RailID: p.RailID, Method: p.Method, Integration: p.Integration, Amount: p.Amount})
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
		if o, known, err := rail.Query(ctx, domain.StatusQuery{PaymentID: p.ID, Live: p.Livemode, Reference: p.RailReference, RailID: p.RailID, Method: p.Method, Integration: p.Integration, Amount: p.Amount}); err == nil && known && o.Success {
			a.logPayment(ctx, p.ID, domain.TrailError, "payment %s (%s): PAID AFTER IT WAS CLOSED, %d cents from a customer, operator reference %s. CHECK BY HAND: the operator check on its trail credits it", p.ID, p.RailReference, p.Amount, o.OperatorReference)
		}
	}
}

func (a *App) RunWorkers(ctx context.Context) {
	// Each run has a deadline, so one call an operator never answers can't
	// stop the worker, and a panic ends that run, not the server.
	tick := func(name string, every, limit time.Duration, f func(ctx context.Context)) {
		run := func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("worker %s: panic: %v\n%s", name, r, debug.Stack())
				}
			}()
			c, cancel := context.WithTimeout(ctx, limit)
			defer cancel()
			f(c)
		}
		go func() {
			t := time.NewTicker(every)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					run()
				}
			}
		}()
	}
	tick("webhooks", time.Second, 2*time.Minute, func(ctx context.Context) {
		for ctx.Err() == nil && a.DeliverDue(ctx, 20) == 20 {
		}
	})
	tick("emails", 2*time.Second, 2*time.Minute, func(ctx context.Context) {
		for ctx.Err() == nil && a.SendDueEmails(ctx, 20) == 20 {
		}
	})
	tick("sweep", 10*time.Second, 3*time.Minute, a.Sweep)
	tick("closed", 5*time.Minute, 5*time.Minute, a.CheckClosedPayments)
	tick("unconfirmed", time.Minute, 3*time.Minute, a.RecheckUnconfirmed)
	tick("billing", 15*time.Second, 3*time.Minute, a.BillingTick)
	tick("microcredit", 30*time.Second, 3*time.Minute, a.CreditTick)
	tick("messages", 2*time.Second, 2*time.Minute, a.SendQueued)
	tick("watchdog", time.Minute, time.Minute, a.Watchdog)
	tick("trail", 6*time.Hour, 30*time.Minute, a.PruneTrail)
	tick("pricing", time.Minute, 10*time.Minute, a.PriceBacklog)
	tick("housekeeping", 24*time.Hour, time.Hour, a.Housekeeping)
	// The first pricing and check run soon after the start, not a day later.
	domain.Safely("startup work", func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
		c, cancel := context.WithTimeout(ctx, time.Hour)
		defer cancel()
		a.PriceBacklog(c)
		a.Housekeeping(c)
	})
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
