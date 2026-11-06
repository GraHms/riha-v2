package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"riha/internal/core/domain"
)

// ClientEvent is something the customer's browser saw on a checkout page.
type ClientEvent struct {
	Name string         `json:"name"`
	At   int64          `json:"at"`
	Data map[string]any `json:"data"`
}

// What a checkout page may report, and how each reads on the trail.
var clientEvents = map[string]func(map[string]any) string{
	"opened": func(d map[string]any) string {
		t := "O cliente abriu o pagamento"
		if c := str(d, "connection"); c != "" {
			t += ", ligação " + c
		}
		if ms := num(d, "ready_ms"); ms > 0 {
			t += fmt.Sprintf(", página pronta em %.1f s", ms/1000)
		}
		return t
	},
	"method": func(d map[string]any) string {
		return "Escolheu " + domain.FirstNonEmpty(domain.MethodNames[str(d, "method")], str(d, "method"))
	},
	"submit": func(d map[string]any) string { return "Carregou em pagar" },
	"hidden": func(d map[string]any) string {
		return "Saiu da página, por exemplo para responder ao pedido no telemóvel"
	},
	"visible": func(d map[string]any) string { return "Voltou à página" },
	"offline": func(d map[string]any) string { return "O telemóvel ficou sem internet" },
	"online":  func(d map[string]any) string { return "A internet voltou" },
	"check":   func(d map[string]any) string { return "Carregou em Verificar o resultado agora" },
	"waiting": func(d map[string]any) string {
		return "A página passou a mostrar a espera pela confirmação no telemóvel"
	},
	"no_answer": func(d map[string]any) string { return "A página deixou de ter resposta do servidor" },
	"result": func(d map[string]any) string {
		return "Viu o resultado: " + domain.FirstNonEmpty(str(d, "status"), "?")
	},
	"error": func(d map[string]any) string { return "Erro no navegador: " + domain.Trim(str(d, "message"), 200) },
}

func str(d map[string]any, k string) string  { v, _ := d[k].(string); return v }
func num(d map[string]any, k string) float64 { v, _ := d[k].(float64); return v }

// ClientTelemetry keeps what a checkout page reports about the customer's side:
// their connection, when they left the page and came back, errors in their browser.
// Only known events are kept, each small.
func (a *App) ClientTelemetry(ctx context.Context, sessionID string, events []ClientEvent) error {
	cs, err := a.Sessions.ByID(ctx, sessionID)
	if err != nil {
		return domain.NotFound("A sessão")
	}
	if len(events) > 30 {
		events = events[:30]
	}
	r := domain.RequestFrom(ctx)
	now := time.Now().UTC()
	for _, e := range events {
		title, ok := clientEvents[e.Name]
		if !ok {
			continue
		}
		data := map[string]any{}
		if b, err := json.Marshal(e.Data); err == nil && len(b) <= 2048 {
			_ = json.Unmarshal(b, &data)
		}
		for k, v := range data {
			if s, ok := v.(string); ok {
				data[k] = domain.MaskPhones(domain.Trim(s, 300))
			}
		}
		data["event"] = e.Name
		at := now
		// The page says how long ago it saw each event; it is trusted only a little.
		if e.At > 0 && e.At < 10*60*1000 {
			at = now.Add(-time.Duration(e.At) * time.Millisecond)
		}
		level := domain.TrailInfo
		if e.Name == "error" || e.Name == "offline" || e.Name == "no_answer" {
			level = domain.TrailWarn
		}
		a.trace(ctx, domain.TrailEvent{MerchantID: cs.MerchantID, Livemode: cs.Livemode, SessionID: cs.ID, PaymentID: strings.TrimSpace(str(data, "payment")),
			Kind: domain.TrailClient, Source: "customer", Level: level, Title: title(data), IP: r.IP, UserAgent: r.UserAgent, Data: data, CreatedAt: at})
	}
	return nil
}
