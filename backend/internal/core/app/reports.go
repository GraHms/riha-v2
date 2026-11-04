package app

import (
	"context"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type PaymentView struct {
	domain.Payment
	LinkName string
	LinkSlug string
}

func (a *App) paymentViews(ctx context.Context, ps []domain.Payment) []PaymentView {
	names := map[string][2]string{}
	out := make([]PaymentView, 0, len(ps))
	for _, p := range ps {
		v := PaymentView{Payment: p}
		if p.PaymentLinkID != nil {
			n, ok := names[*p.PaymentLinkID]
			if !ok {
				if l, err := a.Links.ByID(ctx, *p.PaymentLinkID); err == nil {
					n = [2]string{l.Name, l.Slug}
				}
				names[*p.PaymentLinkID] = n
			}
			v.LinkName, v.LinkSlug = n[0], n[1]
		}
		out = append(out, v)
	}
	return out
}

type Overview struct {
	Available, InTransit, Received, Previous int64
	Days                                     int
	Bars                                     []ports.Bucket
	ByMethod                                 map[string]int64
	Recent                                   []PaymentView
	PendingPayout                            *domain.Payout
	Bank                                     *domain.BankAccount
}

func day(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }

func (a *App) Overview(ctx context.Context, m *domain.Merchant, live bool, days int) (*Overview, error) {
	if days != 7 && days != 30 && days != 365 {
		days = 30
	}
	today := day(a.Now())
	start := today.AddDate(0, 0, -(days - 1))
	prevStart := start.AddDate(0, 0, -days)
	o := &Overview{Days: days, Available: a.Balance(ctx, m.ID, live), InTransit: a.InTransit(ctx, m.ID, live)}
	var err error
	if o.Received, err = a.Reports.SettledVolume(ctx, ports.Scope{MerchantID: m.ID, Live: live, From: &start}); err != nil {
		return nil, err
	}
	o.Previous, _ = a.Reports.SettledVolume(ctx, ports.Scope{MerchantID: m.ID, Live: live, From: &prevStart, To: &start})
	unit := "day"
	if days == 365 {
		unit = "month"
	}
	raw, _ := a.Reports.VolumeBuckets(ctx, ports.Scope{MerchantID: m.ID, Live: live, From: &start}, unit)
	by := map[string]int64{}
	for _, b := range raw {
		by[b.Date.UTC().Format("2006-01-02")] = b.Amount
	}
	if days == 365 {
		first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		for i := 11; i >= 0; i-- {
			t := first.AddDate(0, -i, 0)
			o.Bars = append(o.Bars, ports.Bucket{Date: t, Amount: by[t.Format("2006-01-02")]})
		}
	} else {
		for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
			o.Bars = append(o.Bars, ports.Bucket{Date: d, Amount: by[d.Format("2006-01-02")]})
		}
	}
	o.ByMethod = map[string]int64{"mpesa": 0, "emola": 0, "card": 0}
	bm, _ := a.Reports.VolumeByMethod(ctx, ports.Scope{MerchantID: m.ID, Live: live, From: &start})
	for k, v := range bm {
		o.ByMethod[k] = v
	}
	recent, _ := a.Payments.Find(ctx, ports.PaymentQuery{MerchantID: m.ID, Live: &live, Limit: 5})
	o.Recent = a.paymentViews(ctx, recent)
	open, _ := a.Payouts.Find(ctx, ports.PayoutQuery{MerchantID: m.ID, Live: &live, Statuses: domain.OpenPayoutStatuses, OrderBy: "requested_at", Limit: 1})
	if len(open) > 0 {
		o.PendingPayout = &open[0]
	}
	o.Bank = a.BankAccount(ctx, m.ID)
	return o, nil
}

type PaymentFilter struct {
	Status, Source, Search string
	From, To               *time.Time
	Limit, Offset          int
}

type PaymentPage struct {
	Data   []PaymentView
	Total  int64
	Counts map[string]int64
}

