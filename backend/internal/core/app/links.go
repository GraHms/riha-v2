package app

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type LinkParams struct {
	Name            *string
	Amount          *int64
	CustomAmount    *bool
	Reusable        *bool
	Methods         []string
	Active          *bool
	ExpiresAt       *int64
	Slug            *string
	Kind            string
	Location        string
	Details         *string
	SuggestedAmount *int64
	MinAmount       *int64
	MaxAmount       *int64
	AfterMessage    *string
	RedirectURL     *string
	Archived        *bool
	SubmitType      *string
}

// applyExtras sets what a customer reads before and after paying, and the
// range of a chosen amount. Only what the request carries changes.
func (a *App) applyExtras(l *domain.PaymentLink, p LinkParams) error {
	if p.Details != nil {
		l.Details = domain.Trim(strings.TrimSpace(*p.Details), 500)
	}
	if p.SubmitType != nil {
		t, err := domain.CleanSubmitType(*p.SubmitType)
		if err != nil {
			return err
		}
		l.SubmitType = t
	}
	if p.AfterMessage != nil {
		l.AfterMessage = domain.Trim(strings.TrimSpace(*p.AfterMessage), 300)
	}
	if p.RedirectURL != nil {
		u, err := validURL(*p.RedirectURL, "redirect_url", l.Livemode, false)
		if err != nil {
			return err
		}
		l.RedirectURL = u
	}
	if l.AfterMessage != "" && l.RedirectURL != "" {
		return domain.Invalid("parameter_invalid", "redirect_url", "Escolha mostrar uma mensagem ou enviar o cliente para o seu site, não os dois.")
	}
	set := func(dst *int64, v *int64) {
		if v != nil {
			*dst = *v
		}
	}
	set(&l.SuggestedAmount, p.SuggestedAmount)
	set(&l.MinCustom, p.MinAmount)
	set(&l.MaxCustom, p.MaxAmount)
	if !l.CustomAmount {
		l.SuggestedAmount, l.MinCustom, l.MaxCustom = 0, 0, 0
		return nil
	}
	for _, v := range []struct {
		n     int64
		field string
	}{{l.MinCustom, "min_amount"}, {l.MaxCustom, "max_amount"}, {l.SuggestedAmount, "suggested_amount"}} {
		if v.n < 0 || (v.n > 0 && (v.n < domain.MinAmount || v.n > domain.MaxAmount)) {
			return domain.Invalid("amount_invalid", v.field, fmt.Sprintf("Os valores têm de estar entre %s e %s MT.", domain.FormatShort(domain.MinAmount), domain.FormatShort(domain.MaxAmount)))
		}
	}
	lo, hi := l.CustomRange()
	if lo >= hi {
		return domain.Invalid("amount_invalid", "max_amount", "O valor máximo tem de ser maior do que o mínimo.")
	}
	if l.SuggestedAmount > 0 && (l.SuggestedAmount < lo || l.SuggestedAmount > hi) {
		return domain.Invalid("amount_invalid", "suggested_amount", "O valor sugerido tem de estar entre o mínimo e o máximo.")
	}
	return nil
}

type LinkView struct {
	domain.PaymentLink
	ports.LinkStats
}

