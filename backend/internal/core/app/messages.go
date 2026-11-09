package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// messageRetries are the waits before each new try of a message the provider
// couldn't take: down, busy or out of credit. After the last, the message fails.
var messageRetries = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 3 * time.Hour}

// SendQueued sends the messages that are due. Only one runs at a time: if another
// is already sending, this one leaves the work to it and its next round.
func (a *App) SendQueued(ctx context.Context) {
	if !a.sending.TryLock() {
		return
	}
	defer a.sending.Unlock()
	for i := 0; i < 5; i++ {
		msgs, err := a.Messages.Queued(ctx, a.Now(), 50)
		if err != nil || len(msgs) == 0 {
			return
		}
		for _, msg := range msgs {
			if ctx.Err() != nil {
				return
			}
			a.sendOne(ctx, &msg)
		}
	}
}

// SendSoon sends what is queued without holding up the request that queued it.
func (a *App) SendSoon(ctx context.Context) {
	go a.SendQueued(context.WithoutCancel(ctx))
}

func (a *App) sendOne(ctx context.Context, msg *domain.Message) {
	sent, err := a.Messenger.Send(ctx, ports.OutboundMessage{Channel: msg.Channel, To: msg.To, Sender: msg.Sender, Body: msg.Body, Live: msg.Livemode})
	now := a.Now()
	msg.Attempts++
	switch {
	case err == nil:
		msg.Status, msg.Error, msg.NextAttemptAt, msg.SentAt = domain.MessageSent, "", nil, &now
		msg.ProviderID = sent.ProviderID
		if sent.Channel != "" {
			msg.Channel = sent.Channel
		}
	case errors.Is(err, ports.ErrRetryLater) && msg.Attempts <= len(messageRetries):
		next := now.Add(messageRetries[msg.Attempts-1])
		msg.Error, msg.NextAttemptAt = domain.Trim(err.Error(), 200), &next
	default:
		msg.Status, msg.Error, msg.NextAttemptAt = domain.MessageFailed, domain.Trim(err.Error(), 200), nil
	}
	if err := a.Messages.Save(ctx, msg); err != nil {
		log.Printf("messages: %s not saved after sending: %v", msg.ID, err)
	}
}

// MessageReport records what the SMS provider says happened to a message it took:
// delivered to the phone, or failed. A report for an unknown message, or one that
// would undo a final state, is ignored, so the same report twice changes nothing.
func (a *App) MessageReport(ctx context.Context, providerID, status, why string, at time.Time) error {
	if providerID == "" || status == "" {
		return nil
	}
	msg, err := a.Messages.ByProviderID(ctx, providerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Printf("messages: report for unknown message %.40s", providerID)
			return nil
		}
		return err
	}
	if msg.Status != domain.MessageSent && msg.Status != domain.MessageQueued {
		return nil
	}
	switch status {
	case domain.MessageDelivered:
		if at.IsZero() {
			at = a.Now()
		}
		msg.Status, msg.DeliveredAt, msg.Error = domain.MessageDelivered, &at, ""
	case domain.MessageFailed:
		msg.Status, msg.Error = domain.MessageFailed, domain.Trim(why, 200)
	default:
		return nil
	}
	return a.Messages.Save(ctx, msg)
}

// CheckMessage asks the SMS provider what became of a message, when a delivery
// report says something happened to it. The report itself decides nothing: anyone
// can post one, so only the provider's answer, asked with RIHA's key, counts.
func (a *App) CheckMessage(ctx context.Context, providerID string, reportedAt time.Time) error {
	if providerID == "" {
		return nil
	}
	msg, err := a.Messages.ByProviderID(ctx, providerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Printf("messages: report for unknown message %.40s", providerID)
			return nil
		}
		return err
	}
	if msg.Status != domain.MessageSent {
		return nil
	}
	c, ok := a.Messenger.(ports.MessageStatusChecker)
	if !ok {
		return nil
	}
	status, why, err := c.MessageStatus(ctx, providerID)
	if err != nil {
		return err
	}
	return a.MessageReport(ctx, providerID, status, why, reportedAt)
}

// smsAlerts are the SMS provider's account problems: no credit, a refused key, or
// credit running low.
func (a *App) smsAlerts() []Alert {
	r, ok := a.Messenger.(ports.HealthReporter)
	if !ok {
		return nil
	}
	h, ok := r.Health()
	if !ok {
		return nil
	}
	var out []Alert
	if h.AccountError != "" && time.Since(h.AccountErrorAt) < time.Hour {
		out = append(out, Alert{"sms", domain.TrailError, "As SMS não estão a sair: " + h.AccountError,
			"As mensagens ficam na fila e são tentadas de novo durante algumas horas. Resolva na conta " + h.Provider + "."})
	} else if h.Balance >= 0 && h.Balance < h.LowBalance {
		out = append(out, Alert{"sms_balance", domain.TrailWarn, fmt.Sprintf("Pouco crédito de SMS: %.2f MT", h.Balance),
			"Carregue a conta " + h.Provider + " antes que acabe. Sem crédito, as SMS ficam na fila."})
	}
	return out
}