func statusGroup(s string) []string {
	switch s {
	case "paid":
		return domain.SettledStatuses
	case "pending":
		return []string{domain.PaymentProcessing}
	case "failed":
		return []string{domain.PaymentFailed}
	}
	return nil
}

func (a *App) paymentQuery(merchantID string, live bool, f PaymentFilter) ports.PaymentQuery {
	return ports.PaymentQuery{MerchantID: merchantID, Live: &live, Statuses: statusGroup(f.Status), Source: f.Source, Search: f.Search, From: f.From, To: f.To, Limit: f.Limit, Offset: f.Offset}
}

func (a *App) SearchPayments(ctx context.Context, merchantID string, live bool, f PaymentFilter) (*PaymentPage, error) {
	q := a.paymentQuery(merchantID, live, f)
	ps, err := a.Payments.Find(ctx, q)
	if err != nil {
		return nil, err
	}
	total, _ := a.Payments.Count(ctx, q)
	counts := map[string]int64{}
	for _, k := range []string{"all", "paid", "pending", "failed"} {
		counts[k], _ = a.Payments.Count(ctx, ports.PaymentQuery{MerchantID: merchantID, Live: &live, Statuses: statusGroup(k)})
	}
	return &PaymentPage{Data: a.paymentViews(ctx, ps), Total: total, Counts: counts}, nil
}

func (a *App) ExportPayments(ctx context.Context, merchantID string, live bool, f PaymentFilter) ([]domain.Payment, error) {
	f.Limit, f.Offset = 10000, 0
	return a.Payments.Find(ctx, a.paymentQuery(merchantID, live, f))
}

func (a *App) APIPayments(ctx context.Context, merchantID string, live bool, status, startingAfter string, limit int) ([]domain.Payment, error) {
	q := ports.PaymentQuery{MerchantID: merchantID, Live: &live, Limit: limit}
	if status != "" {
		q.Statuses = []string{status}
	}
	if startingAfter != "" {
		if p, err := a.Payments.ByID(ctx, startingAfter); err == nil {
			q.StartingAfter = p
		}
	}
	return a.Payments.Find(ctx, q)
}

func (a *App) MerchantPayment(ctx context.Context, merchantID string, live bool, idOrRef string) (*domain.Payment, error) {
	p, err := a.Payments.ByID(ctx, idOrRef)
	if err != nil {
		p, err = a.Payments.ByReference(ctx, idOrRef)
	}
	if err != nil || p.MerchantID != merchantID || p.Livemode != live {
		return nil, domain.NotFound("O pagamento " + idOrRef)
	}
	return p, nil
}

type PaymentDetail struct {
	PaymentView
	Session       *domain.CheckoutSession
	APIKey        *domain.APIKey
	CreatedByName string
	Link          *domain.PaymentLink
	Deliveries    []domain.Delivery
	Refunds       []domain.Refund
}

func (a *App) PaymentDetail(ctx context.Context, merchantID string, live bool, id string) (*PaymentDetail, error) {
	p, err := a.MerchantPayment(ctx, merchantID, live, id)
	if err != nil {
		return nil, err
	}
	d := &PaymentDetail{PaymentView: a.paymentViews(ctx, []domain.Payment{*p})[0]}
	if cs, err := a.Sessions.ByID(ctx, p.SessionID); err == nil {
		d.Session = cs
	}
	if p.APIKeyID != nil {
		d.APIKey = a.APIKey(ctx, *p.APIKeyID)
	}
	if p.CreatedBy != "" {
		if u, err := a.Users.ByID(ctx, p.CreatedBy); err == nil {
			d.CreatedByName = u.Name
		}
	}
	if p.PaymentLinkID != nil {
		if l, err := a.Links.ByID(ctx, *p.PaymentLinkID); err == nil {
			d.Link = l
		}
	}
	evs, _ := a.Events.ByObjects(ctx, merchantID, []string{p.ID, p.SessionID})
	ids := make([]string, 0, len(evs))
	for _, e := range evs {
		ids = append(ids, e.ID)
	}
	if len(ids) > 0 {
		d.Deliveries, _ = a.Deliveries.ByEvents(ctx, ids)
	}
	d.Refunds, _ = a.Refunds.ByPayment(ctx, p.ID)
	return d, nil
}

