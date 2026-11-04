package service

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"riha/internal/ids"
	"riha/internal/models"
	"riha/internal/providers"
)

type LinkParams struct {
	Name         *string
	Amount       *int64
	CustomAmount *bool
	Reusable     *bool
	Methods      []string
	Active       *bool
	ExpiresAt    *int64
	Slug         *string
	Kind         string
	Location     string
}

func cleanMethods(in []string) (models.StringList, error) {
	out := models.StringList{}
	for _, m := range in {
		m = strings.ToLower(strings.TrimSpace(m))
		if m == "" {
			continue
		}
		if m != providers.MethodMpesa && m != providers.MethodEmola && m != providers.MethodCard {
			return nil, Invalid("payment_method_invalid", "payment_method_types", fmt.Sprintf("Método %q desconhecido.", m))
		}
		if !out.Has(m) {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		out = append(out, providers.AllMethods...)
	}
	return out, nil
}

func (s *Service) uniqueLinkSlug(merchantID, base string) string {
	slug := base
	for i := 2; ; i++ {
		var n int64
		s.DB.Model(&models.PaymentLink{}).Where("merchant_id = ? AND slug = ?", merchantID, slug).Count(&n)
		if n == 0 {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func (s *Service) CreateLink(m *models.Merchant, live bool, p LinkParams, actor, via string) (*models.PaymentLink, error) {
	if e := CanAcceptPayments(m, live); e != nil {
		return nil, e
	}
	name := ""
	if p.Name != nil {
		name = strings.TrimSpace(*p.Name)
	}
	if name == "" {
		return nil, Invalid("parameter_missing", "name", "Indique uma descrição para o link.")
	}
	custom := p.CustomAmount != nil && *p.CustomAmount
	var amount int64
	if !custom {
		if p.Amount == nil || *p.Amount < MinAmount {
			return nil, Invalid("amount_too_small", "amount", "O valor mínimo é 10,00 MT.")
		}
		if *p.Amount > MaxAmount {
			return nil, Invalid("amount_too_large", "amount", "O valor máximo é 1 000 000,00 MT.")
		}
		amount = *p.Amount
	}
	methods, err := cleanMethods(p.Methods)
	if err != nil {
		return nil, err
	}
	base := Slugify(name)
	if p.Slug != nil && *p.Slug != "" {
		base = Slugify(*p.Slug)
	}
	kind := "link"
	if p.Kind == "qr" {
		kind = "qr"
	}
	l := &models.PaymentLink{ID: ids.New("plink"), MerchantID: m.ID, Livemode: live, Name: trim(name, 200), Slug: s.uniqueLinkSlug(m.ID, base),
		Amount: amount, CustomAmount: custom, Reusable: p.Reusable == nil || *p.Reusable, Methods: methods, Active: true,
		Kind: kind, CreatedBy: actor, CreatedVia: via, Location: p.Location}
	if p.ExpiresAt != nil && *p.ExpiresAt > 0 {
		t := time.Unix(*p.ExpiresAt, 0).UTC()
		if t.Before(s.Now()) {
			return nil, Invalid("expires_at_invalid", "expires_at", "A data de expiração já passou.")
		}
		l.ExpiresAt = &t
	}
	if err := s.DB.Create(l).Error; err != nil {
		return nil, err
	}
	return l, nil
}

func (s *Service) UpdateLink(m *models.Merchant, live bool, id string, p LinkParams) (*models.PaymentLink, error) {
	var l models.PaymentLink
	if err := s.DB.First(&l, "id = ? AND merchant_id = ? AND livemode = ?", id, m.ID, live).Error; err != nil {
		return nil, NotFound("O link")
	}
	if p.Name != nil && strings.TrimSpace(*p.Name) != "" {
		l.Name = trim(*p.Name, 200)
	}
	if p.Active != nil {
		l.Active = *p.Active
	}
	if p.Reusable != nil {
		l.Reusable = *p.Reusable
	}
	if p.Methods != nil {
		ms, err := cleanMethods(p.Methods)
		if err != nil {
			return nil, err
		}
		l.Methods = ms
	}
	if p.Amount != nil && !l.CustomAmount {
		if *p.Amount < MinAmount || *p.Amount > MaxAmount {
			return nil, Invalid("amount_invalid", "amount", "Valor fora dos limites.")
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
	if err := s.DB.Save(&l).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *Service) CreateEndpoint(merchantID string, live bool, raw string, events []string) (*models.WebhookEndpoint, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, Invalid("url_invalid", "url", "URL inválido.")
	}
	if live && u.Scheme != "https" {
		return nil, Invalid("url_invalid", "url", "Em produção o endpoint tem de usar https.")
	}
	evs := models.StringList{}
	for _, e := range events {
		ok := e == "*"
		for _, a := range AllEventTypes {
			if a == e {
				ok = true
			}
		}
		if !ok {
			return nil, Invalid("event_invalid", "enabled_events", fmt.Sprintf("Evento %q desconhecido.", e))
		}
		if !evs.Has(e) {
			evs = append(evs, e)
		}
	}
	if len(evs) == 0 {
		evs = models.StringList{"payment.succeeded", "payment.failed", "payout.paid"}
	}
	var n int64
	s.DB.Model(&models.WebhookEndpoint{}).Where("merchant_id = ? AND livemode = ?", merchantID, live).Count(&n)
	if n >= 16 {
		return nil, Invalid("too_many_endpoints", "url", "Máximo de 16 endpoints por modo.")
	}
	ep := &models.WebhookEndpoint{ID: ids.New("we"), MerchantID: merchantID, Livemode: live, URL: u.String(), Events: evs, Active: true}
	return ep, s.DB.Create(ep).Error
}

var domainRe = strings.NewReplacer("https://", "", "http://", "")

// AddDomain registers a domain for callback_url. Domain ownership checks are
// mocked: the domain is marked verified immediately.
func (s *Service) AddDomain(merchantID, raw string) (*models.AllowedDomain, error) {
	d := strings.ToLower(strings.TrimSpace(domainRe.Replace(raw)))
	d = strings.TrimSuffix(strings.SplitN(d, "/", 2)[0], ".")
	if d == "" || !strings.Contains(d, ".") || strings.ContainsAny(d, " :@") {
		return nil, Invalid("domain_invalid", "domain", "Domínio inválido, por exemplo loja.co.mz.")
	}
	now := s.Now()
	ad := &models.AllowedDomain{ID: ids.New("dom"), MerchantID: merchantID, Domain: d, Token: "riha-verify=" + ids.Token("", 20), VerifiedAt: &now}
	var n int64
	s.DB.Model(&models.AllowedDomain{}).Where("merchant_id = ? AND domain = ?", merchantID, d).Count(&n)
	if n > 0 {
		return nil, Conflict("domain_exists", "Este domínio já está na lista.")
	}
	return ad, s.DB.Create(ad).Error
}
