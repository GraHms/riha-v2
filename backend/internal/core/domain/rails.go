package domain

import (
	"log"
	"regexp"
	"runtime/debug"
	"strings"
)

const (
	MethodMpesa = "mpesa"
	MethodEmola = "emola"
	MethodBim   = "bim"
	MethodCard  = "card"
	// MethodAbroad is a wallet from another country, for a customer abroad
	// paying a Mozambican merchant. A merchant turns it on per link.
	MethodAbroad = "abroad"
)

// AllMethods are offered when nobody says which.
var AllMethods = []string{MethodMpesa, MethodEmola, MethodBim, MethodCard}

// OptInMethods are only offered when asked for by name.
var OptInMethods = []string{MethodAbroad}

var MethodNames = map[string]string{MethodMpesa: "M-Pesa", MethodEmola: "e-Mola", MethodBim: "Bim IZI", MethodCard: "Cartão", MethodAbroad: "Carteira de outro país"}

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
	// Country is where the customer's wallet is, ISO 3166 alpha-2, for a
	// payment from abroad.
	Country string
	// Live is real money. Reference is what the operator knows the payment by.
	Live      bool
	Reference string
	// OnRailID receives the id a rail gives the payment, if it has to be kept to ask
	// about the payment later.
	OnRailID func(string)
	// Integration is the code of the integration to send the payment through;
	// empty takes the method's default.
	Integration string
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
	// RedirectURL is the operator's own page, where the customer finishes
	// paying; empty when they pay on their phone.
	RedirectURL string
}

type StatusQuery struct {
	PaymentID string
	Live      bool
	Reference string
	RailID    string
	Method    string
	// Integration is the code of the integration that took the payment.
	Integration string
	// Urgent is for a callback: the operator just said something changed.
	Urgent bool
	// Amount is what the payment asked for, in cents: an answer that says
	// another amount was paid is not about this payment.
	Amount int64
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

// Safely runs f on its own goroutine: a panic in it is logged, never the end
// of the server, which may be in the middle of other customers' payments.
func Safely(name string, f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("%s: panic: %v\n%s", name, r, debug.Stack())
			}
		}()
		f()
	}()
}

// AbroadCountry is a country customers can pay from with their own wallet.
type AbroadCountry struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Dial     string `json:"dial"`
	Currency string `json:"currency"`
}
