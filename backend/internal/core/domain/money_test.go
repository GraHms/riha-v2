package domain

import "testing"

func TestFee(t *testing.T) {
	cases := []struct {
		amount int64
		bps    int
		fee    int64
	}{
		{125000, 600, 7500},
		{348000, 350, 12180},
		{1001, 600, 60},
		{1009, 600, 61},
		{0, 600, 0},
		{100000, 0, 0},
	}
	for _, c := range cases {
		if got := Fee(c.amount, c.bps); got != c.fee {
			t.Errorf("Fee(%d,%d)=%d want %d", c.amount, c.bps, got, c.fee)
		}
	}
	fee, net := Split(125000, 600)
	if fee+net != 125000 {
		t.Fatal("split must preserve amount")
	}
}

func TestFormat(t *testing.T) {
	if got := Format(4832050); got != "48 320,50" {
		t.Errorf("got %q", got)
	}
	if got := Format(99); got != "0,99" {
		t.Errorf("got %q", got)
	}
	if got := FormatBps(350); got != "3,5%" {
		t.Errorf("got %q", got)
	}
	if got := FormatBps(600); got != "6%" {
		t.Errorf("got %q", got)
	}
}
