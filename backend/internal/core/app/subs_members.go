package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type MemberInput struct {
	PlanIDs     []string          `json:"plan_ids"`
	Phone       string            `json:"phone"`
	PayerName   string            `json:"payer_name"`
	Name        string            `json:"name"`
	Email       string            `json:"email"`
	Fields      map[string]string `json:"fields"`
	FirstPeriod string            `json:"first_period"`
	Channel     string            `json:"channel"`
}

func (a *App) chargeLink(c *domain.Charge) string {
	return strings.TrimRight(a.PublicURL, "/") + "/i/" + c.Token
}

func (a *App) queueMessage(ctx context.Context, m *domain.Merchant, mem *domain.Member, chargeID, kind, body string) error {
	s := a.SubSettingsFor(ctx, m)
	ch := mem.Channel
	if !domain.Contains(s.Channels, ch) || ch == domain.ChannelEmail && mem.Email == "" {
		ch = domain.ChannelSMS
		if !domain.Contains(s.Channels, ch) && len(s.Channels) > 0 && s.Channels[0] != domain.ChannelEmail {
			ch = s.Channels[0]
		}
	}
	to := mem.Phone
	if ch == domain.ChannelEmail {
		to = mem.Email
	}
	return a.Messages.Create(ctx, &domain.Message{ID: domain.NewID("msg"), MerchantID: m.ID, Livemode: mem.Livemode, MemberID: mem.ID, ChargeID: chargeID,
		Channel: ch, To: to, Sender: s.SenderID, Kind: kind, Body: domain.Trim(body, 1000), Status: "queued", CreatedAt: a.Now()})
}

func (a *App) secondChannel(ctx context.Context, m *domain.Merchant, mem *domain.Member) string {
	for _, c := range a.SubSettingsFor(ctx, m).Channels {
		if c != mem.Channel && c != domain.ChannelEmail {
			return c
		}
	}
	return mem.Channel
}

func (a *App) refreshMember(ctx context.Context, memberID string) error {
	mem, err := a.Members.Lock(ctx, memberID)
	if err != nil {
		return err
	}
	subs, err := a.Subs.ByMember(ctx, memberID)
	if err != nil {
		return err
	}
	mem.Status = domain.MemberStatus(subs)
	if len(subs) == 0 {
		mem.Status = domain.SubCanceled
	}
	var names []string
	mem.NextDueAt = nil
	plans := map[string]*domain.Plan{}
	for _, s := range subs {
		if s.Status == domain.SubCanceled || s.Status == domain.SubIncomplete {
			continue
		}
		names = append(names, planOf(plans, a, ctx, s.PlanID).Name)
		if s.NextDueAt != nil && s.Status != domain.SubSuspended && (mem.NextDueAt == nil || s.NextDueAt.Before(*mem.NextDueAt)) {
			d := *s.NextDueAt
			mem.NextDueAt = &d
		}
	}
	mem.PlanNames = domain.Trim(strings.Join(names, " + "), 400)
	unpaid, err := a.Charges.Find(ctx, ports.ChargeQuery{MemberID: memberID, Statuses: []string{domain.ChargeScheduled, domain.ChargeOpen, domain.ChargePastDue}})
	if err != nil {
		return err
	}
	mem.Debt, mem.DebtCount = 0, 0
	for _, c := range unpaid {
		if c.Status == domain.ChargePastDue {
			mem.Debt += c.Amount
			mem.DebtCount++
		}
		if c.Status == domain.ChargeScheduled || c.Status == domain.ChargeOpen {
			if mem.NextDueAt == nil || c.DueAt.Before(*mem.NextDueAt) {
				d := c.DueAt
				mem.NextDueAt = &d
			}
		}
	}
	last, _ := a.Charges.Find(ctx, ports.ChargeQuery{MemberID: memberID, Statuses: []string{domain.ChargePaid, domain.ChargePaidOutside}, OrderBy: "paid_at DESC", Limit: 1})
	if len(last) == 1 {
		mem.LastPaidAt = last[0].PaidAt
		mem.LastPaidVia = last[0].Method
		if last[0].Status == domain.ChargePaidOutside {
			mem.LastPaidVia = "outside"
		}
	}
	mem.UpdatedAt = a.Now()
	return a.Members.Save(ctx, mem)
}

func cleanFields(plans []*domain.Plan, in map[string]string, name string) (map[string]string, string, error) {
	out := map[string]string{}
	for _, p := range plans {
		for i, f := range p.Fields {
			v := domain.Trim(strings.TrimSpace(in[f]), 120)
			if v == "" && i == 0 && name != "" {
				v = name
			}
			if v == "" {
				return nil, "", domain.Invalid("parameter_missing", "fields."+f, "Preencha "+f+".")
			}
			out[f] = v
		}
	}
	if name == "" {
		for _, p := range plans {
			if len(p.Fields) > 0 {
				name = out[p.Fields[0]]
				break
			}
		}
	}
	return out, name, nil
}

type newMember struct {
	plans       []*domain.Plan
	phone       string
	name        string
	payer       string
	email       string
	channel     string
	fields      map[string]string
	firstPeriod string
	status      string
	via         string
	actor       string
	consent     bool
	ip          string
}

