package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"riha/internal/core/domain"
)

func (a *App) CreditSettings(ctx context.Context, merchantID string) domain.CreditSettings {
	if s, err := a.Credit.Settings(ctx, merchantID); err == nil {
		_ = s.Clean()
		return *s
	}
	return domain.DefaultCreditSettings()
}

func (a *App) SaveCreditSettings(ctx context.Context, m *domain.Merchant, in domain.CreditSettings, actor Actor) (domain.CreditSettings, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return in, e
	}
	if e := in.Clean(); e != nil {
		return in, e
	}
	if in.TwoApprovers {
		if n, _ := a.Users.CountByMerchant(ctx, m.ID); n < 2 {
			return in, domain.Invalid("two_approvers_need_team", "two_approvers", "Para pedir duas pessoas a aprovar, a conta precisa de pelo menos duas pessoas.")
		}
	}
	if err := a.Credit.SaveSettings(ctx, m.ID, in); err != nil {
		return in, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.settings_updated", m.ID, "")
	return in, nil
}

// CanDisburse says whether a loan can go out of the RIHA balance to a wallet in this mode.
func (a *App) CanDisburse(live bool) bool {
	if a.Disburser == nil {
		return false
	}
	if c, ok := a.Disburser.(interface{ Available(live bool) bool }); ok {
		return c.Available(live)
	}
	return true
}

type ProductInput struct {
	Name         string   `json:"name"`
	Min          int64    `json:"min"`
	Max          int64    `json:"max"`
	Freq         string   `json:"freq"`
	TermMin      int      `json:"term_min"`
	TermMax      int      `json:"term_max"`
	RateBP       int      `json:"rate_bp"`
	DayMode      string   `json:"day_mode"`
	Mora         bool     `json:"mora"`
	MoraBP       int      `json:"mora_bp"`
	Grace        int      `json:"grace"`
	MoraMaxWeeks int      `json:"mora_max_weeks"`
	EffortPct    int      `json:"effort_pct"`
	Fields       []string `json:"fields"`
	Status       string   `json:"status"`
}

func (in ProductInput) apply(p *domain.CreditProduct) {
	p.Name, p.Min, p.Max, p.Freq, p.TermMin, p.TermMax = in.Name, in.Min, in.Max, in.Freq, in.TermMin, in.TermMax
	p.RateBP, p.DayMode, p.Mora, p.MoraBP, p.Grace, p.MoraMaxWeeks = in.RateBP, in.DayMode, in.Mora, in.MoraBP, in.Grace, in.MoraMaxWeeks
	p.EffortPct, p.Fields = in.EffortPct, in.Fields
	if in.Status == domain.ProductDraft || in.Status == domain.ProductActive {
		p.Status = in.Status
	}
}

func (a *App) CreditProducts(ctx context.Context, m *domain.Merchant, live bool) ([]domain.CreditProduct, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, e
	}
	return a.Credit.Products(ctx, m.ID, live)
}

func (a *App) CreditProduct(ctx context.Context, m *domain.Merchant, live bool, id string) (*domain.CreditProduct, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, e
	}
	p, err := a.Credit.Product(ctx, m.ID, id)
	if err != nil || p.Livemode != live {
		return nil, domain.NotFound("O produto")
	}
	return p, nil
}

func (a *App) CreateCreditProduct(ctx context.Context, m *domain.Merchant, live bool, in ProductInput, actor Actor) (*domain.CreditProduct, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, e
	}
	if ps, _ := a.Credit.Products(ctx, m.ID, live); len(ps) >= 30 {
		return nil, domain.Invalid("too_many_products", "name", "Pode ter até 30 produtos.")
	}
	now := a.Now()
	p := &domain.CreditProduct{ID: domain.NewID("cpr"), MerchantID: m.ID, Livemode: live, Status: domain.ProductActive, CreatedAt: now, UpdatedAt: now}
	in.apply(p)
	if e := p.Validate(); e != nil {
		return nil, e
	}
	if err := a.Credit.CreateProduct(ctx, p); err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.product_created", p.ID, p.Name)
	return p, nil
}

// UpdateCreditProduct changes a product for the requests and loans to come;
// those already offered keep their own terms.
func (a *App) UpdateCreditProduct(ctx context.Context, m *domain.Merchant, live bool, id string, in ProductInput, actor Actor) (*domain.CreditProduct, error) {
	p, err := a.CreditProduct(ctx, m, live, id)
	if err != nil {
		return nil, err
	}
	in.apply(p)
	if e := p.Validate(); e != nil {
		return nil, e
	}
	p.UpdatedAt = a.Now()
	if err := a.Credit.SaveProduct(ctx, p); err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.product_updated", p.ID, p.Name)
	return p, nil
}

