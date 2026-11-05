package app

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

const IssueAhead = 4 * 24 * time.Hour

func (a *App) BillingTick(ctx context.Context) {
	now := a.Now()
	due, _ := a.Subs.DueForIssue(ctx, now.Add(IssueAhead), 200)
	for _, s := range due {
		if err := a.issueNext(ctx, s.ID, false); err != nil {
			log.Printf("issue %s: %v", s.ID, err)
		}
	}
	paused, _ := a.Subs.PausedUntil(ctx, now, 100)
	for _, s := range paused {
		_ = a.Tx.Do(ctx, func(ctx context.Context) error {
			l, err := a.Subs.Lock(ctx, s.ID)
			if err != nil || l.Status != domain.SubPaused {
				return err
			}
			if err := a.reactivate(ctx, l); err != nil {
				return err
			}
			return a.refreshMember(ctx, l.MemberID)
		})
	}
	stale, _ := a.Subs.StaleIncomplete(ctx, now.Add(-24*time.Hour), 100)
	for _, s := range stale {
		_ = a.Tx.Do(ctx, func(ctx context.Context) error {
			l, err := a.Subs.Lock(ctx, s.ID)
			if err != nil || l.Status != domain.SubIncomplete {
				return err
			}
			l.Status, l.CanceledAt, l.UpdatedAt = domain.SubCanceled, &now, now
			if err := a.Subs.Save(ctx, l); err != nil {
				return err
			}
			if err := a.cancelOpenCharges(ctx, l.ID); err != nil {
				return err
			}
			return a.refreshMember(ctx, l.MemberID)
		})
	}
	active, _ := a.Charges.Active(ctx, now.Add(IssueAhead), 300)
	for _, c := range active {
		if err := a.advanceCharge(ctx, c.ID); err != nil {
			log.Printf("charge %s: %v", c.ID, err)
		}
	}
	a.SendQueued(ctx)
}

func (a *App) issueNext(ctx context.Context, subID string, force bool) error {
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		s, err := a.Subs.Lock(ctx, subID)
		if err != nil || s.NextDueAt == nil {
			return err
		}
		p, err := a.Plans.Get(ctx, s.PlanID)
		if err != nil {
			return err
		}
		now := a.Now()
		due := *s.NextDueAt
		if due.After(now.Add(IssueAhead)) && !force {
			return nil
		}
		key := domain.PeriodKey(due)
		if p.Status == domain.PlanArchived {
			s.NextDueAt = nil
			return a.Subs.Save(ctx, s)
		}
		if ok, _ := a.Charges.Exists(ctx, s.ID, key); !ok {
			c := &domain.Charge{ID: domain.NewID("sch"), MerchantID: s.MerchantID, Livemode: s.Livemode, SubscriptionID: s.ID, MemberID: s.MemberID, PlanID: p.ID,
				Period: key, Label: domain.PeriodLabel(p, due), DueAt: due, Status: domain.ChargeScheduled, Token: domain.Token("", 16), CreatedAt: now, UpdatedAt: now}
			if s.PendingFrom != nil && !due.Before(*s.PendingFrom) {
				s.Amount, s.PlanVersion = s.PendingAmount, s.PendingVersion
				s.PendingAmount, s.PendingVersion, s.PendingFrom = 0, 0, nil
			}
			c.Amount = s.Amount
			switch {
			case domain.Contains(s.SkipPeriods, key):
				c.Status = domain.ChargeSkipped
			case s.Status == domain.SubPaused && s.PausedUntil != nil && due.Before(*s.PausedUntil):
				c.Status = domain.ChargeSkipped
			}
			if err := a.Charges.Create(ctx, c); err != nil {
				return err
			}
		}
		s.NextDueAt = nil
		if next, ok := domain.NextDue(p, s.AnchorAt, due); ok {
			s.NextDueAt = &next
		}
		s.UpdatedAt = now
		if err := a.Subs.Save(ctx, s); err != nil {
			return err
		}
		return a.refreshMember(ctx, s.MemberID)
	})
}