type BalanceSummary struct {
	Available, InTransit    int64
	InTransitDue            *time.Time
	PaidPrevCount, PaidPrev int64
	PaidThisCount, PaidThis int64
	CanPayout               bool
}

func (a *App) BalanceSummary(ctx context.Context, m *domain.Merchant, live bool) *BalanceSummary {
	now := a.Now()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	s := &BalanceSummary{Available: a.Balance(ctx, m.ID, live), InTransit: a.InTransit(ctx, m.ID, live), CanPayout: !live || m.Status == domain.MerchantActive}
	s.PaidPrevCount, s.PaidPrev, _ = a.Payouts.PaidBetween(ctx, m.ID, live, month.AddDate(0, -1, 0), month)
	s.PaidThisCount, s.PaidThis, _ = a.Payouts.PaidBetween(ctx, m.ID, live, month, now.Add(time.Hour))
	open, _ := a.Payouts.Find(ctx, ports.PayoutQuery{MerchantID: m.ID, Live: &live, Statuses: domain.OpenPayoutStatuses, OrderBy: "due_at DESC", Limit: 1})
	if len(open) > 0 {
		s.InTransitDue = &open[0].DueAt
	}
	return s
}

type Application struct {
	Merchant  domain.Merchant
	Checks    []domain.KycCheck
	Risk      string
	Documents []domain.Document
	Bank      *domain.BankAccount
}

func (a *App) Application(ctx context.Context, m *domain.Merchant) Application {
	docs := a.DocumentsOf(ctx, m.ID)
	bank := a.BankAccount(ctx, m.ID)
	checks := domain.KycChecks(m, bank, len(docs))
	return Application{Merchant: *m, Checks: checks, Risk: domain.Risk(checks), Documents: docs, Bank: bank}
}

func (a *App) ApprovalQueue(ctx context.Context, statuses []string, limit int) []Application {
	ms, _ := a.Merchants.List(ctx, ports.MerchantFilter{Statuses: statuses, OrderBy: "kyc_submitted_at", Limit: limit})
	out := make([]Application, 0, len(ms))
	for i := range ms {
		out = append(out, a.Application(ctx, &ms[i]))
	}
	return out
}

func (a *App) PendingApprovals(ctx context.Context) int64 {
	n, _ := a.Merchants.Count(ctx, []string{domain.MerchantInReview})
	return n
}

type MerchantRow struct {
	Merchant   domain.Merchant
	Volume30d  int64
	Balance    int64
	MainMethod string
	FeeBps     int
}

func (a *App) MerchantRow(ctx context.Context, m *domain.Merchant) MerchantRow {
	since := a.Now().AddDate(0, 0, -30)
	vol, _ := a.Reports.SettledVolume(ctx, ports.Scope{MerchantID: m.ID, Live: true, From: &since})
	main, _ := a.Reports.MainMethod(ctx, m.ID)
	return MerchantRow{Merchant: *m, Volume30d: vol, Balance: a.Balance(ctx, m.ID, true), MainMethod: main, FeeBps: a.FeeBpsFor(ctx, m)}
}

var statusTabs = map[string][]string{
	"active":    {domain.MerchantActive},
	"review":    {domain.MerchantInReview, domain.MerchantInfoRequested, domain.MerchantOnboarding},
	"suspended": {domain.MerchantSuspended, domain.MerchantRejected},
}