func (a *App) creditMerchant(ctx context.Context, slug string) (*domain.Merchant, error) {
	m, err := a.Merchants.BySlug(ctx, strings.ToLower(strings.TrimSpace(slug)))
	if err != nil || m.Status != domain.MerchantActive || !m.HasMicrocredit() {
		return nil, domain.NotFound("Esta página")
	}
	return m, nil
}

// PublicCreditProducts: the active products a customer can ask for, the live ones
// when there are any.
func (a *App) PublicCreditProducts(ctx context.Context, slug string) (*domain.Merchant, []domain.CreditProduct, bool, error) {
	m, err := a.creditMerchant(ctx, slug)
	if err != nil {
		return nil, nil, false, err
	}
	for _, live := range []bool{true, false} {
		var out []domain.CreditProduct
		ps, _ := a.Credit.Products(ctx, m.ID, live)
		for _, p := range ps {
			if p.Status == domain.ProductActive {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return m, out, live, nil
		}
	}
	return nil, nil, false, domain.NotFound("Esta página")
}

// creditText queues a message to a borrower. Reminders wait for the institution's
// hours; codes and answers to what the customer just did go at once.
func (a *App) creditText(ctx context.Context, m *domain.Merchant, live bool, ref, instalmentID, phone, kind, body string, now bool) error {
	s := a.CreditSettings(ctx, m.ID)
	msg := &domain.Message{ID: domain.NewID("msg"), MerchantID: m.ID, Livemode: live, MemberID: ref, ChargeID: instalmentID,
		Channel: domain.ChannelSMS, To: domain.NormalizeMZPhone(phone), Sender: s.SenderID, Kind: kind, Body: domain.Trim(body, 1000), Status: "queued", CreatedAt: a.Now()}
	if !now {
		if at := s.SendWindow(a.Now()); at.After(a.Now()) {
			msg.NextAttemptAt = &at
		}
	}
	return a.Messages.Create(ctx, msg)
}

func (a *App) creditURL(token string) string { return strings.TrimRight(a.PayURL, "/") + "/c/" + token }

// sendCreditCode sends a 6-digit code by SMS. A number gets at most 5 codes an
// hour, and a new one replaces the last.
func (a *App) sendCreditCode(ctx context.Context, m *domain.Merchant, live bool, subject, purpose, phone, what string) (string, error) {
	phone = domain.NormalizeMZPhone(phone)
	if n, _ := a.Credit.CodesSent(ctx, phone, a.Now().Add(-time.Hour)); n >= 5 {
		return "", domain.Conflict("too_many_codes", "Já enviámos vários códigos para este número. Tente de novo daqui a uma hora.")
	}
	if c, err := a.Credit.LatestCode(ctx, subject, purpose); err == nil && c.UsedAt == nil && a.Now().Sub(c.CreatedAt) < 45*time.Second {
		return "", domain.Conflict("code_just_sent", "Acabámos de enviar um código. Espere um pouco antes de pedir outro.")
	}
	code := domain.CreditCodeDigits()
	now := a.Now()
	c := &domain.CreditCode{ID: domain.NewID("ccd"), Subject: subject, Purpose: purpose, Phone: phone, Hash: domain.Hash(subject + ":" + code),
		ExpiresAt: now.Add(domain.CodeTTL), CreatedAt: now}
	if err := a.Credit.CreateCode(ctx, c); err != nil {
		return "", err
	}
	body := code + " é o seu código RIHA para " + what + " com " + m.Name + ". Não o partilhe com ninguém, nem com quem diga ser da " + m.Name + "."
	if err := a.creditText(ctx, m, live, subject, "", phone, "credit_code", body, true); err != nil {
		return "", err
	}
	a.SendSoon(ctx)
	if live {
		return "", nil
	}
	return code, nil
}

// checkCreditCode uses up a code: right once, within its time, in at most 5 tries.
func (a *App) checkCreditCode(ctx context.Context, subject, purpose, code string) error {
	c, err := a.Credit.LatestCode(ctx, subject, purpose)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Invalid("code_missing", "code", "Peça um código primeiro.")
	}
	if err != nil {
		return err
	}
	switch {
	case c.UsedAt != nil:
		return domain.Invalid("code_used", "code", "Este código já foi usado. Peça outro.")
	case a.Now().After(c.ExpiresAt):
		return domain.Invalid("code_expired", "code", "O código expirou. Peça outro.")
	case c.Tries >= domain.CodeMaxTries:
		return domain.Invalid("code_locked", "code", "Errou o código vezes demais. Peça outro.")
	}
	if domain.Hash(subject+":"+strings.TrimSpace(code)) != c.Hash {
		c.Tries++
		_ = a.Credit.SaveCode(ctx, c)
		return domain.ErrCodeWrong
	}
	now := a.Now()
	c.UsedAt = &now
	return a.Credit.SaveCode(ctx, c)
}
