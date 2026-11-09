package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func (a *App) SubSettingsFor(ctx context.Context, m *domain.Merchant) domain.SubscriptionSettings {
	if s, err := a.SubSettings.Get(ctx, m.ID); err == nil {
		return *s
	}
	return domain.DefaultSubscriptionSettings(m)
}

func (a *App) SaveSubSettings(ctx context.Context, m *domain.Merchant, in domain.SubscriptionSettings, actor Actor) (domain.SubscriptionSettings, error) {
	if e := domain.RequireApproved(m); e != nil {
		return in, e
	}
	in.MerchantID = m.ID
	var disabled []string
	for _, k := range in.DisabledSteps {
		if k != domain.StepLink && k != domain.StepFinal && !domain.Contains(disabled, k) {
			disabled = append(disabled, k)
		}
	}
	in.DisabledSteps = disabled
	if err := in.Validate(); err != nil {
		return in, err
	}
	in.UpdatedAt = a.Now()
	if err := a.SubSettings.Save(ctx, &in); err != nil {
		return in, err
	}
	a.Record(ctx, m.ID, actor, "subscriptions.settings", m.ID, fmt.Sprintf("tolerância %d dias, ação %s", in.ToleranceDays, in.FinalAction))
	return in, nil
}

type PlanInput struct {
	Name       string     `json:"name"`
	Amount     int64      `json:"amount"`
	Interval   string     `json:"interval"`
	Anchor     string     `json:"anchor"`
	BillingDay int        `json:"billing_day"`
	Months     int        `json:"months"`
	Missed     string     `json:"missed"`
	Fields     []string   `json:"fields"`
	Highlights []string   `json:"highlights"`
	Cycles     int        `json:"cycles"`
	EndsAt     *time.Time `json:"ends_at"`
	Draft      bool       `json:"draft"`
}

