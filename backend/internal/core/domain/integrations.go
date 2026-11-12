package domain

import (
	"regexp"
	"time"
)

// Integration codes. A code names an integration everywhere: on each payment it
// took, in a merchant's routes and deals, and in its partner's settlements.
const (
	// IntegrationMtoca is the mtoca team's API: M-Pesa, e-Mola and bim, all
	// collected into the mtoca team's account at bim.
	IntegrationMtoca = "mtoca"
	// IntegrationMpesaDirect is the M-Pesa API on shortcode 900511, which is
	// 92 Interactions'.
	IntegrationMpesaDirect = "mpesa_direct"
	// IntegrationSimulator is test mode's: no money moves.
	IntegrationSimulator = "simulator"
)

var integrationCode = regexp.MustCompile(`^[a-z][a-z0-9_]{1,29}$`)

func ValidIntegrationCode(code string) bool { return integrationCode.MatchString(code) }

// An Integration is a way in for customers' money: a partner's API, and the
// account the money lands in until the partner pays it to RIHA. The deal is what
// the partner takes: PlatformBps is the operator's charge on each payment, by
// method, and ShareBps the partner's part of what is left of RIHA's fee after it.
type Integration struct {
	Code        string
	Name        string
	Partner     string
	Account     string
	PlatformBps map[string]int
	ShareBps    int
	Note        string
	UpdatedAt   time.Time
}

// DefaultIntegrations are the ones RIHA starts with; the backoffice edits them.
// The mtoca deal is 30% of what RIHA's fee leaves after the operator's charge.
// 92 Interactions' isn't agreed, so it takes nothing yet; M-Pesa charges 3% on
// shortcode 900511.
func DefaultIntegrations() []Integration {
	return []Integration{
		{Code: IntegrationMtoca, Name: "mtoca", Partner: "Equipa mtoca", Account: "Conta da mtoca no bim", PlatformBps: map[string]int{}, ShareBps: 3000},
		{Code: IntegrationMpesaDirect, Name: "M-Pesa direto", Partner: "92 Interactions", Account: "M-Pesa · shortcode 900511", PlatformBps: map[string]int{MethodMpesa: 300}, ShareBps: 0},
		{Code: IntegrationSimulator, Name: "Simulador", Partner: "RIHA", Account: "Modo de teste: não há dinheiro", PlatformBps: map[string]int{}, ShareBps: 0},
	}
}

// MerchantRoute sends one merchant's payments by one method through an
// integration other than the default.
type MerchantRoute struct {
	MerchantID  string
	Method      string
	Integration string
	UpdatedBy   string
	UpdatedAt   time.Time
}

// MerchantDeal gives one merchant's payments through an integration a partner
// share other than the integration's own.
type MerchantDeal struct {
	MerchantID  string
	Integration string
	ShareBps    int
	UpdatedBy   string
	UpdatedAt   time.Time
}

// Price is how a payment's fee divides: the operator's charge, the partner's
// share, and what stays with RIHA, which is negative when the operator charges
// more than RIHA's fee.
type Price struct {
	PlatformFee int64
	PartnerFee  int64
}

func (p Price) Margin(fee int64) int64 { return fee - p.PlatformFee - p.PartnerFee }

// PriceOf divides a payment of amount, with RIHA's fee, on a deal: the platform
// fee comes off first, and the partner shares only what is left.
func PriceOf(amount, fee int64, platformBps, shareBps int) Price {
	platform := Fee(amount, platformBps)
	left := fee - platform
	if left < 0 {
		left = 0
	}
	return Price{PlatformFee: platform, PartnerFee: Fee(left, shareBps)}
}

// A PartnerSettlement is money a partner paid RIHA out of what it collected.
type PartnerSettlement struct {
	ID          string
	Integration string
	Amount      int64
	Reference   string
	Note        string
	ReceivedOn  time.Time
	CreatedBy   string
	CreatedAt   time.Time
}
