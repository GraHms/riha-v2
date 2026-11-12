package domain

import "testing"

// QA: no part of a withdrawal may tell its payer to send a negative amount, and
// the parts must add up to the withdrawal and to what reaches the bank.
func TestQAPlanFundingNeverSendsNegative(t *testing.T) {
	cases := []struct {
		name   string
		amount int64
		fee    int64
		held   map[string]int64
	}{
		// Smallest withdrawal to another bank: 100 MT + 120 MT fee, split half/half.
		{"two even parts at the minimum", 22000, 12000, map[string]int64{IntegrationMtoca: 11000}},
		// Two partners and RIHA, each holding a third.
		{"three parts", 30000, 12000, map[string]int64{IntegrationMtoca: 10000, IntegrationMpesaDirect: 10000}},
		// The admin can set the fee up to 1 000 MT.
		{"high fee set by admin", 120000, 100000, map[string]int64{IntegrationMtoca: 60000}},
	}
	for _, c := range cases {
		parts := PlanFunding(c.amount, c.fee, c.held)
		var amount, send int64
		for _, p := range parts {
			amount += p.Amount
			send += p.Send
			if p.Send < 0 {
				t.Errorf("%s: part %s is told to send %d (negative): %+v", c.name, p.Source, p.Send, parts)
			}
		}
		if amount != c.amount || send != c.amount-c.fee {
			t.Errorf("%s: parts sum %d/%d, want %d/%d", c.name, amount, send, c.amount, c.amount-c.fee)
		}
	}
}
