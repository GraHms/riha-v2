package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// SetupIntegrations adds the integrations RIHA starts with, if missing, and
// divides the fee of each payment from before payments recorded it.
func (a *App) SetupIntegrations(ctx context.Context) error {
	if a.Integrations == nil {
		return nil
	}
	defaults := domain.DefaultIntegrations()
	for i := range defaults {
		defaults[i].UpdatedAt = a.Now()
	}
	partners := domain.DefaultPartners()
	for i := range partners {
		partners[i].CreatedAt = a.Now()
	}
	if err := a.Integrations.AddMissingPartners(ctx, partners); err != nil {
		return err
	}
	if err := a.Integrations.AddMissing(ctx, defaults); err != nil {
		return err
	}
	// Integrations from before partners were accounts get theirs.
	list, err := a.Integrations.List(ctx)
	if err != nil {
		return err
	}
	for i := range list {
		if list[i].PartnerCode != "" {
			continue
		}
		list[i].PartnerCode = domain.PartnerRIHA
		for _, d := range defaults {
			if d.Code == list[i].Code {
				list[i].PartnerCode = d.PartnerCode
			}
		}
		if err := a.Integrations.Save(ctx, &list[i]); err != nil {
			return err
		}
	}
	if err := a.fundOpenPayouts(ctx); err != nil {
		return err
	}
	return nil
}

func (a *App) routing() ports.RailRouting {
	r, _ := a.Rail.(ports.RailRouting)
	return r
}

// routeFor names the integration a merchant's payment by method goes through:
// the merchant's route, if it takes the method in this mode, or the default.
func (a *App) routeFor(ctx context.Context, merchantID, method string, live bool) string {
	r := a.routing()
	if r == nil {
		return ""
	}
	want := ""
	if a.Integrations != nil {
		routes, _ := a.Integrations.Routes(ctx, merchantID)
		for _, x := range routes {
			if x.Method == method {
				want = x.Integration
			}
		}
	}
	return r.RouteFor(want, method, live)
}

// priceFor divides a payment's fee on the deal of the integration it went
// through: the merchant's own share if one is set, or the integration's.
// priceFor divides a payment's fee. An error reading the deal is returned,
// never taken for a deal of nothing: the payment stays to be priced again.
func (a *App) priceFor(ctx context.Context, merchantID, code, method string, amount, fee int64) (domain.Price, error) {
	if a.Integrations == nil || code == "" {
		return domain.Price{}, nil
	}
	in, err := a.Integrations.Get(ctx, code)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Price{}, nil
	}
	if err != nil {
		return domain.Price{}, err
	}
	share := in.ShareBps
	deals, err := a.Integrations.Deals(ctx, merchantID)
	if err != nil {
		return domain.Price{}, err
	}
	for _, d := range deals {
		if d.Integration == code {
			share = d.ShareBps
		}
	}
	return domain.PriceOf(amount, fee, in.PlatformBps[method], share), nil
}

// PriceBacklog divides the fee of payments not priced yet: from before
// integrations, or whose pricing failed. It runs in the background, a batch
// at a time, never holding up the start.
func (a *App) PriceBacklog(ctx context.Context) {
	if a.Integrations == nil {
		return
	}
	priced := 0
	for ctx.Err() == nil {
		ps, err := a.Integrations.Unpriced(ctx, 500)
		if err != nil || len(ps) == 0 {
			break
		}
		done := 0
		for i := range ps {
			price, err := a.priceFor(ctx, ps[i].MerchantID, ps[i].Integration, ps[i].Method, ps[i].Amount, ps[i].Fee)
			if err != nil {
				log.Printf("integrations: pricing %s: %v", ps[i].ID, err)
				continue
			}
			if err := a.Integrations.SetPrice(ctx, ps[i].ID, price); err != nil {
				log.Printf("integrations: pricing %s: %v", ps[i].ID, err)
				continue
			}
			done++
		}
		priced += done
		if done == 0 {
			break
		}
	}
	if priced > 0 {
		log.Printf("integrations: divided the fee of %d earlier payments", priced)
	}
}

// IntegrationView is an integration as the backoffice shows it: its deal, the
// methods it takes in live mode and those it's the default for, and how many
// merchants have their own route or deal on it.
type IntegrationView struct {
	domain.Integration
	LiveMethods []string
	DefaultFor  []string
	Routed      int
	Deals       int
}

