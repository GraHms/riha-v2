package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"image"
	"net/http"
	"strings"

	"riha/internal/adapters/ogcard"
	"riha/internal/core/domain"
)

// A merchant's catalog and plans are pages they want found and shared, so they
// are indexed, unless they only have test plans.

const subsMethods = "M-Pesa, e-Mola ou Bim IZI"

func price(p *domain.Plan) string {
	return domain.FormatShort(p.Amount) + " MT / " + domain.IntervalLabel(p.Interval)
}

func isoDuration(interval string) string {
	switch interval {
	case domain.IntervalWeek:
		return "P1W"
	case domain.IntervalQuarter:
		return "P3M"
	case domain.IntervalYear:
		return "P1Y"
	}
	return "P1M"
}

func (srv *Server) offer(p *domain.Plan, url string) domain.M {
	amount := fmt.Sprintf("%d.%02d", p.Amount/100, p.Amount%100)
	o := domain.M{"@type": "Offer", "name": p.Name, "url": url, "price": amount, "priceCurrency": "MZN", "availability": "https://schema.org/InStock",
		"priceSpecification": domain.M{"@type": "UnitPriceSpecification", "price": amount, "priceCurrency": "MZN", "billingDuration": isoDuration(p.Interval), "unitText": domain.IntervalLabel(p.Interval)}}
	if len(p.Highlights) > 0 {
		o["description"] = strings.Join(p.Highlights, ". ")
	}
	return o
}

func (srv *Server) catalogMeta(c context.Context, slug string) pageMeta {
	m, plans, set, err := srv.A.PublicCatalog(c, slug)
	if err != nil {
		return pageMeta{status: http.StatusNotFound, title: "Catálogo não encontrado · RIHA", desc: "Este catálogo não existe ou ainda não tem planos.", image: "/og/site.png", alt: siteTitle}
	}
	l := srv.links(m, set)
	title := m.Name + ": planos e mensalidades"
	if set.CatalogTitle != "" && set.CatalogTitle != m.Name {
		title = set.CatalogTitle + " · " + m.Name
	}
	var names []string
	for _, p := range plans {
		names = append(names, p.Name+" ("+price(&p)+")")
	}
	desc := set.CatalogDescription
	if desc == "" {
		desc = "Subscreva e pague a " + m.Name + " por " + subsMethods + ": " + strings.Join(names, ", ") + "."
	}
	offers := []any{}
	var items strings.Builder
	for _, p := range plans {
		u := l.plan(p.Slug)
		offers = append(offers, srv.offer(&p, u))
		fmt.Fprintf(&items, `<li><a href="%s">%s</a>: %s</li>`, html.EscapeString(u), html.EscapeString(p.Name), html.EscapeString(price(&p)))
	}
	img := fmt.Sprintf("/og/a/%s.%s?v=%s", l.Slug, imgExt(set), version(title, desc, set.CatalogCover, set.CatalogAccent, fmt.Sprint(len(plans), plans[0].Amount)))
	return pageMeta{status: 200, title: title, desc: desc, image: img, imageType: imgType(set), alt: title, canonical: l.URL, index: plans[0].Livemode,
		ld: domain.M{"@context": "https://schema.org", "@type": "OfferCatalog", "name": domain.FirstNonEmpty(set.CatalogTitle, m.Name), "url": l.URL,
			"description": desc, "provider": domain.M{"@type": "Organization", "name": m.Name}, "itemListElement": offers},
		noscript: "<h1>" + html.EscapeString(domain.FirstNonEmpty(set.CatalogTitle, m.Name)) + "</h1><p>" + html.EscapeString(desc) + "</p><ul>" + items.String() + "</ul>"}
}