func (a *App) MerchantDirectory(ctx context.Context, tab, query string) ([]MerchantRow, int64, int64) {
	ms, _ := a.Merchants.List(ctx, ports.MerchantFilter{Statuses: statusTabs[tab], Query: query, OrderBy: "created_at DESC", Limit: 200})
	out := make([]MerchantRow, 0, len(ms))
	for i := range ms {
		out = append(out, a.MerchantRow(ctx, &ms[i]))
	}
	total, _ := a.Merchants.Count(ctx, nil)
	active, _ := a.Merchants.Count(ctx, []string{domain.MerchantActive})
	return out, total, active
}

type MerchantDossier struct {
	Application
	Row            MerchantRow
	InTransit      int64
	DefaultFeeBps  int
	RecentPayments []PaymentView
	Payouts        []PayoutRow
	Audit          []domain.AuditEntry
}

func (a *App) MerchantDossier(ctx context.Context, id string) (*MerchantDossier, error) {
	m, err := a.Merchants.ByID(ctx, id)
	if err != nil {
		return nil, notFound(err, "O comerciante")
	}
	live := true
	d := &MerchantDossier{Application: a.Application(ctx, m), Row: a.MerchantRow(ctx, m), InTransit: a.InTransit(ctx, m.ID, true), DefaultFeeBps: a.DefaultFeeBps(ctx)}
	ps, _ := a.Payments.Find(ctx, ports.PaymentQuery{MerchantID: m.ID, Live: &live, Limit: 10})
	d.RecentPayments = a.paymentViews(ctx, ps)
	d.Payouts = a.PayoutRows(ctx, ports.PayoutQuery{MerchantID: m.ID, Live: &live, OrderBy: "requested_at DESC", Limit: 10})
	d.Audit, _ = a.Audit.ByMerchant(ctx, m.ID, 20)
	return d, nil
}

type PayoutRow struct {
	domain.Payout
	MerchantName string
}

func (a *App) PayoutRows(ctx context.Context, q ports.PayoutQuery) []PayoutRow {
	ps, _ := a.Payouts.Find(ctx, q)
	names := map[string]string{}
	out := make([]PayoutRow, 0, len(ps))
	for _, p := range ps {
		n, ok := names[p.MerchantID]
		if !ok {
			if m, err := a.Merchants.ByID(ctx, p.MerchantID); err == nil {
				n = m.Name
			}
			names[p.MerchantID] = n
		}
		out = append(out, PayoutRow{p, n})
	}
	return out
}

type PayoutStats struct {
	PendingCount, PendingTotal, SentCount, SentTotal, Urgent, PaidCount, AllCount int64
}

func (a *App) PayoutStats(ctx context.Context) PayoutStats {
	live := true
	var s PayoutStats
	s.PendingCount, s.PendingTotal, _ = a.Payouts.Sum(ctx, ports.PayoutQuery{Live: &live, Statuses: domain.PendingPayoutStatuses})
	s.SentCount, s.SentTotal, _ = a.Payouts.Sum(ctx, ports.PayoutQuery{Live: &live, Statuses: []string{domain.PayoutSent}})
	soon := a.Now().Add(8 * time.Hour)
	s.Urgent, _, _ = a.Payouts.Sum(ctx, ports.PayoutQuery{Live: &live, Statuses: domain.PendingPayoutStatuses, DueBefore: &soon})
	s.PaidCount, _, _ = a.Payouts.Sum(ctx, ports.PayoutQuery{Live: &live, Statuses: []string{domain.PayoutPaid}})
	s.AllCount, _, _ = a.Payouts.Sum(ctx, ports.PayoutQuery{Live: &live})
	return s
}

func (a *App) PayoutQueue(ctx context.Context, tab string) []PayoutRow {
	live := true
	q := ports.PayoutQuery{Live: &live, Limit: 300}
	switch tab {
	case "", "pending":
		q.Statuses, q.OrderBy = domain.PendingPayoutStatuses, "due_at"
	case "sent":
		q.Statuses, q.OrderBy = []string{domain.PayoutSent}, "sent_at"
	case "paid":
		q.Statuses, q.OrderBy = []string{domain.PayoutPaid}, "paid_at DESC"
	default:
		q.OrderBy = "requested_at DESC"
	}
	return a.PayoutRows(ctx, q)
}