func (a *App) uniquePlanSlug(ctx context.Context, merchantID, name string) string {
	base := domain.Slugify(name)
	if base == "" {
		base = "plano"
	}
	slug := base
	for i := 2; ; i++ {
		if taken, _ := a.Plans.SlugTaken(ctx, merchantID, slug); !taken {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func (a *App) CreatePlan(ctx context.Context, m *domain.Merchant, live bool, in PlanInput, actor Actor) (*domain.Plan, error) {
	if e := domain.RequireApproved(m); e != nil {
		return nil, e
	}
	now := a.Now()
	p := &domain.Plan{ID: domain.NewID("plan"), MerchantID: m.ID, Livemode: live, Name: in.Name, Status: domain.PlanActive,
		Interval: in.Interval, Anchor: in.Anchor, BillingDay: in.BillingDay, Months: in.Months, Missed: in.Missed, Fields: in.Fields,
		Highlights: in.Highlights, Cycles: in.Cycles, EndsAt: in.EndsAt, Amount: in.Amount, Version: 1, InCatalog: !in.Draft, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now}
	if in.Draft {
		p.Status = domain.PlanDraft
	}
	if err := p.Validate(domain.PlanMinFor(m)); err != nil {
		return nil, err
	}
	if p.EndsAt != nil && !p.EndsAt.After(now) {
		return nil, domain.Invalid("parameter_invalid", "ends_at", "A data de fim tem de ser no futuro.")
	}
	existing, _ := a.Plans.List(ctx, m.ID, live)
	p.CatalogOrder = len(existing)
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		p.Slug = a.uniquePlanSlug(ctx, m.ID, p.Name)
		if err := a.Plans.Create(ctx, p); err != nil {
			return err
		}
		if err := a.Plans.AddVersion(ctx, &domain.PlanVersion{ID: domain.NewID("pv"), PlanID: p.ID, Version: 1, Amount: p.Amount, Scope: "initial", CreatedBy: actor.ID, CreatedAt: now}); err != nil {
			return err
		}
		return a.emit(ctx, m.ID, live, "plan.created", p.ID, domain.PlanResource(p, a.PublicURL, m.Slug), "")
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "plan.create", p.ID, p.Name)
	return p, nil
}

func (a *App) Plan(ctx context.Context, merchantID string, live bool, id string) (*domain.Plan, error) {
	p, err := a.Plans.ByID(ctx, merchantID, id)
	if err != nil || p.Livemode != live {
		return nil, domain.NotFound("O plano")
	}
	return p, nil
}

func (a *App) PlanList(ctx context.Context, merchantID string, live bool) []domain.Plan {
	ps, _ := a.Plans.List(ctx, merchantID, live)
	return ps
}

func (a *App) PlanVersions(ctx context.Context, planID string) []domain.PlanVersion {
	vs, _ := a.Plans.Versions(ctx, planID)
	return vs
}

func (a *App) UpdatePlan(ctx context.Context, m *domain.Merchant, live bool, id string, in PlanInput, actor Actor) (*domain.Plan, error) {
	if e := domain.RequireApproved(m); e != nil {
		return nil, e
	}
	p, err := a.Plan(ctx, m.ID, live, id)
	if err != nil {
		return nil, err
	}
	counts, _ := a.Subs.CountByPlan(ctx, m.ID, live)
	members := int64(0)
	for st, n := range counts[p.ID] {
		if !domain.Closed(st) && st != domain.SubIncomplete {
			members += n
		}
	}
	if members > 0 && (in.Interval != p.Interval || in.Anchor != p.Anchor || in.BillingDay != p.BillingDay) {
		return nil, domain.Invalid("plan_has_members", "interval", "Este plano já tem membros. A frequência e o dia de cobrança não podem mudar; crie um plano novo.")
	}
	if members > 0 && in.Cycles != p.Cycles {
		return nil, domain.Invalid("plan_has_members", "cycles", "Este plano já tem membros. O número de pagamentos não pode mudar; crie um plano novo.")
	}
	if in.Amount != p.Amount {
		return nil, domain.Invalid("use_price_change", "amount", "Para mudar o valor use Mudar valor, que cria uma versão nova.")
	}
	q := *p
	q.Name, q.Interval, q.Anchor, q.BillingDay, q.Months, q.Missed, q.Fields, q.EndsAt = in.Name, in.Interval, in.Anchor, in.BillingDay, in.Months, in.Missed, in.Fields, in.EndsAt
	q.Highlights, q.Cycles = in.Highlights, in.Cycles
	if p.Status == domain.PlanDraft && !in.Draft {
		q.Status, q.InCatalog = domain.PlanActive, true
	}
	if err := q.Validate(domain.PlanMinFor(m)); err != nil {
		return nil, err
	}
	q.UpdatedAt = a.Now()
	if err := a.Tx.Do(ctx, func(ctx context.Context) error {
		if err := a.Plans.Save(ctx, &q); err != nil {
			return err
		}
		return a.emit(ctx, m.ID, live, "plan.updated", q.ID, domain.PlanResource(&q, a.PublicURL, m.Slug), "")
	}); err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "plan.update", q.ID, q.Name)
	return &q, nil
}

func (a *App) ArchivePlan(ctx context.Context, m *domain.Merchant, live bool, id string, actor Actor) error {
	p, err := a.Plan(ctx, m.ID, live, id)
	if err != nil {
		return err
	}
	if subs, _ := a.Subs.ByPlan(ctx, p.ID, append([]string{domain.SubInvited}, domain.LiveSubStatuses...)); len(subs) > 0 {
		return domain.Invalid("plan_has_members", "plan", "Cancele ou mude de plano os membros antes de arquivar.")
	}
	p.Status, p.InCatalog, p.UpdatedAt = domain.PlanArchived, false, a.Now()
	if err := a.Plans.Save(ctx, p); err != nil {
		return err
	}
	a.Record(ctx, m.ID, actor, "plan.archive", p.ID, p.Name)
	return nil
}

const (
	PriceScopeNew      = "new"
	PriceScopeAll      = "all"
	PriceScopeSelected = "selected"
	PriceNoticeDays    = 30
)

type PriceChange struct {
	Plan      *domain.Plan
	Affected  int
	EffectsAt *time.Time
}

func (a *App) ChangePrice(ctx context.Context, m *domain.Merchant, live bool, planID string, amount int64, scope string, subIDs []string, actor Actor) (*PriceChange, error) {
	if e := domain.RequireApproved(m); e != nil {
		return nil, e
	}
	if err := domain.PlanAmountOK(amount, domain.PlanMinFor(m)); err != nil {
		return nil, err
	}
	if scope != PriceScopeNew && scope != PriceScopeAll && scope != PriceScopeSelected {
		return nil, domain.Invalid("parameter_invalid", "scope", "Escolha quem passa a pagar o novo valor.")
	}
	if scope == PriceScopeSelected && len(subIDs) == 0 {
		return nil, domain.Invalid("parameter_missing", "subscriptions", "Escolha os membros que passam ao novo valor.")
	}
	out := &PriceChange{}
	var notices []domain.Subscription
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		p, err := a.Plan(ctx, m.ID, live, planID)
		if err != nil {
			return err
		}
		if amount == p.Amount {
			return domain.Invalid("amount_unchanged", "amount", "O valor novo é igual ao atual.")
		}
		now := a.Now()
		p.Version++
		p.Amount, p.UpdatedAt = amount, now
		if err := a.Plans.Save(ctx, p); err != nil {
			return err
		}
		if err := a.Plans.AddVersion(ctx, &domain.PlanVersion{ID: domain.NewID("pv"), PlanID: p.ID, Version: p.Version, Amount: amount, Scope: scope, CreatedBy: actor.ID, CreatedAt: now}); err != nil {
			return err
		}
		out.Plan = p
		if scope == PriceScopeNew {
			return nil
		}
		subs, err := a.Subs.ByPlan(ctx, p.ID, append([]string{domain.SubInvited}, domain.LiveSubStatuses...))
		if err != nil {
			return err
		}
		from := now.AddDate(0, 0, PriceNoticeDays)
		for _, s := range subs {
			if scope == PriceScopeSelected && !domain.Contains(subIDs, s.ID) {
				continue
			}
			due, ok := domain.NextDue(p, s.AnchorAt, from.Add(-time.Second))
			if !ok {
				continue
			}
			s.PendingAmount, s.PendingVersion, s.PendingFrom, s.UpdatedAt = amount, p.Version, &due, now
			if err := a.Subs.Save(ctx, &s); err != nil {
				return err
			}
			if out.EffectsAt == nil || due.Before(*out.EffectsAt) {
				d := due
				out.EffectsAt = &d
			}
			out.Affected++
			notices = append(notices, s)
		}
		for _, s := range notices {
			mem, err := a.Members.Get(ctx, s.MemberID)
			if err != nil {
				continue
			}
			body := fmt.Sprintf("%s: o valor de %s passa de %s para %s MT a partir de %s.", m.Name, p.Name, domain.FormatShort(s.Amount), domain.FormatShort(amount), domain.LongDate(*s.PendingFrom))
			if err := a.queueMessage(ctx, m, mem, "", "price_notice", body); err != nil {
				return err
			}
		}
		return a.emit(ctx, m.ID, live, "plan.updated", p.ID, domain.PlanResource(p, a.PublicURL, m.Slug), "")
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "plan.price", planID, fmt.Sprintf("v%d %s MT (%s, %d membros)", out.Plan.Version, domain.Format(amount), scope, out.Affected))
	return out, nil
}

