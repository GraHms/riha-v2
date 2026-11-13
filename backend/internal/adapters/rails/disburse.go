package rails

import (
	"context"
	"errors"
	"strings"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// SimulatedDisburser stands in for sending money to a wallet. A number ending
// in 0000 is refused, to try a failed delivery.
type SimulatedDisburser struct{}

func (SimulatedDisburser) Disburse(_ context.Context, d domain.Disbursement) (domain.DisbursementResult, error) {
	if strings.HasSuffix(domain.NormalizeMZPhone(d.Phone), "0000") {
		return domain.DisbursementResult{}, errors.New(domain.MethodNames[d.Method] + " recusou a entrega para " + domain.FormatPhone(d.Phone))
	}
	return domain.DisbursementResult{ProviderRef: "SIM-" + domain.ShortCode(10)}, nil
}

// Disbursers picks the disburser for the mode. A mode without one can't deliver
// through RIHA.
type Disbursers struct {
	Test ports.Disburser
	Live ports.Disburser
}

var ErrNoDisburser = errors.New("a entrega para a carteira não está disponível")

func (d Disbursers) pick(live bool) ports.Disburser {
	if live {
		return d.Live
	}
	return d.Test
}

func (d Disbursers) Available(live bool) bool { return d.pick(live) != nil }

func (d Disbursers) Disburse(ctx context.Context, x domain.Disbursement) (domain.DisbursementResult, error) {
	p := d.pick(x.Live)
	if p == nil {
		return domain.DisbursementResult{}, ErrNoDisburser
	}
	return p.Disburse(ctx, x)
}
