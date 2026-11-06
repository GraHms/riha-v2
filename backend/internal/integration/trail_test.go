package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"riha/internal/adapters/postgres"
	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// One payment leaves one trail: the customer's request with their network, the
// outcome, and the operator's callback with its commission, found by any of its ids.
func TestTrailOfAPayment(t *testing.T) {
	a, ctx := setup(t)
	store, err := postgres.Open(os.Getenv("RIHA_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	trail := postgres.NewTrail(store)
	run, stop := context.WithCancel(context.Background())
	go trail.Run(run)
	a.Trail, a.Insights = trail, postgres.Insights{Store: store}

	m := approved(t, a, ctx, "Loja do Rasto", "rasto@example.com")
	l, err := a.CreateLink(ctx, m, false, app.LinkParams{Name: ptr("Bolo"), Amount: ptr(int64(1000)), Methods: []string{"mpesa"}}, "owner", "dashboard")
	if err != nil {
		t.Fatal(err)
	}
	req := domain.WithRequest(ctx, domain.RequestInfo{ID: "req_trailtest1", IP: "197.235.71.20", UserAgent: "Mozilla/5.0 (Linux; Android 10; K) Chrome/141"})
	cs, err := a.CreateLinkSession(req, l, 0, "whatsapp", "197.235.71.20", "Mozilla/5.0 (Linux; Android 10; K) Chrome/141")
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(req, cs.ID, app.PayInput{Method: "mpesa", Phone: "84 123 4567", PayerIP: "197.235.71.20", PayerUA: "Mozilla/5.0 (Linux; Android 10; K) Chrome/141"})
	if err != nil {
		t.Fatal(err)
	}
	a.RailCallback(req, p.RailReference, "", []byte(`{"operationNumber":"`+p.RailReference+`","amount":10.0,"comissionAmount":0.08,"paymentType":"M","phoneNumber":"258841234567"}`))
	if err := a.ClientTelemetry(req, cs.ID, []app.ClientEvent{{Name: "opened", Data: map[string]any{"connection": "4g", "ready_ms": 900.0}}, {Name: "hacked", Data: map[string]any{"x": 1}}}); err != nil {
		t.Fatal(err)
	}
	// What the mtoca client sends has no level; a good answer must not count as an error.
	trail.Add(domain.TrailEvent{PaymentID: p.ID, Kind: domain.TrailRailResponse, Source: "operator", Status: 200, Title: "O bim respondeu"})
	stop()
	trail.Wait(5 * time.Second)
	if h, err := (postgres.Insights{Store: store}).Health(ctx, time.Now().Add(-time.Hour)); err != nil || h.OperatorErrors != 0 {
		t.Errorf("a good answer counted as an operator error: %+v %v", h, err)
	}

	th, err := a.Thread(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]domain.TrailEvent{}
	for _, ev := range th.Events {
		seen[ev.Kind] = ev
		if strings.Contains(ev.Request+ev.Response+ev.Title, "841234567") {
			t.Errorf("%s: the phone number leaked", ev.Kind)
		}
	}
	for _, k := range []string{domain.TrailCheckoutCreated, domain.TrailSubmitted, domain.TrailPaymentStatus, domain.TrailRailCallback, domain.TrailClient} {
		if _, ok := seen[k]; !ok {
			t.Errorf("no %s on the trail: %v", k, th.Events)
		}
	}
	if ev := seen[domain.TrailSubmitted]; ev.Network != "Vodacom" || ev.RequestID != "req_trailtest1" || ev.MerchantID != m.ID {
		t.Errorf("submitted: %+v", ev)
	}
	if ev := seen[domain.TrailRailCallback]; ev.Data["commission"] != "0.08" || ev.PaymentID != p.ID {
		t.Errorf("callback: %+v", ev)
	}
	for _, ev := range th.Events {
		if ev.Kind == domain.TrailClient && ev.Data["event"] == "hacked" {
			t.Error("an unknown client event was kept")
		}
	}
	for _, q := range []string{p.ID, p.Reference, p.RailReference, "req_trailtest1", cs.ID, "4567"} {
		hits, err := a.FindThreads(ctx, q)
		if err != nil || len(hits) == 0 || hits[0].SessionID != cs.ID {
			t.Errorf("search %q: %+v %v", q, hits, err)
		}
	}
	mv, err := a.AdminMetrics(ctx, ports.MetricsQuery{From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Minute), Live: false})
	if err != nil {
		t.Fatal(err)
	}
	if len(mv.Commission) != 1 || mv.Commission[0].Commission != 0.08 || len(mv.Networks) == 0 || mv.Networks[0].Network != "Vodacom" || mv.Funnel.Opened == 0 {
		t.Errorf("metrics %+v", mv.Metrics)
	}
}
