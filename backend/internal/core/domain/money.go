package domain

import (
	"fmt"
	"strings"
)

const DefaultFeeBps = 600

func Fee(amount int64, bps int) int64 {
	if amount <= 0 || bps <= 0 {
		return 0
	}
	return (amount*int64(bps) + 5000) / 10000
}

func Split(amount int64, bps int) (fee, net int64) {
	fee = Fee(amount, bps)
	return fee, amount - fee
}

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

func FormatBps(bps int) string {
	if bps%100 == 0 {
		return fmt.Sprintf("%d%%", bps/100)
	}
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", float64(bps)/100), "0"), ".")
	return strings.Replace(s, ".", ",", 1) + "%"
}
