package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

var AllEventTypes = []string{
	"checkout.session.completed", "checkout.session.expired",
	"payment.processing", "payment.succeeded", "payment.failed",
	"refund.succeeded", "payout.created", "payout.paid", "payout.failed", "account.updated",
	"plan.created", "plan.updated", "subscription.created", "subscription.updated", "subscription.canceled",
	"subscription_charge.paid", "subscription_charge.paid_outside", "subscription_charge.past_due",
}

var CallbackEvents = map[string]bool{
	"checkout.session.completed": true, "checkout.session.expired": true,
	"payment.succeeded": true, "payment.failed": true,
}

var RetrySchedule = []time.Duration{
	5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 5 * time.Hour, 10 * time.Hour, 24 * time.Hour, 72 * time.Hour,
}

func ValidEvent(e string) bool { return e == "*" || contains(AllEventTypes, e) }

func EventPayload(ev *Event, target string, attempt int) []byte {
	b, _ := json.Marshal(map[string]any{
		"id": ev.ID, "object": "event", "type": ev.Type, "created": ev.CreatedAt.Unix(), "livemode": ev.Livemode,
		"delivery": map[string]any{"target": target, "attempt": attempt},
		"data":     map[string]any{"object": json.RawMessage(ev.Data)},
	})
	return b
}

func Sign(secret string, t int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(t, 10) + "."))
	mac.Write(body)
	return fmt.Sprintf("t=%d,v1=%s", t, hex.EncodeToString(mac.Sum(nil)))
}
