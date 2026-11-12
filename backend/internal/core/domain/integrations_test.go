package domain

import "testing"

func TestPriceOf(t *testing.T) {
	cases := []struct {
		name                      string
		amount, fee               int64
		platformBps, shareBps     int
		platform, partner, margin int64
	}{
		// 1 000 MT at RIHA's 6%: 60 MT. mtoca, with no operator charge yet, takes 30% of it.
		{"mtoca", 100000, 6000, 0, 3000, 0, 1800, 4200},
		// With a 2% operator charge, the 30% is of the 40 MT left.
		{"mtoca after the operator", 100000, 6000, 200, 3000, 2000, 1200, 2800},
		// M-Pesa on 900511 charges 3%; 92 Interactions takes nothing yet.
		{"92 Interactions", 100000, 6000, 300, 0, 3000, 0, 3000},
		// At 2%, RIHA's fee doesn't cover a 3% charge: nothing to share, and RIHA loses.
		{"fee below the charge", 100000, 2000, 300, 3000, 3000, 0, -1000},
		{"nothing", 0, 0, 300, 3000, 0, 0, 0},
	}
	for _, c := range cases {
		p := PriceOf(c.amount, c.fee, c.platformBps, c.shareBps)
		if p.PlatformFee != c.platform || p.PartnerFee != c.partner || p.Margin(c.fee) != c.margin {
			t.Errorf("%s: %+v margin %d", c.name, p, p.Margin(c.fee))
		}
	}
}

func TestIntegrationCodes(t *testing.T) {
	for _, d := range DefaultIntegrations() {
		if !ValidIntegrationCode(d.Code) {
			t.Errorf("default %q isn't a valid code", d.Code)
		}
	}
	for _, bad := range []string{"", "a", "Mtoca", "92i", "m-pesa", "x y"} {
		if ValidIntegrationCode(bad) {
			t.Errorf("%q must not be a code", bad)
		}
	}
}
