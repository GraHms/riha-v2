package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"riha/internal/ids"
	"riha/internal/models"
)

// AllEventTypes lists every event a webhook endpoint may subscribe to.
var AllEventTypes = []string{
	"checkout.session.completed", "checkout.session.expired",
	"payment.processing", "payment.succeeded", "payment.failed",
	"refund.succeeded", "payout.created", "payout.paid", "payout.failed", "account.updated",
}

// callbackEvents are the only events sent to a session's callback_url.
var callbackEvents = map[string]bool{
	"checkout.session.completed": true, "checkout.session.expired": true,
	"payment.succeeded": true, "payment.failed": true,
}

// RetrySchedule: first attempt immediately, then 7 retries over 3 days (8 in total).
var RetrySchedule = []time.Duration{
	5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 5 * time.Hour, 10 * time.Hour, 24 * time.Hour, 72 * time.Hour,
}

// emit stores an event and fans it out (in the same transaction, as an
// outbox) to the account endpoints and, for session events, the callback_url.
func (s *Service) emit(tx *gorm.DB, merchantID string, live bool, typ, objectID string, obj any, callbackURL string) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	ev := &models.Event{ID: ids.New("evt"), MerchantID: merchantID, Livemode: live, Type: typ, ObjectID: objectID, Data: string(data), CreatedAt: s.Now()}
	if err := tx.Create(ev).Error; err != nil {
		return err
	}
	now := s.Now()
	var eps []models.WebhookEndpoint
	tx.Where("merchant_id = ? AND livemode = ? AND active = ?", merchantID, live, true).Find(&eps)
	for _, ep := range eps {
		if !ep.Events.Has(typ) && !ep.Events.Has("*") {
			continue
		}
		epID := ep.ID
		if err := tx.Create(&models.WebhookDelivery{ID: ids.New("whd"), EventID: ev.ID, MerchantID: merchantID, Livemode: live, EventType: typ,
			Target: models.TargetEndpoint, EndpointID: &epID, URL: ep.URL, Status: models.DeliveryPending, NextAttemptAt: &now}).Error; err != nil {
			return err
		}
	}
	if callbackURL != "" && callbackEvents[typ] {
		if err := tx.Create(&models.WebhookDelivery{ID: ids.New("whd"), EventID: ev.ID, MerchantID: merchantID, Livemode: live, EventType: typ,
			Target: models.TargetCallback, URL: callbackURL, Status: models.DeliveryPending, NextAttemptAt: &now}).Error; err != nil {
			return err
		}
	}
	return nil
}

// EmitTest sends a test event to every endpoint of the mode.
func (s *Service) EmitTest(merchantID string, live bool) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		return s.emit(tx, merchantID, live, "payment.succeeded", "pay_test_event", map[string]any{
			"id": "pay_test_event", "object": "payment", "status": "succeeded", "amount": 125000, "livemode": live, "test_event": true,
		}, "")
	})
}

// EventPayload builds the JSON body sent for a delivery attempt.
func EventPayload(ev *models.Event, target string, attempt int) []byte {
	body := map[string]any{
		"id": ev.ID, "object": "event", "type": ev.Type, "created": ev.CreatedAt.Unix(), "livemode": ev.Livemode,
		"delivery": map[string]any{"target": target, "attempt": attempt},
		"data":     map[string]any{"object": json.RawMessage(ev.Data)},
	}
	b, _ := json.Marshal(body)
	return b
}

// Sign returns the Riha-Signature header value: t=<unix>,v1=<hmac>.
func Sign(secret string, t int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(t, 10) + "."))
	mac.Write(body)
	return fmt.Sprintf("t=%d,v1=%s", t, hex.EncodeToString(mac.Sum(nil)))
}

// DeliverDue sends up to n due webhook deliveries.
func (s *Service) DeliverDue(ctx context.Context, client *http.Client, n int) int {
	var batch []models.WebhookDelivery
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND next_attempt_at <= ?", models.DeliveryPending, s.Now()).
			Order("next_attempt_at").Limit(n).Find(&batch).Error; err != nil {
			return err
		}
		// Lease: push next_attempt_at forward so concurrent workers skip them.
		lease := s.Now().Add(2 * time.Minute)
		for i := range batch {
			tx.Model(&batch[i]).Update("next_attempt_at", lease)
		}
		return nil
	})
	if err != nil {
		log.Printf("webhooks: %v", err)
		return 0
	}
	for i := range batch {
		s.deliver(ctx, client, &batch[i])
	}
	return len(batch)
}