func stepBase(c *domain.Charge) time.Time {
	if c.First && c.CreatedAt.After(c.DueAt) {
		return domain.LocalDate(c.CreatedAt).UTC()
	}
	return c.DueAt
}

func (a *App) advanceCharge(ctx context.Context, id string) error {
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		c, err := a.Charges.Lock(ctx, id)
		if err != nil {
			return err
		}
		if c.Status != domain.ChargeScheduled && !c.Unpaid() {
			return nil
		}
		s, err := a.Subs.Lock(ctx, c.SubscriptionID)
		if err != nil {
			return err
		}
		if s.Status == domain.SubIncomplete || s.Status == domain.SubInvited || domain.Closed(s.Status) {
			return nil
		}
		m, err := a.Merchants.ByID(ctx, c.MerchantID)
		if err != nil {
			return err
		}
		mem, err := a.Members.Get(ctx, c.MemberID)
		if err != nil {
			return err
		}
		p, err := a.Plans.Get(ctx, c.PlanID)
		if err != nil {
			return err
		}
		set := a.SubSettingsFor(ctx, m)
		now := a.Now()
		var pending []domain.ReminderStep
		for _, st := range domain.ReminderSteps(set.ToleranceDays) {
			if !c.Done(st.Key) && !domain.StepAt(stepBase(c), st).After(now) {
				pending = append(pending, st)
			}
		}
		if len(pending) == 0 {
			return nil
		}
		if domain.QuietHours(now) {
			return nil
		}
		for i, st := range pending {
			speak := i == len(pending)-1 && set.StepOn(st.Key)
			if err := a.runStep(ctx, m, set, p, s, mem, c, st, speak); err != nil {
				return err
			}
			c.Steps = append(c.Steps, st.Key)
			if c.Status != domain.ChargeScheduled && !c.Unpaid() {
				break
			}
		}
		c.UpdatedAt = now
		if err := a.Charges.Save(ctx, c); err != nil {
			return err
		}
		if err := a.Subs.Save(ctx, s); err != nil {
			return err
		}
		return a.refreshMember(ctx, mem.ID)
	})
}

func (a *App) memberUnpaid(ctx context.Context, memberID string) []domain.Charge {
	cs, _ := a.Charges.Find(ctx, ports.ChargeQuery{MemberID: memberID, Statuses: domain.UnpaidChargeStatuses})
	return cs
}

