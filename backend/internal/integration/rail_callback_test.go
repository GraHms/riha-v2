package integration

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"riha/internal/adapters/httpapi"
	"riha/internal/core/app"
	"riha/internal/core/domain"
)

// statusRail stands in for the operator: nothing ends unless it says so when asked.
type statusRail struct {
	mu   sync.Mutex
	out  domain.Outcome
	done bool
}

func (r *statusRail) Initiate(context.Context, domain.ChargeRequest, func(domain.Outcome)) (domain.ChargeResult, error) {
	return domain.ChargeResult{Async: true, Last4: "4567"}, nil
}

func (r *statusRail) Query(context.Context, domain.StatusQuery) (domain.Outcome, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.out, r.done, nil
}

type callbackLog struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *callbackLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *callbackLog) text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func TestBimCallback(t *testing.T) {
	a, ctx := setup(t)
	rail := &statusRail{}
	a.Rail = rail
	m := approved(t, a, ctx, "Loja Callback", "callback@mtoca.test")
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 5000, Description: "Callback", Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "841234567"})
	if err != nil {
		t.Fatal(err)
	}
	logs := &callbackLog{}
	log.SetOutput(logs)
	defer log.SetOutput(os.Stderr)
	srv := httptest.NewServer(httpapi.New(a, httpapi.Config{}))
	defer srv.Close()
	send := func(contentType, body string) {
		t.Helper()
		res, err := http.Post(srv.URL+"/callbacks/bim", contentType, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("status %d: bim must always get an OK", res.StatusCode)
		}
	}
	status := func() string {
		got, _ := a.Payments.ByID(ctx, p.ID)
		return got.Status
	}
	seen := func(s string) {
		t.Helper()
		for i := 0; i < 100; i++ {
			if strings.Contains(logs.text(), s) {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("expected %q in the logs: %s", s, logs.text())
	}

	// Whatever a callback claims, only the operator's own answer counts.
	send("application/json", `{"operationNumber":"`+p.RailReference+`","paymentStatus":"P","phoneNumber":"258841234567"}`)
	seen("the operator has no answer")
	if status() != domain.PaymentProcessing {
		t.Fatalf("a forged callback ended the payment: %s", status())
	}
	if text := logs.text(); strings.Contains(text, "841234567") || !strings.Contains(text, "<number>") {
		t.Fatalf("the number must not reach the logs: %s", text)
	}
	send("text/plain", "not json at all")
	seen("no operation number found")
	send("application/json", `{"data":{"OperationNumber":"000000000000"}}`)
	seen(`no payment for operation "000000000000"`)

	// A callback brings the answer forward when the operator confirms it.
	rail.mu.Lock()
	rail.out, rail.done = domain.Outcome{Success: true, OperatorReference: "NEP1"}, true
	rail.mu.Unlock()
	send("application/x-www-form-urlencoded", "operationNumber="+p.RailReference)
	seen("ended, success true")
	if status() != domain.PaymentSucceeded {
		t.Fatalf("got %s", status())
	}
	send("application/json", `{"operationNumber":"`+p.RailReference+`"}`)
	seen("has already ended")
}
