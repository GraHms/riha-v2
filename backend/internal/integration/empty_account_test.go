package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// A merchant with no plans is every merchant on their first visit. A null list
// here blanked the whole dashboard, because the page reads its length.
func TestSubscriptionOverviewOfAnEmptyAccount(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Loja Vazia", "vazia@example.com")
	b, err := json.Marshal(a.SubscriptionOverview(ctx, m, true))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Fatalf("the page needs lists, not nulls: %s", b)
	}
}

// The overview's chart swallowed this query's error, so it stayed empty with money in the account.
func TestSalesChartCountsPayments(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Loja Vende", "vende@example.com")
	p := pay(t, a, ctx, m, 50000, "841234567")
	if p.Status != domain.PaymentSucceeded {
		t.Fatalf("payment: %s", p.Status)
	}
	for _, unit := range []string{"day", "month"} {
		rows, err := a.Reports.VolumeBuckets(ctx, ports.Scope{MerchantID: m.ID}, unit)
		if err != nil {
			t.Fatalf("%s: %v", unit, err)
		}
		if len(rows) != 1 || rows[0].Amount != 50000 {
			t.Fatalf("%s: the payment must be in the chart: %+v", unit, rows)
		}
	}
}
