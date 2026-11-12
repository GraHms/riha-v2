package domain

import (
	"fmt"
	"regexp"
	"strings"
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
	// RIHA's money is at Millennium bim. A transfer to another bank costs RIHA
	// DefaultTransferFee; from DefaultFreeTransferFrom RIHA pays it itself.
	DefaultTransferFee      = 12_000
	DefaultFreeTransferFrom = 1_000_000
	// BimBankCode is how a NIB at Millennium bim starts.
	BimBankCode = "0001"
)

// IsBimAccount says whether a bank account is at Millennium bim, where a transfer
// from RIHA's own account costs nothing. The NIB's first four digits name the bank.
func IsBimAccount(bankName, nib string) bool {
	if strings.HasPrefix(nib, BimBankCode) {
		return true
	}
	n := strings.ToLower(bankName)
	return strings.Contains(n, "millennium") || n == "bim" || strings.HasPrefix(n, "bim ")
}

// TransferPricing is what a withdrawal to another bank costs the merchant: Fee,
// unless the amount is at least FreeFrom, or the account is at bim.
type TransferPricing struct {
	Fee      int64
	FreeFrom int64
}

func (t TransferPricing) FeeFor(bankName, nib string, amount int64) int64 {
	if IsBimAccount(bankName, nib) || amount >= t.FreeFrom {
		return 0
	}
	return t.Fee
}

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
	ID         string
	Number     int64
	MerchantID string
	Livemode   bool
	// Amount leaves the merchant's balance; Fee is the transfer fee within it, so
	// the bank receives Net().
	Amount        int64
	Fee           int64
	Status        string
	BankName      string
	Holder        string
	NIB           string
	// BankChangedAt is when the merchant last replaced the account it is
	// paid to, for staff to look twice at a recent change.
	BankChangedAt *time.Time
	RequestedBy   string
	RequestedAt   time.Time
	DueAt         time.Time
	ReviewedAt    *time.Time
	SentAt        *time.Time
	PaidAt        *time.Time
	ClosedAt      *time.Time
	FailureReason string
	ReviewedBy    string
	ReceiptPath   string
	ReceiptName   string
	ReceiptType   string
	ReceiptSize   int64
	ReceiptHash   string
	EmailedAt     *time.Time
	UpdatedAt     time.Time
}

func (p *Payout) HasReceipt() bool { return p.ReceiptPath != "" }

func (p *Payout) Ref() string { return fmt.Sprintf("LV-%04d", p.Number) }

// Net is what reaches the merchant's bank: the amount less the transfer fee.
func (p *Payout) Net() int64 { return p.Amount - p.Fee }

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
		if p.Livemode && !p.HasReceipt() {
			return t, Invalid("receipt_required", "receipt", "Carregue o comprovativo bancário antes de marcar como pago.")
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