func (a *App) PayoutsForExport(ctx context.Context, ids []string) []domain.Payout {
	live := true
	q := ports.PayoutQuery{Live: &live, IDs: ids, OrderBy: "due_at"}
	if len(ids) == 0 {
		q.Statuses = domain.OpenPayoutStatuses
	}
	ps, _ := a.Payouts.Find(ctx, q)
	return ps
}

type PlatformOverview struct {
	VolumeToday, VolumeWeekAgo, TxToday, TxOkToday, Volume14d int64
	Stack                                                     []map[string]any
	Health                                                    map[string]ports.MethodHealth
	Approvals                                                 []Application
	PendingApprovals                                          int64
	PayoutsDue                                                []PayoutRow
	PayoutStats                                               PayoutStats
}

func (a *App) PlatformOverview(ctx context.Context) *PlatformOverview {
	now := a.Now()
	today := day(now)
	weekAgo := today.AddDate(0, 0, -7)
	weekAgoNow := weekAgo.Add(now.Sub(today))
	o := &PlatformOverview{}
	o.VolumeToday, _ = a.Reports.SettledVolume(ctx, ports.Scope{Live: true, From: &today})
	o.VolumeWeekAgo, _ = a.Reports.SettledVolume(ctx, ports.Scope{Live: true, From: &weekAgo, To: &weekAgoNow})
	o.TxToday, _ = a.Reports.Attempts(ctx, ports.Scope{Live: true, From: &today})
	o.TxOkToday, _ = a.Reports.SettledCount(ctx, ports.Scope{Live: true, From: &today})
	start := today.AddDate(0, 0, -13)
	rows, _ := a.Reports.VolumeByDayAndMethod(ctx, ports.Scope{Live: true, From: &start})
	byDay := map[string]map[string]any{}
	for _, r := range rows {
		k := r.Date.UTC().Format("2006-01-02")
		if byDay[k] == nil {
			byDay[k] = map[string]any{"mpesa": int64(0), "emola": int64(0), "card": int64(0)}
		}
		byDay[k][r.Method] = r.Amount
		o.Volume14d += r.Amount
	}
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		k := d.Format("2006-01-02")
		v := byDay[k]
		if v == nil {
			v = map[string]any{"mpesa": int64(0), "emola": int64(0), "card": int64(0)}
		}
		v["date"] = k
		o.Stack = append(o.Stack, v)
	}
	hs, _ := a.Reports.Health(ctx, now.Add(-24*time.Hour))
	o.Health = map[string]ports.MethodHealth{}
	for _, h := range hs {
		o.Health[h.Method] = h
	}
	o.Approvals = a.ApprovalQueue(ctx, []string{domain.MerchantInReview}, 4)
	o.PendingApprovals = a.PendingApprovals(ctx)
	live := true
	o.PayoutsDue = a.PayoutRows(ctx, ports.PayoutQuery{Live: &live, Statuses: domain.PendingPayoutStatuses, OrderBy: "due_at", Limit: 4})
	o.PayoutStats = a.PayoutStats(ctx)
	return o
}

func (a *App) PlatformPayments(ctx context.Context, ref string) []PaymentView {
	live := true
	if ref != "" {
		p, err := a.Payments.ByID(ctx, ref)
		if err != nil {
			p, err = a.Payments.ByReference(ctx, ref)
		}
		if err != nil {
			return nil
		}
		return a.paymentViews(ctx, []domain.Payment{*p})
	}
	ps, _ := a.Payments.Find(ctx, ports.PaymentQuery{Live: &live, Limit: 100})
	return a.paymentViews(ctx, ps)
}

func (a *App) AuditTrail(ctx context.Context, merchantID string) []domain.AuditEntry {
	e, _ := a.Audit.ByMerchant(ctx, merchantID, 20)
	return e
}
