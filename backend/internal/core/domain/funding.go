package domain

import (
	"sort"
	"time"
)

// FundRIHA is a withdrawal paid from RIHA's own account, for money no partner holds.
const FundRIHA = "riha"

// A part is open until its payer pays it. A partner saying it paid makes it
// claimed, and RIHA confirming the transfer on the statement makes it paid:
// a partner's word alone never closes a merchant's withdrawal.
const (
	FundingOpen      = "open"
	FundingClaimed   = "claimed"
	FundingPaid      = "paid"
	FundingCancelled = "cancelled"
)

// PayoutFunding is the part of a live withdrawal one source pays: a partner's
// integration, from the account the merchant's money landed in, or RIHA. Amount
// comes off what that source holds for the merchant; Send is what it transfers
// to the merchant's bank, less the transfer fee if this part carries it.
type PayoutFunding struct {
	ID          string
	PayoutID    string
	MerchantID  string
	Source      string
	Amount      int64
	Send        int64
	Status      string
	Reference   string
	ReceiptPath string
	ReceiptName string
	ReceiptType string
	ReceiptSize int64
	ReceiptHash string
	PaidAt      *time.Time // when its payer said it paid
	PaidBy      string
	ConfirmedAt *time.Time // when RIHA saw the transfer
	ConfirmedBy string
	// Planned is the partner first asked to pay this part, when RIHA paid
	// it instead: the money stays with that partner, owed to RIHA.
	Planned   string
	Warning   string // what staff should check before confirming
	CreatedAt time.Time
}

func (f *PayoutFunding) HasReceipt() bool { return f.ReceiptPath != "" }

// Sent is a part whose money its payer says left: claimed or paid.
func (f *PayoutFunding) Sent() bool { return f.Status == FundingClaimed || f.Status == FundingPaid }

// PlanFunding splits a withdrawal of amount, with its transfer fee, between the
// sources that hold the merchant's money (by integration code). One source pays
// it all when one can, the one holding most, so the merchant gets one transfer;
// otherwise the fullest pay first. What no source holds comes from RIHA. The
// transfer fee comes off one part, which sends that much less.
func PlanFunding(amount, fee int64, held map[string]int64) []PayoutFunding {
	type src struct {
		code string
		held int64
	}
	srcs := []src{}
	for code, h := range held {
		if h > 0 && code != FundRIHA {
			srcs = append(srcs, src{code, h})
		}
	}
	sort.Slice(srcs, func(i, j int) bool {
		if srcs[i].held != srcs[j].held {
			return srcs[i].held > srcs[j].held
		}
		return srcs[i].code < srcs[j].code
	})
	out := []PayoutFunding{}
	if len(srcs) > 0 && srcs[0].held >= amount {
		srcs = srcs[:1]
	}
	left := amount
	for _, s := range srcs {
		if left == 0 {
			break
		}
		part := min(s.held, left)
		out = append(out, PayoutFunding{Source: s.code, Amount: part, Send: part, Status: FundingOpen})
		left -= part
	}
	if left > 0 {
		out = append(out, PayoutFunding{Source: FundRIHA, Amount: left, Send: left, Status: FundingOpen})
	}
	// The fee comes off one part that can bear it, RIHA's first, then the
	// largest. When none can, RIHA pays the whole withdrawal: the merchant
	// gets one transfer and no part is ever told to send less than nothing.
	carry := -1
	for i := range out {
		if out[i].Amount <= fee {
			continue
		}
		if carry < 0 || out[i].Source == FundRIHA || (out[carry].Source != FundRIHA && out[i].Amount > out[carry].Amount) {
			carry = i
		}
	}
	if carry < 0 {
		return []PayoutFunding{{Source: FundRIHA, Amount: amount, Send: amount - fee, Status: FundingOpen}}
	}
	out[carry].Send -= fee
	return out
}