func (srv *Server) planMeta(c context.Context, slug, planSlug string) pageMeta {
	m, p, err := srv.A.PublicPlan(c, slug, planSlug)
	if err != nil {
		return pageMeta{status: http.StatusNotFound, title: "Plano não encontrado · RIHA", desc: "Este plano não existe ou já não está disponível.", image: "/og/site.png", alt: siteTitle}
	}
	set := srv.A.SubSettingsFor(c, m)
	l := srv.links(m, set)
	title := p.Name + " · " + m.Name
	desc := price(p) + ". " + p.CustomerWhen(srv.A.Now()) + "."
	if d := p.DurationLabel(); d != "" {
		desc += " " + d + "."
	}
	if len(p.Highlights) > 0 {
		desc = strings.Join(p.Highlights, ". ") + ". " + desc
	}
	desc += " Pague por " + subsMethods + "."
	img := fmt.Sprintf("/og/s/%s/%s.%s?v=%s", l.Slug, p.Slug, imgExt(set), version(p.Name, m.Name, fmt.Sprint(p.Amount, p.Interval), set.CatalogCover, set.CatalogAccent))
	return pageMeta{status: 200, title: title, desc: desc, image: img, imageType: imgType(set), alt: title, canonical: l.plan(p.Slug), index: p.Livemode,
		ld:       domain.M{"@context": "https://schema.org", "@type": "Product", "name": p.Name, "brand": domain.M{"@type": "Organization", "name": m.Name}, "offers": srv.offer(p, l.plan(p.Slug))},
		noscript: "<h1>" + html.EscapeString(p.Name) + "</h1><p>" + html.EscapeString(desc) + "</p>"}
}

func imgExt(s domain.SubscriptionSettings) string {
	if s.CatalogCover != "" {
		return "jpg"
	}
	return "png"
}

func imgType(s domain.SubscriptionSettings) string {
	if s.CatalogCover != "" {
		return "image/jpeg"
	}
	return "image/png"
}

func (srv *Server) brand(c context.Context, slug string, s domain.SubscriptionSettings) (image.Image, *ogcard.Card) {
	card := &ogcard.Card{Accent: ogcard.ParseColor(s.CatalogAccent)}
	if s.CatalogCover == "" {
		return nil, card
	}
	b, err := srv.A.CatalogCover(c, slug)
	if err != nil {
		return nil, card
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, card
	}
	card.Cover = img
	return img, card
}

func (srv *Server) catalogCard(c context.Context, slug string) (ogcard.Card, string, bool) {
	m, plans, set, err := srv.A.PublicCatalog(c, slug)
	if err != nil {
		return ogcard.Card{}, "", false
	}
	_, card := srv.brand(c, slug, set)
	low := plans[0]
	for _, p := range plans {
		if domain.MonthlyEquivalent(p.Amount, p.Interval) < domain.MonthlyEquivalent(low.Amount, low.Interval) {
			low = p
		}
	}
	card.Kicker = m.Name
	card.Title = domain.FirstNonEmpty(set.CatalogTitle, "Planos e mensalidades")
	n := fmt.Sprintf("%d planos", len(plans))
	if len(plans) == 1 {
		n = plans[0].Name
	}
	card.Body = n + " · M-Pesa, e-Mola ou Bim IZI"
	card.Big = price(&low)
	if len(plans) > 1 {
		// "Desde 800 MT / mês" doesn't fit: the period goes in the line above.
		card.Big = "Desde " + domain.FormatShort(low.Amount) + " MT"
		card.Body = n + " · desde " + domain.FormatShort(low.Amount) + " MT por " + domain.IntervalLabel(low.Interval)
	}
	card.Foot = strings.TrimPrefix(strings.TrimPrefix(srv.links(m, set).URL, "https://"), "http://")
	return *card, set.CatalogCover + set.CatalogAccent, true
}

func (srv *Server) planCard(c context.Context, slug, planSlug string) (ogcard.Card, string, bool) {
	m, p, err := srv.A.PublicPlan(c, slug, planSlug)
	if err != nil {
		return ogcard.Card{}, "", false
	}
	set := srv.A.SubSettingsFor(c, m)
	_, card := srv.brand(c, slug, set)
	card.Kicker = m.Name
	card.Title = p.Name
	card.Body = p.CustomerWhen(srv.A.Now())
	if d := p.DurationLabel(); d != "" {
		card.Body += " · " + d
	}
	card.Big = price(p)
	card.Foot = strings.TrimPrefix(strings.TrimPrefix(srv.links(m, set).plan(p.Slug), "https://"), "http://")
	return *card, set.CatalogCover + set.CatalogAccent, true
}

// catalogURLs lists the catalogs with live plans, for the sitemap.
func (srv *Server) catalogURLs(c context.Context) []string {
	ids, _ := srv.A.Plans.CatalogMerchants(c, 2000)
	var out []string
	for _, id := range ids {
		m, err := srv.A.Merchants.ByID(c, id)
		if err != nil || m.Status != domain.MerchantActive {
			continue
		}
		out = append(out, srv.linksFor(c, m).URL)
	}
	return out
}
