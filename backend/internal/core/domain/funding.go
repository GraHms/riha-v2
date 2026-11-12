package domain

import (
	"sort"
	"time"
)

// FundRIHA is a withdrawal paid from RIHA's own account, for money no partner holds.
const FundRIHA = "riha"

const (
	FundingOpen      = "open"
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
	PaidAt      *time.Time
	PaidBy      string
	CreatedAt   time.Time
}

func (f *PayoutFunding) HasReceipt() bool { return f.ReceiptPath != "" }

// PlanFunding splits a withdrawal of amount, with its transfer fee, between the
// sources that hold the merchant's money (by integration code). One source pays
// it all when one can, the one holding most, so the merchant gets one transfer;
// otherwise the fullest pay first. What no source holds comes from RIHA. The
// fee rides on the first part, which sends that much less.
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
	// The fee comes off the largest part, which can always bear it: the whole
	// withdrawal is at least the minimum plus the fee.
	big := 0
	for i := range out {
		if out[i].Amount > out[big].Amount {
			big = i
		}
	}
	out[big].Send -= fee
	return out
}
