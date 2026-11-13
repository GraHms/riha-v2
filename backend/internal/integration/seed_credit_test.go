package integration

import (
	"testing"

	"riha/internal/seed"
)

// The demo data runs on any date and gives the Microcrédito panel loans in each state.
func TestSeedRuns(t *testing.T) {
	a, ctx := setup(t)
	if err := seed.Run(ctx, a); err != nil {
		t.Fatal(err)
	}
	u, err := a.Users.ByEmail(ctx, seed.CreditEmail)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := a.Merchants.ByID(ctx, *u.MerchantID)
	_, counts, err := a.CreditLoans(ctx, m, true, "ativos", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if counts["ativos"] < 6 || counts["atraso"] < 2 || counts["acordo"] != 1 || counts["aceitar"] != 1 || counts["liquidados"] != 1 {
		t.Fatalf("loans by tab %v", counts)
	}
	rs, _, _ := a.CreditRequests(ctx, m, true, "decidir", "", "")
	if len(rs) != 5 {
		t.Fatalf("%d requests to decide", len(rs))
	}
	o, err := a.CreditOverview(ctx, m, true)
	if err != nil || o.Book <= 0 || len(o.Attention) == 0 {
		t.Fatalf("overview %v %+v", err, o)
	}
}
