package domain

import (
	"fmt"
	"regexp"
	"time"
)

const (
	PayoutNew      = "new"
	PayoutReview   = "review"
	PayoutSent     = "sent"
	PayoutPaid     = "paid"
	PayoutRejected = "rejected"
	PayoutFailed   = "failed"
)

const (
	PayoutSLA = 72 * time.Hour
	MinPayout = 10000
)

const (
	ActReview  = "review"
	ActApprove = "approve"
	ActPaid    = "paid"
	ActReject  = "reject"
	ActFail    = "fail"
)

var OpenPayoutStatuses = []string{PayoutNew, PayoutReview, PayoutSent}
var PendingPayoutStatuses = []string{PayoutNew, PayoutReview}

type Payout struct {
	ID            string
	Number        int64
	MerchantID    string
	Livemode      bool
	Amount        int64
	Status        string
	BankName      string
	Holder        string
	NIB           string
	RequestedBy   string
	RequestedAt   time.Time
	DueAt         time.Time
	ReviewedAt    *time.Time
	SentAt        *time.Time
	PaidAt        *time.Time
	ClosedAt      *time.Time
	FailureReason string
	ReviewedBy    string
	UpdatedAt     time.Time
}

func (p *Payout) Ref() string { return fmt.Sprintf("LV-%04d", p.Number) }

func (p *Payout) Pending() bool { return p.Status == PayoutNew || p.Status == PayoutReview }

type Transition struct {
	Reversal bool
	Event    string
}

func (p *Payout) Apply(action, reason, actor string, now time.Time) (Transition, error) {
	bad := Invalid("payout_state", "status", fmt.Sprintf("Não é possível %s um levantamento %s.", action, p.Status))
	var t Transition
	switch action {
	case ActReview:
		if p.Status != PayoutNew {
			return t, bad
		}
		p.Status, p.ReviewedAt = PayoutReview, &now
	case ActApprove:
		if !p.Pending() {
			return t, bad
		}
		p.Status, p.SentAt = PayoutSent, &now
		if p.ReviewedAt == nil {
			p.ReviewedAt = &now
		}
	case ActPaid:
		if p.Status != PayoutSent {
			return t, bad
		}
		p.Status, p.PaidAt, p.ClosedAt = PayoutPaid, &now, &now
		t.Event = "payout.paid"
	case ActReject:
		if !p.Pending() {
			return t, bad
		}
		p.Status, p.ClosedAt, p.FailureReason = PayoutRejected, &now, reason
		t = Transition{Reversal: true, Event: "payout.failed"}
	case ActFail:
		if p.Status != PayoutSent {
			return t, bad
		}
		p.Status, p.ClosedAt, p.FailureReason = PayoutFailed, &now, reason
		if reason == "" {
			p.FailureReason = "Transferência devolvida pelo banco (NIB inválido)."
		}
		t = Transition{Reversal: true, Event: "payout.failed"}
	default:
		return t, Invalid("action_invalid", "action", "Ação desconhecida.")
	}
	p.ReviewedBy = actor
	return t, nil
}

var nibRe = regexp.MustCompile(`^[0-9]{21}$`)

func ValidNIB(n string) bool { return nibRe.MatchString(n) }
