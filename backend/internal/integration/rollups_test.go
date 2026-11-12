package integration

import (
	"context"
	"testing"
	"time"

	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// The daily rollups and balances, kept by triggers, always add up to the
// payments and the ledger, whatever happens to them.
func TestRollupsAddUp(t *testing.T) {
	a, ctx := settlementApp(t)
	m := qaMerchant(t, a, ctx, 900, qaBim)
	payLive(t, a, ctx, m, 75000, "mpesa", "841234231")
	p := pay(t, a, ctx, m, 5000, "841234567")
	if _, err := a.Refund(ctx, m.ID, false, p.ID, 2000, "", "u"); err != nil {
		t.Fatal(err)
	}
	po, err := a.RequestPayout(ctx, m.ID, true, 100000, "u")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.TransitionPayout(ctx, po.ID, domain.ActReject, "teste", "admin"); err != nil {
		t.Fatal(err)
	}
	a.PriceBacklog(ctx)
	noDrift(t, a, ctx)

	before := a.Balance(ctx, m.ID, true)
	if err := a.Reports.RebuildRollups(ctx); err != nil {
		t.Fatal(err)
	}
	noDrift(t, a, ctx)
	if after := a.Balance(ctx, m.ID, true); after != before {
		t.Fatalf("balance %d before a rebuild, %d after", before, after)
	}
	// What the console reads from the rollups.
	rows, _, _ := a.MerchantDirectory(ctx, "active", "QA Loja 900")
	if len(rows) != 1 || rows[0].Balance != a.Balance(ctx, m.ID, true) || rows[0].Volume30d != 375000 || rows[0].MainMethod == "" {
		t.Fatalf("directory %+v", rows)
	}
	page := a.PlatformPayments(ctx, app.PlatformPaymentsQuery{View: "paid", MerchantID: m.ID})
	if page.Total != 3 || len(page.Data) != 3 {
		t.Fatalf("paid payments: total %d, rows %d", page.Total, len(page.Data))
	}
	if o := a.PlatformOverview(ctx); o.TxOkToday < 3 || o.VolumeToday < 375000 {
		t.Fatalf("overview today: %d payments, %d", o.TxOkToday, o.VolumeToday)
	}
	if _, err := a.AdminMetrics(ctx, ports.MetricsQuery{From: time.Now().AddDate(0, 0, -7), To: time.Now(), Live: true}); err != nil {
		t.Fatal(err)
	}
	rep, err := a.Settlement(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var collected int64
	for _, pp := range rep.Partners {
		collected += pp.AllTime.Amount
	}
	if collected != 375000 {
		t.Fatalf("settlement collected %d, want 375000", collected)
	}
}

func noDrift(t *testing.T, a *app.App, ctx context.Context) {
	t.Helper()
	n, err := a.Reports.RollupDrift(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("%d rollup rows differ from the payments and the ledger", n)
	}
}
