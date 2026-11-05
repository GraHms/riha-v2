package mtoca

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

const nep = "8d2e0220af6b4c1d918e2868746e7560"

// fake answers as mtoca's production API did on 2026-11-05: 201s, the id on create,
// "+258" refused, and any status field beyond the two it takes refused.
type fake struct {
	mu       sync.Mutex
	creates  []map[string]any
	statuses []map[string]any
	reply    map[string]any
	status   int
	create   int
	message  string
	noID     bool
	hang     time.Duration
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var b map[string]any
	_ = json.NewDecoder(r.Body).Decode(&b)
	if r.URL.Path == "/api/payments" && f.hang > 0 {
		time.Sleep(f.hang)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	fail := func(code int, msg string) {
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "ERR", "message": msg})
	}
	switch r.URL.Path {
	case "/api/payments":
		f.creates = append(f.creates, b)
		if strings.HasPrefix(b["phoneNumber"].(string), "+") {
			fail(400, "not a valid mz-msisdn")
			return
		}
		if f.create != 0 {
			fail(f.create, f.message)
			return
		}
		w.WriteHeader(201)
		out := map[string]any{"operationNumber": b["operationNumber"], "vendorTransactionStatus": "OK"}
		if !f.noID {
			out["vendorTransactionId_NEP"] = nep
		}
		_ = json.NewEncoder(w).Encode(out)
	case "/api/payments/status":
		f.statuses = append(f.statuses, b)
		if len(b) != 2 || b["transactionID_NEP"] == nil || b["operationNumber"] == nil {
			fail(400, "invalid or missing field")
			return
		}
		if f.status != 0 {
			fail(f.status, "EOF")
			return
		}
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(f.reply)
	default:
		w.WriteHeader(404)
	}
}

func client(t *testing.T, f *fake) *Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL + "/", Token: "tok", Channels: map[string]string{"mpesa": "MPESA", "emola": "EMOLA", "bim": "BIM"}, PollEvery: 10 * time.Millisecond, PollFor: 300 * time.Millisecond, CreateFor: 200 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type run struct {
	done chan domain.Outcome
	ids  chan string
}

func charge(t *testing.T, c *Client) (domain.ChargeResult, run) {
	t.Helper()
	r := run{done: make(chan domain.Outcome, 1), ids: make(chan string, 1)}
	res, err := c.Initiate(context.Background(), domain.ChargeRequest{PaymentID: "pay_1", Reference: "123456789012", Description: "Propina de novembro", Method: "mpesa", Amount: 10050, Phone: "84 825 5237", OnRailID: func(id string) { r.ids <- id }}, func(o domain.Outcome) { r.done <- o })
	if err != nil {
		t.Fatal(err)
	}
	return res, r
}

func wait(t *testing.T, done chan domain.Outcome) domain.Outcome {
	t.Helper()
	select {
	case o := <-done:
		return o
	case <-time.After(3 * time.Second):
		t.Fatal("no outcome")
	}
	return domain.Outcome{}
}

