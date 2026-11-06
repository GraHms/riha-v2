package netinfo

import (
	"testing"
	"time"
)

func TestLookup(t *testing.T) {
	start := time.Now()
	Load()
	t.Logf("loaded %d networks and %d ranges in %s", len(nets), len(ranges), time.Since(start))
	for ip, want := range map[string]string{
		"197.235.71.20": "Vodacom", "197.218.123.9": "Movitel", "197.249.232.7": "TVCabo",
		"104.28.82.5": "Cloudflare (relay, por exemplo iCloud Private Relay)", "102.176.248.84": "BIM (banco)",
	} {
		n, ok := Lookup(ip)
		if !ok || n.Label != want {
			t.Errorf("%s: %+v %v, want %s", ip, n, ok, want)
		}
	}
	for _, ip := range []string{"10.0.0.1", "127.0.0.1", "::1", "2001:db8::1", "nonsense", ""} {
		if n, ok := Lookup(ip); ok {
			t.Errorf("%s should not be known: %+v", ip, n)
		}
	}
	if n, _ := Lookup("197.235.71.20"); n.Country != "MZ" || n.Kind != "mobile" {
		t.Errorf("Vodacom: %+v", n)
	}
}
