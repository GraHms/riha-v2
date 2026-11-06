package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type Deps struct {
	Tx           ports.Tx
	Users        ports.Users
	AuthSessions ports.AuthSessions
	Merchants    ports.Merchants
	Documents    ports.Documents
	Banks        ports.BankAccounts
	Keys         ports.APIKeys
	Links        ports.Links
	Sessions     ports.CheckoutSessions
	Payments     ports.Payments
	Refunds      ports.Refunds
	Ledger       ports.Ledger
	Payouts      ports.Payouts
	Endpoints    ports.Endpoints
	Domains      ports.Domains
	Events       ports.Events
	Deliveries   ports.Deliveries
	RequestLogs  ports.RequestLogs
	Idempotency  ports.Idempotency
	Audit        ports.Audit
	Settings     ports.Settings
	Reports      ports.Reports
	Rail         ports.Rail
	Sender       ports.WebhookSender
	Files        ports.FileStore
	Hasher       ports.PasswordHasher
	Renderer     ports.Renderer
	Mailer       ports.Mailer
	Emails       ports.Emails
	Resets       ports.PasswordResets
	Plans        ports.Plans
	Members      ports.Members
	Subs         ports.Subscriptions
	Charges      ports.Charges
	Messages     ports.Messages
	SubSettings  ports.SubscriptionSettingsRepo
	Messenger    ports.Messenger
	Sheets       ports.Sheet
	// Trail is optional: without it nothing is traced.
	Trail    ports.Trail
	Insights ports.Insights
}

type App struct {
	Deps
	PublicURL            string
	PayURL               string // the address in checkout and payment links
	AllowPrivateNetworks bool
	AdminNotifyEmail     string
	// BackupStatusFile is where the server's backup writes the time of its last success.
	BackupStatusFile string
	// ComingSoon lists the methods that can't take money yet.
	ComingSoon []string
	// NoLiveRefunds: a live refund must send the money back, and no rail can yet.
	NoLiveRefunds bool
	Now           func() time.Time
}

func New(d Deps, publicURL string) *App {
	return &App{Deps: d, PublicURL: publicURL, PayURL: publicURL, Now: func() time.Time { return time.Now().UTC() }}
}

func notFound(err error, what string) error {
	if errors.Is(err, domain.ErrNotFound) {
		return domain.NotFound(what)
	}
	return err
}

const SettingDefaultFee = "default_fee_bps"

func (a *App) DefaultFeeBps(ctx context.Context) int {
	if v, err := a.Settings.Get(ctx, SettingDefaultFee); err == nil {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return domain.DefaultFeeBps
}

func (a *App) SetDefaultFeeBps(ctx context.Context, bps int) error {
	if bps < 0 || bps > 2000 {
		return domain.Invalid("parameter_invalid", "default_fee_bps", "A taxa tem de estar entre 0% e 20%.")
	}
	return a.Settings.Set(ctx, SettingDefaultFee, strconv.Itoa(bps))
}

func (a *App) FeeBpsFor(ctx context.Context, m *domain.Merchant) int {
	if m.FeeBps != nil {
		return *m.FeeBps
	}
	return a.DefaultFeeBps(ctx)
}

func (a *App) Balance(ctx context.Context, merchantID string, live bool) int64 {
	b, _ := a.Ledger.Balance(ctx, merchantID, live)
	return b
}

func (a *App) InTransit(ctx context.Context, merchantID string, live bool) int64 {
	_, total, _ := a.Payouts.Sum(ctx, ports.PayoutQuery{MerchantID: merchantID, Live: &live, Statuses: domain.OpenPayoutStatuses})
	return total
}

func (a *App) ledger(ctx context.Context, merchantID string, live bool, typ string, amount int64, ref, desc string) error {
	return a.Ledger.Add(ctx, &domain.LedgerEntry{ID: domain.NewID("le"), MerchantID: merchantID, Livemode: live, Type: typ,
		Amount: amount, ReferenceID: ref, Description: desc, CreatedAt: a.Now()})
}

type Actor struct {
	ID   string
	Role string
	IP   string
}

func (a *App) Record(ctx context.Context, merchantID string, actor Actor, action, target, details string) {
	_ = a.Audit.Record(ctx, &domain.AuditEntry{ID: domain.NewID("aud"), MerchantID: merchantID, ActorID: actor.ID, ActorRole: actor.Role,
		Action: action, Target: target, Details: details, IP: actor.IP, CreatedAt: a.Now()})
}

func (a *App) emit(ctx context.Context, merchantID string, live bool, typ, objectID string, obj any, callbackURL string) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	now := a.Now()
	ev := &domain.Event{ID: domain.NewID("evt"), MerchantID: merchantID, Livemode: live, Type: typ, ObjectID: objectID, Data: data, CreatedAt: now}
	if err := a.Events.Create(ctx, ev); err != nil {
		return err
	}
	eps, err := a.Endpoints.List(ctx, merchantID, live)
	if err != nil {
		return err
	}
	for _, ep := range eps {
		if !ep.Wants(typ) {
			continue
		}
		id := ep.ID
		if err := a.Deliveries.Create(ctx, &domain.Delivery{ID: domain.NewID("whd"), EventID: ev.ID, MerchantID: merchantID, Livemode: live,
			EventType: typ, Target: domain.TargetEndpoint, EndpointID: &id, URL: ep.URL, Status: domain.DeliveryPending, NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
	}
	if callbackURL != "" && domain.CallbackEvents[typ] {
		return a.Deliveries.Create(ctx, &domain.Delivery{ID: domain.NewID("whd"), EventID: ev.ID, MerchantID: merchantID, Livemode: live,
			EventType: typ, Target: domain.TargetCallback, URL: callbackURL, Status: domain.DeliveryPending, NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now})
	}
	return nil
}

// trace adds an event to the trail, with the request that caused it.
func (a *App) trace(ctx context.Context, ev domain.TrailEvent) {
	if a.Trail == nil {
		return
	}
	r := domain.RequestFrom(ctx)
	ev.RequestID = domain.FirstNonEmpty(ev.RequestID, r.ID)
	if ev.Level == "" {
		ev.Level = domain.TrailInfo
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	a.Trail.Add(ev)
}

// logPayment writes a line to the log and to the payment's trail, so what was
// logged about a payment outlives the log.
func (a *App) logPayment(ctx context.Context, paymentID, level, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	a.trace(ctx, domain.TrailEvent{PaymentID: paymentID, Kind: domain.TrailLog, Source: "riha", Level: level, Title: msg})
}

// TraceAPI puts a call to the public API on the trail.
func (a *App) TraceAPI(ctx context.Context, ev domain.TrailEvent) {
	ev.Kind, ev.Source = domain.TrailAPI, "api"
	ev.Title = fmt.Sprintf("API: %s %s respondeu %d em %d ms", ev.Method, ev.Path, ev.Status, ev.DurationMs)
	switch {
	case ev.Status >= 500:
		ev.Level = domain.TrailError
	case ev.Status >= 400:
		ev.Level = domain.TrailWarn
	}
	a.trace(ctx, ev)
}
