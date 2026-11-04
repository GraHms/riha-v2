// Package providers abstracts the payment rails (M-Pesa, e-Mola, cards).
//
// Only mock implementations exist for now. The interface is shaped like the
// real integrations: a wallet push (USSD/STK prompt) is asynchronous and the
// operator calls us back; a card authorisation is synchronous.
package providers

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"riha/internal/ids"
)

const (
	MethodMpesa = "mpesa"
	MethodEmola = "emola"
	MethodCard  = "card"
)

var AllMethods = []string{MethodMpesa, MethodEmola, MethodCard}

// Failure codes, as documented in the public API.
const (
	FailPinIncorrect        = "pin_incorrect"
	FailInsufficientFunds   = "insufficient_funds"
	FailCustomerTimeout     = "customer_timeout"
	FailCustomerCancelled   = "customer_cancelled"
	FailCardDeclined        = "card_declined"
	FailOperatorUnavailable = "operator_unavailable"
)

var FailureMessages = map[string]string{
	FailPinIncorrect:        "PIN errado três vezes. O cliente pode tentar de novo com o mesmo link.",
	FailInsufficientFunds:   "Saldo insuficiente na carteira ou cartão do cliente.",
	FailCustomerTimeout:     "O cliente não confirmou o pedido a tempo.",
	FailCustomerCancelled:   "O cliente cancelou o pedido no telemóvel.",
	FailCardDeclined:        "Cartão recusado pelo banco emissor.",
	FailOperatorUnavailable: "O operador não respondeu. Tente de novo dentro de minutos.",
}

// Outcome is the final result reported by a provider.
type Outcome struct {
	Success           bool
	FailureCode       string
	OperatorReference string
}

// ChargeRequest describes one attempt.
type ChargeRequest struct {
	PaymentID  string
	Method     string
	Amount     int64
	Phone      string // local 9 digits for wallets (84xxxxxxx)
	CardNumber string
	CardExpiry string
	CardCVC    string
	Descriptor string
}

// Result of initiating a charge.
type Result struct {
	// Async means the customer must confirm on their phone; the outcome will
	// arrive through the callback passed to Initiate.
	Async   bool
	Outcome Outcome // set when !Async
	Brand   string
	Last4   string
}

// Provider initiates charges.
type Provider interface {
	Initiate(req ChargeRequest, onComplete func(Outcome)) (Result, error)
}

// ValidationError is returned for bad customer input (shown on checkout).
type ValidationError struct{ Field, Msg string }

func (e *ValidationError) Error() string { return e.Msg }

var digits = regexp.MustCompile(`\D`)

// NormalizeMZPhone accepts "+258 84 123 4567", "841234567", "258841234567".
func NormalizeMZPhone(p string) string {
	d := digits.ReplaceAllString(p, "")
	d = strings.TrimPrefix(d, "258")
	return d
}

// ---------------------------------------------------------------- mock

// Mock simulates every rail. Test numbers (last 4 digits of the phone):
//
//	0000 → pin_incorrect, 1111 → insufficient_funds, 2222 → customer_timeout,
//	3333 → customer_cancelled, 4444 → operator_unavailable, anything else succeeds.
//
// Test cards: 4000 0000 0000 0002 → card_declined,
// 4000 0000 0000 9995 → insufficient_funds, any other Luhn-valid number succeeds.
type Mock struct {
	Delay time.Duration
	// After schedules f; replaced in tests.
	After func(d time.Duration, f func())
}

func NewMock(delay time.Duration) *Mock {
	return &Mock{Delay: delay, After: func(d time.Duration, f func()) { time.AfterFunc(d, f) }}
}

func walletOutcome(phone string) (Outcome, time.Duration) {
	last4 := phone[len(phone)-4:]
	switch last4 {
	case "0000":
		return Outcome{FailureCode: FailPinIncorrect}, 1
	case "1111":
		return Outcome{FailureCode: FailInsufficientFunds}, 1
	case "2222":
		return Outcome{FailureCode: FailCustomerTimeout}, 3
	case "3333":
		return Outcome{FailureCode: FailCustomerCancelled}, 1
	case "4444":
		return Outcome{FailureCode: FailOperatorUnavailable}, 0
	}
	return Outcome{Success: true, OperatorReference: ids.Short(10)}, 1
}

func (m *Mock) Initiate(req ChargeRequest, onComplete func(Outcome)) (Result, error) {
	switch req.Method {
	case MethodMpesa, MethodEmola:
		phone := NormalizeMZPhone(req.Phone)
		if len(phone) != 9 {
			return Result{}, &ValidationError{"phone", "Número inválido. Use 9 dígitos, por exemplo 84 123 4567."}
		}
		pre := phone[:2]
		if req.Method == MethodMpesa && pre != "84" && pre != "85" {
			return Result{}, &ValidationError{"phone", "Números M-Pesa começam por 84 ou 85."}
		}
		if req.Method == MethodEmola && pre != "86" && pre != "87" {
			return Result{}, &ValidationError{"phone", "Números e-Mola começam por 86 ou 87."}
		}
		out, mult := walletOutcome(phone)
		if mult == 0 { // operator down: fail immediately
			return Result{Outcome: out, Last4: phone[5:]}, nil
		}
		m.After(m.Delay*mult, func() { onComplete(out) })
		return Result{Async: true, Last4: phone[5:]}, nil
	case MethodCard:
		num := digits.ReplaceAllString(req.CardNumber, "")
		if len(num) < 13 || len(num) > 19 || !luhn(num) {
			return Result{}, &ValidationError{"card_number", "Número de cartão inválido."}
		}
		if !validExpiry(req.CardExpiry) {
			return Result{}, &ValidationError{"card_expiry", "Validade inválida. Use MM / AA."}
		}
		if c := digits.ReplaceAllString(req.CardCVC, ""); len(c) < 3 || len(c) > 4 {
			return Result{}, &ValidationError{"card_cvc", "CVV inválido."}
		}
		brand := "Visa"
		if num[0] == '5' || num[0] == '2' {
			brand = "Mastercard"
		}
		res := Result{Brand: brand, Last4: num[len(num)-4:]}
		switch num {
		case "4000000000000002":
			res.Outcome = Outcome{FailureCode: FailCardDeclined}
		case "4000000000009995":
			res.Outcome = Outcome{FailureCode: FailInsufficientFunds}
		default:
			res.Outcome = Outcome{Success: true, OperatorReference: "AUTH" + ids.Short(6)}
		}
		return res, nil
	}
	return Result{}, &ValidationError{"method", fmt.Sprintf("Método %q não suportado.", req.Method)}
}

func luhn(s string) bool {
	sum, alt := 0, false
	for i := len(s) - 1; i >= 0; i-- {
		d := int(s[i] - '0')
		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return sum%10 == 0
}

func validExpiry(e string) bool {
	d := digits.ReplaceAllString(e, "")
	if len(d) != 4 {
		return false
	}
	mm := int(d[0]-'0')*10 + int(d[1]-'0')
	yy := 2000 + int(d[2]-'0')*10 + int(d[3]-'0')
	if mm < 1 || mm > 12 {
		return false
	}
	now := time.Now()
	if yy < now.Year() || (yy == now.Year() && mm < int(now.Month())) {
		return false
	}
	return true
}
