package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"riha/internal/adapters/rails"
	"riha/internal/core/app"
	"riha/internal/core/domain"
)

// settlementApp takes live money through two integrations, as production does:
// M-Pesa direct by default for M-Pesa, and mtoca for e-Mola, or for M-Pesa when
// a merchant is sent through it.
func settlementApp(t *testing.T) (*app.App, context.Context) {
	a, ctx := setup(t)
	mock := rails.NewMock(0)
	mock.After = func(_ time.Duration, f func()) { f() }
	a.Rail = rails.Modes{
		Test: rails.Single(domain.IntegrationSimulator, mock),
		Live: rails.Router{Via: map[string]rails.Via{
			domain.IntegrationMtoca:       {Rail: mock, Methods: []string{"mpesa", "emola", "bim"}},
			domain.IntegrationMpesaDirect: {Rail: mock, Methods: []string{"mpesa"}},
		}, Default: map[string]string{"mpesa": domain.IntegrationMpesaDirect, "emola": domain.IntegrationMtoca, "bim": domain.IntegrationMtoca}},
	}
	if err := a.SetupIntegrations(ctx); err != nil {
		t.Fatal(err)
	}
	return a, ctx
}

func payLive(t *testing.T, a *app.App, ctx context.Context, m *domain.Merchant, amount int64, method, phone string) *domain.Payment {
	t.Helper()
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: amount, PaymentMethodTypes: []string{method}, Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: method, Phone: phone})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.PaymentSucceeded {
		t.Fatalf("payment %s %+v", p.Status, p)
	}
	return p
}

var admin = app.Actor{ID: "usr_admin", Role: domain.RoleAdmin}