func (a *App) runStep(ctx context.Context, m *domain.Merchant, set domain.SubscriptionSettings, p *domain.Plan, s *domain.Subscription, mem *domain.Member, c *domain.Charge, st domain.ReminderStep, speak bool) error {
	now := a.Now()
	link := a.chargeLink(c)
	who := firstName(mem.Name)
	say := func(kind, body string) error {
		if !speak {
			return nil
		}
		return a.queueMessage(ctx, m, mem, c.ID, kind, body)
	}
	switch st.Key {
	case domain.StepBefore:
		if c.Status != domain.ChargeScheduled {
			return nil
		}
		return say("reminder", fmt.Sprintf("%s: %s de %s (%s MT) vence a %s. Enviamos o link nesse dia.", m.Name, strings.ToLower(c.Label), who, domain.FormatShort(c.Amount), domain.ShortDate(c.DueAt)))
	case domain.StepLink:
		if c.Status == domain.ChargeScheduled {
			c.Status, c.LinkSentAt = domain.ChargeOpen, &now
		}
		var debt int64
		for _, u := range a.memberUnpaid(ctx, mem.ID) {
			if u.ID != c.ID && u.Status == domain.ChargePastDue {
				debt += u.Amount
			}
		}
		body := fmt.Sprintf("%s: %s de %s, %s MT. Pague aqui: %s", m.Name, strings.ToLower(c.Label), who, domain.FormatShort(c.Amount), link)
		if debt > 0 {
			body = fmt.Sprintf("%s: %s de %s, %s MT, e %s MT em dívida. Pague aqui: %s", m.Name, strings.ToLower(c.Label), who, domain.FormatShort(c.Amount), domain.FormatShort(debt), link)
		}
		return say("link", body)
	case domain.StepEvening:
		if c.OpenedAt != nil || !c.Unpaid() {
			return nil
		}
		return say("reminder", fmt.Sprintf("%s: lembrete, %s de %s (%s MT) vence hoje. %s", m.Name, strings.ToLower(c.Label), who, domain.FormatShort(c.Amount), link))
	case domain.StepResend1:
		if !c.Unpaid() {
			return nil
		}
		if c.Status == domain.ChargeOpen {
			c.Status = domain.ChargePastDue
			if s.Status == domain.SubActive {
				s.Status = domain.SubPastDue
			}
			if err := a.emit(ctx, m.ID, c.Livemode, "subscription_charge.past_due", c.ID, domain.ChargeResource(c, a.PublicURL), ""); err != nil {
				return err
			}
		}
		return say("resend", fmt.Sprintf("%s: %s de %s (%s MT) está em atraso. Pague aqui: %s", m.Name, strings.ToLower(c.Label), who, domain.FormatShort(c.Amount), link))
	case domain.StepResend2:
		if !c.Unpaid() || !speak {
			return nil
		}
		orig := mem.Channel
		mem.Channel = a.secondChannel(ctx, m, mem)
		defer func() { mem.Channel = orig }()
		var total int64
		for _, u := range a.memberUnpaid(ctx, mem.ID) {
			total += u.Amount
		}
		return a.queueMessage(ctx, m, mem, c.ID, "resend", fmt.Sprintf("%s: %s tem %s MT por pagar. Pague aqui: %s", m.Name, who, domain.FormatShort(total), link))
	case domain.StepLast:
		if !c.Unpaid() {
			return nil
		}
		end := domain.ShortDate(domain.StepAt(stepBase(c), domain.ReminderStep{Days: set.ToleranceDays, Hour: 9}))
		what := map[string]string{domain.FinalSuspend: "a assinatura fica suspensa", domain.FinalCancel: "a assinatura é cancelada", domain.FinalKeep: "o valor continua em dívida"}[set.FinalAction]
		if p.Missed == domain.MissedForfeit {
			what = "perde " + strings.ToLower(c.Label)
		}
		return say("last_warning", fmt.Sprintf("%s: último aviso. Se %s de %s não for pago até %s, %s. %s", m.Name, strings.ToLower(c.Label), who, end, what, link))
	case domain.StepFinal:
		if !c.Unpaid() {
			return nil
		}
		if p.Missed == domain.MissedForfeit {
			c.Status = domain.ChargeForfeited
			others := 0
			for _, u := range a.memberUnpaid(ctx, mem.ID) {
				if u.ID != c.ID && u.SubscriptionID == s.ID && u.Status == domain.ChargePastDue {
					others++
				}
			}
			if others == 0 && s.Status == domain.SubPastDue {
				s.Status = domain.SubActive
			}
			return say("final", fmt.Sprintf("%s: %s de %s não foi pago e não será cobrado. O serviço fica suspenso nesse período.", m.Name, strings.ToLower(c.Label), who))
		}
		switch set.FinalAction {
		case domain.FinalSuspend:
			if s.Status != domain.SubSuspended {
				s.Status = domain.SubSuspended
				if err := a.emit(ctx, m.ID, s.Livemode, "subscription.updated", s.ID, domain.SubscriptionResource(s, mem), ""); err != nil {
					return err
				}
			}
			return say("final", fmt.Sprintf("%s: a assinatura de %s está suspensa por falta de pagamento. Pague para reativar: %s", m.Name, who, link))
		case domain.FinalCancel:
			s.Status, s.CanceledAt, s.NextDueAt = domain.SubCanceled, &now, nil
			if err := a.cancelScheduled(ctx, s.ID); err != nil {
				return err
			}
			if err := a.emit(ctx, m.ID, s.Livemode, "subscription.canceled", s.ID, domain.SubscriptionResource(s, mem), ""); err != nil {
				return err
			}
			return say("final", fmt.Sprintf("%s: a assinatura de %s foi cancelada por falta de pagamento.", m.Name, who))
		}
	}
	return nil
}

