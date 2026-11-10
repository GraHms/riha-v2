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

var MethodNames = map[string]string{MethodMpesa: "M-Pesa", MethodEmola: "e-Mola", MethodBim: "Bim IZI", MethodCard: "Cartão"}

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
	FailPhoneInvalid        = "phone_invalid"
	FailAccountInactive     = "account_inactive"
	FailPaymentDeclined     = "payment_declined"
	FailRejected            = "payment_rejected"
	FailCanceled            = "payment_canceled"
	FailNotCompleted        = "payment_not_completed"
	FailNotConfirmed        = "payment_not_confirmed"
)

var FailureMessages = map[string]string{
	FailPinIncorrect:        "PIN errado três vezes. O cliente pode tentar de novo com o mesmo link.",
	FailInsufficientFunds:   "Saldo insuficiente para este pagamento. Carregue a conta ou escolha um valor mais baixo.",
	FailCustomerTimeout:     "O cliente não confirmou o pedido a tempo.",
	FailCustomerCancelled:   "O cliente cancelou o pedido no telemóvel.",
	FailCardDeclined:        "Cartão recusado pelo banco emissor.",
	FailOperatorUnavailable: "O operador não respondeu. Tente de novo dentro de minutos.",
	FailPhoneInvalid:        "Este número não é uma conta M-Pesa válida.",
	FailAccountInactive:     "A conta M-Pesa do cliente está inativa. Contacte a assistência M-Pesa.",
	FailPaymentDeclined:     "O M-Pesa recusou o pagamento. Verifique a conta e tente de novo.",
	FailRejected:            "O pagamento foi recusado. Confirme o saldo e tente de novo.",
	FailCanceled:            "O pagamento foi cancelado.",
	FailNotCompleted:        "O pagamento não foi concluído. Confirme o saldo e tente de novo.",
	FailNotConfirmed:        "O operador nunca confirmou este pagamento, por isso não foi cobrado.",
}

type ChargeRequest struct {
	PaymentID   string
	Description string
	Method      string
	Amount      int64
	Phone       string
	CardNumber  string
	CardExpiry  string
	CardCVC     string
	Descriptor  string
	// Live is real money. Reference is what the operator knows the payment by.
	Live      bool
	Reference string
	// OnRailID receives the id a rail gives the payment, if it has to be kept to ask
	// about the payment later.
	OnRailID func(string)
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
	PaymentID string
	Live      bool
	Reference string
	RailID    string
	Method    string
	// Urgent is for a callback: the operator just said something changed.
	Urgent bool
}

var attemptShape = regexp.MustCompile(`^[A-Z2-9-]{9,20}$`)

// CleanAttemptID drops an id the page's own code wouldn't have made, so it reads as none.
func CleanAttemptID(id string) string {
	if attemptShape.MatchString(id) {
		return id
	}
	return ""
}

// OperationNumber is the reference mtoca takes: 12 digits, not starting with 0.
func OperationNumber() string { return randFrom("123456789", 1) + randFrom("0123456789", 11) }

type InputError struct{ Field, Msg string }

func (e *InputError) Error() string { return e.Msg }

// RailReference is the reference an operator sees: letters and digits only, and
// at most 20 of them, which is all M-Pesa accepts.
func RailReference() string { return "RIHA" + ShortCode(16) }

var nonDigits = regexp.MustCompile(`\D`)

func Digits(s string) string { return nonDigits.ReplaceAllString(s, "") }

func NormalizeMZPhone(p string) string { return strings.TrimPrefix(Digits(p), "258") }
