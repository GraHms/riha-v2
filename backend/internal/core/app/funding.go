package app

import (
	"context"
	"fmt"

	"riha/internal/core/domain"
)

// Withdrawals are paid from where the merchant's money is: each live
// withdrawal is split between the partners whose accounts collected it, and
// each pays its part to the merchant's bank. RIHA pays what no partner holds.

// room is what each integration's partner still holds overall and isn't yet
// asked to pay out: it can't be asked for more than it has, whatever one
// merchant's payments through it add up to.
func (a *App) room(ctx context.Context) map[string]int64 {
	out := map[string]int64{}
	all, err := a.Integrations.Collected(ctx, nil, nil)
	if err != nil {
		return out
	}
	for _, r := range all {
		var c Collected
		c.add(r)
		out[r.Integration] += c.DueToRIHA()
	}
	received, _ := a.Integrations.SettledTotals(ctx)
	funding, _ := a.Integrations.FundingTotals(ctx)
	for code := range out {
		out[code] -= received[code] + funding[code][domain.FundingOpen] + funding[code][domain.FundingPaid]
	}
	return out
}

// fund splits a live withdrawal between the sources holding the merchant's money.
func (a *App) fund(ctx context.Context, p *domain.Payout) error {
	if !p.Livemode || a.Integrations == nil {
		return nil
	}
	held, err := a.Integrations.HeldFor(ctx, p.MerchantID)
	if err != nil {
		return err
	}
	room := a.room(ctx)
	for code := range held {
		held[code] = min(held[code], room[code])
	}
	for _, f := range domain.PlanFunding(p.Amount, p.Fee, held) {
		f.ID, f.PayoutID, f.MerchantID, f.CreatedAt = domain.NewID("fnd"), p.ID, p.MerchantID, a.Now()
		if err := a.Integrations.CreateFunding(ctx, &f); err != nil {
			return err
		}
	}
	return nil
}

// fundOpenPayouts splits the live withdrawals still under way from before
// withdrawals were split.
func (a *App) fundOpenPayouts(ctx context.Context) error {
	ps, err := a.Integrations.UnfundedPayouts(ctx, domain.OpenPayoutStatuses)
	if err != nil {
		return err
	}
	for i := range ps {
		if err := a.Tx.Do(ctx, func(ctx context.Context) error { return a.fund(ctx, &ps[i]) }); err != nil {
			return err
		}
	}
	return nil
}