func (a *App) SendQueued(ctx context.Context) {
	for i := 0; i < 5; i++ {
		msgs, err := a.Messages.Queued(ctx, 50)
		if err != nil || len(msgs) == 0 {
			return
		}
		for _, msg := range msgs {
			err := a.Messenger.Send(ctx, ports.OutboundMessage{Channel: msg.Channel, To: msg.To, Sender: msg.Sender, Body: msg.Body})
			msg.Status = "sent"
			if err != nil {
				msg.Status, msg.Error = "failed", domain.Trim(err.Error(), 200)
			}
			_ = a.Messages.Save(ctx, &msg)
		}
	}
}

func (a *App) SendNextNow(ctx context.Context, m *domain.Merchant, live bool, subID string, actor Actor) error {
	if live {
		return domain.Forbidden("test_mode_only", "Só disponível em modo de teste.")
	}
	s, err := a.Subs.Get(ctx, subID)
	if err != nil || s.MerchantID != m.ID || s.Livemode {
		return domain.NotFound("A assinatura")
	}
	if domain.Closed(s.Status) || s.Status == domain.SubIncomplete || s.Status == domain.SubInvited || s.Status == domain.SubSuspended {
		return domain.Invalid("subscription_state", "subscription", "Esta assinatura não está ativa.")
	}
	sched, _ := a.Charges.Find(ctx, ports.ChargeQuery{SubscriptionIDs: []string{s.ID}, Statuses: []string{domain.ChargeScheduled}, Limit: 1})
	if len(sched) == 0 {
		if s.NextDueAt == nil {
			return domain.Invalid("nothing_due", "subscription", "Não há próximas cobranças.")
		}
		if err := a.issueNext(ctx, s.ID, true); err != nil {
			return err
		}
		sched, _ = a.Charges.Find(ctx, ports.ChargeQuery{SubscriptionIDs: []string{s.ID}, Statuses: []string{domain.ChargeScheduled}, Limit: 1})
		if len(sched) == 0 {
			return domain.Invalid("nothing_due", "subscription", "A próxima cobrança foi saltada.")
		}
	}
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		c, err := a.Charges.Lock(ctx, sched[0].ID)
		if err != nil {
			return err
		}
		mem, _ := a.Members.Get(ctx, c.MemberID)
		now := a.Now()
		c.Status, c.LinkSentAt, c.UpdatedAt = domain.ChargeOpen, &now, now
		c.Steps = append(c.Steps, domain.StepBefore)
		if err := a.Charges.Save(ctx, c); err != nil {
			return err
		}
		if err := a.queueMessage(ctx, m, mem, c.ID, "link", fmt.Sprintf("%s: %s de %s, %s MT. Pague aqui: %s", m.Name, strings.ToLower(c.Label), firstName(mem.Name), domain.FormatShort(c.Amount), a.chargeLink(c))); err != nil {
			return err
		}
		return a.refreshMember(ctx, mem.ID)
	})
	if err == nil {
		a.Record(ctx, m.ID, actor, "charge.send_now", subID, "teste")
		a.SendQueued(ctx)
	}
	return err
}

type InvoiceLine struct {
	Charge       domain.Charge
	Plan         *domain.Plan
	Subscription *domain.Subscription
}

type InvoiceView struct {
	Merchant     *domain.Merchant
	Member       *domain.Member
	Charge       *domain.Charge
	Lines        []InvoiceLine
	Total        int64
	NeedsConsent bool
	Agreements   []string
	Payment      *domain.Payment
	Paid         []InvoiceLine
	NextDue      *time.Time
	NextAmount   int64
	Skipped      []string
	Subs         []domain.Subscription
	Plans        map[string]*domain.Plan
}