func TestPaidAfterPolling(t *testing.T) {
	f := &fake{reply: map[string]any{"paymentStatus": ""}}
	res, r := charge(t, client(t, f))
	if !res.Async || res.Last4 != "5237" {
		t.Fatalf("result %+v", res)
	}
	if id := <-r.ids; id != nep {
		t.Fatalf("the id must be kept: %q", id)
	}
	time.Sleep(40 * time.Millisecond)
	f.mu.Lock()
	f.reply = map[string]any{"paymentStatus": "P", "transactionID_NEP": nep, "paymentType": "M", "operationNumber": "123456789012"}
	f.mu.Unlock()
	if o := wait(t, r.done); !o.Success || o.OperatorReference != nep {
		t.Fatalf("outcome %+v", o)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cr := f.creates[0]
	if cr["operationNumber"] != "123456789012" || cr["channel"] != "MPESA" || cr["amount"] != 100.5 || cr["phoneNumber"] != "258848255237" || cr["description"] != "Propina de novembro" {
		t.Fatalf("create body %v", cr)
	}
}

// mtoca answers only when bim does, so the checkout must not wait for it.
func TestInitiateDoesNotWaitForTheAnswer(t *testing.T) {
	f := &fake{hang: 100 * time.Millisecond, reply: map[string]any{"paymentStatus": "P"}}
	start := time.Now()
	res, r := charge(t, client(t, f))
	if took := time.Since(start); took > 50*time.Millisecond || !res.Async {
		t.Fatalf("took %s: %+v", took, res)
	}
	if o := wait(t, r.done); !o.Success || o.OperatorReference != nep {
		t.Fatalf("outcome %+v", o)
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
		{map[string]any{"paymentStatus": "R", "rejectionReason": "99"}, false, domain.FailRejected},
		{map[string]any{"paymentStatus": "C", "cancelationStatus": "S"}, false, domain.FailCanceled},
		{map[string]any{"paymentStatus": "P", "transactionID_NEP": nep}, true, ""},
	}
	for _, tc := range cases {
		o, final := Interpret(tc.reply)
		if !final || o.Success != tc.ok || o.FailureCode != tc.code {
			t.Fatalf("%v → %+v %v", tc.reply, o, final)
		}
	}
	for _, s := range []string{"", "N"} {
		if _, final := Interpret(map[string]any{"paymentStatus": s, "errorMessage": "Transacção não encontrada"}); final {
			t.Fatalf("status %q is pending", s)
		}
	}
	// The create call's OK only means bim took it: it must never read as paid.
	if o, final := Interpret(map[string]any{"vendorTransactionStatus": "OK", "vendorTransactionId_NEP": nep}); final || o.Success {
		t.Fatal("a created payment is not a paid one")
	}
}

func TestRefusedCreate(t *testing.T) {
	cases := []struct {
		status int
		msg    string
		code   string
	}{
		{400, "invalid field", domain.FailRejected},
		{409, "Saldo insuficiente", domain.FailRejected},
		{409, "EOF", domain.FailOperatorUnavailable},
		{409, `Post "https://x": context deadline exceeded`, domain.FailOperatorUnavailable},
		{401, "no", domain.FailOperatorUnavailable},
		{429, "slow", domain.FailOperatorUnavailable},
	}
	for _, tc := range cases {
		res, r := charge(t, client(t, &fake{create: tc.status, message: tc.msg}))
		if !res.Async {
			t.Fatalf("%d: %+v", tc.status, res)
		}
		if o := wait(t, r.done); o.Success || o.FailureCode != tc.code {
			t.Fatalf("%d %q: %+v", tc.status, tc.msg, o)
		}
	}
}

// With no answer there is no id to ask with: the outcome is left for the sweep to close.
func TestLostAnswerIsNotCalledAFailure(t *testing.T) {
	for name, f := range map[string]*fake{"5xx": {create: 503, message: "bad gateway"}, "timeout": {hang: 400 * time.Millisecond}, "no id": {noID: true}} {
		_, r := charge(t, client(t, f))
		select {
		case o := <-r.done:
			t.Fatalf("%s: the customer may have paid, so it must not end here: %+v", name, o)
		case <-time.After(500 * time.Millisecond):
		}
		f.mu.Lock()
		if len(f.statuses) != 0 {
			t.Fatalf("%s: no id to ask with: %v", name, f.statuses)
		}
		f.mu.Unlock()
	}
}

func TestQuery(t *testing.T) {
	f := &fake{reply: map[string]any{}}
	c := client(t, f)
	if _, known, err := c.Query(context.Background(), domain.StatusQuery{Reference: "1", Method: "mpesa"}); known || err != nil {
		t.Fatalf("no id reads as no record: %v %v", known, err)
	}
	q := domain.StatusQuery{Reference: "999", RailID: nep, Method: "mpesa"}
	for i := 0; i < 2; i++ {
		// Pending must not read as "no record", which closes the payment after 5 minutes.
		if _, known, err := c.Query(context.Background(), q); known || !errors.Is(err, ports.ErrStillPending) {
			t.Fatalf("check %d: known %v, err %v", i, known, err)
		}
	}
	if len(f.statuses) != 1 {
		t.Fatalf("second check within 30 s must not hit the api, got %d calls", len(f.statuses))
	}
	f.status = 409
	if _, known, err := c.Query(context.Background(), domain.StatusQuery{Reference: "998", RailID: nep, Method: "mpesa"}); known || err == nil || errors.Is(err, ports.ErrStillPending) {
		t.Fatalf("a failed check is an error: %v %v", known, err)
	}
}

func TestThreeWallets(t *testing.T) {
	f := &fake{reply: map[string]any{"paymentStatus": "P"}}
	c := client(t, f)
	for _, tc := range []struct{ method, phone, channel string }{{"mpesa", "841112233", "MPESA"}, {"emola", "871112233", "EMOLA"}, {"bim", "821112233", "BIM"}} {
		done := make(chan domain.Outcome, 1)
		if _, err := c.Initiate(context.Background(), domain.ChargeRequest{Reference: "1" + tc.phone, Method: tc.method, Amount: 500, Phone: tc.phone}, func(o domain.Outcome) { done <- o }); err != nil {
			t.Fatalf("%s: %v", tc.method, err)
		}
		if o := wait(t, done); !o.Success {
			t.Fatalf("%s: %+v", tc.method, o)
		}
		f.mu.Lock()
		last := f.creates[len(f.creates)-1]
		f.mu.Unlock()
		if last["channel"] != tc.channel {
			t.Fatalf("%s sent channel %v", tc.method, last["channel"])
		}
	}
	if _, err := c.Initiate(context.Background(), domain.ChargeRequest{Reference: "2", Method: "emola", Amount: 500, Phone: "841112233"}, nil); err == nil {
		t.Fatal("an M-Pesa number on e-Mola must be refused before calling mtoca")
	}
	if n := c.NewReference("mpesa", true); len(n) != 12 {
		t.Fatalf("reference %q", n)
	}
}
