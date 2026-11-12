package domain

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	MinAmount = 1000
	MaxAmount = 100_000_000
	// PlanMinAmount is the least a subscription plan may charge: below it, the SMS a
	// member gets each period cost more than RIHA earns on the payment.
	PlanMinAmount = 50_000
	// BetaMinAmount is the least a beta account may charge anywhere, to test with
	// real money at almost no cost.
	BetaMinAmount = 100
)

// MinAmountFor is the least a merchant may charge in one payment.
func MinAmountFor(m *Merchant) int64 {
	if m != nil && m.Beta {
		return BetaMinAmount
	}
	return MinAmount
}

// PlanMinFor is the least a merchant's subscription plan may charge.
func PlanMinFor(m *Merchant) int64 {
	if m != nil && m.Beta {
		return BetaMinAmount
	}
	return PlanMinAmount
}

func RequireApproved(m *Merchant) *Error {
	switch m.Status {
	case MerchantActive:
		return nil
	case MerchantSuspended:
		return Forbidden("account_suspended", "Esta conta está suspensa. Contacte o suporte da RIHA: support@rihagateway.com.")
	case MerchantRejected:
		return Forbidden("account_rejected", "A verificação desta conta foi rejeitada.")
	}
	return Forbidden("account_not_approved", "A sua conta ainda não foi aprovada. Nenhuma operação é possível até a equipa RIHA concluir a verificação.")
}

func Slugify(s string) string {
	s = norm.NFD.String(strings.ToLower(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" {
		out = "loja"
	}
	return out
}

func Descriptor(name string) string {
	var b strings.Builder
	for _, r := range "RIHA*" + strings.ToUpper(norm.NFD.String(name)) {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '*') {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > 22 {
		out = out[:22]
	}
	return strings.TrimSpace(out)
}

func Initials(name string) string {
	name = strings.SplitN(name, ",", 2)[0]
	var out []rune
	for _, w := range strings.Fields(name) {
		r := []rune(w)
		if len(r) > 0 && strings.ToUpper(string(r[0])) == string(r[0]) && len(out) < 2 {
			out = append(out, r[0])
		}
	}
	if len(out) == 0 && len(name) > 0 {
		out = append(out, []rune(strings.ToUpper(name))[0])
	}
	return string(out)
}

func Trim(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

func FirstNonEmpty(v ...string) string {
	for _, x := range v {
		if x != "" {
			return x
		}
	}
	return ""
}

func CleanMethods(in []string) ([]string, error) {
	out := []string{}
	for _, m := range in {
		m = strings.ToLower(strings.TrimSpace(m))
		if m == "" {
			continue
		}
		if !contains(AllMethods, m) && !contains(OptInMethods, m) {
			return nil, Invalid("payment_method_invalid", "payment_method_types", fmt.Sprintf("Método %q desconhecido. Use mpesa, emola, bim, card ou abroad.", m))
		}
		if !contains(out, m) {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		out = append(out, AllMethods...)
	}
	return out, nil
}

type KycCheck struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Note  string `json:"note"`
}

func KycChecks(m *Merchant, bank *BankAccount, documents int) []KycCheck {
	out := []KycCheck{}
	nuit := Digits(m.NUIT)
	switch {
	case len(nuit) != 9:
		out = append(out, KycCheck{"bad", "NUIT inválido", "O NUIT tem de ter 9 dígitos."})
	case strings.HasPrefix(nuit, "409"):
		out = append(out, KycCheck{"bad", "NUIT não encontrado", "Sem registo na Autoridade Tributária (simulado)."})
	default:
		out = append(out, KycCheck{"ok", "NUIT válido", "Confirmado na Autoridade Tributária (simulado)."})
	}
	if bank == nil {
		out = append(out, KycCheck{"warn", "Sem conta bancária", "O comerciante ainda não indicou o NIB."})
	} else {
		h := strings.ToLower(strings.TrimSpace(bank.Holder))
		if h == strings.ToLower(m.LegalName) || h == strings.ToLower(m.Name) || h == strings.ToLower(m.OwnerName) {
			out = append(out, KycCheck{"ok", "Titular da conta coincide", bank.BankName + ": " + bank.Holder})
		} else {
			out = append(out, KycCheck{"warn", "Titular da conta diferente", "Conta em nome de " + bank.Holder + ", não de " + FirstNonEmpty(m.LegalName, m.Name)})
		}
	}
	if documents >= 2 {
		out = append(out, KycCheck{"ok", "Documentos carregados", fmt.Sprintf("%d documentos", documents)})
	} else {
		out = append(out, KycCheck{"warn", "Documentos em falta", "Precisa do BI e do alvará ou certidão comercial."})
	}
	if strings.Contains(m.ExpectedVolume, "Mais de 1") {
		out = append(out, KycCheck{"warn", "Volume elevado", "Requer revisão de dois administradores."})
	}
	return out
}

func Risk(checks []KycCheck) string {
	warn := 0
	for _, c := range checks {
		if c.Kind == "bad" {
			return "Alto"
		}
		if c.Kind == "warn" {
			warn++
		}
	}
	if warn > 0 {
		return "Médio"
	}
	return "Baixo"
}