func (a *App) uniqueLinkSlug(ctx context.Context, merchantID, base string) string {
	slug := base
	for i := 2; ; i++ {
		if taken, _ := a.Links.SlugTaken(ctx, merchantID, slug); !taken {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func (a *App) CreateLink(ctx context.Context, m *domain.Merchant, live bool, p LinkParams, actor, via string) (*domain.PaymentLink, error) {
	if e := domain.RequireApproved(m); e != nil {
		return nil, e
	}
	name := ""
	if p.Name != nil {
		name = strings.TrimSpace(*p.Name)
	}
	if name == "" {
		return nil, domain.Invalid("parameter_missing", "name", "Indique uma descrição para o link.")
	}
	custom := p.CustomAmount != nil && *p.CustomAmount
	var amount int64
	if !custom {
		if p.Amount == nil || *p.Amount < domain.MinAmount {
			return nil, domain.Invalid("amount_too_small", "amount", "O valor mínimo é 10,00 MT.")
		}
		if *p.Amount > domain.MaxAmount {
			return nil, domain.Invalid("amount_too_large", "amount", "O valor máximo é 1 000 000,00 MT.")
		}
		amount = *p.Amount
	}
	methods, err := a.cleanMethods(p.Methods)
	if err != nil {
		return nil, err
	}
	base := domain.Slugify(name)
	if p.Slug != nil && *p.Slug != "" {
		base = domain.Slugify(*p.Slug)
	}
	kind := "link"
	if p.Kind == "qr" {
		kind = "qr"
	}
	now := a.Now()
	l := &domain.PaymentLink{ID: domain.NewID("plink"), MerchantID: m.ID, Livemode: live, Name: domain.Trim(name, 200), Slug: a.uniqueLinkSlug(ctx, m.ID, base),
		Amount: amount, CustomAmount: custom, Reusable: p.Reusable == nil || *p.Reusable, Methods: methods, Active: true,
		Kind: kind, CreatedBy: actor, CreatedVia: via, Location: p.Location, CreatedAt: now}
	if err := a.applyExtras(l, p); err != nil {
		return nil, err
	}
	if p.ExpiresAt != nil && *p.ExpiresAt > 0 {
		t := time.Unix(*p.ExpiresAt, 0).UTC()
		if t.Before(now) {
			return nil, domain.Invalid("expires_at_invalid", "expires_at", "A data de expiração já passou.")
		}
		l.ExpiresAt = &t
	}
	return l, a.Links.Create(ctx, l)
}

func (a *App) merchantLink(ctx context.Context, merchantID string, live bool, id string) (*domain.PaymentLink, error) {
	l, err := a.Links.ByID(ctx, id)
	if err != nil || l.MerchantID != merchantID || l.Livemode != live {
		return nil, domain.NotFound("O link")
	}
	return l, nil
}

func (a *App) UpdateLink(ctx context.Context, m *domain.Merchant, live bool, id string, p LinkParams) (*domain.PaymentLink, error) {
	if e := domain.RequireApproved(m); e != nil {
		return nil, e
	}
	l, err := a.merchantLink(ctx, m.ID, live, id)
	if err != nil {
		return nil, err
	}
	if p.Name != nil && strings.TrimSpace(*p.Name) != "" {
		l.Name = domain.Trim(*p.Name, 200)
	}
	if p.Archived != nil {
		l.Archived = *p.Archived
		// Archiving switches the link off. Restoring leaves it off until asked.
		if l.Archived {
			l.Active = false
		}
	}
	if p.Active != nil && !l.Archived {
		l.Active = *p.Active
	}
	if p.Reusable != nil {
		l.Reusable = *p.Reusable
	}
	if p.Methods != nil {
		if l.Methods, err = a.cleanMethods(p.Methods); err != nil {
			return nil, err
		}
	}
	if p.Amount != nil && !l.CustomAmount {
		if *p.Amount < domain.MinAmount || *p.Amount > domain.MaxAmount {
			return nil, domain.Invalid("amount_invalid", "amount", "Valor fora dos limites.")
		}
		l.Amount = *p.Amount
	}
	if p.ExpiresAt != nil {
		if *p.ExpiresAt == 0 {
			l.ExpiresAt = nil
		} else {
			t := time.Unix(*p.ExpiresAt, 0).UTC()
			l.ExpiresAt = &t
		}
	}
	if err := a.applyExtras(l, p); err != nil {
		return nil, err
	}
	return l, a.Links.Save(ctx, l)
}

func (a *App) ListLinks(ctx context.Context, merchantID string, live bool, limit int) ([]LinkView, error) {
	ls, err := a.Links.List(ctx, merchantID, live, limit)
	if err != nil {
		return nil, err
	}
	out := make([]LinkView, 0, len(ls))
	for _, l := range ls {
		st, _ := a.Links.Stats(ctx, l.ID)
		out = append(out, LinkView{l, st})
	}
	return out, nil
}

// MerchantLink is one of a merchant's links, with its figures.
func (a *App) MerchantLink(ctx context.Context, merchantID string, live bool, id string) (LinkView, error) {
	l, err := a.merchantLink(ctx, merchantID, live, id)
	if err != nil {
		return LinkView{}, err
	}
	return a.LinkView(ctx, l), nil
}

func (a *App) LinkView(ctx context.Context, l *domain.PaymentLink) LinkView {
	st, _ := a.Links.Stats(ctx, l.ID)
	return LinkView{*l, st}
}

func (a *App) PublicLink(ctx context.Context, merchantSlug, slug string) (*domain.PaymentLink, *domain.Merchant, error) {
	m, err := a.Merchants.BySlug(ctx, merchantSlug)
	if err != nil {
		return nil, nil, domain.NotFound("O link")
	}
	l, err := a.Links.BySlug(ctx, m.ID, slug)
	if err != nil {
		return nil, nil, domain.NotFound("O link")
	}
	return l, m, nil
}

func (a *App) CreateEndpoint(ctx context.Context, merchantID string, live bool, raw string, events []string) (*domain.WebhookEndpoint, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, domain.Invalid("url_invalid", "url", "URL inválido.")
	}
	if live && u.Scheme != "https" {
		return nil, domain.Invalid("url_invalid", "url", "Em produção o endpoint tem de usar https.")
	}
	if !a.AllowPrivateNetworks && privateHost(u.Hostname()) {
		return nil, domain.Invalid("url_not_allowed", "url", "O endpoint tem de ser um endereço público.")
	}
	evs := []string{}
	for _, e := range events {
		if !domain.ValidEvent(e) {
			return nil, domain.Invalid("event_invalid", "enabled_events", fmt.Sprintf("Evento %q desconhecido.", e))
		}
		if !domain.Contains(evs, e) {
			evs = append(evs, e)
		}
	}
	if len(evs) == 0 {
		evs = []string{"payment.succeeded", "payment.failed", "payout.paid"}
	}
	existing, _ := a.Endpoints.List(ctx, merchantID, live)
	if len(existing) >= 16 {
		return nil, domain.Invalid("too_many_endpoints", "url", "Máximo de 16 endpoints por modo.")
	}
	ep := &domain.WebhookEndpoint{ID: domain.NewID("we"), MerchantID: merchantID, Livemode: live, URL: u.String(), Events: evs, Active: true, CreatedAt: a.Now()}
	return ep, a.Endpoints.Create(ctx, ep)
}

func (a *App) UpdateEndpoint(ctx context.Context, merchantID, id string, events []string, active *bool) error {
	ep, err := a.Endpoints.ByID(ctx, merchantID, id)
	if err != nil {
		return domain.NotFound("O endpoint")
	}
	if events != nil {
		evs := []string{}
		for _, e := range events {
			if domain.Contains(domain.AllEventTypes, e) && !domain.Contains(evs, e) {
				evs = append(evs, e)
			}
		}
		ep.Events = evs
	}
	if active != nil {
		ep.Active = *active
	}
	return a.Endpoints.Save(ctx, ep)
}

func (a *App) DeleteEndpoint(ctx context.Context, merchantID string, live *bool, id string) (bool, error) {
	return a.Endpoints.Delete(ctx, merchantID, live, id)
}

func (a *App) ListEndpoints(ctx context.Context, merchantID string, live bool) ([]domain.WebhookEndpoint, error) {
	return a.Endpoints.List(ctx, merchantID, live)
}

func privateHost(h string) bool {
	h = strings.ToLower(strings.TrimSuffix(h, "."))
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified())
}

var schemeStripper = strings.NewReplacer("https://", "", "http://", "")

func (a *App) AddDomain(ctx context.Context, merchantID, raw string) (*domain.AllowedDomain, error) {
	d := strings.ToLower(strings.TrimSpace(schemeStripper.Replace(raw)))
	d = strings.TrimSuffix(strings.SplitN(d, "/", 2)[0], ".")
	if d == "" || !strings.Contains(d, ".") || strings.ContainsAny(d, " :@") {
		return nil, domain.Invalid("domain_invalid", "domain", "Domínio inválido, por exemplo loja.co.mz.")
	}
	doms, _ := a.Domains.List(ctx, merchantID)
	for _, x := range doms {
		if x.Domain == d {
			return nil, domain.Conflict("domain_exists", "Este domínio já está na lista.")
		}
	}
	now := a.Now()
	ad := &domain.AllowedDomain{ID: domain.NewID("dom"), MerchantID: merchantID, Domain: d, Token: "riha-verify=" + domain.Token("", 20), VerifiedAt: &now, CreatedAt: now}
	return ad, a.Domains.Create(ctx, ad)
}