func (a *App) IntegrationViews(ctx context.Context) ([]IntegrationView, error) {
	list, err := a.Integrations.List(ctx)
	if err != nil {
		return nil, err
	}
	routes := map[string][]string{}
	if r := a.routing(); r != nil {
		routes = r.Routes(true)
	}
	all, _ := a.Integrations.Routes(ctx, "")
	deals, _ := a.Integrations.Deals(ctx, "")
	out := make([]IntegrationView, 0, len(list))
	for _, in := range list {
		v := IntegrationView{Integration: in, LiveMethods: []string{}, DefaultFor: []string{}}
		for _, m := range domain.AllMethods {
			for i, code := range routes[m] {
				if code == in.Code {
					v.LiveMethods = append(v.LiveMethods, m)
					if i == 0 {
						v.DefaultFor = append(v.DefaultFor, m)
					}
				}
			}
		}
		seen := map[string]bool{}
		for _, x := range all {
			if x.Integration == in.Code && !seen[x.MerchantID] {
				seen[x.MerchantID] = true
				v.Routed++
			}
		}
		for _, d := range deals {
			if d.Integration == in.Code {
				v.Deals++
			}
		}
		out = append(out, v)
	}
	return out, nil
}

// IntegrationEdit changes an integration's description and deal; nil leaves a
// field as it is.
type IntegrationEdit struct {
	Name, PartnerCode, Account, Note *string
	PlatformBps                      map[string]int
	ShareBps                         *int
}

func bpsOK(bps int, max int) bool { return bps >= 0 && bps <= max }

func (a *App) EditIntegration(ctx context.Context, code string, e IntegrationEdit, actor Actor) (*domain.Integration, error) {
	in, err := a.Integrations.Get(ctx, code)
	if err != nil {
		return nil, notFound(err, "A integração")
	}
	before := describeDeal(in)
	set := func(dst *string, v *string, field string, max int) error {
		if v == nil {
			return nil
		}
		s := strings.TrimSpace(*v)
		if len(s) > max || (s == "" && field != "note") {
			return domain.Invalid("parameter_invalid", field, "Valor inválido.")
		}
		*dst = s
		return nil
	}
	for _, f := range []struct {
		dst   *string
		v     *string
		field string
		max   int
	}{{&in.Name, e.Name, "name", 80}, {&in.Account, e.Account, "account", 200}, {&in.Note, e.Note, "note", 500}} {
		if err := set(f.dst, f.v, f.field, f.max); err != nil {
			return nil, err
		}
	}
	if e.PartnerCode != nil && *e.PartnerCode != in.PartnerCode {
		if _, err := a.Integrations.Partner(ctx, *e.PartnerCode); err != nil {
			return nil, domain.Invalid("partner_invalid", "partner_code", "Escolha um parceiro da lista.")
		}
		a.Record(ctx, "", actor, "integration.partner", code, in.PartnerCode+" → "+*e.PartnerCode)
		in.PartnerCode = *e.PartnerCode
	}
	if e.PlatformBps != nil {
		platform := map[string]int{}
		for m, bps := range e.PlatformBps {
			if !domain.Contains(domain.AllMethods, m) || !bpsOK(bps, 2000) {
				return nil, domain.Invalid("platform_fee_invalid", "platform_bps", "A taxa do operador tem de estar entre 0% e 20%.")
			}
			if bps > 0 {
				platform[m] = bps
			}
		}
		in.PlatformBps = platform
	}
	if e.ShareBps != nil {
		if !bpsOK(*e.ShareBps, 10000) {
			return nil, domain.Invalid("share_invalid", "share_bps", "A parte do parceiro tem de estar entre 0% e 100%.")
		}
		in.ShareBps = *e.ShareBps
	}
	in.UpdatedAt = a.Now()
	if err := a.Integrations.Save(ctx, in); err != nil {
		return nil, err
	}
	if after := describeDeal(in); after != before {
		a.Record(ctx, "", actor, "integration.deal", code, before+" → "+after)
	} else {
		a.Record(ctx, "", actor, "integration.changed", code, in.Name+" · "+in.PartnerCode+" · "+in.Account)
	}
	return in, nil
}

func describeDeal(in *domain.Integration) string {
	parts := []string{}
	for _, m := range domain.AllMethods {
		if bps := in.PlatformBps[m]; bps > 0 {
			parts = append(parts, domain.MethodNames[m]+" "+domain.FormatBps(bps))
		}
	}
	platform := "sem taxa do operador"
	if len(parts) > 0 {
		platform = "operador " + strings.Join(parts, ", ")
	}
	return fmt.Sprintf("%s ao parceiro, %s", domain.FormatBps(in.ShareBps), platform)
}

