package domain

import (
	"regexp"
	"strings"
)

const (
	MethodMpesa = "mpesa"
	MethodEmola = "emola"
	MethodBim   = "bim"
	MethodCard  = "card"
)

var AllMethods = []string{MethodMpesa, MethodEmola, MethodBim, MethodCard}

var MethodNames = map[string]string{MethodMpesa: "M-Pesa", MethodEmola: "e-Mola", MethodBim: "bim", MethodCard: "Cartão"}

func IsWallet(m string) bool { return m == MethodMpesa || m == MethodEmola || m == MethodBim }

func WalletNumberError(method, phone string) *InputError {
	p := NormalizeMZPhone(phone)
	if len(p) != 9 || p[0] != '8' || !strings.ContainsRune("234567", rune(p[1])) {
		return &InputError{Field: "phone", Msg: "Número inválido. Use 9 dígitos, por exemplo 84 123 4567."}
	}
	switch {
	case method == MethodMpesa && p[1] != '4' && p[1] != '5':
		return &InputError{Field: "phone", Msg: "Números M-Pesa começam por 84 ou 85."}
	case method == MethodEmola && p[1] != '6' && p[1] != '7':
		return &InputError{Field: "phone", Msg: "Números e-Mola começam por 86 ou 87."}
	}
	return nil
}

const (
	FailPinIncorrect        = "pin_incorrect"
	FailInsufficientFunds   = "insufficient_funds"
	FailCustomerTimeout     = "customer_timeout"
	FailCustomerCancelled   = "customer_cancelled"
	FailCardDeclined        = "card_declined"
	FailOperatorUnavailable = "operator_unavailable"
	FailRejected            = "payment_rejected"
	FailCanceled            = "payment_canceled"
)

var FailureMessages = map[string]string{
	FailPinIncorrect:        "PIN errado três vezes. O cliente pode tentar de novo com o mesmo link.",
	FailInsufficientFunds:   "Saldo insuficiente na carteira ou cartão do cliente.",
	FailCustomerTimeout:     "O cliente não confirmou o pedido a tempo.",
	FailCustomerCancelled:   "O cliente cancelou o pedido no telemóvel.",
	FailCardDeclined:        "Cartão recusado pelo banco emissor.",
	FailOperatorUnavailable: "O operador não respondeu. Tente de novo dentro de minutos.",
	FailRejected:            "O pagamento foi recusado. Confirme o saldo e tente de novo.",
	FailCanceled:            "O pagamento foi cancelado.",
}

type ChargeRequest struct {
	PaymentID   string
	Reference   string
	Description string
	Method      string
	Amount      int64
	Phone       string
	CardNumber  string
	CardExpiry  string
	CardCVC     string
	Descriptor  string
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

type StatusQuery struct {
	PaymentID   string
	Reference   string
	Method      string
	Amount      int64
	Phone       string
	Description string
}

func OperationNumber() string { return randFrom("123456789", 1) + randFrom("0123456789", 11) }

type InputError struct{ Field, Msg string }

func (e *InputError) Error() string { return e.Msg }

var nonDigits = regexp.MustCompile(`\D`)

func Digits(s string) string { return nonDigits.ReplaceAllString(s, "") }

func NormalizeMZPhone(p string) string { return strings.TrimPrefix(Digits(p), "258") }
