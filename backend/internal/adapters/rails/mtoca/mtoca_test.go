package mtoca

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"riha/internal/core/domain"
)

type fake struct {
	mu       sync.Mutex
	creates  []map[string]any
	statuses []map[string]any
	reply    map[string]any
	create   int
	drop     bool
	auth     string
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auth = r.Header.Get("Authorization")
	var b map[string]any
	_ = json.NewDecoder(r.Body).Decode(&b)
	switch r.URL.Path {
	case "/api/payments":
		f.creates = append(f.creates, b)
		if f.drop {
			time.Sleep(300 * time.Millisecond)
			return
		}
		if f.create != 0 {
			w.WriteHeader(f.create)
			_, _ = w.Write([]byte(`{"message":"bad"}`))
			return
		}
		_, _ = w.Write([]byte(`{"operationNumber":"` + b["operationNumber"].(string) + `","message":"Pedido enviado"}`))
	case "/api/payments/status":
		f.statuses = append(f.statuses, b)
		_ = json.NewEncoder(w).Encode(f.reply)
	default:
		w.WriteHeader(404)
	}
}

func client(t *testing.T, f *fake, timeout time.Duration) *Client {
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL + "/", Token: "tok", Channels: map[string]string{"mpesa": "BIM"}, PollEvery: 20 * time.Millisecond, PollFor: time.Second,
		HTTP: &http.Client{Timeout: timeout}})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func charge(c *Client, t *testing.T) (domain.ChargeResult, chan domain.Outcome) {
	done := make(chan domain.Outcome, 1)
	res, err := c.Initiate(context.Background(), domain.ChargeRequest{PaymentID: "pay_1", Reference: "123456789012", Description: "Propina de novembro", Method: "mpesa", Amount: 10050, Phone: "84 825 5237"}, func(o domain.Outcome) { done <- o })
	if err != nil {
		t.Fatal(err)
	}
	return res, done
}

func wait(t *testing.T, done chan domain.Outcome) domain.Outcome {
	select {
	case o := <-done:
		return o
	case <-time.After(3 * time.Second):
		t.Fatal("no outcome")
	}
	return domain.Outcome{}
}

func TestPaidAfterPolling(t *testing.T) {
	f := &fake{reply: map[string]any{}}
	c := client(t, f, 5*time.Second)
	res, done := charge(c, t)
	if !res.Async || res.Last4 != "5237" {
		t.Fatalf("result %+v", res)
	}
	time.Sleep(80 * time.Millisecond)
	f.mu.Lock()
	f.reply = map[string]any{"paymentStatus": "P", "transactionId": "MP261105.1234"}
	f.mu.Unlock()
	o := wait(t, done)
	if !o.Success || o.OperatorReference != "MP261105.1234" {
		t.Fatalf("outcome %+v", o)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cr, st := f.creates[0], f.statuses[0]
	if cr["operationNumber"] != "123456789012" || cr["channel"] != "BIM" || cr["amount"] != 100.5 || cr["phoneNumber"] != "+258848255237" || cr["description"] != "Propina de novembro" {
		t.Fatalf("create body %v", cr)
	}
	if st["phoneNumber"] != "258848255237" || st["operationNumber"] != "123456789012" || st["amount"] != 100.5 {
		t.Fatalf("status body %v", st)
	}
	if f.auth != "Bearer tok" {
		t.Fatalf("auth %q", f.auth)
	}
}

func TestStatusCodes(t *testing.T) {
	cases := []struct {
		reply map[string]any
		ok    bool
		code  string
	}{
		{map[string]any{"paymentStatus": "R", "rejectionReason": "01"}, false, domain.FailCustomerTimeout},
		{map[string]any{"paymentStatus": "R", "rejectionReason": "02"}, false, domain.FailRejected},
		{map[string]any{"paymentStatus": "C", "cancelationStatus": "S"}, false, domain.FailCanceled},
		{map[string]any{"data": map[string]any{"PaymentStatus": "p"}}, true, ""},
	}
	for _, tc := range cases {
		o, final := Interpret(tc.reply)
		if !final || o.Success != tc.ok || o.FailureCode != tc.code {
			t.Fatalf("%v → %+v %v", tc.reply, o, final)
		}
	}
	if _, final := Interpret(map[string]any{"paymentStatus": ""}); final {
		t.Fatal("empty status is pending")
	}
}

func TestRefusedAndUncertainCreate(t *testing.T) {
	f := &fake{create: 400}
	c := client(t, f, 5*time.Second)
	res, _ := charge(c, t)
	if res.Async || res.Outcome.FailureCode != domain.FailRejected {
		t.Fatalf("400 must fail now: %+v", res)
	}
	f.create = 503
	if res, _ = charge(c, t); res.Outcome.FailureCode != domain.FailOperatorUnavailable {
		t.Fatalf("503: %+v", res)
	}
	g := &fake{drop: true, reply: map[string]any{"paymentStatus": "P"}}
	c2 := client(t, g, 100*time.Millisecond)
	res, done := charge(c2, t)
	if !res.Async {
		t.Fatal("a timed out create is uncertain and must be polled, not failed")
	}
	if o := wait(t, done); !o.Success || o.OperatorReference != "123456789012" {
		t.Fatalf("outcome %+v", o)
	}
}

func TestInputValidation(t *testing.T) {
	c := client(t, &fake{}, time.Second)
	if _, err := c.Initiate(context.Background(), domain.ChargeRequest{Reference: "1", Method: "emola", Amount: 100, Phone: "861234567"}, nil); err == nil {
		t.Fatal("unmapped method")
	}
	if _, err := c.Initiate(context.Background(), domain.ChargeRequest{Reference: "1", Method: "mpesa", Amount: 100, Phone: "12345"}, nil); err == nil {
		t.Fatal("bad phone")
	}
}

func TestStatusIsThrottled(t *testing.T) {
	f := &fake{reply: map[string]any{}}
	c := client(t, f, time.Second)
	q := domain.StatusQuery{Reference: "999", Method: "mpesa", Amount: 100, Phone: "848255237"}
	_, _, _ = c.Status(context.Background(), q)
	_, _, _ = c.Status(context.Background(), q)
	if len(f.statuses) != 1 {
		t.Fatalf("second check within 30 s must not hit the api, got %d calls", len(f.statuses))
	}
}
