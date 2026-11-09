package app

import (
	"context"
	"sort"
	"strings"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// The member's page, at /m/<member token>: their subscriptions, what they paid and
// what failed, a way to pay what's due and to cancel. The token is the member's
// own, sent only to them, so the page needs no login.

type PortalView struct {
	Merchant *domain.Merchant
	Member   *domain.Member
	Subs     []PortalSub
	// Charges are the member's payments, newest first, without the cancelled ones.
	Charges []domain.Charge
	Plans   map[string]*domain.Plan
	// Due is what the member owes now, and PayToken the invoice that takes it.
	Due      int64
	PayToken string
}

type PortalSub struct {
	Sub  domain.Subscription
	Plan *domain.Plan
	Debt int64
}

func (a *App) memberLink(mem *domain.Member) string {
	return strings.TrimRight(a.PublicURL, "/") + "/m/" + mem.Token
}

func (a *App) MemberPortal(ctx context.Context, token string) (*PortalView, error) {
	if len(token) < 16 {
		return nil, domain.NotFound("Este link")
	}
	mem, err := a.Members.ByToken(ctx, token)
	if err != nil {
		return nil, domain.NotFound("Este link")
	}
	m, err := a.Merchants.ByID(ctx, mem.MerchantID)
	if err != nil {
		return nil, err
	}
	v := &PortalView{Merchant: m, Member: mem, Plans: map[string]*domain.Plan{}}
	subs, err := a.Subs.ByMember(ctx, mem.ID)
	if err != nil {
		return nil, err
	}
	charges, err := a.Charges.Find(ctx, ports.ChargeQuery{MemberID: mem.ID})
	if err != nil {
		return nil, err
	}
	debt := map[string]int64{}
	var oldest *domain.Charge
	for i := range charges {
		c := &charges[i]
		if c.Status == domain.ChargeCanceled {
			continue
		}
		v.Charges = append(v.Charges, *c)
		if c.Unpaid() {
			debt[c.SubscriptionID] += c.Amount
			v.Due += c.Amount
			if oldest == nil || c.DueAt.Before(oldest.DueAt) {
				oldest = c
			}
		}
		planOf(v.Plans, a, ctx, c.PlanID)
	}
	if oldest != nil {
		v.PayToken = oldest.Token
	}
	sort.SliceStable(v.Charges, func(i, j int) bool { return v.Charges[i].DueAt.After(v.Charges[j].DueAt) })
	if len(v.Charges) > 36 {
		v.Charges = v.Charges[:36]
	}
	for _, s := range subs {
		// A signup that was never finished isn't a subscription the member has.
		if s.Status == domain.SubCanceled && s.ActivatedAt == nil {
			continue
		}
		v.Subs = append(v.Subs, PortalSub{Sub: s, Plan: planOf(v.Plans, a, ctx, s.PlanID), Debt: debt[s.ID]})
	}
	sort.SliceStable(v.Subs, func(i, j int) bool {
		return !domain.Closed(v.Subs[i].Sub.Status) && domain.Closed(v.Subs[j].Sub.Status)
	})
	return v, nil
}

// CancelByMember is the member cancelling one of their own subscriptions from their
// page. Future charges stop; what is already overdue stays owed.
func (a *App) CancelByMember(ctx context.Context, token, subID, ip string) error {
	mem, err := a.Members.ByToken(ctx, token)
	if err != nil || len(token) < 16 {
		return domain.NotFound("Este link")
	}
	var merchantID string
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		s, err := a.Subs.Lock(ctx, subID)
		if err != nil || s.MemberID != mem.ID {
			return domain.NotFound("A inscrição")
		}
		if domain.Closed(s.Status) {
			return nil
		}
		m, err := a.Merchants.ByID(ctx, s.MerchantID)
		if err != nil {
			return err
		}
		merchantID = m.ID
		return a.cancelSub(ctx, m, s, mem, "cancelada pelo cliente")
	})
	if err == nil && merchantID != "" {
		a.Record(ctx, merchantID, Actor{ID: "member:" + mem.ID, Role: "member", IP: ip}, "subscription.cancel", subID, "cancelada pelo cliente na sua página")
		a.SendSoon(ctx)
	}
	return err
}
