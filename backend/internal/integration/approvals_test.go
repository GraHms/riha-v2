package integration

import (
	"fmt"
	"testing"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

// The leads list pages, searches and sorts, so a new signup can always be
// found however many old accounts are waiting.
func TestLeadsQueue(t *testing.T) {
	a, ctx := setup(t)
	for i := 1; i <= 7; i++ {
		in := app.SignupInput{BusinessName: fmt.Sprintf("Loja %02d", i), Email: fmt.Sprintf("loja%d@example.com", i), Phone: fmt.Sprintf("84600000%d", i), Password: "segredo123"}
		if _, _, err := a.Signup(ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	leads := []string{domain.MerchantLead}
	names := func(as []app.Application) (out []string) {
		for _, x := range as {
			out = append(out, x.Merchant.Name)
		}
		return
	}

	p1, total := a.ApprovalQueue(ctx, app.ApprovalQuery{Statuses: leads, PerPage: 3})
	if total != 7 || len(p1) != 3 || p1[0].Merchant.Name != "Loja 07" {
		t.Fatalf("leads must list newest first, 3 a page of 7: %d %v", total, names(p1))
	}
	p3, _ := a.ApprovalQueue(ctx, app.ApprovalQuery{Statuses: leads, PerPage: 3, Page: 3})
	if len(p3) != 1 || p3[0].Merchant.Name != "Loja 01" {
		t.Fatalf("the last page holds the oldest: %v", names(p3))
	}
	byName, _ := a.ApprovalQueue(ctx, app.ApprovalQuery{Statuses: leads, Sort: "name", PerPage: 2})
	if names(byName)[0] != "Loja 01" || names(byName)[1] != "Loja 02" {
		t.Fatalf("by name: %v", names(byName))
	}
	for query, want := range map[string]string{"loja 05": "Loja 05", "LOJA3@EXAMPLE": "Loja 03", "84 600 0004": "Loja 04", "+258846000006": "Loja 06"} {
		got, n := a.ApprovalQueue(ctx, app.ApprovalQuery{Statuses: leads, Query: query})
		if n != 1 || got[0].Merchant.Name != want {
			t.Fatalf("search %q found %d: %v", query, n, names(got))
		}
	}
	if _, n := a.ApprovalQueue(ctx, app.ApprovalQuery{Statuses: leads, Query: "100%"}); n != 0 {
		t.Fatalf("a %% in the search is a character, not a wildcard: found %d", n)
	}
}
