package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"riha/internal/adapters/httpapi"
	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// flakySMS refuses the first n messages as the provider being out of credit, then
// takes them, or refuses one number for good.
type flakySMS struct {
	refuse int
	sent   []ports.OutboundMessage
	// status is what the provider answers when asked about a message.
	status  map[string]string
	lookups int
}

func (f *flakySMS) MessageStatus(_ context.Context, id string) (string, string, error) {
	f.lookups++
	if f.status[id] == domain.MessageFailed {
		return domain.MessageFailed, "rejeitada pela operadora", nil
	}
	return f.status[id], "", nil
}

func (f *flakySMS) Send(_ context.Context, m ports.OutboundMessage) (ports.SentMessage, error) {
	if strings.HasSuffix(m.To, "0000") {
		return ports.SentMessage{}, fmt.Errorf("MozeSMS recusou: número inválido")
	}
	if f.refuse > 0 {
		f.refuse--
		return ports.SentMessage{}, fmt.Errorf("sem crédito na MozeSMS: %w", ports.ErrRetryLater)
	}
	f.sent = append(f.sent, m)
	return ports.SentMessage{ProviderID: fmt.Sprintf("moze-%d", len(f.sent)), Channel: domain.ChannelSMS}, nil
}

func TestSMSRetriesAndDeliveryReports(t *testing.T) {
	a, ctx := setup(t)
	clock := at("2026-11-09 10:00")
	a.Now = func() time.Time { return clock }
	sms := &flakySMS{refuse: 1, status: map[string]string{}}
	a.Messenger = sms
	m := approved(t, a, ctx, "Ginásio Força", "forca@ginasio.co.mz")
	queue := func(id, to, channel string) {
		if err := a.Messages.Create(ctx, &domain.Message{ID: id, MerchantID: m.ID, Livemode: true, MemberID: "mem_1", Channel: channel, To: to,
			Sender: "Forca", Kind: "before", Body: "Ginásio Força: a mensalidade vence amanhã", Status: domain.MessageQueued, CreatedAt: clock}); err != nil {
			t.Fatal(err)
		}
	}
	get := func(id string) domain.Message {
		ms, err := a.Messages.ByMember(ctx, "mem_1", 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range ms {
			if x.ID == id {
				return x
			}
		}
		t.Fatalf("message %s not found", id)
		return domain.Message{}
	}
	queue("msg_a", "841110001", domain.ChannelWhatsApp)
	queue("msg_bad", "841230000", domain.ChannelSMS)

	// Out of credit: the message waits a minute; the bad number fails for good.
	a.SendQueued(ctx)
	x := get("msg_a")
	if x.Status != domain.MessageQueued || x.Attempts != 1 || x.NextAttemptAt == nil || !x.NextAttemptAt.Equal(clock.Add(time.Minute)) || !strings.Contains(x.Error, "sem crédito") {
		t.Fatalf("after a refusal: %+v", x)
	}
	if b := get("msg_bad"); b.Status != domain.MessageFailed || b.Error != "MozeSMS recusou: número inválido" {
		t.Fatalf("bad number: %+v", b)
	}
	a.SendQueued(ctx)
	if len(sms.sent) != 0 {
		t.Fatal("a message was sent again before its wait was over")
	}
	clock = clock.Add(2 * time.Minute)
	a.SendQueued(ctx)
	x = get("msg_a")
	if x.Status != domain.MessageSent || x.ProviderID != "moze-1" || x.Channel != domain.ChannelSMS || x.Error != "" || x.SentAt == nil || len(sms.sent) != 1 || !sms.sent[0].Live {
		t.Fatalf("after the wait: %+v, sent %+v", x, sms.sent)
	}

	// A report only makes RIHA ask MozeSMS: while MozeSMS says it isn't final, a
	// report claiming delivery changes nothing.
	srv := httptest.NewServer(httpapi.New(a, httpapi.Config{SMSWebhookSecret: "whsec"}))
	defer srv.Close()
	report := `{"event":"sms.delivered","timestamp":"2026-11-09T12:00:35+02:00","webhook_id":"w1","data":{"message_id":"moze-1","external_id":"SMS_1","phone":"258841110001",
		"status":"entregue","gateway_status":"DELIVERED","error_code":"","delivery_time":"2026-11-09 12:00:31","delivered_to_device":true}}`
	post := func(body string) int {
		res, err := http.Post(srv.URL+"/callbacks/mozesms", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if code := post(report); code != 200 || get("msg_a").Status != domain.MessageSent || sms.lookups != 1 {
		t.Fatalf("a report MozeSMS doesn't back changed the message (%d, %s, %d lookups)", code, get("msg_a").Status, sms.lookups)
	}
	sms.status["moze-1"] = domain.MessageDelivered
	if code := post(report); code != 200 {
		t.Fatalf("report got %d", code)
	}
	x = get("msg_a")
	if x.Status != domain.MessageDelivered || x.DeliveredAt == nil || !x.DeliveredAt.Equal(at("2026-11-09 12:00").Add(31*time.Second)) {
		t.Fatalf("after the report: %+v", x)
	}
	// Once final, reports don't even ask; unknown messages and junk are ignored.
	sms.status["moze-1"] = domain.MessageFailed
	_ = post(strings.Replace(report, "sms.delivered", "sms.failed", 1))
	if code := post(strings.Replace(report, "moze-1", "moze-404", 1)); code != 200 || get("msg_a").Status != domain.MessageDelivered || sms.lookups != 2 {
		t.Fatalf("a later report changed a delivered message (%d, %d lookups)", code, sms.lookups)
	}
	if code := post("not json"); code != 200 {
		t.Fatalf("junk got %d", code)
	}
}

func TestSMSGivesUpAfterTheLastWait(t *testing.T) {
	a, ctx := setup(t)
	clock := at("2026-11-09 10:00")
	a.Now = func() time.Time { return clock }
	a.Messenger = &flakySMS{refuse: 100}
	m := approved(t, a, ctx, "Loja Maputo", "loja@maputo.co.mz")
	if err := a.Messages.Create(ctx, &domain.Message{ID: "msg_z", MerchantID: m.ID, Livemode: true, MemberID: "mem_2", Channel: domain.ChannelSMS, To: "841112222",
		Kind: "before", Body: "x", Status: domain.MessageQueued, CreatedAt: clock}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		a.SendQueued(ctx)
		clock = clock.Add(4 * time.Hour)
	}
	ms, _ := a.Messages.ByMember(ctx, "mem_2", 1)
	if len(ms) != 1 || ms[0].Status != domain.MessageFailed || ms[0].Attempts != 6 || !strings.Contains(ms[0].Error, "sem crédito") {
		t.Fatalf("after every wait: %+v", ms)
	}
}

func kinds(t *testing.T, a *app.App, ctx context.Context, memberID string) map[string][]domain.Message {
	t.Helper()
	ms, err := a.Messages.ByMember(ctx, memberID, 50)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]domain.Message{}
	for _, m := range ms {
		out[m.Kind] = append(out[m.Kind], m)
	}
	return out
}

func TestCatalogSignupGetsWelcomeOrOneReminder(t *testing.T) {
	a, ctx := setup(t)
	clock := at("2026-10-19 10:00")
	a.Now = func() time.Time { return clock }
	m := approved(t, a, ctx, "Escola Sol", "escola@sol.co.mz")
	p := schoolPlan(t, a, ctx, m, "Propina 5ª a 7ª classe", 250000, domain.MissedDebt)
	m.Status = domain.MerchantActive

	// Signs up and pays: one welcome, which says when the next payment is, and no
	// separate receipt.
	tok, err := a.Subscribe(ctx, m.Slug, app.SubscribeInput{MemberInput: app.MemberInput{PlanIDs: []string{p.ID}, Phone: "871112233",
		Fields: map[string]string{"Nome do aluno": "Ivan Nhantumbo", "Turma": "6ª A"}}, Accept: true})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := a.Invoice(ctx, tok, false)
	if _, err := a.PayInvoice(ctx, tok, app.InvoicePay{Method: "emola", UseLast: true}); err != nil {
		t.Fatal(err)
	}
	got := kinds(t, a, ctx, v.Member.ID)
	if len(got["welcome"]) != 1 || len(got["receipt"]) != 0 {
		t.Fatalf("after the first payment: %+v", got)
	}
	body := got["welcome"][0].Body
	link := "http://localhost:5173/m/" + v.Member.Token
	if body != "Escola Sol: inscrição de Ivan em Propina 5ª a 7ª classe ativa. Pagou 2\u00a0500 MT. Próximo: 2\u00a0500 MT a 5 nov. Gerir: "+link || len(v.Member.Token) < 16 {
		t.Fatalf("welcome %q", body)
	}

	// Signs up and leaves: one reminder after an hour, never two, and none for the
	// one who paid.
	tok2, err := a.Subscribe(ctx, m.Slug, app.SubscribeInput{MemberInput: app.MemberInput{PlanIDs: []string{p.ID}, Phone: "841114455",
		Fields: map[string]string{"Nome do aluno": "Rita Cossa", "Turma": "5ª B"}}, Accept: true})
	if err != nil {
		t.Fatal(err)
	}
	v2, _ := a.Invoice(ctx, tok2, false)
	clock = clock.Add(30 * time.Minute)
	a.BillingTick(ctx)
	if n := len(kinds(t, a, ctx, v2.Member.ID)["unfinished"]); n != 0 {
		t.Fatalf("reminded after 30 minutes: %d", n)
	}
	clock = clock.Add(31 * time.Minute)
	a.BillingTick(ctx)
	clock = clock.Add(2 * time.Hour)
	a.BillingTick(ctx)
	un := kinds(t, a, ctx, v2.Member.ID)["unfinished"]
	if len(un) != 1 || un[0].Body != "Escola Sol: a inscrição de Rita Cossa ficou por concluir. Pague hoje para ativar: http://localhost:5173/i/"+tok2 {
		t.Fatalf("unfinished reminders %+v", un)
	}
	if n := len(kinds(t, a, ctx, v.Member.ID)["unfinished"]); n != 0 {
		t.Fatalf("the member who paid got %d reminders", n)
	}
	// Paying from the reminder still welcomes her.
	if _, err := a.PayInvoice(ctx, tok2, app.InvoicePay{Method: "mpesa", UseLast: true}); err != nil {
		t.Fatal(err)
	}
	if n := len(kinds(t, a, ctx, v2.Member.ID)["welcome"]); n != 1 {
		t.Fatalf("welcome after the reminder: %d", n)
	}
}

func TestMemberPageAndCancel(t *testing.T) {
	a, ctx := setup(t)
	clock := at("2026-10-19 10:00")
	a.Now = func() time.Time { return clock }
	m := approved(t, a, ctx, "Ginásio Força", "forca@ginasio.co.mz")
	p := schoolPlan(t, a, ctx, m, "Mensalidade", 150000, domain.MissedDebt)
	m.Status = domain.MerchantActive
	join := func(phone, name string) *app.InvoiceView {
		tok, err := a.Subscribe(ctx, m.Slug, app.SubscribeInput{MemberInput: app.MemberInput{PlanIDs: []string{p.ID}, Phone: phone,
			Fields: map[string]string{"Nome do aluno": name, "Turma": "A"}}, Accept: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.PayInvoice(ctx, tok, app.InvoicePay{Method: "mpesa", UseLast: true}); err != nil {
			t.Fatal(err)
		}
		v, _ := a.Invoice(ctx, tok, false)
		return v
	}
	ana, rui := join("841112233", "Ana Tembe"), join("841114455", "Rui Sitoe")

	srv := httptest.NewServer(httpapi.New(a, httpapi.Config{}))
	defer srv.Close()
	call := func(method, path, body string) (int, map[string]any) {
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Requested-With", "riha")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}
	code, page := call("GET", "/api/public/members/"+ana.Member.Token, "")
	subs, _ := page["subscriptions"].([]any)
	pays, _ := page["payments"].([]any)
	if code != 200 || len(subs) != 1 || len(pays) == 0 || page["due"].(float64) != 0 {
		t.Fatalf("page %d %v", code, page)
	}
	sub := subs[0].(map[string]any)
	if sub["plan_name"] != "Mensalidade" || sub["status"] != domain.SubActive || sub["can_cancel"] != true || sub["next_due"] == "" {
		t.Fatalf("subscription %v", sub)
	}
	if first := pays[len(pays)-1].(map[string]any); first["status"] != domain.ChargePaid || first["method"] != "M-Pesa" {
		t.Fatalf("first payment %v", first)
	}
	if code, _ := call("GET", "/api/public/members/not-a-real-token-at-all", ""); code != 404 {
		t.Fatalf("unknown token got %d", code)
	}
	// Ana's link can't cancel Rui's subscription.
	ruiSubs, _ := a.Subs.ByMember(ctx, rui.Member.ID)
	if code, _ := call("POST", "/api/public/members/"+ana.Member.Token+"/cancel", `{"subscription_id":"`+ruiSubs[0].ID+`"}`); code != 404 {
		t.Fatalf("cancelling someone else's subscription got %d", code)
	}
	if s, _ := a.Subs.Get(ctx, ruiSubs[0].ID); s.Status != domain.SubActive {
		t.Fatalf("rui's subscription is %s", s.Status)
	}
	code, page = call("POST", "/api/public/members/"+ana.Member.Token+"/cancel", `{"subscription_id":"`+sub["id"].(string)+`"}`)
	sub = page["subscriptions"].([]any)[0].(map[string]any)
	if code != 200 || sub["status"] != domain.SubCanceled || sub["can_cancel"] != false || sub["next_due"] != "" {
		t.Fatalf("after cancelling %d %v", code, sub)
	}
	got := kinds(t, a, ctx, ana.Member.ID)
	if len(got["cancel"]) != 1 || !strings.Contains(got["cancel"][0].Body, "Mensalidade de Ana Tembe foi cancelada") {
		t.Fatalf("cancel message %+v", got["cancel"])
	}
	// Cancelling again changes nothing and sends nothing.
	if code, _ := call("POST", "/api/public/members/"+ana.Member.Token+"/cancel", `{"subscription_id":"`+sub["id"].(string)+`"}`); code != 200 || len(kinds(t, a, ctx, ana.Member.ID)["cancel"]) != 1 {
		t.Fatalf("second cancel %d", code)
	}
}

func TestPlanMinimumAndBetaProgram(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Loja Beta", "loja@beta.co.mz")
	plan := func(amount int64) (*domain.Plan, error) {
		return a.CreatePlan(ctx, m, true, app.PlanInput{Name: "Teste", Amount: amount, Interval: domain.IntervalWeek, Anchor: domain.AnchorFixed,
			BillingDay: 1, Months: domain.AllMonths, Missed: domain.MissedDebt}, app.Actor{ID: "owner"})
	}
	if _, err := plan(40_000); err == nil || !strings.Contains(err.Error(), "pelo menos 500 MT") {
		t.Fatalf("a 400 MT plan: %v", err)
	}
	if _, err := plan(50_000); err != nil {
		t.Fatalf("a 500 MT plan: %v", err)
	}
	one, name := int64(100), "Teste 1 MT"
	if _, err := a.CreateLink(ctx, m, true, app.LinkParams{Name: &name, Amount: &one}, "owner", "dashboard"); err == nil {
		t.Fatal("a 1 MT link outside the beta")
	}

	if err := a.SetMerchantBeta(ctx, m.ID, true, app.Actor{ID: "admin", Role: "admin"}); err != nil {
		t.Fatal(err)
	}
	m, _ = a.Merchants.ByID(ctx, m.ID)
	p, err := plan(100)
	if err != nil {
		t.Fatalf("a 1 MT plan in the beta: %v", err)
	}
	if _, err := a.CreateLink(ctx, m, true, app.LinkParams{Name: &name, Amount: &one}, "owner", "dashboard"); err != nil {
		t.Fatalf("a 1 MT link in the beta: %v", err)
	}
	// The plan's 1 MT invoice gets through the checkout too.
	tok, err := a.Subscribe(ctx, m.Slug, app.SubscribeInput{MemberInput: app.MemberInput{PlanIDs: []string{p.ID}, Phone: "841112233", Name: "Ismael"}, Accept: true})
	if err != nil {
		t.Fatal(err)
	}
	if pay, err := a.PayInvoice(ctx, tok, app.InvoicePay{Method: "mpesa", UseLast: true}); err != nil || pay.Amount != 100 {
		t.Fatalf("paying 1 MT: %+v %v", pay, err)
	}
}