func TestPaymentsGoThroughTheirIntegrationOnItsDeal(t *testing.T) {
	a, ctx := settlementApp(t)
	shop := approved(t, a, ctx, "Loja Um", "um@example.com")
	events := approved(t, a, ctx, "MyEvents", "events@example.com")

	// 1 000 MT by M-Pesa goes direct: 60 MT fee, M-Pesa's 3% is 30 MT, 92 Interactions takes nothing yet.
	p := payLive(t, a, ctx, shop, 100000, "mpesa", "841234231")
	if p.Integration != domain.IntegrationMpesaDirect || p.Fee != 6000 || p.PlatformFee != 3000 || p.PartnerFee != 0 || !p.Priced {
		t.Fatalf("direct %+v", p)
	}
	// e-Mola goes through mtoca: 30% of the 60 MT fee.
	e := payLive(t, a, ctx, shop, 100000, "emola", "861234231")
	if e.Integration != domain.IntegrationMtoca || e.PlatformFee != 0 || e.PartnerFee != 1800 {
		t.Fatalf("mtoca %+v", e)
	}

	// MyEvents is sent through mtoca for M-Pesa, on a 20% share.
	if err := a.SetMerchantRoute(ctx, events.ID, "mpesa", domain.IntegrationMtoca, admin); err != nil {
		t.Fatal(err)
	}
	r := payLive(t, a, ctx, events, 100000, "mpesa", "841234231")
	if r.Integration != domain.IntegrationMtoca || r.PartnerFee != 1800 {
		t.Fatalf("routed %+v", r)
	}
	share := 2000
	if err := a.SetMerchantDeal(ctx, events.ID, domain.IntegrationMtoca, &share, admin); err != nil {
		t.Fatal(err)
	}
	r2 := payLive(t, a, ctx, events, 100000, "mpesa", "841234231")
	if r2.PartnerFee != 1200 {
		t.Fatalf("own deal %+v", r2)
	}
	if again, _ := a.Payments.ByID(ctx, r.ID); again.PartnerFee != 1800 {
		t.Fatalf("a new deal doesn't change payments already made: %d", again.PartnerFee)
	}
	// Other merchants keep the default route and deal.
	if p2 := payLive(t, a, ctx, shop, 100000, "mpesa", "841234231"); p2.Integration != domain.IntegrationMpesaDirect {
		t.Fatalf("default %+v", p2)
	}

	// An integration that doesn't take a method can't be chosen for it.
	var de *domain.Error
	if err := a.SetMerchantRoute(ctx, events.ID, "emola", domain.IntegrationMpesaDirect, admin); !errors.As(err, &de) || de.Code != "integration_unavailable" {
		t.Fatalf("emola through M-Pesa direct: %v", err)
	}
	routing, err := a.MerchantRouting(ctx, events.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range routing.Methods {
		if m.Method == "mpesa" && (m.Default != domain.IntegrationMpesaDirect || m.Chosen != domain.IntegrationMtoca || m.Via != domain.IntegrationMtoca || len(m.Options) != 2) {
			t.Fatalf("mpesa route %+v", m)
		}
	}
	// Back to the default.
	if err := a.SetMerchantRoute(ctx, events.ID, "mpesa", "", admin); err != nil {
		t.Fatal(err)
	}
	if p3 := payLive(t, a, ctx, events, 100000, "mpesa", "841234231"); p3.Integration != domain.IntegrationMpesaDirect {
		t.Fatalf("back to default %+v", p3)
	}

	// The operator's charge on mtoca, once known, comes off before the share.
	two := map[string]int{"emola": 200}
	if _, err := a.EditIntegration(ctx, domain.IntegrationMtoca, app.IntegrationEdit{PlatformBps: two}, admin); err != nil {
		t.Fatal(err)
	}
	e2 := payLive(t, a, ctx, shop, 100000, "emola", "861234231")
	if e2.PlatformFee != 2000 || e2.PartnerFee != 1200 {
		t.Fatalf("after the operator %+v", e2)
	}
	bad := 10001
	if _, err := a.EditIntegration(ctx, domain.IntegrationMtoca, app.IntegrationEdit{ShareBps: &bad}, admin); err == nil {
		t.Fatal("a share over 100% must be refused")
	}
	// Test payments stay out of it.
	cs, _ := a.CreateSession(ctx, shop, false, app.SessionParams{Amount: 100000, Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if tp, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "841234231"}); err != nil || tp.Integration != domain.IntegrationSimulator {
		t.Fatalf("test payment %+v %v", tp, err)
	}
}

func TestSettlementSaysWhereTheMoneyIs(t *testing.T) {
	a, ctx := settlementApp(t)
	shop := approved(t, a, ctx, "Loja Um", "um@example.com")
	payLive(t, a, ctx, shop, 100000, "mpesa", "841234231") // direct: 30 platform, 0 partner
	payLive(t, a, ctx, shop, 200000, "emola", "861234231") // mtoca: 120 fee, 36 partner

	if _, err := a.RecordSettlement(ctx, app.SettlementInput{Integration: domain.IntegrationMtoca, Amount: 100000, Reference: "TRF-1", ReceivedOn: time.Now().Format("2006-01-02")}, admin); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []app.SettlementInput{
		{Integration: "nobody", Amount: 100, Reference: "x", ReceivedOn: "2026-11-01"},
		{Integration: domain.IntegrationMtoca, Amount: 0, Reference: "x", ReceivedOn: "2026-11-01"},
		{Integration: domain.IntegrationMtoca, Amount: 100, Reference: "", ReceivedOn: "2026-11-01"},
		{Integration: domain.IntegrationMtoca, Amount: 100, Reference: "x", ReceivedOn: time.Now().AddDate(0, 0, 2).Format("2006-01-02")},
	} {
		if _, err := a.RecordSettlement(ctx, bad, admin); err == nil {
			t.Fatalf("must be refused: %+v", bad)
		}
	}

	rep, err := a.Settlement(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]app.PartnerPosition{}
	for _, p := range rep.Partners {
		pos[p.Integration.Code] = p
	}
	direct, mt := pos[domain.IntegrationMpesaDirect], pos[domain.IntegrationMtoca]
	// 92 Interactions holds 1 000 MT less M-Pesa's 30 MT.
	if direct.Held != 97000 || direct.AllTime.Margin() != 3000 || direct.PartnerName != "92 Interactions" {
		t.Fatalf("direct %+v", direct)
	}
	// mtoca took 2 000 MT, keeps 36 MT, and has paid 1 000 MT.
	if mt.AllTime.Amount != 200000 || mt.AllTime.PartnerFee != 3600 || mt.Received != 100000 || mt.Held != 96400 || mt.LastPaid == nil || len(mt.Merchants) != 1 {
		t.Fatalf("mtoca %+v", mt)
	}
	if _, ok := pos[domain.IntegrationSimulator]; ok {
		t.Fatal("the simulator holds no money")
	}
	// The shop is owed 2 820 MT; what is left of the money is RIHA's margin.
	if rep.OwedToShops != 282000 || rep.HeldOutside != 193400 || rep.AtRIHA != 100000 {
		t.Fatalf("position owed %d held %d at riha %d", rep.OwedToShops, rep.HeldOutside, rep.AtRIHA)
	}
	if margin := direct.AllTime.Margin() + mt.AllTime.Margin(); rep.RIHAOwn != margin || margin != 3000+8400 {
		t.Fatalf("riha's own %d, margin %d", rep.RIHAOwn, margin)
	}

	// A period before the payments is empty, but the position is all-time.
	old := time.Now().AddDate(0, -2, 0)
	end := old.AddDate(0, 0, 7)
	rep, _ = a.Settlement(ctx, &old, &end)
	for _, p := range rep.Partners {
		if p.Period.Count != 0 || (p.Integration.Code == domain.IntegrationMtoca && p.Held != 96400) {
			t.Fatalf("period %+v", p)
		}
	}
	if err := a.DeleteSettlement(ctx, mt.LastPaid.ID, admin); err != nil {
		t.Fatal(err)
	}
	rep, _ = a.Settlement(ctx, nil, nil)
	if rep.HeldOutside != 293400 {
		t.Fatalf("after deleting the settlement %d", rep.HeldOutside)
	}
}