type CatalogEntry struct {
	PlanID    string `json:"plan_id"`
	InCatalog bool   `json:"in_catalog"`
}

// CatalogPage is what a merchant sets about their catalog page.
type CatalogPage struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Accent      string `json:"accent"`
	Background  string `json:"background"`
}

// CatalogSlug is the address of a merchant's catalog: the one they chose, or
// their account's.
func CatalogSlug(m *domain.Merchant, s domain.SubscriptionSettings) string {
	return domain.FirstNonEmpty(s.CatalogSlug, m.Slug)
}

// CatalogSlugFree checks an address for a merchant's catalog.
func (a *App) CatalogSlugFree(ctx context.Context, m *domain.Merchant, slug string) (string, error) {
	slug, err := domain.CleanCatalogSlug(slug)
	if err != nil {
		return slug, err
	}
	if other, err := a.SubSettings.ByCatalogSlug(ctx, slug); err == nil && other.MerchantID != m.ID {
		return slug, domain.Invalid("slug_taken", "slug", "Este endereço já está a ser usado. Escolha outro.")
	}
	if other, err := a.Merchants.BySlug(ctx, slug); err == nil && other.ID != m.ID {
		return slug, domain.Invalid("slug_taken", "slug", "Este endereço já está a ser usado. Escolha outro.")
	}
	return slug, nil
}