// MerchantRouting is, for one merchant, each method's integrations in live
// mode (the default first), the one it goes through, and the share of each
// integration's partner on its payments.
type MerchantRouting struct {
	Methods []MethodRoute
	Deals   []DealView
}

type MethodRoute struct {
	Method  string
	Options []string
	Default string
	Chosen  string // the merchant's own route, if any
	Via     string // where its next payment goes
}

type DealView struct {
	Integration string
	Name        string
	DefaultBps  int
	ShareBps    *int // the merchant's own, if any
}

func (a *App) MerchantRouting(ctx context.Context, merchantID string) (*MerchantRouting, error) {
	if _, err := a.Merchants.ByID(ctx, merchantID); err != nil {
		return nil, notFound(err, "O comerciante")
	}
	out := &MerchantRouting{Methods: []MethodRoute{}, Deals: []DealView{}}
	routes := map[string][]string{}
	if r := a.routing(); r != nil {
		routes = r.Routes(true)
	}
	own, _ := a.Integrations.Routes(ctx, merchantID)
	chosen := map[string]string{}
	for _, x := range own {
		chosen[x.Method] = x.Integration
	}
	for _, m := range domain.AllMethods {
		if len(routes[m]) == 0 {
			continue
		}
		out.Methods = append(out.Methods, MethodRoute{Method: m, Options: routes[m], Default: routes[m][0], Chosen: chosen[m], Via: a.routeFor(ctx, merchantID, m, true)})
	}
	list, _ := a.Integrations.List(ctx)
	deals, _ := a.Integrations.Deals(ctx, merchantID)
	for _, in := range list {
		if in.Code == domain.IntegrationSimulator {
			continue
		}
		v := DealView{Integration: in.Code, Name: in.Name, DefaultBps: in.ShareBps}
		for _, d := range deals {
			if d.Integration == in.Code {
				bps := d.ShareBps
				v.ShareBps = &bps
			}
		}
		out.Deals = append(out.Deals, v)
	}
	return out, nil
}

// SetMerchantRoute sends a merchant's live payments by method through an
// integration, or with "" back to the default. It applies from the next
// payment; those under way end where they began.
func (a *App) SetMerchantRoute(ctx context.Context, merchantID, method, code string, actor Actor) error {
	m, err := a.Merchants.ByID(ctx, merchantID)
	if err != nil {
		return notFound(err, "O comerciante")
	}
	r := a.routing()
	if r == nil || !domain.Contains(domain.AllMethods, method) {
		return domain.Invalid("parameter_invalid", "method", "Método inválido.")
	}
	old := a.routeFor(ctx, m.ID, method, true)
	if code == "" {
		if err := a.Integrations.DeleteRoute(ctx, m.ID, method); err != nil {
			return err
		}
	} else {
		if r.RouteFor(code, method, true) != code {
			return domain.Invalid("integration_unavailable", "integration", "Esta integração não recebe "+domain.MethodNames[method]+" neste servidor.")
		}
		if err := a.Integrations.SetRoute(ctx, &domain.MerchantRoute{MerchantID: m.ID, Method: method, Integration: code, UpdatedBy: actor.ID, UpdatedAt: a.Now()}); err != nil {
			return err
		}
	}
	a.Record(ctx, m.ID, actor, "route.changed", method, fmt.Sprintf("%s: %s → %s", domain.MethodNames[method], old, a.routeFor(ctx, m.ID, method, true)))
	return nil
}

// SetMerchantDeal gives a merchant's payments through an integration its own
// partner share, or with nil the integration's. It applies from the next
// payment; those already made keep the division they were made with.
func (a *App) SetMerchantDeal(ctx context.Context, merchantID, code string, shareBps *int, actor Actor) error {
	m, err := a.Merchants.ByID(ctx, merchantID)
	if err != nil {
		return notFound(err, "O comerciante")
	}
	in, err := a.Integrations.Get(ctx, code)
	if err != nil {
		return notFound(err, "A integração")
	}
	if shareBps == nil {
		if err := a.Integrations.DeleteDeal(ctx, m.ID, code); err != nil {
			return err
		}
		a.Record(ctx, m.ID, actor, "deal.changed", code, in.Name+": a parte do parceiro volta a "+domain.FormatBps(in.ShareBps))
		return nil
	}
	if !bpsOK(*shareBps, 10000) {
		return domain.Invalid("share_invalid", "share_bps", "A parte do parceiro tem de estar entre 0% e 100%.")
	}
	if err := a.Integrations.SetDeal(ctx, &domain.MerchantDeal{MerchantID: m.ID, Integration: code, ShareBps: *shareBps, UpdatedBy: actor.ID, UpdatedAt: a.Now()}); err != nil {
		return err
	}
	a.Record(ctx, m.ID, actor, "deal.changed", code, in.Name+": a parte do parceiro passa a "+domain.FormatBps(*shareBps))
	return nil
}