func (a *App) resolveMember(ctx context.Context, m *domain.Merchant, live bool, in MemberInput, status, via string) (*newMember, error) {
	if len(in.PlanIDs) == 0 {
		return nil, domain.Invalid("parameter_missing", "plan_ids", "Escolha pelo menos um plano.")
	}
	if len(in.PlanIDs) > 5 {
		return nil, domain.Invalid("parameter_invalid", "plan_ids", "No máximo 5 planos de cada vez.")
	}
	var plans []*domain.Plan
	seen := map[string]bool{}
	for _, id := range in.PlanIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		p, err := a.Plans.ByID(ctx, m.ID, id)
		if err != nil || p.Livemode != live || p.Status != domain.PlanActive {
			return nil, domain.Invalid("plan_invalid", "plan_ids", "Plano indisponível.")
		}
		plans = append(plans, p)
	}
	if !domain.ValidMZMobile(in.Phone) {
		return nil, domain.Invalid("phone_invalid", "phone", "Indique um número de telemóvel moçambicano válido.")
	}
	payer := domain.Trim(strings.TrimSpace(in.PayerName), 200)
	fields, name, err := cleanFields(plans, in.Fields, domain.Trim(strings.TrimSpace(in.Name), 200))
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = payer
	}
	if len([]rune(name)) < 2 {
		return nil, domain.Invalid("parameter_missing", "name", "Indique o nome.")
	}
	ch := in.Channel
	if ch != domain.ChannelWhatsApp && ch != domain.ChannelEmail {
		ch = domain.ChannelSMS
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if ch == domain.ChannelEmail && !strings.Contains(email, "@") {
		return nil, domain.Invalid("parameter_invalid", "email", "Indique um email válido.")
	}
	return &newMember{plans: plans, phone: domain.NormalizeMZPhone(in.Phone), name: name, payer: payer, email: domain.Trim(email, 200),
		channel: ch, fields: fields, firstPeriod: in.FirstPeriod, status: status, via: via}, nil
}

func (a *App) createMembership(ctx context.Context, m *domain.Merchant, live bool, nm *newMember) (*domain.Member, []domain.Charge, error) {
	now := a.Now()
	mem, err := a.Members.Identity(ctx, m.ID, live, nm.phone, domain.NameKey(nm.name))
	if err != nil && !isNotFound(err) {
		return nil, nil, err
	}
	if mem == nil {
		mem = &domain.Member{ID: domain.NewID("mem"), MerchantID: m.ID, Livemode: live, Name: nm.name, NameKey: domain.NameKey(nm.name), Phone: nm.phone,
			Status: nm.status, Token: domain.Token("", 24), CreatedVia: nm.via, CreatedBy: nm.actor, CreatedAt: now}
		if err := a.Members.Create(ctx, mem); err != nil {
			return nil, nil, err
		}
	} else if _, err := a.Members.Lock(ctx, mem.ID); err != nil {
		return nil, nil, err
	}
	mem.PayerName = domain.FirstNonEmpty(nm.payer, mem.PayerName)
	mem.Email = domain.FirstNonEmpty(nm.email, mem.Email)
	mem.Channel = nm.channel
	if mem.Fields == nil {
		mem.Fields = map[string]string{}
	}
	for k, v := range nm.fields {
		mem.Fields[k] = v
	}
	if err := a.Members.Save(ctx, mem); err != nil {
		return nil, nil, err
	}
	existing, err := a.Subs.ByMember(ctx, mem.ID)
	if err != nil {
		return nil, nil, err
	}
	var charges []domain.Charge
	for _, p := range nm.plans {
		for _, s := range existing {
			if s.PlanID == p.ID && s.Status != domain.SubCanceled {
				if s.Status == domain.SubInvited && nm.via != "link" {
					return nil, nil, domain.Conflict("already_invited", fmt.Sprintf("%s já foi convidado para %s. Use Reenviar link.", mem.Name, p.Name))
				}
				if s.Status == domain.SubIncomplete || s.Status == domain.SubInvited {
					s.Status = domain.SubCanceled
					s.CanceledAt = &now
					if err := a.Subs.Save(ctx, &s); err != nil {
						return nil, nil, err
					}
					if err := a.cancelOpenCharges(ctx, s.ID); err != nil {
						return nil, nil, err
					}
					continue
				}
				return nil, nil, domain.Conflict("already_subscribed", fmt.Sprintf("%s já tem o plano %s.", mem.Name, p.Name))
			}
		}
		first, ok := domain.FirstDue(p, now)
		if nm.firstPeriod != "" {
			ok = false
			for _, d := range a.FirstPeriods(p, 6) {
				if domain.PeriodKey(d) == nm.firstPeriod {
					first, ok = d, true
				}
			}
		}
		if !ok {
			return nil, nil, domain.Invalid("no_periods", "first_period", fmt.Sprintf("O plano %s não tem mais cobranças.", p.Name))
		}
		s := &domain.Subscription{ID: domain.NewID("sub"), MerchantID: m.ID, Livemode: live, MemberID: mem.ID, PlanID: p.ID, PlanVersion: p.Version,
			Amount: p.Amount, Status: nm.status, AnchorAt: first.UTC(), CreatedVia: nm.via, CreatedBy: nm.actor, CreatedAt: now, UpdatedAt: now}
		if next, ok := domain.NextDue(p, first, first); ok {
			s.NextDueAt = &next
		}
		if nm.consent {
			s.ConsentText = domain.Agreement(m.Name, p, p.Amount, nm.phone, nm.channel)
			s.ConsentAt, s.ConsentIP = &now, nm.ip
		}
		if err := a.Subs.Create(ctx, s); err != nil {
			return nil, nil, err
		}
		c := domain.Charge{ID: domain.NewID("sch"), MerchantID: m.ID, Livemode: live, SubscriptionID: s.ID, MemberID: mem.ID, PlanID: p.ID,
			Period: domain.PeriodKey(first), Label: domain.PeriodLabel(p, first), DueAt: first.UTC(), Amount: s.Amount, Status: domain.ChargeOpen,
			First: true, Token: domain.Token("", 16), Steps: []string{domain.StepBefore, domain.StepLink, domain.StepEvening}, LinkSentAt: &now, CreatedAt: now, UpdatedAt: now}
		if err := a.Charges.Create(ctx, &c); err != nil {
			return nil, nil, err
		}
		charges = append(charges, c)
	}
	if err := a.refreshMember(ctx, mem.ID); err != nil {
		return nil, nil, err
	}
	mem, _ = a.Members.Get(ctx, mem.ID)
	return mem, charges, nil
}

