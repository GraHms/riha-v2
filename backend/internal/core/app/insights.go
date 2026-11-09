package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// Thread is a checkout, its payments and everything that happened to them.
type Thread struct {
	Session  *domain.CheckoutSession
	Merchant *domain.Merchant
	Payments []domain.Payment
	Events   []domain.TrailEvent
}

// Thread finds what happened to a checkout or a payment, by either id.
func (a *App) Thread(ctx context.Context, id string) (*Thread, error) {
	if a.Trail == nil {
		return nil, domain.NotFound("O registo")
	}
	sessionID := id
	if strings.HasPrefix(id, "pay_") {
		p, err := a.Payments.ByID(ctx, id)
		if err != nil {
			return nil, domain.NotFound("O pagamento")
		}
		sessionID = p.SessionID
	}
	cs, err := a.Sessions.ByID(ctx, sessionID)
	if err != nil {
		return nil, domain.NotFound("O pagamento")
	}
	m, _ := a.Merchants.ByID(ctx, cs.MerchantID)
	ps, _ := a.Payments.Find(ctx, ports.PaymentQuery{SessionID: cs.ID, Limit: 50})
	sort.Slice(ps, func(i, j int) bool { return ps[i].CreatedAt.Before(ps[j].CreatedAt) })
	ids := make([]string, len(ps))
	for i := range ps {
		ids[i] = ps[i].ID
	}
	evs, err := a.Trail.Thread(ctx, cs.ID, ids)
	if err != nil {
		return nil, err
	}
	return &Thread{Session: cs, Merchant: m, Payments: ps, Events: evs}, nil
}

func (a *App) FindThreads(ctx context.Context, q string) ([]ports.ThreadHit, error) {
	if a.Insights == nil {
		return nil, nil
	}
	return a.Insights.FindThreads(ctx, q, 50)
}

func (a *App) TrailEvents(ctx context.Context, q ports.TrailQuery) ([]domain.TrailEvent, error) {
	if a.Trail == nil {
		return nil, nil
	}
	return a.Trail.Recent(ctx, q)
}