func (s *Service) deliver(ctx context.Context, client *http.Client, d *models.WebhookDelivery) {
	var ev models.Event
	if err := s.DB.First(&ev, "id = ?", d.EventID).Error; err != nil {
		return
	}
	var m models.Merchant
	if err := s.DB.First(&m, "id = ?", d.MerchantID).Error; err != nil {
		return
	}
	attempt := d.Attempts + 1
	body := EventPayload(&ev, d.Target, attempt)
	ts := s.Now().Unix()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.URL, bytes.NewReader(body))
	code, errMsg := 0, ""
	start := time.Now()
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Riha-Webhooks/1.0")
		req.Header.Set("Riha-Signature", Sign(m.SigningSecret(d.Livemode), ts, body))
		req.Header.Set("Riha-Event-Id", ev.ID)
		var resp *http.Response
		resp, err = client.Do(req)
		if err == nil {
			code = resp.StatusCode
			io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			resp.Body.Close()
		}
	}
	if err != nil {
		errMsg = err.Error()
		if len(errMsg) > 500 {
			errMsg = errMsg[:500]
		}
	}
	upd := map[string]any{"attempts": attempt, "last_status_code": code, "last_error": errMsg, "last_duration_ms": time.Since(start).Milliseconds()}
	switch {
	case code >= 200 && code < 300:
		upd["status"], upd["next_attempt_at"] = models.DeliverySucceeded, nil
	case attempt > len(RetrySchedule):
		upd["status"], upd["next_attempt_at"] = models.DeliveryFailed, nil
	default:
		next := s.Now().Add(RetrySchedule[attempt-1])
		upd["next_attempt_at"] = next
	}
	s.DB.Model(d).Updates(upd)
}

// Resend queues a delivery again immediately.
func (s *Service) Resend(merchantID, deliveryID string) error {
	now := s.Now()
	res := s.DB.Model(&models.WebhookDelivery{}).Where("id = ? AND merchant_id = ?", deliveryID, merchantID).
		Updates(map[string]any{"status": models.DeliveryPending, "next_attempt_at": now})
	if res.RowsAffected == 0 {
		return NotFound("A entrega")
	}
	return nil
}

// Sweep performs periodic housekeeping: expire sessions, time out stuck
// wallet prompts, and auto-settle test-mode payouts.
func (s *Service) Sweep() {
	now := s.Now()
	var stale []models.Payment
	s.DB.Where("status = ? AND created_at < ?", models.PaymentProcessing, now.Add(-3*time.Minute)).Limit(100).Find(&stale)
	for _, p := range stale {
		s.CompletePayment(p.ID, timeoutOutcome)
	}
	var exp []models.CheckoutSession
	s.DB.Where("status = ? AND expires_at < ?", models.SessionOpen, now).Limit(200).Find(&exp)
	for _, cs := range exp {
		s.ExpireSession("", false, cs.ID)
	}
	// Test mode payouts settle by themselves so integrators can see the whole lifecycle.
	var tp []models.Payout
	s.DB.Where("livemode = ? AND status IN ? AND updated_at < ?", false, []string{models.PayoutNew, models.PayoutSent}, now.Add(-20*time.Second)).Limit(50).Find(&tp)
	for _, p := range tp {
		if p.Status == models.PayoutNew {
			s.TransitionPayout(p.ID, ActApprove, "", "system")
		} else {
			s.TransitionPayout(p.ID, ActPaid, "", "system")
		}
	}
	// Idempotency keys live 24h.
	s.DB.Where("created_at < ?", now.Add(-24*time.Hour)).Delete(&models.IdempotencyRecord{})
}

// RunWorkers starts background loops until ctx is cancelled.
func (s *Service) RunWorkers(ctx context.Context) {
	client := &http.Client{Timeout: 10 * time.Second}
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				for s.DeliverDue(ctx, client, 20) == 20 {
				}
			}
		}
	}()
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.Sweep()
			}
		}
	}()
}