func (a *App) cancelOpenCharges(ctx context.Context, subID string) error {
	cs, err := a.Charges.Find(ctx, ports.ChargeQuery{SubscriptionIDs: []string{subID}, Statuses: []string{domain.ChargeScheduled, domain.ChargeOpen}})
	if err != nil {
		return err
	}
	for _, c := range cs {
		c.Status, c.UpdatedAt = domain.ChargeCanceled, a.Now()
		if err := a.Charges.Save(ctx, &c); err != nil {
			return err
		}
	}
	return nil
}

func firstName(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return s
}

func (a *App) inviteBody(m *domain.Merchant, mem *domain.Member, charges []domain.Charge, plans map[string]*domain.Plan, link string) string {
	var names []string
	var total int64
	for _, c := range charges {
		names = append(names, plans[c.PlanID].Name)
		total += c.Amount
	}
	return fmt.Sprintf("%s: inscrição de %s em %s. Primeiro pagamento %s MT. Aceite e pague aqui: %s", m.Name, mem.Name, strings.Join(names, " + "), domain.FormatShort(total), link)
}

func (a *App) InviteMember(ctx context.Context, m *domain.Merchant, live bool, in MemberInput, actor Actor) (*domain.Member, error) {
	if e := domain.RequireApproved(m); e != nil {
		return nil, e
	}
	nm, err := a.resolveMember(ctx, m, live, in, domain.SubInvited, "dashboard")
	if err != nil {
		return nil, err
	}
	nm.actor = actor.ID
	return a.invite(ctx, m, live, nm, actor)
}

func (a *App) invite(ctx context.Context, m *domain.Merchant, live bool, nm *newMember, actor Actor) (*domain.Member, error) {
	var mem *domain.Member
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		var charges []domain.Charge
		var err error
		mem, charges, err = a.createMembership(ctx, m, live, nm)
		if err != nil {
			return err
		}
		plans := map[string]*domain.Plan{}
		for _, p := range nm.plans {
			plans[p.ID] = p
		}
		if err := a.queueMessage(ctx, m, mem, charges[0].ID, "invite", a.inviteBody(m, mem, charges, plans, a.chargeLink(&charges[0]))); err != nil {
			return err
		}
		for _, c := range charges {
			s, _ := a.Subs.Get(ctx, c.SubscriptionID)
			if err := a.emit(ctx, m.ID, live, "subscription.created", s.ID, domain.SubscriptionResource(s, mem), ""); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "member.invite", mem.ID, mem.Name)
	return mem, nil
}

type SubscribeInput struct {
	MemberInput
	Accept bool   `json:"accept"`
	IP     string `json:"-"`
}

func (a *App) AgreementPreview(ctx context.Context, m *domain.Merchant, plans []*domain.Plan, phone, channel string) []string {
	var out []string
	for _, p := range plans {
		out = append(out, domain.Agreement(m.Name, p, p.Amount, phone, channel))
	}
	return out
}

func (a *App) Subscribe(ctx context.Context, merchantSlug string, in SubscribeInput) (string, error) {
	m, err := a.publicMerchant(ctx, merchantSlug)
	if err != nil {
		return "", err
	}
	if !in.Accept {
		return "", domain.Invalid("agreement_required", "accept", "Aceite o acordo de pagamento para continuar.")
	}
	var live bool
	if len(in.PlanIDs) > 0 {
		if p, err := a.Plans.Get(ctx, in.PlanIDs[0]); err == nil && p.MerchantID == m.ID {
			live = p.Livemode
		}
	}
	if len(in.PlanIDs) > 1 {
		if !a.SubSettingsFor(ctx, m).CatalogMulti {
			return "", domain.Invalid("parameter_invalid", "plan_ids", "Escolha apenas um plano.")
		}
		for _, id := range in.PlanIDs {
			if p, err := a.Plans.Get(ctx, id); err != nil || !p.InCatalog {
				return "", domain.Invalid("plan_invalid", "plan_ids", "Plano indisponível.")
			}
		}
	}
	nm, err := a.resolveMember(ctx, m, live, in.MemberInput, domain.SubIncomplete, "link")
	if err != nil {
		return "", err
	}
	nm.consent, nm.ip = true, domain.Trim(in.IP, 64)
	var token string
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		_, charges, err := a.createMembership(ctx, m, live, nm)
		if err != nil {
			return err
		}
		token = charges[0].Token
		return nil
	})
	return token, err
}

func (a *App) Member(ctx context.Context, merchantID string, live bool, id string) (*domain.Member, error) {
	mem, err := a.Members.ByID(ctx, merchantID, id)
	if err != nil || mem.Livemode != live {
		return nil, domain.NotFound("O membro")
	}
	return mem, nil
}

func (a *App) memberSub(ctx context.Context, merchantID string, live bool, subID string) (*domain.Subscription, *domain.Member, error) {
	s, err := a.Subs.Lock(ctx, subID)
	if err != nil || s.MerchantID != merchantID || s.Livemode != live {
		return nil, nil, domain.NotFound("A assinatura")
	}
	mem, err := a.Members.Get(ctx, s.MemberID)
	if err != nil {
		return nil, nil, err
	}
	return s, mem, nil
}