func (a *App) SaveCatalog(ctx context.Context, m *domain.Merchant, live bool, entries []CatalogEntry, multi bool, page *CatalogPage, actor Actor) error {
	if e := domain.RequireApproved(m); e != nil {
		return e
	}
	if page != nil && page.Slug != "" && page.Slug != m.Slug {
		slug, err := a.CatalogSlugFree(ctx, m, page.Slug)
		if err != nil {
			return err
		}
		page.Slug = slug
	}
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		for i, e := range entries {
			p, err := a.Plan(ctx, m.ID, live, e.PlanID)
			if err != nil {
				return err
			}
			p.CatalogOrder, p.InCatalog, p.UpdatedAt = i, e.InCatalog && p.Status == domain.PlanActive, a.Now()
			if err := a.Plans.Save(ctx, p); err != nil {
				return err
			}
		}
		s := a.SubSettingsFor(ctx, m)
		s.CatalogMulti, s.UpdatedAt = multi, a.Now()
		if page != nil {
			s.CatalogSlug, s.CatalogTitle, s.CatalogDescription, s.CatalogAccent, s.CatalogBackground = page.Slug, page.Title, page.Description, page.Accent, page.Background
			if s.CatalogSlug == m.Slug {
				s.CatalogSlug = ""
			}
			if err := s.ValidateCatalog(); err != nil {
				return err
			}
		}
		return a.SubSettings.Save(ctx, &s)
	})
	if err == nil {
		a.Record(ctx, m.ID, actor, "catalog.update", m.ID, fmt.Sprintf("%d planos", len(entries)))
	}
	return err
}

func (a *App) PublicMerchant(ctx context.Context, slug string) (*domain.Merchant, error) {
	return a.publicMerchant(ctx, slug)
}

func (a *App) PublicPlanByID(ctx context.Context, merchantID, id string) *domain.Plan {
	p, err := a.Plans.Get(ctx, id)
	if err != nil || p.MerchantID != merchantID || p.Status != domain.PlanActive {
		return nil
	}
	return p
}

// publicMerchant finds a merchant by their catalog's address, or by their
// account's slug, which is the address until they choose one.
func (a *App) publicMerchant(ctx context.Context, slug string) (*domain.Merchant, error) {
	var m *domain.Merchant
	var err error
	if s, e := a.SubSettings.ByCatalogSlug(ctx, slug); e == nil {
		m, err = a.Merchants.ByID(ctx, s.MerchantID)
	} else {
		m, err = a.Merchants.BySlug(ctx, slug)
	}
	if err != nil || m.Status != domain.MerchantActive {
		return nil, domain.NotFound("Esta página")
	}
	return m, nil
}

func (a *App) PublicPlan(ctx context.Context, merchantSlug, planSlug string) (*domain.Merchant, *domain.Plan, error) {
	m, err := a.publicMerchant(ctx, merchantSlug)
	if err != nil {
		return nil, nil, err
	}
	p, err := a.Plans.BySlug(ctx, m.ID, planSlug)
	if err != nil {
		return nil, nil, domain.NotFound("Este plano")
	}
	return m, p, nil
}

func (a *App) PublicCatalog(ctx context.Context, merchantSlug string) (*domain.Merchant, []domain.Plan, domain.SubscriptionSettings, error) {
	m, err := a.publicMerchant(ctx, merchantSlug)
	if err != nil {
		return nil, nil, domain.SubscriptionSettings{}, err
	}
	var out []domain.Plan
	for _, live := range []bool{true, false} {
		for _, p := range a.PlanList(ctx, m.ID, live) {
			if p.InCatalog && p.Status == domain.PlanActive {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			break
		}
	}
	if len(out) == 0 {
		return nil, nil, domain.SubscriptionSettings{}, domain.NotFound("Este catálogo")
	}
	return m, out, a.SubSettingsFor(ctx, m), nil
}

func (a *App) FirstPeriods(p *domain.Plan, n int) []time.Time {
	first, ok := domain.FirstDue(p, a.Now())
	if !ok {
		return nil
	}
	out := []time.Time{first.UTC()}
	for len(out) < n {
		d, ok := domain.NextDue(p, first, out[len(out)-1])
		if !ok {
			break
		}
		out = append(out, d)
	}
	return out
}

func planOf(plans map[string]*domain.Plan, a *App, ctx context.Context, id string) *domain.Plan {
	if p := plans[id]; p != nil {
		return p
	}
	p, err := a.Plans.Get(ctx, id)
	if err != nil {
		return &domain.Plan{ID: id, Name: "Plano removido", Months: domain.AllMonths, Interval: domain.IntervalMonth}
	}
	plans[id] = p
	return p
}

func isNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }

func liveQuery(live bool) *bool { return &live }

var _ = strings.TrimSpace
var _ = ports.ChargeQuery{}
