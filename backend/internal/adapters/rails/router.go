package rails

import (
	"context"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type Router struct {
	ByMethod map[string]ports.Rail
	Fallback ports.Rail
}

func (r Router) rail(method string) ports.Rail {
	if x := r.ByMethod[method]; x != nil {
		return x
	}
	return r.Fallback
}

func (r Router) Initiate(ctx context.Context, req domain.ChargeRequest, onComplete func(domain.Outcome)) (domain.ChargeResult, error) {
	x := r.rail(req.Method)
	if x == nil {
		return domain.ChargeResult{}, &domain.InputError{Field: "method", Msg: "Este método de pagamento ainda não está disponível."}
	}
	return x.Initiate(ctx, req, onComplete)
}

func (r Router) Status(ctx context.Context, q domain.StatusQuery) (domain.Outcome, bool, error) {
	if sr, ok := r.rail(q.Method).(ports.StatusRail); ok {
		return sr.Status(ctx, q)
	}
	return domain.Outcome{}, false, ports.ErrNoStatusCheck
}
