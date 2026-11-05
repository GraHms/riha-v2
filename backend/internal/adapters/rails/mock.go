package rails

import (
	"context"
	"fmt"
	"time"

	"riha/internal/core/domain"
)

type Mock struct {
	Delay time.Duration
	After func(d time.Duration, f func())
}

func NewMock(delay time.Duration) *Mock {
	return &Mock{Delay: delay, After: func(d time.Duration, f func()) { time.AfterFunc(d, f) }}
}

func walletOutcome(phone string) (domain.Outcome, time.Duration) {
	switch phone[len(phone)-4:] {
	case "0000":
		return domain.Outcome{FailureCode: domain.FailPinIncorrect}, 1
	case "1111":
		return domain.Outcome{FailureCode: domain.FailInsufficientFunds}, 1
	case "2222":
		return domain.Outcome{FailureCode: domain.FailCustomerTimeout}, 3
	case "3333":
		return domain.Outcome{FailureCode: domain.FailCustomerCancelled}, 1
	case "4444":
		return domain.Outcome{FailureCode: domain.FailOperatorUnavailable}, 0
	}
	return domain.Outcome{Success: true, OperatorReference: domain.ShortCode(10)}, 1
}

func (m *Mock) Initiate(_ context.Context, req domain.ChargeRequest, onComplete func(domain.Outcome)) (domain.ChargeResult, error) {
	switch req.Method {
	case domain.MethodMpesa, domain.MethodEmola, domain.MethodBim:
		if e := domain.WalletNumberError(req.Method, req.Phone); e != nil {
			return domain.ChargeResult{}, e
		}
		phone := domain.NormalizeMZPhone(req.Phone)
		out, mult := walletOutcome(phone)
		if mult == 0 {
			return domain.ChargeResult{Outcome: out, Last4: phone[5:]}, nil
		}
		m.After(m.Delay*mult, func() { onComplete(out) })
		return domain.ChargeResult{Async: true, Last4: phone[5:]}, nil
	case domain.MethodCard:
		num := domain.Digits(req.CardNumber)
		if len(num) < 13 || len(num) > 19 || !luhn(num) {
			return domain.ChargeResult{}, &domain.InputError{Field: "card_number", Msg: "Número de cartão inválido."}
		}
		if !validExpiry(req.CardExpiry) {
			return domain.ChargeResult{}, &domain.InputError{Field: "card_expiry", Msg: "Validade inválida. Use MM / AA."}
		}
		if c := domain.Digits(req.CardCVC); len(c) < 3 || len(c) > 4 {
			return domain.ChargeResult{}, &domain.InputError{Field: "card_cvc", Msg: "CVV inválido."}
		}
		brand := "Visa"
		if num[0] == '5' || num[0] == '2' {
			brand = "Mastercard"
		}
		res := domain.ChargeResult{Brand: brand, Last4: num[len(num)-4:]}
		switch num {
		case "4000000000000002":
			res.Outcome = domain.Outcome{FailureCode: domain.FailCardDeclined}
		case "4000000000009995":
			res.Outcome = domain.Outcome{FailureCode: domain.FailInsufficientFunds}
		default:
			res.Outcome = domain.Outcome{Success: true, OperatorReference: "AUTH" + domain.ShortCode(6)}
		}
		return res, nil
	}
	return domain.ChargeResult{}, &domain.InputError{Field: "method", Msg: fmt.Sprintf("Método %q não suportado.", req.Method)}
}

func luhn(s string) bool {
	sum, alt := 0, false
	for i := len(s) - 1; i >= 0; i-- {
		d := int(s[i] - '0')
		if alt {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return sum%10 == 0
}

func validExpiry(e string) bool {
	d := domain.Digits(e)
	if len(d) != 4 {
		return false
	}
	mm := int(d[0]-'0')*10 + int(d[1]-'0')
	yy := 2000 + int(d[2]-'0')*10 + int(d[3]-'0')
	now := time.Now()
	return mm >= 1 && mm <= 12 && (yy > now.Year() || (yy == now.Year() && mm >= int(now.Month())))
}
