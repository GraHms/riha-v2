package app

import (
	"context"
	"sort"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// What a partner sees of RIHA: only its own integrations, their agreement, the
// money they collected and hold, and the withdrawals they pay. Never customers'
// details, nor other partners'.

// PartnerDeal is one merchant's own share on a partner's integration.
type PartnerDeal struct {
	MerchantName string
	ShareBps     int
}

type PartnerIntegration struct {
	Integration domain.Integration
	Methods     []string
	Deals       []PartnerDeal
	AllTime     Collected
	Received    int64
	PaidOut     int64
	Reserved    int64
	Held        int64
}

type PartnerHome struct {
	Partner      domain.Partner
	Integrations []PartnerIntegration
	Held         int64
	Reserved     int64
	ToPay        int64 // withdrawals approved and waiting for the partner
	ToPayCount   int
	Days         []ports.DayRow
	Period       Collected
	Merchants    []MerchantCollected
	Settlements  []domain.PartnerSettlement
}

// PartnerHome is a partner's account for days from to to (inclusive).
func (a *App) PartnerHome(ctx context.Context, code string, from, to time.Time) (*PartnerHome, error) {
	p, err := a.Integrations.Partner(ctx, code)
	if err != nil {
		return nil, notFound(err, "O parceiro")
	}
	ins := a.integrationsOf(ctx, code)
	h := &PartnerHome{Partner: *p, Integrations: []PartnerIntegration{}, Days: []ports.DayRow{}, Merchants: []MerchantCollected{}, Settlements: []domain.PartnerSettlement{}}
	if len(ins) == 0 {
		return h, nil
	}
	codes := []string{}
	mine := map[string]bool{}
	for _, in := range ins {
		codes = append(codes, in.Code)
		mine[in.Code] = true
	}
	all, err := a.Integrations.Collected(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	end := to.AddDate(0, 0, 1)
	period, err := a.Integrations.Collected(ctx, &from, &end)
	if err != nil {
		return nil, err
	}
	received, _ := a.Integrations.SettledTotals(ctx)
	funding, _ := a.Integrations.FundingTotals(ctx)
	routes := map[string][]string{}
	if r := a.routing(); r != nil {
		routes = r.Routes(true)
	}
	deals, _ := a.Integrations.Deals(ctx, "")
	names := map[string]string{}
	name := func(id string) string {
		if n, ok := names[id]; ok {
			return n
		}
		n := ""
		if m, err := a.Merchants.ByID(ctx, id); err == nil {
			n = m.Name
		}
		names[id] = n
		return n
	}
	for _, in := range ins {
		pi := PartnerIntegration{Integration: in, Methods: []string{}, Deals: []PartnerDeal{}, Received: received[in.Code],
			PaidOut: funding[in.Code][domain.FundingPaid], Reserved: funding[in.Code][domain.FundingOpen]}
		for _, m := range domain.AllMethods {
			if domain.Contains(routes[m], in.Code) {
				pi.Methods = append(pi.Methods, m)
			}
		}
		for _, d := range deals {
			if d.Integration == in.Code {
				pi.Deals = append(pi.Deals, PartnerDeal{MerchantName: name(d.MerchantID), ShareBps: d.ShareBps})
			}
		}
		for _, r := range all {
			if r.Integration == in.Code {
				pi.AllTime.add(r)
			}
		}
		pi.Held = pi.AllTime.DueToRIHA() - pi.Received - pi.PaidOut
		h.Held += pi.Held
		h.Reserved += pi.Reserved
		h.Integrations = append(h.Integrations, pi)
	}
	byMerchant := map[string]*MerchantCollected{}
	for _, r := range period {
		if !mine[r.Integration] {
			continue
		}
		h.Period.add(r)
		mc := byMerchant[r.MerchantID]
		if mc == nil {
			mc = &MerchantCollected{MerchantID: r.MerchantID, Name: name(r.MerchantID)}
			byMerchant[r.MerchantID] = mc
		}
		mc.add(r)
	}
	for _, mc := range byMerchant {
		h.Merchants = append(h.Merchants, *mc)
	}
	sort.Slice(h.Merchants, func(i, j int) bool { return h.Merchants[i].Amount > h.Merchants[j].Amount })
	if h.Days, err = a.Integrations.CollectedDaily(ctx, codes, from, end); err != nil {
		return nil, err
	}
	toPay, _ := a.PartnerFunding(ctx, PartnerFundingQuery{Partner: code, Tab: "to_pay"})
	for _, f := range toPay {
		h.ToPay += f.Send
		h.ToPayCount++
	}
	for _, c := range codes {
		ss, _ := a.Integrations.Settlements(ctx, c, 20)
		h.Settlements = append(h.Settlements, ss...)
	}
	sort.Slice(h.Settlements, func(i, j int) bool { return h.Settlements[i].ReceivedOn.After(h.Settlements[j].ReceivedOn) })
	return h, nil
}
