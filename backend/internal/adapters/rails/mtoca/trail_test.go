package mtoca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"riha/internal/core/domain"
)

// The trail sees what went to mtoca and what came back, with the request id, and
// the status questions only when the answer changes.
func TestTrailOfAPayment(t *testing.T) {
	f := &fake{reply: map[string]any{"paymentStatus": "N", "paymentType": "M"}}
	var hdr sync.Map
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr.Store(r.URL.Path, r.Header.Get("X-Request-Id"))
		f.ServeHTTP(w, r)
	}))
	defer srv.Close()
	var mu sync.Mutex
	var evs []domain.TrailEvent
	c, err := New(Config{BaseURL: srv.URL, Channels: map[string]string{"mpesa": "MPESA"}, PollEvery: 10 * time.Millisecond, PollFor: time.Second, CreateFor: time.Second,
		Trace: func(ev domain.TrailEvent) { mu.Lock(); evs = append(evs, ev); mu.Unlock() }})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan domain.Outcome, 1)
	ctx := domain.WithRequest(context.Background(), domain.RequestInfo{ID: "req_abc"})
	if _, err := c.Initiate(ctx, domain.ChargeRequest{PaymentID: "pay_1", Reference: "123456789012", Method: "mpesa", Amount: 1000, Phone: "841234567"}, func(o domain.Outcome) { done <- o }); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	f.mu.Lock()
	f.reply = map[string]any{"paymentStatus": "P", "paymentType": "M", "transactionID_NEP": nep}
	f.mu.Unlock()
	if o := wait(t, done); !o.Success {
		t.Fatalf("outcome %+v", o)
	}
	for _, p := range []string{"/api/payments", "/api/payments/status"} {
		if v, _ := hdr.Load(p); v != "req_abc" {
			t.Errorf("%s: X-Request-Id %v", p, v)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	kinds := []string{}
	for _, ev := range evs {
		kinds = append(kinds, ev.Kind)
		if ev.PaymentID != "pay_1" || ev.RequestID != "req_abc" {
			t.Errorf("event %s without its payment or request: %+v", ev.Kind, ev)
		}
		if strings.Contains(ev.Request, "841234567") {
			t.Errorf("the phone number leaked: %s", ev.Request)
		}
	}
	want := "rail.request rail.response rail.status rail.status"
	if got := strings.Join(kinds, " "); got != want {
		t.Fatalf("events %q, want %q (one per change, not one per question)", got, want)
	}
	if evs[2].Title != "Estado no bim: pendente (N)" || evs[3].Title != "Estado no bim: pago (P)" {
		t.Errorf("titles %q, %q", evs[2].Title, evs[3].Title)
	}
	if n, _ := evs[3].Data["questions"].(int); n < 2 {
		t.Errorf("the questions asked must be counted: %v", evs[3].Data)
	}
}