func (a *App) UpdateMember(ctx context.Context, m *domain.Merchant, live bool, id string, in MemberInput, actor Actor) (*domain.Member, error) {
	mem, err := a.Member(ctx, m.ID, live, id)
	if err != nil {
		return nil, err
	}
	if in.Phone != "" {
		if !domain.ValidMZMobile(in.Phone) {
			return nil, domain.Invalid("phone_invalid", "phone", "Indique um número de telemóvel moçambicano válido.")
		}
		mem.Phone = domain.NormalizeMZPhone(in.Phone)
	}
	if n := domain.Trim(strings.TrimSpace(in.Name), 200); n != "" {
		mem.Name, mem.NameKey = n, domain.NameKey(n)
	}
	mem.PayerName = domain.Trim(strings.TrimSpace(in.PayerName), 200)
	if in.Channel == domain.ChannelSMS || in.Channel == domain.ChannelWhatsApp || in.Channel == domain.ChannelEmail {
		mem.Channel = in.Channel
	}
	if in.Email != "" {
		mem.Email = domain.Trim(strings.ToLower(strings.TrimSpace(in.Email)), 200)
	}
	for k, v := range in.Fields {
		if _, ok := mem.Fields[k]; ok && strings.TrimSpace(v) != "" {
			mem.Fields[k] = domain.Trim(strings.TrimSpace(v), 120)
		}
	}
	mem.UpdatedAt = a.Now()
	if err := a.Members.Save(ctx, mem); err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "member.update", mem.ID, mem.Name)
	return mem, nil
}

const MaxResendsPerDay = 3

func (a *App) ResendLink(ctx context.Context, m *domain.Merchant, live bool, memberID string, actor Actor) error {
	if e := domain.RequireApproved(m); e != nil {
		return e
	}
	mem, err := a.Member(ctx, m.ID, live, memberID)
	if err != nil {
		return err
	}
	if n, _ := a.Messages.CountRecent(ctx, mem.ID, "resend", a.Now().Add(-24*time.Hour)); n >= MaxResendsPerDay {
		return domain.Forbidden("too_many_resends", "Já reenviou 3 vezes nas últimas 24 horas.")
	}
	unpaid, _ := a.Charges.Find(ctx, ports.ChargeQuery{MemberID: mem.ID, Statuses: domain.UnpaidChargeStatuses})
	if len(unpaid) == 0 {
		return domain.Invalid("nothing_due", "member", "Não há cobranças por pagar.")
	}
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		c, err := a.Charges.Lock(ctx, unpaid[0].ID)
		if err != nil {
			return err
		}
		c.Resends++
		c.UpdatedAt = a.Now()
		if err := a.Charges.Save(ctx, c); err != nil {
			return err
		}
		var total int64
		for _, u := range unpaid {
			total += u.Amount
		}
		body := fmt.Sprintf("%s: pagamento de %s, %s MT. Pague aqui: %s", m.Name, mem.Name, domain.FormatShort(total), a.chargeLink(c))
		if mem.Status == domain.SubInvited {
			body = fmt.Sprintf("%s: lembrete da inscrição de %s. Aceite e pague aqui: %s", m.Name, mem.Name, a.chargeLink(c))
		}
		return a.queueMessage(ctx, m, mem, c.ID, "resend", body)
	})
	if err == nil {
		a.Record(ctx, m.ID, actor, "member.resend", mem.ID, mem.Name)
	}
	return err
}

func (a *App) ResendInvites(ctx context.Context, m *domain.Merchant, live bool, olderThan time.Duration, actor Actor) (int, error) {
	mems, err := a.Members.Find(ctx, ports.MemberQuery{MerchantID: m.ID, Live: live, Statuses: []string{domain.SubInvited}, Limit: 500})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, mem := range mems {
		if a.Now().Sub(mem.CreatedAt) < olderThan {
			continue
		}
		if a.ResendLink(ctx, m, live, mem.ID, actor) == nil {
			n++
		}
	}
	return n, nil
}

func (a *App) SkipNext(ctx context.Context, m *domain.Merchant, live bool, subID string, actor Actor) (string, error) {
	var label string
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		s, mem, err := a.memberSub(ctx, m.ID, live, subID)
		if err != nil {
			return err
		}
		if s.Status == domain.SubCanceled || s.Status == domain.SubIncomplete || s.Status == domain.SubInvited {
			return domain.Invalid("subscription_state", "subscription", "Esta assinatura não está ativa.")
		}
		p, err := a.Plans.Get(ctx, s.PlanID)
		if err != nil {
			return err
		}
		sched, _ := a.Charges.Find(ctx, ports.ChargeQuery{SubscriptionIDs: []string{s.ID}, Statuses: []string{domain.ChargeScheduled}, Limit: 1})
		if len(sched) == 1 {
			c := sched[0]
			c.Status, c.UpdatedAt = domain.ChargeSkipped, a.Now()
			label = c.Label
			if err := a.Charges.Save(ctx, &c); err != nil {
				return err
			}
		} else if s.NextDueAt != nil {
			key := domain.PeriodKey(*s.NextDueAt)
			for _, k := range s.SkipPeriods {
				if k == key {
					return domain.Invalid("already_skipped", "subscription", "A próxima cobrança já vai ser saltada.")
				}
			}
			s.SkipPeriods = append(s.SkipPeriods, key)
			label = domain.PeriodLabel(p, *s.NextDueAt)
			s.UpdatedAt = a.Now()
			if err := a.Subs.Save(ctx, s); err != nil {
				return err
			}
		} else {
			return domain.Invalid("nothing_due", "subscription", "Não há próximas cobranças.")
		}
		if err := a.refreshMember(ctx, mem.ID); err != nil {
			return err
		}
		return a.queueMessage(ctx, m, mem, "", "skip", fmt.Sprintf("%s: %s de %s não vai ser cobrado.", m.Name, label, mem.Name))
	})
	if err == nil {
		a.Record(ctx, m.ID, actor, "subscription.skip", subID, label)
	}
	return label, err
}

