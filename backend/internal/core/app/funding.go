package app

import (
	"context"
	"fmt"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// Withdrawals are paid from where the merchant's money is: each live
// withdrawal is split between the partners whose accounts collected it, and
// each pays its part to the merchant's bank. RIHA pays what no partner holds.

// room is what each integration's partner still holds overall and isn't yet
// asked to pay out: it can't be asked for more than it has, whatever one
// merchant's payments through it add up to.
func (a *App) room(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	all, err := a.Integrations.Collected(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	for _, r := range all {
		var c Collected
		c.add(r)
		out[r.Integration] += c.DueToRIHA()
	}
	received, err := a.Integrations.SettledTotals(ctx)
	if err != nil {
		return nil, err
	}
	funding, err := a.Integrations.FundingTotals(ctx)
	if err != nil {
		return nil, err
	}
	for code := range out {
		f := funding[code]
		out[code] -= received[code] + f[domain.FundingOpen] + f[domain.FundingClaimed] + f[domain.FundingPaid]
	}
	return out, nil
}

// fund splits a live withdrawal between the sources holding the merchant's
// money. It runs in the withdrawal's transaction, one withdrawal at a time,
// so two merchants withdrawing at once never share a partner's room.
func (a *App) fund(ctx context.Context, p *domain.Payout) error {
	if !p.Livemode || a.Integrations == nil {
		return nil
	}
	if err := a.Integrations.LockRoom(ctx); err != nil {
		return err
	}
	held, err := a.Integrations.HeldFor(ctx, p.MerchantID)
	if err != nil {
		return err
	}
	room, err := a.room(ctx)
	if err != nil {
		return err
	}
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

// cancelFunding ends the parts of a withdrawal that won't be paid, with the
// withdrawal locked. A withdrawal with a part already sent to the merchant's
// bank can't be undone: giving the merchant the whole amount back would pay
// that part twice.
func (a *App) cancelFunding(ctx context.Context, p *domain.Payout) error {
	if a.Integrations == nil {
		return nil
	}
	fs, err := a.Integrations.FundingOf(ctx, []string{p.ID})
	if err != nil {
		return err
	}
	names := map[string]string{}
	for i := range fs {
		if fs[i].Sent() {
			if len(names) == 0 {
				names = a.SourceNames(ctx)
			}
			who := names[fs[i].Source]
			if who == "" {
				who = fs[i].Source
			}
			return domain.Invalid("payout_part_sent", "status", fmt.Sprintf("%s já transferiu %s MT de %s para o comerciante. Recuse esse pagamento, se não aconteceu, antes de anular o levantamento.",
				who, domain.FormatShort(fs[i].Send), p.Ref()))
		}
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

// fundingDone tells whether a withdrawal's parts are all paid and confirmed.
// One without parts (test mode, or from before the split) is done.
func (a *App) fundingDone(ctx context.Context, payoutID string) (bool, error) {
	if a.Integrations == nil {
		return true, nil
	}
	fs, err := a.Integrations.FundingOf(ctx, []string{payoutID})
	if err != nil {
		return false, err
	}
	for _, f := range fs {
		if f.Status == domain.FundingOpen || f.Status == domain.FundingClaimed {
			return false, nil
		}
	}
	return true, nil
}

// finishIfPaid closes a withdrawal, in the caller's transaction with it
// locked, once its last part is confirmed: with this receipt if it has none.
func (a *App) finishIfPaid(ctx context.Context, p *domain.Payout, r savedReceipt, actor string) error {
	done, err := a.fundingDone(ctx, p.ID)
	if err != nil || !done {
		return err
	}
	if !p.HasReceipt() {
		p.ReceiptPath, p.ReceiptName, p.ReceiptType, p.ReceiptSize, p.ReceiptHash = r.Path, r.Name, r.Type, r.Size, r.Hash
		if err := a.Payouts.Save(ctx, p); err != nil {
			return err
		}
	}
	_, err = a.TransitionPayout(ctx, p.ID, domain.ActPaid, "", actor)
	return err
}

func receiptOf(f *domain.PayoutFunding) savedReceipt {
	return savedReceipt{Path: f.ReceiptPath, Name: f.ReceiptName, Type: f.ReceiptType, Size: f.ReceiptSize, Hash: f.ReceiptHash}
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
	// The withdrawal's state is asked of the database, so a long list of one
	// kind never hides another.
	statuses, payouts, limit := []string{domain.FundingOpen}, []string{domain.PayoutSent}, 1000
	switch q.Tab {
	case "waiting":
		payouts = domain.PendingPayoutStatuses
	case "paid":
		statuses, payouts, limit = []string{domain.FundingClaimed, domain.FundingPaid}, nil, 300
	}
	fs, err := a.Integrations.FundingFrom(ctx, codes, statuses, payouts, limit)
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
	ids := make([]string, 0, len(fs))
	for _, f := range fs {
		ids = append(ids, f.PayoutID)
	}
	payouts := map[string]*domain.Payout{}
	if len(ids) > 0 {
		ps, _ := a.Payouts.Find(ctx, ports.PayoutQuery{IDs: ids})
		for i := range ps {
			payouts[ps[i].ID] = &ps[i]
		}
	}
	mids := make([]string, 0, len(fs))
	for _, f := range fs {
		mids = append(mids, f.MerchantID)
	}
	merchants := a.merchantsByID(ctx, mids)
	out := []FundingRow{}
	for _, f := range fs {
		p := payouts[f.PayoutID]
		if p == nil || !keep(p) {
			continue
		}
		out = append(out, FundingRow{PayoutFunding: f, Payout: *p, MerchantName: merchantName(merchants, f.MerchantID), SourceName: byCode[f.Source]})
	}
	return out
}

// partnerPays tells whether source is one of partner's integrations.
func (a *App) partnerPays(ctx context.Context, partner, source string) bool {
	for _, in := range a.integrationsOf(ctx, partner) {
		if in.Code == source {
			return true
		}
	}
	return false
}

// PayFunding records one part of a withdrawal paid, with the transfer's
// receipt and reference, once RIHA approved the withdrawal. A partner pays
// only its own parts, and its word makes the part claimed until RIHA confirms
// the transfer; staff recording it make it paid. When every part is paid, so
// is the withdrawal, and the merchant is told.
func (a *App) PayFunding(ctx context.Context, id, partner, reference string, up Upload, actor string) (*domain.PayoutFunding, error) {
	ref := domain.Trim(reference, 120)
	if ref == "" {
		return nil, domain.Invalid("reference_required", "reference", "Indique a referência da transferência.")
	}
	f, err := a.Integrations.LockFunding(ctx, id)
	if err != nil || (partner != "" && !a.partnerPays(ctx, partner, f.Source)) {
		return nil, domain.NotFound("O levantamento")
	}
	notOpen := domain.Invalid("payout_state", "status", "Esta parte do levantamento já não está por pagar.")
	check := func(f *domain.PayoutFunding, p *domain.Payout) error {
		if f.Status != domain.FundingOpen {
			return notOpen
		}
		if p.Status != domain.PayoutSent {
			return domain.Invalid("payout_state", "status", fmt.Sprintf("%s ainda não foi aprovado pela RIHA.", p.Ref()))
		}
		return nil
	}
	p, err := a.Payouts.Lock(ctx, f.PayoutID)
	if err != nil {
		return nil, err
	}
	if err := check(f, p); err != nil {
		return nil, err
	}
	r, err := a.saveReceipt(ctx, up)
	if err != nil {
		return nil, err
	}
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		// The withdrawal first, then its part: whoever changes either holds both.
		if p, err = a.Payouts.Lock(ctx, f.PayoutID); err != nil {
			return err
		}
		if f, err = a.Integrations.LockFunding(ctx, id); err != nil {
			return err
		}
		if err := check(f, p); err != nil {
			return err
		}
		now := a.Now()
		f.PaidAt, f.PaidBy, f.Reference, f.Warning = &now, actor, ref, ""
		f.ReceiptPath, f.ReceiptName, f.ReceiptType, f.ReceiptSize, f.ReceiptHash = r.Path, r.Name, r.Type, r.Size, r.Hash
		if used, err := a.Integrations.ReceiptUsed(ctx, r.Hash, f.ID); err != nil {
			return err
		} else if used {
			f.Warning = "Este comprovativo já foi usado noutro pagamento."
		}
		if partner != "" {
			f.Status = domain.FundingClaimed
		} else {
			f.Status, f.ConfirmedAt, f.ConfirmedBy = domain.FundingPaid, &now, actor
		}
		if err := a.Integrations.SaveFunding(ctx, f); err != nil {
			return err
		}
		return a.finishIfPaid(ctx, p, r, actor)
	})
	if err != nil {
		return nil, err
	}
	who, act := Actor{ID: actor, Role: domain.RoleAdmin}, "payout.part_paid"
	if partner != "" {
		who, act = Actor{ID: actor, Role: domain.RolePartner}, "payout.part_claimed"
	}
	a.Record(ctx, p.MerchantID, who, act, p.ID, fmt.Sprintf("%s: %s MT por %s · %s", p.Ref(), domain.FormatShort(f.Send), f.Source, ref))
	return f, nil
}

// ConfirmFunding is RIHA seeing, on its statement, the transfers partners
// said they made: each part becomes paid, and a withdrawal whose last part
// it was is paid and its merchant told.
func (a *App) ConfirmFunding(ctx context.Context, ids []string, actor string) ([]domain.PayoutFunding, error) {
	if len(ids) == 0 {
		return nil, domain.Invalid("parameter_missing", "ids", "Escolha pelo menos um pagamento.")
	}
	out := []domain.PayoutFunding{}
	for _, id := range ids {
		f0, err := a.Integrations.LockFunding(ctx, id)
		if err != nil {
			return out, domain.NotFound("O pagamento")
		}
		var f *domain.PayoutFunding
		var p *domain.Payout
		err = a.Tx.Do(ctx, func(ctx context.Context) error {
			if p, err = a.Payouts.Lock(ctx, f0.PayoutID); err != nil {
				return err
			}
			if f, err = a.Integrations.LockFunding(ctx, id); err != nil {
				return err
			}
			if f.Status != domain.FundingClaimed {
				return domain.Invalid("payout_state", "status", fmt.Sprintf("O pagamento de %s MT para %s não está por confirmar.", domain.FormatShort(f.Send), p.Ref()))
			}
			now := a.Now()
			f.Status, f.ConfirmedAt, f.ConfirmedBy = domain.FundingPaid, &now, actor
			if err := a.Integrations.SaveFunding(ctx, f); err != nil {
				return err
			}
			return a.finishIfPaid(ctx, p, receiptOf(f), actor)
		})
		if err != nil {
			return out, err
		}
		a.Record(ctx, p.MerchantID, Actor{ID: actor, Role: domain.RoleAdmin}, "payout.part_confirmed", p.ID,
			fmt.Sprintf("%s: %s MT por %s · %s", p.Ref(), domain.FormatShort(f.Send), f.Source, f.Reference))
		out = append(out, *f)
	}
	return out, nil
}

// RejectClaim is RIHA not finding a partner's transfer: the part is open
// again, for the partner to pay, with why.
func (a *App) RejectClaim(ctx context.Context, id, reason, actor string) (*domain.PayoutFunding, error) {
	reason = domain.Trim(reason, 200)
	if reason == "" {
		return nil, domain.Invalid("reason_required", "reason", "Diga ao parceiro porque recusa o pagamento.")
	}
	f0, err := a.Integrations.LockFunding(ctx, id)
	if err != nil {
		return nil, domain.NotFound("O pagamento")
	}
	var f *domain.PayoutFunding
	var p *domain.Payout
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		if p, err = a.Payouts.Lock(ctx, f0.PayoutID); err != nil {
			return err
		}
		if f, err = a.Integrations.LockFunding(ctx, id); err != nil {
			return err
		}
		if f.Status != domain.FundingClaimed {
			return domain.Invalid("payout_state", "status", "Este pagamento não está por confirmar.")
		}
		f.Status, f.PaidAt, f.Warning = domain.FundingOpen, nil, "Recusado pela RIHA: "+reason
		return a.Integrations.SaveFunding(ctx, f)
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, p.MerchantID, Actor{ID: actor, Role: domain.RoleAdmin}, "payout.part_rejected", p.ID,
		fmt.Sprintf("%s: %s MT por %s · %s", p.Ref(), domain.FormatShort(f.Send), f.Source, reason))
	return f, nil
}

// FundingReceipt is the receipt of a paid part, for its partner or for staff.
func (a *App) FundingReceipt(ctx context.Context, id, partner string) (*domain.Attachment, error) {
	f, err := a.Integrations.LockFunding(ctx, id)
	if err != nil || !f.HasReceipt() {
		return nil, domain.NotFound("O comprovativo")
	}
	if partner != "" && !a.partnerPays(ctx, partner, f.Source) {
		return nil, domain.NotFound("O comprovativo")
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
