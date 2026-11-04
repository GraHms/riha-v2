// Package money contains fee arithmetic and formatting for metical amounts in
// centavos.
package money

import (
	"fmt"
	"strings"
)

// DefaultFeeBps is the platform default fee: 6%.
const DefaultFeeBps = 600

// Fee computes round-half-up(amount * bps / 10000). amount is in centavos.
func Fee(amount int64, bps int) int64 {
	if amount <= 0 || bps <= 0 {
		return 0
	}
	return (amount*int64(bps) + 5000) / 10000
}

// Split returns fee and net for an amount.
func Split(amount int64, bps int) (fee, net int64) {
	fee = Fee(amount, bps)
	return fee, amount - fee
}

// Format renders centavos as "1 250,00" (Mozambican/Portuguese style).
func Format(c int64) string {
	neg := c < 0
	if neg {
		c = -c
	}
	i, d := c/100, c%100
	s := fmt.Sprintf("%d", i)
	var b strings.Builder
	for k, r := range s {
		if k > 0 && (len(s)-k)%3 == 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	out := fmt.Sprintf("%s,%02d", b.String(), d)
	if neg {
		return "-" + out
	}
	return out
}

// FormatBps renders 600 as "6%" and 350 as "3,5%".
func FormatBps(bps int) string {
	if bps%100 == 0 {
		return fmt.Sprintf("%d%%", bps/100)
	}
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", float64(bps)/100), "0"), ".")
	return strings.Replace(s, ".", ",", 1) + "%"
}