func (a *App) PauseSubscription(ctx context.Context, m *domain.Merchant, live bool, subID string, until time.Time, actor Actor) error {
	until = domain.LocalDate(until).UTC()
	if !until.After(a.Now()) || until.After(a.Now().AddDate(1, 0, 0)) {
		return domain.Invalid("parameter_invalid", "until", "Escolha uma data nos próximos 12 meses.")
	}
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		s, mem, err := a.memberSub(ctx, m.ID, live, subID)
		if err != nil {
			return err
		}
		if s.Status != domain.SubActive && s.Status != domain.SubPastDue && s.Status != domain.SubPaused {
			return domain.Invalid("subscription_state", "subscription", "Só é possível pausar uma assinatura ativa.")
		}
		s.Status, s.PausedUntil, s.UpdatedAt = domain.SubPaused, &until, a.Now()
		if err := a.Subs.Save(ctx, s); err != nil {
			return err
		}
		sched, _ := a.Charges.Find(ctx, ports.ChargeQuery{SubscriptionIDs: []string{s.ID}, Statuses: []string{domain.ChargeScheduled}})
		for _, c := range sched {
			if c.DueAt.Before(until) {
				c.Status, c.UpdatedAt = domain.ChargeSkipped, a.Now()
				if err := a.Charges.Save(ctx, &c); err != nil {
					return err
				}
			}
		}
		if err := a.refreshMember(ctx, mem.ID); err != nil {
			return err
		}
		if err := a.queueMessage(ctx, m, mem, "", "pause", fmt.Sprintf("%s: a assinatura de %s está pausada até %s. Não há cobranças até lá.", m.Name, mem.Name, domain.LongDate(until))); err != nil {
			return err
		}
		return a.emit(ctx, m.ID, live, "subscription.updated", s.ID, domain.SubscriptionResource(s, mem), "")
	})
	if err == nil {
		a.Record(ctx, m.ID, actor, "subscription.pause", subID, domain.LongDate(until))
	}
	return err
}

func (a *App) ResumeSubscription(ctx context.Context, m *domain.Merchant, live bool, subID string, actor Actor) error {
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		s, mem, err := a.memberSub(ctx, m.ID, live, subID)
		if err != nil {
			return err
		}
		if s.Status != domain.SubPaused && s.Status != domain.SubSuspended {
			return domain.Invalid("subscription_state", "subscription", "Esta assinatura não está pausada nem suspensa.")
		}
		if err := a.reactivate(ctx, s); err != nil {
			return err
		}
		if err := a.refreshMember(ctx, mem.ID); err != nil {
			return err
		}
		return a.emit(ctx, m.ID, live, "subscription.updated", s.ID, domain.SubscriptionResource(s, mem), "")
	})
	if err == nil {
		a.Record(ctx, m.ID, actor, "subscription.resume", subID, "")
	}
	return err
}

func (a *App) reactivate(ctx context.Context, s *domain.Subscription) error {
	now := a.Now()
	if s.Status == domain.SubSuspended || s.Status == domain.SubPaused {
		if p, err := a.Plans.Get(ctx, s.PlanID); err == nil && (s.NextDueAt == nil || s.NextDueAt.Before(now)) {
			if next, ok := domain.NextDue(p, s.AnchorAt, now); ok {
				s.NextDueAt = &next
			}
		}
	}
	unpaid, _ := a.Charges.Find(ctx, ports.ChargeQuery{SubscriptionIDs: []string{s.ID}, Statuses: []string{domain.ChargePastDue}})
	s.Status = domain.SubActive
	if len(unpaid) > 0 {
		s.Status = domain.SubPastDue
	}
	if s.ActivatedAt == nil {
		s.ActivatedAt = &now
	}
	s.PausedUntil, s.UpdatedAt = nil, now
	return a.Subs.Save(ctx, s)
}

func (a *App) ChangeSubscriptionPlan(ctx context.Context, m *domain.Merchant, live bool, subID, planID string, actor Actor) error {
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		s, mem, err := a.memberSub(ctx, m.ID, live, subID)
		if err != nil {
			return err
		}
		if s.Status == domain.SubCanceled || s.Status == domain.SubIncomplete {
			return domain.Invalid("subscription_state", "subscription", "Esta assinatura não está ativa.")
		}
		p, err := a.Plan(ctx, m.ID, live, planID)
		if err != nil || p.Status != domain.PlanActive {
			return domain.Invalid("plan_invalid", "plan_id", "Plano indisponível.")
		}
		if p.ID == s.PlanID {
			return domain.Invalid("plan_unchanged", "plan_id", "O membro já está neste plano.")
		}
		others, _ := a.Subs.ByMember(ctx, mem.ID)
		for _, o := range others {
			if o.PlanID == p.ID && o.Status != domain.SubCanceled {
				return domain.Conflict("already_subscribed", mem.Name+" já tem esse plano.")
			}
		}
		for _, f := range p.Fields {
			if strings.TrimSpace(mem.Fields[f]) == "" {
				return domain.Invalid("parameter_missing", "fields."+f, "Preencha "+f+" do membro antes de mudar de plano.")
			}
		}
		if err := a.cancelScheduled(ctx, s.ID); err != nil {
			return err
		}
		now := a.Now()
		first, ok := domain.FirstDue(p, now)
		if !ok {
			return domain.Invalid("no_periods", "plan_id", "Esse plano não tem mais cobranças.")
		}
		s.PlanID, s.PlanVersion, s.Amount, s.AnchorAt = p.ID, p.Version, p.Amount, first.UTC()
		s.PendingAmount, s.PendingVersion, s.PendingFrom, s.SkipPeriods = 0, 0, nil, nil
		s.NextDueAt = nil
		if next, ok := domain.NextDue(p, first, now); ok {
			s.NextDueAt = &next
		}
		s.UpdatedAt = now
		if err := a.Subs.Save(ctx, s); err != nil {
			return err
		}
		if err := a.refreshMember(ctx, mem.ID); err != nil {
			return err
		}
		if err := a.queueMessage(ctx, m, mem, "", "plan_change", fmt.Sprintf("%s: %s passa ao plano %s, %s MT por %s.", m.Name, mem.Name, p.Name, domain.FormatShort(p.Amount), domain.IntervalLabel(p.Interval))); err != nil {
			return err
		}
		return a.emit(ctx, m.ID, live, "subscription.updated", s.ID, domain.SubscriptionResource(s, mem), "")
	})
	if err == nil {
		a.Record(ctx, m.ID, actor, "subscription.plan", subID, planID)
	}
	return err
}