func (a *App) Invoice(ctx context.Context, token string, touch bool) (*InvoiceView, error) {
	if len(token) < 12 {
		return nil, domain.NotFound("Este link")
	}
	c, err := a.Charges.ByToken(ctx, token)
	if err != nil || c.Status == domain.ChargeCanceled {
		return nil, domain.NotFound("Este link")
	}
	m, err := a.Merchants.ByID(ctx, c.MerchantID)
	if err != nil {
		return nil, err
	}
	mem, err := a.Members.Get(ctx, c.MemberID)
	if err != nil {
		return nil, err
	}
	if touch && c.OpenedAt == nil && c.Unpaid() {
		now := a.Now()
		c.OpenedAt = &now
		_ = a.Charges.Save(ctx, c)
	}
	v := &InvoiceView{Merchant: m, Member: mem, Charge: c, Plans: map[string]*domain.Plan{}}
	subs, _ := a.Subs.ByMember(ctx, mem.ID)
	v.Subs = subs
	subByID := map[string]*domain.Subscription{}
	for i := range subs {
		subByID[subs[i].ID] = &subs[i]
	}
	unpaid := a.memberUnpaid(ctx, mem.ID)
	for _, u := range unpaid {
		s := subByID[u.SubscriptionID]
		if s == nil || domain.Closed(s.Status) {
			continue
		}
		if (s.Status == domain.SubIncomplete || s.Status == domain.SubInvited) && u.ID != c.ID && !(u.First && c.First) {
			continue
		}
		v.Lines = append(v.Lines, InvoiceLine{Charge: u, Plan: planOf(v.Plans, a, ctx, u.PlanID), Subscription: s})
		v.Total += u.Amount
		if s.ConsentAt == nil {
			v.NeedsConsent = true
			v.Agreements = append(v.Agreements, domain.Agreement(m.Name, planOf(v.Plans, a, ctx, u.PlanID), u.Amount, mem.Phone, mem.Channel, s.AnchorAt))
		}
	}
	sort.SliceStable(v.Lines, func(i, j int) bool {
		if v.Lines[i].Charge.ID == c.ID {
			return true
		}
		if v.Lines[j].Charge.ID == c.ID {
			return false
		}
		return v.Lines[i].Charge.DueAt.After(v.Lines[j].Charge.DueAt)
	})
	if c.SessionID != nil {
		v.Payment = a.LatestPayment(ctx, *c.SessionID)
	}
	if c.Settled() {
		if s := subByID[c.SubscriptionID]; s != nil {
			v.Paid = append(v.Paid, InvoiceLine{Charge: *c, Plan: planOf(v.Plans, a, ctx, c.PlanID), Subscription: s})
			if c.SessionID != nil {
				same, _ := a.Charges.Find(ctx, ports.ChargeQuery{SessionID: *c.SessionID})
				for _, o := range same {
					if o.ID != c.ID {
						v.Paid = append(v.Paid, InvoiceLine{Charge: o, Plan: planOf(v.Plans, a, ctx, o.PlanID), Subscription: subByID[o.SubscriptionID]})
					}
				}
			}
			if s.NextDueAt != nil && s.Status != domain.SubCanceled {
				d := *s.NextDueAt
				v.NextDue, v.NextAmount = &d, s.Amount
				if s.PendingFrom != nil && !d.Before(*s.PendingFrom) {
					v.NextAmount = s.PendingAmount
				}
				p := planOf(v.Plans, a, ctx, s.PlanID)
				last := c.DueAt
				for k := last.AddDate(0, 1, 0); k.Before(d) && p.Interval == domain.IntervalMonth; k = k.AddDate(0, 1, 0) {
					v.Skipped = append(v.Skipped, domain.MonthName(k.In(domain.Maputo).Month()))
				}
			}
		}
	}
	return v, nil
}

type InvoicePay struct {
	ChargeIDs  []string `json:"charge_ids"`
	Method     string   `json:"method"`
	Phone      string   `json:"phone"`
	UseLast    bool     `json:"use_last"`
	CardNumber string   `json:"card_number"`
	CardExpiry string   `json:"card_expiry"`
	CardCVC    string   `json:"card_cvc"`
	Accept     bool     `json:"accept"`
	IP         string   `json:"-"`
	UA         string   `json:"-"`
}

