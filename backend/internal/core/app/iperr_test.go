package app

import "testing"

func TestIPsLeaveDeliveryErrors(t *testing.T) {
	for in, want := range map[string]string{
		`Post "https://loja.co.mz/hook": dial tcp 10.0.3.7:443: connect: connection refused`: `Post "https://loja.co.mz/hook": dial tcp <endereço>: connect: connection refused`,
		`dial tcp [fd00::1]:443: i/o timeout`:                                             `dial tcp <endereço>: i/o timeout`,
		`Post "https://loja.co.mz:8443/x": context deadline exceeded`:                      `Post "https://loja.co.mz:8443/x": context deadline exceeded`,
	} {
		if got := ipInError.ReplaceAllString(in, "<endereço>"); got != want {
			t.Errorf("%s\n got %s", in, got)
		}
	}
}
