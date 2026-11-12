package rails

import (
	"context"
	"sort"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// Via is one integration a Router can send payments through: its rail and the
// methods it takes. No methods means any, as the simulator.
type Via struct {
	Rail    ports.Rail
	Methods []string
}

func (v Via) takes(method string) bool {
	return len(v.Methods) == 0 || domain.Contains(v.Methods, method)
}

// Router sends each payment through an integration, by code: the one the
// payment names if it takes the method, or else the method's default, or else
// the fallback. A method nothing takes is refused.
type Router struct {
	Via      map[string]Via
	Default  map[string]string // method → code
	Fallback string            // code
}

// Single is a Router with one integration for every method, for tests.
func Single(code string, r ports.Rail) Router {
	return Router{Via: map[string]Via{code: {Rail: r}}, Fallback: code}
}

// RouteFor names the integration a payment by method goes through when it
// asks for want, or "" when none takes the method. A Router is one mode's.
func (r Router) RouteFor(want, method string, _ bool) string {
	if v, ok := r.Via[want]; ok && v.takes(method) {
		return want
	}
	if code := r.Default[method]; code != "" {
		if v, ok := r.Via[code]; ok && v.takes(method) {
			return code
		}
	}
	if v, ok := r.Via[r.Fallback]; ok && v.takes(method) {
		return r.Fallback
	}
	return ""
}

func (r Router) rail(want, method string) ports.Rail {
	if code := r.RouteFor(want, method, false); code != "" {
		return r.Via[code].Rail
	}
	return nil
}

// Routes says which integrations take each method, the default first.
func (r Router) Routes(bool) map[string][]string {
	out := map[string][]string{}
	for _, m := range domain.AllMethods {
		def := r.RouteFor("", m, false)
		if def == "" {
			continue
		}
		codes := []string{def}
		others := []string{}
		for code, v := range r.Via {
			if code != def && len(v.Methods) > 0 && v.takes(m) {
				others = append(others, code)
			}
		}
		sort.Strings(others)
		out[m] = append(codes, others...)
	}
	return out
}

func (r Router) Initiate(ctx context.Context, req domain.ChargeRequest, onComplete func(domain.Outcome)) (domain.ChargeResult, error) {
	x := r.rail(req.Integration, req.Method)
	if x == nil {
		return domain.ChargeResult{}, &domain.InputError{Field: "method", Msg: "Este método de pagamento ainda não está disponível."}
	}
	return x.Initiate(ctx, req, onComplete)
}

func (r Router) Query(ctx context.Context, q domain.StatusQuery) (domain.Outcome, bool, error) {
	if x, ok := r.rail(q.Integration, q.Method).(ports.RailQuery); ok {
		return x.Query(ctx, q)
	}
	return domain.Outcome{}, false, ports.ErrNoStatusCheck
}

func (r Router) ReferenceVia(integration, method string, live bool) string {
	if x, ok := r.rail(integration, method).(ports.RailReferencer); ok {
		return x.NewReference(method, live)
	}
	return domain.RailReference()
}

func (r Router) NewReference(method string, live bool) string {
	return r.ReferenceVia("", method, live)
}