func (a *App) PayInvoice(ctx context.Context, token string, in InvoicePay) (*domain.Payment, error) {
	v, err := a.Invoice(ctx, token, false)
	if err != nil {
		return nil, err
	}
	if e := domain.RequireApproved(v.Merchant); e != nil {
		return nil, domain.Forbidden("merchant_unavailable", "Este comerciante não pode receber pagamentos agora.")
	}
	if len(v.Lines) == 0 {
		return nil, domain.Invalid("nothing_due", "charge", "Não há nada por pagar neste link.")
	}
	chosen := map[string]bool{}
	for _, id := range in.ChargeIDs {
		chosen[id] = true
	}
	if len(chosen) == 0 {
		for _, l := range v.Lines {
			chosen[l.Charge.ID] = true
		}
	}
	var lines []InvoiceLine
	var total int64
	for _, l := range v.Lines {
		if chosen[l.Charge.ID] {
			lines = append(lines, l)
			total += l.Charge.Amount
		}
	}
	if len(lines) != len(chosen) {
		return nil, domain.Invalid("charge_invalid", "charge_ids", "Escolha cobranças deste link.")
	}
	for _, l := range lines {
		for _, o := range v.Lines {
			if o.Subscription.ID == l.Subscription.ID && o.Charge.DueAt.Before(l.Charge.DueAt) && !chosen[o.Charge.ID] {
				return nil, domain.Invalid("pay_oldest_first", "charge_ids", "Pague primeiro "+strings.ToLower(o.Charge.Label)+".")
			}
		}
	}
	needsConsent := false
	for _, l := range lines {
		if l.Subscription.ConsentAt == nil {
			needsConsent = true
		}
	}
	if needsConsent && !in.Accept {
		return nil, domain.Invalid("agreement_required", "accept", "Aceite o acordo de pagamento para continuar.")
	}
	phone := in.Phone
	if in.UseLast || phone == "" {
		phone = domain.FirstNonEmpty(v.Member.LastPhone, v.Member.Phone)
	}
	in.Method = strings.ToLower(in.Method)
	if in.Method == "" {
		in.Method = domain.FirstNonEmpty(v.Member.LastMethod, domain.WalletFor(phone))
	}
	var labels []string
	ids := []string{}
	for _, l := range lines {
		labels = append(labels, l.Plan.Name+" · "+l.Charge.Label)
		ids = append(ids, l.Charge.ID)
	}
	cs, err := a.CreateSession(ctx, v.Merchant, v.Member.Livemode, SessionParams{
		Amount: total, Description: domain.Trim(strings.Join(labels, " + "), 300), PaymentMethodTypes: domain.AllMethods,
		ClientReferenceID: v.Member.ID, Customer: &Customer{Name: v.Member.Name, Phone: "+258" + domain.NormalizeMZPhone(phone)},
		Metadata: map[string]any{"subscription_charges": strings.Join(ids, ",")},
		Origin:   domain.Origin{SourceType: domain.SourceSubscription, IP: in.IP, UserAgent: in.UA, Channel: v.Member.Channel, CreatedBy: "subscription"},
	})
	if err != nil {
		return nil, err
	}
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		now := a.Now()
		for _, l := range lines {
			c, err := a.Charges.Lock(ctx, l.Charge.ID)
			if err != nil {
				return err
			}
			if !c.Unpaid() {
				return domain.Invalid("charge_state", "charge_ids", c.Label+" já foi pago.")
			}
			if c.SessionID != nil {
				if p := a.LatestPayment(ctx, *c.SessionID); p != nil && p.Status == domain.PaymentProcessing {
					return domain.Conflict("payment_in_progress", "Já existe um pagamento em curso. Confirme no telemóvel ou aguarde.")
				}
			}
			c.SessionID, c.UpdatedAt = &cs.ID, now
			if err := a.Charges.Save(ctx, c); err != nil {
				return err
			}
			if needsConsent {
				s, err := a.Subs.Lock(ctx, c.SubscriptionID)
				if err != nil {
					return err
				}
				if s.ConsentAt == nil {
					s.ConsentText = domain.Agreement(v.Merchant.Name, l.Plan, c.Amount, v.Member.Phone, v.Member.Channel, s.AnchorAt)
					s.ConsentAt, s.ConsentIP, s.UpdatedAt = &now, domain.Trim(in.IP, 64), now
					if err := a.Subs.Save(ctx, s); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return a.Pay(ctx, cs.ID, PayInput{Method: in.Method, Phone: phone, CardNumber: in.CardNumber, CardExpiry: in.CardExpiry, CardCVC: in.CardCVC,
		Name: v.Member.PayerName, PayerIP: in.IP, PayerUA: in.UA})
}

func (a *App) settleCharges(ctx context.Context, cs *domain.CheckoutSession, p *domain.Payment) error {
	charges, err := a.Charges.Find(ctx, ports.ChargeQuery{SessionID: cs.ID})
	if err != nil {
		return err
	}
	now := a.Now()
	members := map[string]bool{}
	for _, c := range charges {
		l, err := a.Charges.Lock(ctx, c.ID)
		if err != nil {
			return err
		}
		if !l.Unpaid() {
			continue
		}
		l.Status, l.PaidAt, l.PaymentID, l.Method, l.LastFailure, l.UpdatedAt = domain.ChargePaid, &now, &p.ID, p.Method, "", now
		if err := a.Charges.Save(ctx, l); err != nil {
			return err
		}
		s, err := a.Subs.Lock(ctx, l.SubscriptionID)
		if err != nil {
			return err
		}
		mem, err := a.Members.Get(ctx, l.MemberID)
		if err != nil {
			return err
		}
		wasNew := s.Status == domain.SubIncomplete || s.Status == domain.SubInvited
		if !domain.Closed(s.Status) {
			if wasNew {
				s.Status = domain.SubActive
			}
			if err := a.reactivate(ctx, s); err != nil {
				return err
			}
		}
		if err := a.emit(ctx, l.MerchantID, l.Livemode, "subscription_charge.paid", l.ID, domain.ChargeResource(l, a.PublicURL), ""); err != nil {
			return err
		}
		if wasNew {
			if err := a.emit(ctx, l.MerchantID, l.Livemode, "subscription.created", s.ID, domain.SubscriptionResource(s, mem), ""); err != nil {
				return err
			}
		}
		members[l.MemberID] = true
	}
	for id := range members {
		mem, err := a.Members.Lock(ctx, id)
		if err != nil {
			return err
		}
		mem.LastMethod = p.Method
		if p.Method != domain.MethodCard {
			mem.LastPhone = domain.NormalizeMZPhone(cs.CustomerPhone)
		}
		if err := a.Members.Save(ctx, mem); err != nil {
			return err
		}
		if err := a.refreshMember(ctx, id); err != nil {
			return err
		}
		m, err := a.Merchants.ByID(ctx, mem.MerchantID)
		if err != nil {
			return err
		}
		if a.SubSettingsFor(ctx, m).ReceiptSMS {
			if err := a.queueMessage(ctx, m, mem, "", "receipt", fmt.Sprintf("%s: recebemos %s MT de %s. Ref. %s. Obrigado.", m.Name, domain.FormatShort(p.Amount), mem.Name, p.Reference)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *App) chargesFailed(ctx context.Context, cs *domain.CheckoutSession, p *domain.Payment) error {
	charges, err := a.Charges.Find(ctx, ports.ChargeQuery{SessionID: cs.ID})
	if err != nil {
		return err
	}
	label := domain.MethodNames[p.Method]
	for _, c := range charges {
		if !c.Unpaid() {
			continue
		}
		c.LastFailure, c.UpdatedAt = domain.Trim(label+": "+strings.ToLower(domain.FirstNonEmpty(p.FailureMessage, p.FailureCode)), 200), a.Now()
		if err := a.Charges.Save(ctx, &c); err != nil {
			return err
		}
	}
	return nil
}