func (a *App) cancelScheduled(ctx context.Context, subID string) error {
	cs, err := a.Charges.Find(ctx, ports.ChargeQuery{SubscriptionIDs: []string{subID}, Statuses: []string{domain.ChargeScheduled}})
	if err != nil {
		return err
	}
	for _, c := range cs {
		c.Status, c.UpdatedAt = domain.ChargeCanceled, a.Now()
		if err := a.Charges.Save(ctx, &c); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) CancelSubscription(ctx context.Context, m *domain.Merchant, live bool, subID, reason string, actor Actor) error {
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		s, mem, err := a.memberSub(ctx, m.ID, live, subID)
		if err != nil {
			return err
		}
		if s.Status == domain.SubCanceled {
			return nil
		}
		return a.cancelSub(ctx, m, s, mem, reason)
	})
	if err == nil {
		a.Record(ctx, m.ID, actor, "subscription.cancel", subID, reason)
	}
	return err
}

func (a *App) cancelSub(ctx context.Context, m *domain.Merchant, s *domain.Subscription, mem *domain.Member, reason string) error {
	now := a.Now()
	notify := s.Status != domain.SubInvited && s.Status != domain.SubIncomplete
	s.Status, s.CanceledAt, s.NextDueAt, s.UpdatedAt = domain.SubCanceled, &now, nil, now
	if err := a.Subs.Save(ctx, s); err != nil {
		return err
	}
	if err := a.cancelOpenCharges(ctx, s.ID); err != nil {
		return err
	}
	if err := a.refreshMember(ctx, mem.ID); err != nil {
		return err
	}
	if notify {
		p, _ := a.Plans.Get(ctx, s.PlanID)
		name := "a assinatura"
		if p != nil {
			name = p.Name
		}
		if err := a.queueMessage(ctx, m, mem, "", "cancel", fmt.Sprintf("%s: %s de %s foi cancelada. Não vai receber mais cobranças.", m.Name, name, mem.Name)); err != nil {
			return err
		}
	}
	return a.emit(ctx, m.ID, s.Livemode, "subscription.canceled", s.ID, domain.SubscriptionResource(s, mem), "")
}

type OutsideInput struct {
	ChargeIDs []string
	Method    string
	Ref       string
	Date      time.Time
	Proof     *Upload
}