type Alert struct {
	Key    string `json:"key"`
	Level  string `json:"level"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type MetricsView struct {
	ports.Metrics
	TrailWritten int64      `json:"trail_written"`
	TrailDropped int64      `json:"trail_dropped"`
	LastBackup   *time.Time `json:"last_backup"`
	Alerts       []Alert    `json:"alerts"`
}

func (a *App) AdminMetrics(ctx context.Context, q ports.MetricsQuery) (MetricsView, error) {
	var v MetricsView
	if a.Insights == nil {
		return v, nil
	}
	m, err := a.Insights.Metrics(ctx, q)
	if err != nil {
		return v, err
	}
	v.Metrics = m
	if a.Trail != nil {
		v.TrailWritten, v.TrailDropped = a.Trail.Written(), a.Trail.Dropped()
	}
	v.LastBackup = a.lastBackup()
	v.Alerts = a.alerts(ctx)
	return v, nil
}

func (a *App) lastBackup() *time.Time {
	if a.BackupStatusFile == "" {
		return nil
	}
	b, err := os.ReadFile(a.BackupStatusFile)
	if err != nil {
		return nil
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(b)))
	if err != nil {
		return nil
	}
	return &t
}

// alerts says what is wrong now: in the last 15 minutes, or still.
func (a *App) alerts(ctx context.Context) []Alert {
	var out []Alert
	if a.Insights == nil {
		return out
	}
	h, err := a.Insights.Health(ctx, time.Now().Add(-15*time.Minute))
	if err != nil {
		return []Alert{{Key: "health", Level: domain.TrailError, Title: "Não foi possível ler o estado do sistema", Detail: err.Error()}}
	}
	add := func(ok bool, key, level, title, detail string) {
		if ok {
			out = append(out, Alert{key, level, title, detail})
		}
	}
	add(h.StuckPayments > 0, "stuck", domain.TrailError, fmt.Sprintf("%d pagamentos à espera há mais de 15 minutos", h.StuckPayments),
		"O operador não deu resposta final. Veja o registo de cada um.")
	add(h.OperatorErrors >= 3, "operator", domain.TrailError, fmt.Sprintf("O mtoca ou o bim falhou %d vezes em 15 minutos", h.OperatorErrors),
		"Pedidos sem resposta ou recusados pelo mtoca. Os clientes podem não estar a receber o pedido no telemóvel.")
	add(h.APIServerErrors >= 3, "api", domain.TrailError, fmt.Sprintf("%d erros 5xx na API em 15 minutos", h.APIServerErrors), "As integrações dos comerciantes estão a receber erros do servidor.")
	add(h.ErrorLogs > 0, "errors", domain.TrailError, fmt.Sprintf("%d erros graves registados em 15 minutos", h.ErrorLogs), "Por exemplo um pagamento pago depois de fechado. Veja os eventos com nível erro.")
	add(h.FailedWebhooks > 0, "webhooks", domain.TrailWarn, fmt.Sprintf("%d webhooks deixaram de ser tentados em 15 minutos", h.FailedWebhooks), "O servidor do comerciante não aceitou depois de todas as tentativas.")
	if a.Trail != nil && a.Trail.Dropped() > 0 {
		add(true, "trail", domain.TrailWarn, fmt.Sprintf("%d eventos do registo perdidos desde o arranque", a.Trail.Dropped()), "A fila do registo encheu ou a base de dados recusou a escrita.")
	}
	if a.BackupStatusFile != "" {
		if t := a.lastBackup(); t == nil {
			add(true, "backup", domain.TrailError, "Sem registo de cópia de segurança", "O backup noturno ainda não correu, ou falhou sempre.")
		} else if age := time.Since(*t); age > 26*time.Hour {
			add(true, "backup", domain.TrailError, fmt.Sprintf("A última cópia de segurança tem %d horas", int(age.Hours())), "O backup noturno falhou. Veja journalctl -u riha-backup.")
		}
	}
	out = append(out, a.smsAlerts()...)
	return out
}

var watch struct {
	sync.Mutex
	sent map[string]time.Time
}

// Watchdog tells the admin, by email and on the trail, about a new problem, and
// about the same problem again after two hours if it is still there.
func (a *App) Watchdog(ctx context.Context) {
	watch.Lock()
	defer watch.Unlock()
	if watch.sent == nil {
		watch.sent = map[string]time.Time{}
	}
	for _, al := range a.alerts(ctx) {
		if t, ok := watch.sent[al.Key]; ok && time.Since(t) < 2*time.Hour {
			continue
		}
		watch.sent[al.Key] = time.Now()
		a.trace(ctx, domain.TrailEvent{Kind: "alert", Source: "riha", Level: al.Level, Title: al.Title, Data: map[string]any{"key": al.Key, "detail": al.Detail}})
		if err := a.adminAlert(ctx, "admin.alert", domain.Notice{Subject: "RIHA: " + al.Title, Title: al.Title,
			Lines: []string{al.Detail}, Action: "Ver as métricas", URL: a.link("/admin/metricas")}); err != nil {
			log.Printf("watchdog: %v", err)
		}
	}
}

// PruneTrail empties the bodies of trail events after 90 days and deletes the
// events after 400.
func (a *App) PruneTrail(ctx context.Context) {
	if a.Trail == nil {
		return
	}
	now := time.Now()
	if b, e, err := a.Trail.Prune(ctx, now.AddDate(0, 0, -90), now.AddDate(0, 0, -400)); err != nil {
		log.Printf("trail prune: %v", err)
	} else if b+e > 0 {
		log.Printf("trail prune: %d bodies emptied, %d events deleted", b, e)
	}
}

// ActiveAlerts is what is wrong now, for the admin's menu.
func (a *App) ActiveAlerts(ctx context.Context) []Alert { return a.alerts(ctx) }
