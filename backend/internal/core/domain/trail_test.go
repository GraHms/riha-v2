package domain

import (
	"strings"
	"testing"
)

func TestRedactBody(t *testing.T) {
	in := `{"operationNumber":"843879365739","phoneNumber":"258841234567","msisdn":258861234567,"amount":10.0,"comissionAmount":0.08,
	"token":"abc","Authorization":"Bearer x","card_number":"4242424242424242","pin":"1234","nested":{"api_key":"sk_live_1","note":"ligue 84 123 4567 ou 841234567"},
	"failure_code":"pin_incorrect","transactionID_NEP":"a0e9e3a16dbe4631a885cd87ce4b6003"}`
	out := RedactBody([]byte(in))
	for _, bad := range []string{"841234567", "84 123 4567", "861234567", "abc", "Bearer", "4242424242424242", "1234\"", "sk_live_1"} {
		if strings.Contains(out, bad) {
			t.Errorf("%q leaked:\n%s", bad, out)
		}
	}
	for _, good := range []string{"843879365739", "84•••4567", "86•••4567", "0.08", "pin_incorrect", "a0e9e3a16dbe4631a885cd87ce4b6003"} {
		if !strings.Contains(out, good) {
			t.Errorf("%q missing:\n%s", good, out)
		}
	}
	if got := RedactBody([]byte("phone=841234567&op=727799641600")); strings.Contains(got, "841234567") || !strings.Contains(got, "727799641600") {
		t.Errorf("form body: %s", got)
	}
	if got := RedactBody([]byte(strings.Repeat("x", TrailBodyLimit+50))); len(got) > TrailBodyLimit+40 {
		t.Errorf("not cut: %d", len(got))
	}
}
