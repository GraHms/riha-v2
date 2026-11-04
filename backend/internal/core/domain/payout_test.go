package domain

import (
	"testing"
	"time"
)

func TestPayoutTransitions(t *testing.T) {
	now := time.Now()
	p := &Payout{Status: PayoutNew}
	if _, err := p.Apply(ActPaid, "", "a", now); err == nil {
		t.Fatal("new payout cannot be paid")
	}
	if tr, err := p.Apply(ActApprove, "", "a", now); err != nil || tr.Reversal || p.Status != PayoutSent {
		t.Fatalf("approve: %v %+v", err, p)
	}
	if _, err := p.Apply(ActReject, "x", "a", now); err == nil {
		t.Fatal("sent payout cannot be rejected")
	}
	tr, err := p.Apply(ActFail, "", "a", now)
	if err != nil || !tr.Reversal || p.Status != PayoutFailed || p.FailureReason == "" {
		t.Fatalf("fail: %v %+v", err, p)
	}
	q := &Payout{Status: PayoutReview}
	if tr, err := q.Apply(ActReject, "titular", "a", now); err != nil || !tr.Reversal || tr.Event != "payout.failed" {
		t.Fatalf("reject: %v %+v", err, tr)
	}
}
