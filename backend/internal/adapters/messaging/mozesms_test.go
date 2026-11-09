package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func mozeServer(t *testing.T, status int, reply string, got *map[string]string) *MozeSMS {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sms/send" || r.Header.Get("X-API-Key") != "k" || r.Header.Get("X-API-Secret") != "s" {
			t.Errorf("request %s key=%q", r.URL.Path, r.Header.Get("X-API-Key"))
		}
		if got != nil {
			_ = json.NewDecoder(r.Body).Decode(got)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	c, err := NewMozeSMS(MozeSMSConfig{BaseURL: srv.URL, APIKey: "k", APISecret: "s", Sender: "RIHA", Senders: []string{"EPrimavera"}, LowBalance: 100})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestMozeSMSSends(t *testing.T) {
	var got map[string]string
	c := mozeServer(t, 200, `{"success":true,"data":{"id":"MTIzfDE3MDAwMDAwMDB8Nw==","status":"enviado","parts":1,"cost":1.35,"remaining_balance":44.15}}`, &got)
	sent, err := c.Send(context.Background(), ports.OutboundMessage{Channel: "sms", To: "84 123 4567", Sender: "EPrimavera", Body: "Escola: a propina não foi paga", Live: true})
	if err != nil || sent.ProviderID != "MTIzfDE3MDAwMDAwMDB8Nw==" || sent.Channel != domain.ChannelSMS {
		t.Fatalf("sent %+v %v", sent, err)
	}
	if got["phone"] != "258841234567" || got["sender_id"] != "EPrimavera" || got["message"] != "Escola: a propina nao foi paga" {
		t.Fatalf("body %v", got)
	}
	h, _ := c.Health()
	if h.Balance != 44.15 || h.AccountError != "" {
		t.Fatalf("health %+v", h)
	}
}

func TestMozeSMSUnapprovedSenderUsesRIHAs(t *testing.T) {
	var got map[string]string
	c := mozeServer(t, 200, `{"success":true,"data":{"id":"x1"}}`, &got)
	if _, err := c.Send(context.Background(), ports.OutboundMessage{To: "841234567", Sender: "Ginasio", Body: "ola"}); err != nil {
		t.Fatal(err)
	}
	if got["sender_id"] != "RIHA" {
		t.Fatalf("sender %q", got["sender_id"])
	}
}

func TestMozeSMSRefusals(t *testing.T) {
	for _, tc := range []struct {
		status  int
		reply   string
		retry   bool
		account string
	}{
		{402, `{"success":false,"error":"Créditos insuficientes"}`, true, "sem crédito na MozeSMS"},
		{401, `{"success":false,"error":"Token inválido"}`, true, "a MozeSMS recusou a conta (Token inválido)"},
		{429, `{"success":false,"error":"Too many"}`, true, ""},
		{503, `oops`, true, ""},
		{400, `{"success":false,"error":"Número inválido"}`, false, ""},
		{200, `{"success":false,"error":"Remetente não aprovado"}`, false, ""},
	} {
		c := mozeServer(t, tc.status, tc.reply, nil)
		_, err := c.Send(context.Background(), ports.OutboundMessage{To: "841234567", Body: "x"})
		if err == nil || errors.Is(err, ports.ErrRetryLater) != tc.retry {
			t.Fatalf("%d: err %v, want retry=%v", tc.status, err, tc.retry)
		}
		if h, _ := c.Health(); h.AccountError != tc.account {
			t.Fatalf("%d: account error %q", tc.status, h.AccountError)
		}
	}
}

func TestMozeSMSNoAnswerIsRetried(t *testing.T) {
	c, _ := NewMozeSMS(MozeSMSConfig{BaseURL: "http://127.0.0.1:1", APIKey: "k", APISecret: "s"})
	if _, err := c.Send(context.Background(), ports.OutboundMessage{To: "841234567", Body: "x"}); !errors.Is(err, ports.ErrRetryLater) {
		t.Fatalf("err %v", err)
	}
	if _, err := c.Send(context.Background(), ports.OutboundMessage{To: "21 123 456", Body: "x"}); err == nil || errors.Is(err, ports.ErrRetryLater) {
		t.Fatalf("a landline must fail for good: %v", err)
	}
}

func TestPlainSMS(t *testing.T) {
	for in, want := range map[string]string{
		"Propina de Março: não pagámos à escola — já!": "Propina de Marco: nao pagamos à escola - ja!",
		"É ótimo, Ação, «ok»":                          "É otimo, Acao, «ok»",
		"Olá 👋":                                        "Olá 👋",
	} {
		if strings.Contains(want, "«") {
			want = in // « isn't GSM nor turned: the text stays as it was
		}
		if got := PlainSMS(in); got != want {
			t.Errorf("PlainSMS(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReportOutcome(t *testing.T) {
	var r MozeSMSReport
	_ = json.Unmarshal([]byte(`{"event":"sms.delivered","data":{"message_id":"m1","status":"entregue","gateway_status":"DELIVERED","delivery_time":"2026-04-23 12:33:41"}}`), &r)
	if s, _ := r.Outcome(); s != domain.MessageDelivered {
		t.Fatalf("delivered: %q", s)
	}
	r.Event, r.Data.GatewayStatus, r.Data.ErrorCode = "sms.expired", "EXPIRED", "27"
	if s, why := r.Outcome(); s != domain.MessageFailed || why != "expirou sem chegar ao telefone (código 27)" {
		t.Fatalf("expired: %q %q", s, why)
	}
	r.Event, r.Data.GatewayStatus = "sms.sent", "ACCEPTED"
	if s, _ := r.Outcome(); s != "" {
		t.Fatalf("accepted is not final: %q", s)
	}
}

func TestRouterKeepsTestModeInTheOutbox(t *testing.T) {
	sms := &countSMS{}
	r := Router{SMS: sms, Outbox: &Outbox{}}
	if _, err := r.Send(context.Background(), ports.OutboundMessage{Channel: "sms", To: "841234567", Body: "x"}); err != nil || sms.n != 0 {
		t.Fatalf("test mode reached the provider: %d %v", sms.n, err)
	}
	if _, err := r.Send(context.Background(), ports.OutboundMessage{Channel: "email", To: "a@b.co", Body: "x", Live: true}); err != nil || sms.n != 0 {
		t.Fatalf("email reached the SMS provider")
	}
	sent, err := r.Send(context.Background(), ports.OutboundMessage{Channel: "whatsapp", To: "841234567", Body: "x", Live: true})
	if err != nil || sms.n != 1 || sms.last.Channel != domain.ChannelSMS || sent.Channel != domain.ChannelSMS {
		t.Fatalf("live WhatsApp must go by SMS: %d %+v %v", sms.n, sent, err)
	}
}

type countSMS struct {
	n    int
	last ports.OutboundMessage
}

func (c *countSMS) Send(_ context.Context, m ports.OutboundMessage) (ports.SentMessage, error) {
	c.n++
	c.last = m
	return ports.SentMessage{ProviderID: "p", Channel: m.Channel}, nil
}