// SettlementInput is money a partner paid RIHA, as the admin records it.
type SettlementInput struct {
	Integration string
	Amount      int64
	Reference   string
	Note        string
	ReceivedOn  string // YYYY-MM-DD
}

func (a *App) RecordSettlement(ctx context.Context, in SettlementInput, actor Actor) (*domain.PartnerSettlement, error) {
	if _, err := a.Integrations.Get(ctx, in.Integration); err != nil {
		return nil, notFound(err, "A integração")
	}
	if in.Amount == 0 || in.Amount > 1_000_000_000_00 || in.Amount < -1_000_000_000_00 {
		return nil, domain.Invalid("amount_invalid", "amount", "Indique o valor recebido.")
	}
	day, err := time.Parse("2006-01-02", in.ReceivedOn)
	if err != nil || day.After(a.Now()) {
		return nil, domain.Invalid("date_invalid", "received_on", "Indique a data em que o dinheiro chegou.")
	}
	ref, note := domain.Trim(in.Reference, 120), domain.Trim(in.Note, 500)
	if ref == "" {
		return nil, domain.Invalid("reference_required", "reference", "Indique a referência da transferência.")
	}
	s := &domain.PartnerSettlement{ID: domain.NewID("stl"), Integration: in.Integration, Amount: in.Amount, Reference: ref, Note: note,
		ReceivedOn: day, CreatedBy: actor.ID, CreatedAt: a.Now()}
	if err := a.Integrations.AddSettlement(ctx, s); err != nil {
		return nil, err
	}
	a.Record(ctx, "", actor, "settlement.recorded", s.ID, fmt.Sprintf("%s: %s MT · %s · %s", in.Integration, domain.FormatShort(s.Amount), ref, in.ReceivedOn))
	return s, nil
}

func (a *App) DeleteSettlement(ctx context.Context, id string, actor Actor) error {
	s, err := a.Integrations.Settlement(ctx, id)
	if err != nil {
		return notFound(err, "O recebimento")
	}
	if err := a.Integrations.DeleteSettlement(ctx, id); err != nil {
		return err
	}
	a.Record(ctx, "", actor, "settlement.deleted", id, fmt.Sprintf("%s: %s MT · %s · %s", s.Integration, domain.FormatShort(s.Amount), s.Reference, s.ReceivedOn.Format("2006-01-02")))
	return nil
}

// Collected sums payments through one integration, overall or for one merchant.
type Collected struct {
	Count       int64
	Amount      int64 // what customers paid
	Fee         int64 // RIHA's fee on it
	PlatformFee int64 // the operator's charge
	PartnerFee  int64 // the partner's share of RIHA's fee
	Refunded    int64
}

func (c *Collected) add(r ports.CollectedRow) {
	c.Count += r.Count
	c.Amount += r.Amount
	c.Fee += r.Fee
	c.PlatformFee += r.PlatformFee
	c.PartnerFee += r.PartnerFee
	c.Refunded += r.Refunded
}

// Margin is what RIHA keeps of its fee.
func (c Collected) Margin() int64 { return c.Fee - c.PlatformFee - c.PartnerFee }

// DueToRIHA is what the partner's account received and owes RIHA: what was
// paid, less the operator's charge, the partner's share and what went back to
// customers.
func (c Collected) DueToRIHA() int64 { return c.Amount - c.PlatformFee - c.PartnerFee - c.Refunded }

type MerchantCollected struct {
	MerchantID string
	Name       string
	Collected
}

