package app

import (
	"context"
	"sort"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type PlanStats struct {
	Members  int64            `json:"members"`
	ByStatus map[string]int64 `json:"by_status"`
	Monthly  int64            `json:"monthly"`
}

func (a *App) PlanStatsFor(ctx context.Context, merchantID string, live bool) map[string]PlanStats {
	counts, _ := a.Subs.CountByPlan(ctx, merchantID, live)
	subs, _ := a.Subs.Live(ctx, merchantID, live)
	plans := map[string]*domain.Plan{}
	out := map[string]PlanStats{}
	for id, by := range counts {
		st := PlanStats{ByStatus: by}
		for k, n := range by {
			if k != domain.SubCanceled && k != domain.SubIncomplete {
				st.Members += n
			}
		}
		out[id] = st
	}
	for _, s := range subs {
		if s.Status != domain.SubActive && s.Status != domain.SubPastDue {
			continue
		}
		st := out[s.PlanID]
		st.Monthly += domain.MonthlyEquivalent(s.Amount, planOf(plans, a, ctx, s.PlanID).Interval)
		out[s.PlanID] = st
	}
	return out
}

type MonthBreakdown struct {
	Month          time.Time `json:"-"`
	Label          string    `json:"label"`
	Expected       int64     `json:"expected"`
	RIHA           int64     `json:"riha"`
	RihaCount      int64     `json:"riha_count"`
	Outside        int64     `json:"outside"`
	OutsideCount   int64     `json:"outside_count"`
	Debt           int64     `json:"debt"`
	DebtCount      int64     `json:"debt_count"`
	Open           int64     `json:"open"`
	OpenCount      int64     `json:"open_count"`
	ReceivedInRiha int64     `json:"received_in_riha"`
}

type Attention struct {
	Kind   string `json:"kind"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Member string `json:"member_id,omitempty"`
	Tone   string `json:"tone"`
}

type Upcoming struct {
	When   string `json:"when"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
	Count  int    `json:"count"`
	Total  int64  `json:"total"`
	Empty  bool   `json:"empty"`
}

type SubsOverview struct {
	Monthly      int64            `json:"monthly"`
	Members      int64            `json:"members"`
	Plans        int              `json:"plans"`
	Invites      int64            `json:"invites"`
	Debt         int64            `json:"debt"`
	DebtMembers  int64            `json:"debt_members"`
	MaxDebtCount int              `json:"max_debt_count"`
	Counts       map[string]int64 `json:"counts"`
	Month        MonthBreakdown   `json:"month"`
	Attention    []Attention      `json:"attention"`
	Upcoming     []Upcoming       `json:"upcoming"`
}

func monthStart(t time.Time) time.Time {
	l := t.In(domain.Maputo)
	return time.Date(l.Year(), l.Month(), 1, 0, 0, 0, 0, domain.Maputo)
}

func (a *App) SubscriptionOverview(ctx context.Context, m *domain.Merchant, live bool) SubsOverview {
	now := a.Now()
	o := SubsOverview{Counts: map[string]int64{}}
	o.Counts, _ = a.Members.CountByStatus(ctx, m.ID, live)
	for k, n := range o.Counts {
		if k == domain.SubActive || k == domain.SubPastDue || k == domain.SubPaused || k == domain.SubSuspended {
			o.Members += n
		}
	}
	o.Invites = o.Counts[domain.SubInvited]
	plans := map[string]*domain.Plan{}
	activePlans := 0
	for _, p := range a.PlanList(ctx, m.ID, live) {
		pp := p
		plans[p.ID] = &pp
		if p.Status == domain.PlanActive {
			activePlans++
		}
	}
	subs, _ := a.Subs.Live(ctx, m.ID, live)
	used := map[string]bool{}
	for _, s := range subs {
		if s.Status == domain.SubActive || s.Status == domain.SubPastDue {
			o.Monthly += domain.MonthlyEquivalent(s.Amount, planOf(plans, a, ctx, s.PlanID).Interval)
			used[s.PlanID] = true
		}
	}
	o.Plans = len(used)
	if o.Plans == 0 {
		o.Plans = activePlans
	}
	debtors, _ := a.Members.Find(ctx, ports.MemberQuery{MerchantID: m.ID, Live: live, Statuses: []string{domain.SubPastDue, domain.SubSuspended}, Limit: 500})
	for _, d := range debtors {
		if d.Debt > 0 {
			o.Debt += d.Debt
			o.DebtMembers++
			if d.DebtCount > o.MaxDebtCount {
				o.MaxDebtCount = d.DebtCount
			}
		}
	}
	month := monthStart(now)
	if cs, _ := a.Charges.Find(ctx, ports.ChargeQuery{MerchantID: m.ID, Live: &live, DueFrom: ptr(now), DueTo: ptr(month.AddDate(0, 1, 0).UTC()), Statuses: []string{domain.ChargeScheduled, domain.ChargeOpen}, Limit: 1}); len(cs) > 0 {
		month = month.AddDate(0, -1, 0)
	}
	next := month.AddDate(0, 1, 0)
	o.Month = MonthBreakdown{Month: month, Label: domain.MonthName(month.Month())}
	sums, _ := a.Charges.Sums(ctx, ports.ChargeQuery{MerchantID: m.ID, Live: &live, DueFrom: ptr(month.UTC()), DueTo: ptr(next.UTC()), ExcludeInvites: true})
	for _, s := range sums {
		switch s.Status {
		case domain.ChargePaid:
			o.Month.RIHA, o.Month.RihaCount = s.Total, s.Count
		case domain.ChargePaidOutside:
			o.Month.Outside, o.Month.OutsideCount = s.Total, s.Count
		case domain.ChargePastDue:
			o.Month.Debt, o.Month.DebtCount = s.Total, s.Count
		case domain.ChargeOpen, domain.ChargeScheduled:
			o.Month.Open += s.Total
			o.Month.OpenCount += s.Count
		default:
			continue
		}
		o.Month.Expected += s.Total
	}
	paid, _ := a.Charges.Sums(ctx, ports.ChargeQuery{MerchantID: m.ID, Live: &live, Statuses: []string{domain.ChargePaid}, PaidFrom: ptr(month.UTC()), PaidTo: ptr(next.UTC())})
	for _, s := range paid {
		o.Month.ReceivedInRiha += s.Total
	}
	for _, d := range debtors {
		if d.DebtCount >= 2 {
			o.Attention = append(o.Attention, Attention{Kind: "debt", Tone: "sun", Member: d.ID, Title: d.Name + " · " + itoa(d.DebtCount) + " meses em dívida", Detail: domain.FormatShort(d.Debt) + " MT"})
		}
		if len(o.Attention) >= 4 {
			break
		}
	}
	if d := len(debtors); d > 0 && len(o.Attention) == 0 {
		o.Attention = append(o.Attention, Attention{Kind: "debt", Tone: "sun", Title: itoa(d) + " em dívida · " + domain.FormatShort(o.Debt) + " MT", Detail: "Lembretes a correr."})
	}
	failed, _ := a.Messages.Failed(ctx, m.ID, live, now.AddDate(0, 0, -14), 50)
	seen := map[string]bool{}
	for _, f := range failed {
		if !seen[f.To] {
			seen[f.To] = true
		}
	}
	if len(seen) > 0 {
		var first domain.Message
		for _, f := range failed {
			first = f
			break
		}
		title := itoa(len(seen)) + " número sem entrega"
		if len(seen) > 1 {
			title = itoa(len(seen)) + " números sem entrega"
		}
		o.Attention = append(o.Attention, Attention{Kind: "delivery", Tone: "red", Member: first.MemberID, Title: title, Detail: chanName(first.Channel) + " falhou para " + domain.FormatPhone(first.To)})
	}
	invites, _ := a.Members.Find(ctx, ports.MemberQuery{MerchantID: m.ID, Live: live, Statuses: []string{domain.SubInvited}, Limit: 500})
	old := 0
	for _, i := range invites {
		if now.Sub(i.CreatedAt) > 72*time.Hour {
			old++
		}
	}
	if old > 0 {
		t := itoa(old) + " convites por aceitar há mais de 3 dias"
		if old == 1 {
			t = "1 convite por aceitar há mais de 3 dias"
		}
		o.Attention = append(o.Attention, Attention{Kind: "invites", Tone: "grey", Title: t, Detail: "resend"})
	}
	sched, _ := a.Charges.Find(ctx, ports.ChargeQuery{MerchantID: m.ID, Live: &live, Statuses: []string{domain.ChargeScheduled}, DueFrom: ptr(now), DueTo: ptr(now.AddDate(0, 0, 40)), Limit: 2000})
	o.Upcoming = a.upcoming(ctx, subs, sched, plans, now)
	return o
}

func chanName(c string) string {
	switch c {
	case domain.ChannelWhatsApp:
		return "WhatsApp"
	case domain.ChannelEmail:
		return "Email"
	}
	return "SMS"
}

func (a *App) upcoming(ctx context.Context, subs []domain.Subscription, sched []domain.Charge, plans map[string]*domain.Plan, now time.Time) []Upcoming {
	type group struct {
		u     Upcoming
		plans map[string]bool
		at    time.Time
	}
	groups := map[string]*group{}
	horizon := now.AddDate(0, 0, 40)
	add := func(planID string, due time.Time, amt int64) {
		p := planOf(plans, a, ctx, planID)
		var key, when, label string
		if p.Anchor == domain.AnchorSignup {
			mo := monthStart(due)
			key, when, label = "s"+mo.Format("2006-01"), "ao longo de "+domain.ShortDate(mo)[2:], "aniversário"
		} else {
			key, when, label = "f"+domain.PeriodKey(due), domain.ShortDate(due)+" 09:00", "dia fixo"
		}
		g := groups[key]
		if g == nil {
			g = &group{u: Upcoming{When: when, Label: label}, plans: map[string]bool{}, at: due}
			groups[key] = g
		}
		g.u.Count++
		g.u.Total += amt
		g.plans[p.Name] = true
	}
	for _, c := range sched {
		add(c.PlanID, c.DueAt, c.Amount)
	}
	for _, s := range subs {
		if s.Status != domain.SubActive && s.Status != domain.SubPastDue || s.NextDueAt == nil || s.NextDueAt.After(horizon) {
			continue
		}
		amt := s.Amount
		if s.PendingFrom != nil && !s.NextDueAt.Before(*s.PendingFrom) {
			amt = s.PendingAmount
		}
		add(s.PlanID, *s.NextDueAt, amt)
	}
	var gs []*group
	for _, g := range groups {
		names := make([]string, 0, len(g.plans))
		for n := range g.plans {
			names = append(names, n)
		}
		sort.Strings(names)
		if len(names) == 1 {
			g.u.Detail = names[0]
		} else {
			g.u.Detail = itoa(len(names)) + " planos"
		}
		g.u.Label = itoa(g.u.Count) + " links · " + g.u.Label
		gs = append(gs, g)
	}
	sort.Slice(gs, func(i, j int) bool { return gs[i].at.Before(gs[j].at) })
	out := []Upcoming{}
	for _, g := range gs {
		out = append(out, g.u)
	}
	var off []string
	cur := monthStart(now)
	for k := 1; k <= 4; k++ {
		mo := cur.AddDate(0, k, 0).Month()
		on := false
		for _, p := range plans {
			if p.Status == domain.PlanActive && p.MonthOn(mo) {
				on = true
			}
		}
		if on {
			if len(off) > 0 {
				break
			}
			continue
		}
		off = append(off, domain.ShortDate(time.Date(2000, mo, 1, 0, 0, 0, 0, domain.Maputo))[2:])
	}
	if len(off) > 0 && len(plans) > 0 {
		w := off[0]
		if len(off) > 1 {
			w = off[0] + " · " + off[len(off)-1]
		}
		out = append(out, Upcoming{When: w, Label: "Sem cobranças", Detail: "meses desligados", Empty: true})
	}
	return out
}

func ptr[T any](v T) *T { return &v }

func (a *App) MemberList(ctx context.Context, q ports.MemberQuery) ([]domain.Member, int64) {
	n, _ := a.Members.Count(ctx, q)
	ms, _ := a.Members.Find(ctx, q)
	return ms, n
}

func (a *App) MemberCounts(ctx context.Context, merchantID string, live bool) map[string]int64 {
	c, _ := a.Members.CountByStatus(ctx, merchantID, live)
	return c
}

func (a *App) MemberFieldValues(ctx context.Context, merchantID string, live bool, key string) []string {
	v, _ := a.Members.FieldValues(ctx, merchantID, live, key)
	return v
}

type MemberDetail struct {
	Member   *domain.Member
	Subs     []domain.Subscription
	Plans    map[string]*domain.Plan
	Charges  []domain.Charge
	Messages []domain.Message
}

func (a *App) MemberDetailFor(ctx context.Context, merchantID string, live bool, id string) (*MemberDetail, error) {
	mem, err := a.Member(ctx, merchantID, live, id)
	if err != nil {
		return nil, err
	}
	d := &MemberDetail{Member: mem, Plans: map[string]*domain.Plan{}}
	d.Subs, _ = a.Subs.ByMember(ctx, mem.ID)
	for _, s := range d.Subs {
		planOf(d.Plans, a, ctx, s.PlanID)
	}
	d.Charges, _ = a.Charges.Find(ctx, ports.ChargeQuery{MemberID: mem.ID, OrderBy: "due_at DESC", Limit: 60})
	d.Messages, _ = a.Messages.ByMember(ctx, mem.ID, 40)
	return d, nil
}

func (a *App) PlanMembers(ctx context.Context, merchantID string, live bool, planID string) []domain.Subscription {
	subs, _ := a.Subs.ByPlan(ctx, planID, append([]string{domain.SubInvited}, domain.LiveSubStatuses...))
	return subs
}

func (a *App) ChargesFor(ctx context.Context, q ports.ChargeQuery) []domain.Charge {
	cs, _ := a.Charges.Find(ctx, q)
	return cs
}

func (a *App) SubscriptionList(ctx context.Context, merchantID string, live bool, memberID string, statuses []string, limit int) []domain.Subscription {
	subs, _ := a.Subs.List(ctx, merchantID, live, memberID, statuses, limit)
	return subs
}

func (a *App) SubscriptionByID(ctx context.Context, merchantID string, live bool, id string) (*domain.Subscription, *domain.Member, error) {
	s, err := a.Subs.Get(ctx, id)
	if err != nil || s.MerchantID != merchantID || s.Livemode != live || s.Status == domain.SubIncomplete {
		return nil, nil, domain.NotFound("A assinatura " + id)
	}
	mem, err := a.Members.Get(ctx, s.MemberID)
	return s, mem, err
}

func (a *App) MemberByID(ctx context.Context, id string) *domain.Member {
	m, err := a.Members.Get(ctx, id)
	if err != nil {
		return nil
	}
	return m
}
