package domain

import "testing"

func TestPlanFunding(t *testing.T) {
	type part struct {
		src          string
		amount, send int64
	}
	cases := []struct {
		name        string
		amount, fee int64
		held        map[string]int64
		want        []part
	}{
		{"one wallet holds it all", 50000, 0, map[string]int64{"mtoca": 80000, "mpesa_direct": 60000}, []part{{"mtoca", 50000, 50000}}},
		{"the fullest that can, even if the other can too", 50000, 12000, map[string]int64{"mtoca": 50000, "mpesa_direct": 70000}, []part{{"mpesa_direct", 50000, 38000}}},
		{"both wallets, fullest first", 100000, 0, map[string]int64{"mtoca": 30000, "mpesa_direct": 70000}, []part{{"mpesa_direct", 70000, 70000}, {"mtoca", 30000, 30000}}},
		{"RIHA covers what no wallet holds", 100000, 12000, map[string]int64{"mtoca": 30000}, []part{{"mtoca", 30000, 30000}, {"riha", 70000, 58000}}},
		{"nothing held", 20000, 0, map[string]int64{"mtoca": 0, "mpesa_direct": -500}, []part{{"riha", 20000, 20000}}},
	}
	for _, c := range cases {
		got := PlanFunding(c.amount, c.fee, c.held)
		if len(got) != len(c.want) {
			t.Fatalf("%s: %+v", c.name, got)
		}
		var sum, sent int64
		for i, w := range c.want {
			if got[i].Source != w.src || got[i].Amount != w.amount || got[i].Send != w.send || got[i].Status != FundingOpen {
				t.Fatalf("%s: part %d %+v, want %+v", c.name, i, got[i], w)
			}
			sum += got[i].Amount
			sent += got[i].Send
		}
		if sum != c.amount || sent != c.amount-c.fee {
			t.Fatalf("%s: parts add to %d and send %d", c.name, sum, sent)
		}
	}
}

func TestPermissions(t *testing.T) {
	if got := CleanPermissions([]string{"team", "nope", "approvals", "team"}, true); len(got) != 2 || got[0] != PermApprovals || got[1] != PermTeam {
		t.Fatalf("staff %v", got)
	}
	if got := CleanPermissions([]string{"approvals", "money"}, false); len(got) != 1 || got[0] != PartnerMoney {
		t.Fatalf("partner %v", got)
	}
	owner := &User{Role: RoleAdmin, Permissions: []string{PermAll}}
	finance := &User{Role: RoleAdmin, Permissions: []string{PermPayouts}}
	shop := &User{Role: RoleMerchant, Permissions: []string{PermAll}}
	if !owner.Can(PermTeam) || !finance.Can(PermPayouts) || finance.Can(PermTeam) || shop.Can(PermPayouts) {
		t.Fatal("can")
	}
	now := owner.CreatedAt
	owner.DisabledAt = &now
	if owner.Can(PermTeam) {
		t.Fatal("a closed account can't do anything")
	}
}
