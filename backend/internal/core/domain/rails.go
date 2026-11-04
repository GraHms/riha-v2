package domain

import (
	"regexp"
	"strings"
)

const (
	MethodMpesa = "mpesa"
	MethodEmola = "emola"
	MethodCard  = "card"
)

var AllMethods = []string{MethodMpesa, MethodEmola, MethodCard}

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

type ChargeRequest struct {
	PaymentID  string
	Method     string
	Amount     int64
	Phone      string
	CardNumber string
	CardExpiry string
	CardCVC    string
	Descriptor string
}

type Outcome struct {
	Success           bool
	FailureCode       string
	OperatorReference string
}

type ChargeResult struct {
	Async   bool
	Outcome Outcome
	Brand   string
	Last4   string
}

type InputError struct{ Field, Msg string }

func (e *InputError) Error() string { return e.Msg }

var nonDigits = regexp.MustCompile(`\D`)

func Digits(s string) string { return nonDigits.ReplaceAllString(s, "") }

func NormalizeMZPhone(p string) string { return strings.TrimPrefix(Digits(p), "258") }