func (a *App) MarkPaidOutside(ctx context.Context, m *domain.Merchant, live bool, memberID string, in OutsideInput, actor Actor, actorName string) ([]domain.Charge, error) {
	if e := domain.RequireApproved(m); e != nil {
		return nil, e
	}
	if len(in.ChargeIDs) == 0 {
		return nil, domain.Invalid("parameter_missing", "charge_ids", "Escolha pelo menos uma cobrança.")
	}
	if _, ok := domain.OutsideMethods[in.Method]; !ok {
		return nil, domain.Invalid("parameter_invalid", "method", "Escolha como foi pago.")
	}
	date := domain.LocalDate(in.Date)
	if in.Date.IsZero() {
		date = domain.LocalDate(a.Now())
	}
	if date.After(a.Now()) || date.Before(a.Now().AddDate(-1, 0, 0)) {
		return nil, domain.Invalid("parameter_invalid", "date", "A data tem de estar nos últimos 12 meses e não pode ser futura.")
	}
	settings := a.SubSettingsFor(ctx, m)
	var path, name, hash string
	if in.Proof != nil && in.Proof.Size > 0 {
		if in.Proof.Size > 8<<20 {
			return nil, domain.Invalid("proof_invalid", "proof", "O comprovativo tem de ter menos de 8 MB.")
		}
		ext := strings.ToLower(filepath.Ext(in.Proof.FileName))
		ct, ok := receiptTypes[ext]
		if !ok || !sniff(ct, in.Proof.Head) {
			return nil, domain.Invalid("proof_invalid", "proof", "Use um comprovativo em PDF, JPG ou PNG.")
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(in.Proof.Body); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(buf.Bytes())
		hash, name = hex.EncodeToString(sum[:]), domain.Trim(filepath.Base(in.Proof.FileName), 200)
		p, err := a.Files.Save(ctx, "outside-proofs", domain.NewID("prf")+ext, receiptTypes[ext], bytes.NewReader(buf.Bytes()))
		if err != nil {
			return nil, err
		}
		path = p
	} else if settings.RequireProof {
		return nil, domain.Invalid("proof_required", "proof", "Carregue o comprovativo. É obrigatório nas definições.")
	}
	var out []domain.Charge
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		mem, err := a.Members.Lock(ctx, memberID)
		if err != nil || mem.MerchantID != m.ID || mem.Livemode != live {
			return domain.NotFound("O membro")
		}
		now := a.Now()
		subs := map[string]bool{}
		for _, id := range in.ChargeIDs {
			c, err := a.Charges.Lock(ctx, id)
			if err != nil || c.MemberID != mem.ID {
				return domain.NotFound("A cobrança")
			}
			if !c.Unpaid() && c.Status != domain.ChargeScheduled {
				return domain.Invalid("charge_state", "charge_ids", c.Label+" já não está por pagar.")
			}
			c.Status, c.PaidAt, c.Method, c.OutsideMethod, c.OutsideRef = domain.ChargePaidOutside, &date, "", in.Method, domain.Trim(strings.TrimSpace(in.Ref), 100)
			c.OutsideProofPath, c.OutsideProofName, c.OutsideProofHash, c.MarkedBy, c.UpdatedAt = path, name, hash, domain.Trim(actorName, 200), now
			c.SessionID = nil
			if err := a.Charges.Save(ctx, c); err != nil {
				return err
			}
			subs[c.SubscriptionID] = true
			out = append(out, *c)
			if err := a.emit(ctx, m.ID, live, "subscription_charge.paid_outside", c.ID, domain.ChargeResource(c, a.PublicURL), ""); err != nil {
				return err
			}
		}
		for id := range subs {
			s, err := a.Subs.Lock(ctx, id)
			if err != nil {
				return err
			}
			if s.Status == domain.SubPastDue || s.Status == domain.SubSuspended || s.Status == domain.SubInvited {
				if err := a.reactivate(ctx, s); err != nil {
					return err
				}
			}
		}
		if err := a.refreshMember(ctx, mem.ID); err != nil {
			return err
		}
		if settings.ReceiptSMS {
			var total int64
			var labels []string
			for _, c := range out {
				total += c.Amount
				labels = append(labels, c.Label)
			}
			body := fmt.Sprintf("%s: recebemos %s MT de %s (%s, %s). Obrigado.", m.Name, domain.FormatShort(total), mem.Name, strings.Join(labels, ", "), strings.ToLower(domain.OutsideMethods[in.Method]))
			return a.queueMessage(ctx, m, mem, out[0].ID, "receipt", body)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "charge.paid_outside", memberID, fmt.Sprintf("%d cobranças, %s", len(out), domain.OutsideMethods[in.Method]))
	return out, nil
}

func (a *App) OutsideProof(ctx context.Context, merchantID string, live bool, chargeID string) (*domain.Attachment, error) {
	cs, err := a.Charges.Find(ctx, ports.ChargeQuery{MerchantID: merchantID, Live: &live, IDs: []string{chargeID}, Limit: 1})
	if err != nil || len(cs) == 0 || cs[0].OutsideProofPath == "" {
		return nil, domain.NotFound("O comprovativo")
	}
	data, err := a.Files.Read(ctx, cs[0].OutsideProofPath)
	if err != nil {
		return nil, domain.NotFound("O comprovativo")
	}
	ct := receiptTypes[strings.ToLower(filepath.Ext(cs[0].OutsideProofPath))]
	return &domain.Attachment{Name: cs[0].OutsideProofName, ContentType: ct, Data: data}, nil
}

type ImportRow struct {
	Line      int               `json:"line"`
	Name      string            `json:"name"`
	PayerName string            `json:"payer_name"`
	Phone     string            `json:"phone"`
	PlanIDs   []string          `json:"plan_ids"`
	PlanLabel string            `json:"plan_label"`
	Fields    map[string]string `json:"fields"`
	Error     string            `json:"error,omitempty"`
}

type ImportPreview struct {
	Rows    []ImportRow `json:"rows"`
	Ready   int         `json:"ready"`
	Invalid int         `json:"invalid"`
	Columns []string    `json:"columns"`
}

func fold(s string) string {
	t := norm.NFD.String(strings.NewReplacer("ª", "a", "º", "o").Replace(strings.ToLower(strings.TrimSpace(s))))
	var b strings.Builder
	for _, r := range t {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func matchPlan(plans []domain.Plan, cell string) *domain.Plan {
	c := fold(cell)
	if c == "" {
		return nil
	}
	var hit *domain.Plan
	for i := range plans {
		n := fold(plans[i].Name)
		if n == c {
			return &plans[i]
		}
		if strings.Contains(n, c) || strings.Contains(c, n) || tokensIn(c, n) {
			if hit != nil {
				return nil
			}
			hit = &plans[i]
		}
	}
	return hit
}

func tokensIn(cell, name string) bool {
	toks := strings.Fields(cell)
	if len(toks) == 0 {
		return false
	}
	for _, t := range toks {
		if !strings.Contains(name, t) {
			return false
		}
	}
	return true
}

func (a *App) PreviewImport(ctx context.Context, m *domain.Merchant, live bool, fileName string, data []byte, defaultPlan string) (*ImportPreview, error) {
	if len(data) > 4<<20 {
		return nil, domain.Invalid("file_invalid", "file", "A folha tem de ter menos de 4 MB.")
	}
	rows, err := a.Sheets.Rows(fileName, data)
	if err != nil || len(rows) < 2 {
		return nil, domain.Invalid("file_invalid", "file", "Use uma folha .xlsx ou .csv com uma linha de cabeçalho e pelo menos um membro.")
	}
	plans := []domain.Plan{}
	for _, p := range a.PlanList(ctx, m.ID, live) {
		if p.Status == domain.PlanActive {
			plans = append(plans, p)
		}
	}
	if len(plans) == 0 {
		return nil, domain.Invalid("no_plans", "plan", "Crie um plano antes de importar membros.")
	}
	fieldNames := map[string]string{}
	for _, p := range plans {
		for _, f := range p.Fields {
			fieldNames[fold(f)] = f
		}
	}
	col := map[string]int{}
	fieldCols := map[int]string{}
	var cols []string
	for i, h := range rows[0] {
		f := fold(h)
		switch {
		case f == "":
			continue
		case fieldNames[f] != "":
			fieldCols[i] = fieldNames[f]
			cols = append(cols, fieldNames[f])
			if _, ok := col["name"]; !ok && strings.Contains(f, "nome") {
				col["name"] = i
			}
			continue
		case strings.Contains(f, "numero") || strings.Contains(f, "telefone") || strings.Contains(f, "telemovel") || strings.Contains(f, "contacto") || strings.Contains(f, "celular"):
			col["phone"] = i
		case strings.HasPrefix(f, "plano"):
			col["plan"] = i
		case strings.Contains(f, "encarregado") || strings.Contains(f, "quem paga") || strings.Contains(f, "pai") || strings.Contains(f, "responsavel"):
			col["payer"] = i
		case strings.Contains(f, "nome") || f == "aluno" || f == "membro" || f == "cliente":
			if _, ok := col["name"]; !ok {
				col["name"] = i
			}
		default:
			continue
		}
		cols = append(cols, h)
	}
	if _, ok := col["phone"]; !ok {
		return nil, domain.Invalid("file_invalid", "file", "Não encontrámos a coluna do número de telefone.")
	}
	var def *domain.Plan
	for i := range plans {
		if plans[i].ID == defaultPlan {
			def = &plans[i]
		}
	}
	if def == nil && len(plans) == 1 {
		def = &plans[0]
	}
	cell := func(r []string, i int) string {
		if i < len(r) {
			return strings.TrimSpace(r[i])
		}
		return ""
	}
	out := &ImportPreview{Columns: cols}
	seen := map[string]bool{}
	for li, r := range rows[1:] {
		empty := true
		for _, v := range r {
			if strings.TrimSpace(v) != "" {
				empty = false
			}
		}
		if empty {
			continue
		}
		row := ImportRow{Line: li + 2, Fields: map[string]string{}}
		for i, f := range fieldCols {
			row.Fields[f] = cell(r, i)
		}
		if i, ok := col["name"]; ok {
			row.Name = cell(r, i)
		}
		if i, ok := col["payer"]; ok {
			row.PayerName = cell(r, i)
		}
		row.Phone = cell(r, col["phone"])
		var chosen []domain.Plan
		if i, ok := col["plan"]; ok && cell(r, i) != "" {
			row.PlanLabel = cell(r, i)
			for _, part := range strings.FieldsFunc(row.PlanLabel, func(r rune) bool { return r == '+' || r == ';' || r == '/' }) {
				if p := matchPlan(plans, part); p != nil {
					chosen = append(chosen, *p)
				} else {
					row.Error = "Plano não reconhecido: " + strings.TrimSpace(part)
				}
			}
		} else if def != nil {
			chosen = []domain.Plan{*def}
		} else {
			row.Error = "Indique o plano"
		}
		var labels []string
		for _, p := range chosen {
			row.PlanIDs = append(row.PlanIDs, p.ID)
			labels = append(labels, p.Name)
			for i, f := range p.Fields {
				if row.Fields[f] == "" && i == 0 && row.Name != "" {
					row.Fields[f] = row.Name
				}
				if row.Fields[f] == "" && row.Error == "" {
					row.Error = "Falta " + f
				}
			}
		}
		if len(labels) > 0 {
			row.PlanLabel = strings.Join(labels, " + ")
		}
		if row.Name == "" {
			for _, p := range chosen {
				if len(p.Fields) > 0 {
					row.Name = row.Fields[p.Fields[0]]
					break
				}
			}
		}
		if row.Name == "" {
			row.Name = row.PayerName
		}
		switch {
		case row.Error != "":
		case !domain.ValidMZMobile(row.Phone):
			row.Error = "Número inválido"
		case row.Name == "":
			row.Error = "Falta o nome"
		}
		key := domain.NormalizeMZPhone(row.Phone) + "|" + domain.NameKey(row.Name)
		if row.Error == "" && seen[key] {
			row.Error = "Repetido na folha"
		}
		if row.Error == "" {
			if mem, err := a.Members.Identity(ctx, m.ID, live, domain.NormalizeMZPhone(row.Phone), domain.NameKey(row.Name)); err == nil {
				subs, _ := a.Subs.ByMember(ctx, mem.ID)
				for _, s := range subs {
					if domain.Contains(row.PlanIDs, s.PlanID) && s.Status != domain.SubCanceled && s.Status != domain.SubIncomplete {
						row.Error = "Já é membro deste plano"
					}
				}
			}
		}
		seen[key] = true
		if row.Error == "" {
			out.Ready++
		} else {
			out.Invalid++
		}
		out.Rows = append(out.Rows, row)
	}
	sort.SliceStable(out.Rows, func(i, j int) bool { return out.Rows[i].Error != "" && out.Rows[j].Error == "" })
	return out, nil
}

type ImportResult struct {
	Invited int      `json:"invited"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}

func (a *App) ImportMembers(ctx context.Context, m *domain.Merchant, live bool, fileName string, data []byte, defaultPlan string, actor Actor) (*ImportResult, error) {
	if e := domain.RequireApproved(m); e != nil {
		return nil, e
	}
	pv, err := a.PreviewImport(ctx, m, live, fileName, data, defaultPlan)
	if err != nil {
		return nil, err
	}
	res := &ImportResult{}
	for _, r := range pv.Rows {
		if r.Error != "" {
			res.Skipped++
			continue
		}
		nm, err := a.resolveMember(ctx, m, live, MemberInput{PlanIDs: r.PlanIDs, Phone: r.Phone, PayerName: r.PayerName, Name: r.Name, Fields: r.Fields}, domain.SubInvited, "import")
		if err == nil {
			nm.actor = actor.ID
			_, err = a.invite(ctx, m, live, nm, actor)
		}
		if err != nil {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("Linha %d: %s", r.Line, errText(err)))
			continue
		}
		res.Invited++
	}
	a.Record(ctx, m.ID, actor, "member.import", m.ID, fmt.Sprintf("%d convites, %d ignorados", res.Invited, res.Skipped))
	return res, nil
}

func errText(err error) string {
	if e, ok := err.(*domain.Error); ok {
		return e.Message
	}
	return "erro"
}