// PartnerPosition is one integration in the settlement report: all it has
// collected and what its partner still holds for RIHA, and the period's
// payments by merchant.
type PartnerPosition struct {
	Integration domain.Integration
	PartnerName string
	AllTime     Collected
	Received    int64 // paid to RIHA so far
	PaidOut     int64 // paid to merchants, for their withdrawals
	Reserved    int64 // withdrawals it is to pay, still held
	Held        int64 // still with the partner, the reserved included
	LastPaid    *domain.PartnerSettlement
	Period      Collected
	Merchants   []MerchantCollected
}

// SettlementReport is where live money is: with partners, at RIHA, and owed to
// merchants.
type SettlementReport struct {
	From, To    *time.Time
	Partners    []PartnerPosition
	Merchants   ports.MerchantMoney
	OwedToShops int64 // merchants' balances and withdrawals under way
	HeldOutside int64 // with partners
	AtRIHA      int64 // received from partners, less withdrawals RIHA paid itself
	RIHAOwn     int64 // what of it all is RIHA's: held and received, less what is owed
	Period      Collected
	Unassigned  Collected // paid payments no integration claims
	Recent      []domain.PartnerSettlement
}

func (a *App) Settlement(ctx context.Context, from, to *time.Time) (*SettlementReport, error) {
	list, err := a.Integrations.List(ctx)
	if err != nil {
		return nil, err
	}
	all, err := a.Integrations.Collected(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	period := all
	if from != nil || to != nil {
		if period, err = a.Integrations.Collected(ctx, from, to); err != nil {
			return nil, err
		}
	}
	received, err := a.Integrations.SettledTotals(ctx)
	if err != nil {
		return nil, err
	}
	funding, err := a.Integrations.FundingTotals(ctx)
	if err != nil {
		return nil, err
	}
	partnerNames := a.partnerNames(ctx)
	money, err := a.Integrations.MerchantMoney(ctx)
	if err != nil {
		return nil, err
	}
	recent, _ := a.Integrations.Settlements(ctx, "", 30)
	rep := &SettlementReport{From: from, To: to, Merchants: money, Recent: recent, Partners: []PartnerPosition{}}
	ids := []string{}
	for _, r := range period {
		ids = append(ids, r.MerchantID)
	}
	merchants := a.merchantsByID(ctx, ids)
	name := func(id string) string { return merchantName(merchants, id) }
	known := map[string]bool{}
	for _, in := range list {
		known[in.Code] = true
		if in.Code == domain.IntegrationSimulator {
			continue
		}
		p := PartnerPosition{Integration: in, PartnerName: partnerNames[in.PartnerCode], Merchants: []MerchantCollected{}, Received: received[in.Code],
			PaidOut: funding[in.Code][domain.FundingPaid], Reserved: funding[in.Code][domain.FundingOpen] + funding[in.Code][domain.FundingClaimed]}
		for _, r := range all {
			if r.Integration == in.Code {
				p.AllTime.add(r)
			}
		}
		for _, r := range period {
			if r.Integration == in.Code {
				p.Period.add(r)
				mc := MerchantCollected{MerchantID: r.MerchantID, Name: name(r.MerchantID)}
				mc.add(r)
				p.Merchants = append(p.Merchants, mc)
			}
		}
		sort.SliceStable(p.Merchants, func(i, j int) bool { return p.Merchants[i].Amount > p.Merchants[j].Amount })
		p.Held = p.AllTime.DueToRIHA() - p.Received - p.PaidOut
		if last, _ := a.Integrations.Settlements(ctx, in.Code, 1); len(last) > 0 {
			p.LastPaid = &last[0]
		}
		rep.Partners = append(rep.Partners, p)
		rep.HeldOutside += p.Held
		rep.AtRIHA += p.Received
		rep.Period.Count += p.Period.Count
		rep.Period.Amount += p.Period.Amount
		rep.Period.Fee += p.Period.Fee
		rep.Period.PlatformFee += p.Period.PlatformFee
		rep.Period.PartnerFee += p.Period.PartnerFee
		rep.Period.Refunded += p.Period.Refunded
	}
	for _, r := range all {
		if !known[r.Integration] {
			rep.Unassigned.add(r)
		}
	}
	rep.AtRIHA -= money.PaidOut
	rep.OwedToShops = money.Balances + money.InTransit
	rep.RIHAOwn = rep.HeldOutside + rep.AtRIHA - rep.OwedToShops
	return rep, nil
}

func (a *App) partnerNames(ctx context.Context) map[string]string {
	out := map[string]string{}
	ps, _ := a.Integrations.Partners(ctx)
	for _, p := range ps {
		out[p.Code] = p.Name
	}
	return out
}