// cancelFunding ends the open parts of a withdrawal that won't be paid.
func (a *App) cancelFunding(ctx context.Context, payoutID string) error {
	if a.Integrations == nil {
		return nil
	}
	fs, err := a.Integrations.FundingOf(ctx, []string{payoutID})
	if err != nil {
		return err
	}
	for i := range fs {
		if fs[i].Status == domain.FundingOpen {
			fs[i].Status = domain.FundingCancelled
			if err := a.Integrations.SaveFunding(ctx, &fs[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

// payFundingOf marks a withdrawal's open parts paid with one receipt: those
// of source, or with "" all of them.
func (a *App) payFundingOf(ctx context.Context, payoutID string, r savedReceipt, source, actor string) error {
	if a.Integrations == nil {
		return nil
	}
	fs, err := a.Integrations.FundingOf(ctx, []string{payoutID})
	if err != nil {
		return err
	}
	now := a.Now()
	for i := range fs {
		f := &fs[i]
		if f.Status != domain.FundingOpen || (source != "" && f.Source != source) {
			continue
		}
		f.Status, f.PaidAt, f.PaidBy = domain.FundingPaid, &now, actor
		f.ReceiptPath, f.ReceiptName, f.ReceiptType, f.ReceiptSize, f.ReceiptHash = r.Path, r.Name, r.Type, r.Size, r.Hash
		if err := a.Integrations.SaveFunding(ctx, f); err != nil {
			return err
		}
	}
	return nil
}

// FundingRow is one part of a withdrawal, with what its payer needs to send it.
type FundingRow struct {
	domain.PayoutFunding
	Payout       domain.Payout
	MerchantName string
	SourceName   string // the integration's partner, or RIHA
}

// PayoutFunding gives each withdrawal's parts, by withdrawal id.
func (a *App) PayoutFunding(ctx context.Context, ids []string) map[string][]domain.PayoutFunding {
	out := map[string][]domain.PayoutFunding{}
	if a.Integrations == nil {
		return out
	}
	fs, _ := a.Integrations.FundingOf(ctx, ids)
	for _, f := range fs {
		out[f.PayoutID] = append(out[f.PayoutID], f)
	}
	return out
}

// PartnerFundingQuery picks a partner's parts of withdrawals: those it is
// waiting for RIHA to approve, those to pay now, or those paid.
type PartnerFundingQuery struct {
	Partner string
	Tab     string // waiting, to_pay, paid
}

func (a *App) integrationsOf(ctx context.Context, partner string) []domain.Integration {
	out := []domain.Integration{}
	list, _ := a.Integrations.List(ctx)
	for _, in := range list {
		if in.PartnerCode == partner && in.Code != domain.IntegrationSimulator {
			out = append(out, in)
		}
	}
	return out
}

func (a *App) PartnerFunding(ctx context.Context, q PartnerFundingQuery) ([]FundingRow, error) {
	codes := []string{}
	for _, in := range a.integrationsOf(ctx, q.Partner) {
		codes = append(codes, in.Code)
	}
	if len(codes) == 0 {
		return []FundingRow{}, nil
	}
	statuses := []string{domain.FundingOpen}
	if q.Tab == "paid" {
		statuses = []string{domain.FundingPaid}
	}
	fs, err := a.Integrations.FundingFrom(ctx, codes, statuses, 200)
	if err != nil {
		return nil, err
	}
	return a.fundingRows(ctx, fs, func(p *domain.Payout) bool {
		switch q.Tab {
		case "waiting":
			return p.Pending()
		case "to_pay":
			return p.Status == domain.PayoutSent
		}
		return true
	}), nil
}

func (a *App) fundingRows(ctx context.Context, fs []domain.PayoutFunding, keep func(*domain.Payout) bool) []FundingRow {
	byCode := a.SourceNames(ctx)
	merchants := map[string]string{}
	out := []FundingRow{}
	for _, f := range fs {
		p, err := a.Payouts.Lock(ctx, f.PayoutID)
		if err != nil || !keep(p) {
			continue
		}
		if _, ok := merchants[f.MerchantID]; !ok {
			if m, err := a.Merchants.ByID(ctx, f.MerchantID); err == nil {
				merchants[f.MerchantID] = m.Name
			}
		}
		out = append(out, FundingRow{PayoutFunding: f, Payout: *p, MerchantName: merchants[f.MerchantID], SourceName: byCode[f.Source]})
	}
	return out
}

// PayFunding marks one part of a withdrawal paid, with the transfer's receipt
// and reference. A partner pays only its own parts, once RIHA approved the
// withdrawal; when every part is paid, the withdrawal is, and the merchant is told.
func (a *App) PayFunding(ctx context.Context, id, partner, reference string, up Upload, actor string) (*domain.PayoutFunding, error) {
	ref := domain.Trim(reference, 120)
	if ref == "" {
		return nil, domain.Invalid("reference_required", "reference", "Indique a referência da transferência.")
	}
	f, err := a.Integrations.LockFunding(ctx, id)
	if err != nil {
		return nil, notFound(err, "O levantamento")
	}
	if partner != "" {
		mine := false
		for _, in := range a.integrationsOf(ctx, partner) {
			mine = mine || in.Code == f.Source
		}
		if !mine {
			return nil, domain.NotFound("O levantamento")
		}
	}
	p, err := a.Payouts.Lock(ctx, f.PayoutID)
	if err != nil {
		return nil, err
	}
	if f.Status != domain.FundingOpen {
		return nil, domain.Invalid("payout_state", "status", "Esta parte do levantamento já não está por pagar.")
	}
	if p.Status != domain.PayoutSent {
		return nil, domain.Invalid("payout_state", "status", fmt.Sprintf("%s ainda não foi aprovado pela RIHA.", p.Ref()))
	}
	r, err := a.saveReceipt(ctx, up)
	if err != nil {
		return nil, err
	}
	var all []domain.PayoutFunding
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		f, err = a.Integrations.LockFunding(ctx, id)
		if err != nil {
			return err
		}
		if f.Status != domain.FundingOpen {
			return domain.Invalid("payout_state", "status", "Esta parte do levantamento já não está por pagar.")
		}
		now := a.Now()
		f.Status, f.PaidAt, f.PaidBy, f.Reference = domain.FundingPaid, &now, actor, ref
		f.ReceiptPath, f.ReceiptName, f.ReceiptType, f.ReceiptSize, f.ReceiptHash = r.Path, r.Name, r.Type, r.Size, r.Hash
		if err := a.Integrations.SaveFunding(ctx, f); err != nil {
			return err
		}
		all, err = a.Integrations.FundingOf(ctx, []string{p.ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, p.MerchantID, Actor{ID: actor, Role: map[bool]string{true: domain.RolePartner, false: domain.RoleAdmin}[partner != ""]}, "payout.part_paid", p.ID,
		fmt.Sprintf("%s: %s MT por %s · %s", p.Ref(), domain.FormatShort(f.Send), f.Source, ref))
	for _, x := range all {
		if x.Status == domain.FundingOpen {
			return f, nil
		}
	}
	// The last part is paid: so is the withdrawal, with this receipt if it has none.
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		locked, err := a.Payouts.Lock(ctx, p.ID)
		if err != nil {
			return err
		}
		if !locked.HasReceipt() {
			locked.ReceiptPath, locked.ReceiptName, locked.ReceiptType, locked.ReceiptSize, locked.ReceiptHash = r.Path, r.Name, r.Type, r.Size, r.Hash
			return a.Payouts.Save(ctx, locked)
		}
		return nil
	})
	if err != nil {
		return f, err
	}
	if _, err := a.TransitionPayout(ctx, p.ID, domain.ActPaid, "", actor); err != nil {
		return f, err
	}
	return f, nil
}

// FundingReceipt is the receipt of a paid part, for its partner or for staff.
func (a *App) FundingReceipt(ctx context.Context, id, partner string) (*domain.Attachment, error) {
	f, err := a.Integrations.LockFunding(ctx, id)
	if err != nil || !f.HasReceipt() {
		return nil, domain.NotFound("O comprovativo")
	}
	if partner != "" {
		mine := false
		for _, in := range a.integrationsOf(ctx, partner) {
			mine = mine || in.Code == f.Source
		}
		if !mine {
			return nil, domain.NotFound("O comprovativo")
		}
	}
	data, err := a.Files.Read(ctx, f.ReceiptPath)
	if err != nil {
		return nil, domain.NotFound("O comprovativo")
	}
	return &domain.Attachment{Name: f.ReceiptName, ContentType: f.ReceiptType, Data: data}, nil
}

// SourceNames names who pays from each source: an integration's partner, or RIHA.
func (a *App) SourceNames(ctx context.Context) map[string]string {
	names := a.partnerNames(ctx)
	out := map[string]string{domain.FundRIHA: "RIHA"}
	list, _ := a.Integrations.List(ctx)
	for _, in := range list {
		out[in.Code] = names[in.PartnerCode]
	}
	return out
}
