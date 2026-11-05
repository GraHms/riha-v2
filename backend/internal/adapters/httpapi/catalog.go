package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

// A merchant's catalog lives at an address of their choosing. With subdomains
// on, that is escola.rihagateway.com, and its plans are escola.rihagateway.com/propina.
// Without, it is rihagateway.com/a/escola. The older addresses keep working
// either way, because they are printed on QR codes.

type catalogLinks struct {
	Slug    string
	URL     string
	sub     bool
	base    string
	account string
}

func (l catalogLinks) plan(slug string) string {
	if l.sub {
		return l.URL + "/" + slug
	}
	return domain.PlanURL(l.base, l.account, slug)
}

// publicBase is the scheme and host of the public site, such as https://rihagateway.com.
func (srv *Server) publicBase() (scheme, host string) {
	u, err := url.Parse(srv.A.PublicURL)
	if err != nil || u.Host == "" {
		return "https", "rihagateway.com"
	}
	return u.Scheme, strings.ToLower(u.Host)
}

func (srv *Server) links(m *domain.Merchant, s domain.SubscriptionSettings) catalogLinks {
	slug := app.CatalogSlug(m, s)
	base := strings.TrimRight(srv.A.PublicURL, "/")
	l := catalogLinks{Slug: slug, URL: base + "/a/" + slug, base: base, account: m.Slug}
	if _, err := domain.CleanCatalogSlug(slug); srv.cfg.CatalogSubdomains && err == nil {
		scheme, host := srv.publicBase()
		l.URL, l.sub = scheme+"://"+slug+"."+host, true
	}
	return l
}

func (srv *Server) linksFor(c context.Context, m *domain.Merchant) catalogLinks {
	return srv.links(m, srv.A.SubSettingsFor(c, m))
}

// catalogHost reads the catalog's address from a subdomain of the public site.
func (srv *Server) catalogHost(host string) string {
	_, base := srv.publicBase()
	host = strings.ToLower(host)
	if !strings.HasSuffix(host, "."+base) {
		return ""
	}
	sub := strings.TrimSuffix(host, "."+base)
	if strings.Contains(sub, ".") {
		return ""
	}
	if _, err := domain.CleanCatalogSlug(sub); err != nil {
		return ""
	}
	return sub
}

func (srv *Server) catalogPageJSON(m *domain.Merchant, s domain.SubscriptionSettings) gin.H {
	l := srv.links(m, s)
	cover := ""
	if s.CatalogCover != "" {
		cover = "/api/public/subscriptions/cover/" + l.Slug + "?v=" + version(s.CatalogCover)
	}
	return gin.H{"slug": l.Slug, "url": l.URL, "subdomain": l.sub, "title": domain.FirstNonEmpty(s.CatalogTitle, m.Name), "custom_title": s.CatalogTitle,
		"description": s.CatalogDescription, "cover_url": cover, "accent": s.CatalogAccent, "background": s.CatalogBackground, "multi": s.CatalogMulti}
}

func (srv *Server) getCatalog(c *gin.Context) {
	m := merchant(c)
	_, host := srv.publicBase()
	out := srv.catalogPageJSON(m, srv.A.SubSettingsFor(ctx(c), m))
	out["account_slug"] = m.Slug
	out["host"] = host
	c.JSON(200, out)
}

func (srv *Server) checkCatalogSlug(c *gin.Context) {
	m := merchant(c)
	v := strings.ToLower(strings.TrimSpace(c.Query("value")))
	if v == m.Slug {
		c.JSON(200, gin.H{"ok": true, "slug": v})
		return
	}
	slug, err := srv.A.CatalogSlugFree(ctx(c), m, v)
	if err != nil {
		msg := err.Error()
		if e, ok := err.(*domain.Error); ok {
			msg = e.Message
		}
		c.JSON(200, gin.H{"ok": false, "slug": slug, "message": msg})
		return
	}
	c.JSON(200, gin.H{"ok": true, "slug": slug})
}

func (srv *Server) putCatalogCover(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		bad(c, "Escolha uma imagem.")
		return
	}
	f, err := fh.Open()
	if err != nil {
		bad(c, "Não foi possível ler a imagem.")
		return
	}
	defer f.Close()
	m := merchant(c)
	s, err := srv.A.SaveCatalogCover(ctx(c), m, f, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.catalogPageJSON(m, s))
}

func (srv *Server) deleteCatalogCover(c *gin.Context) {
	m := merchant(c)
	s, err := srv.A.RemoveCatalogCover(ctx(c), m, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, srv.catalogPageJSON(m, s))
}

func (srv *Server) publicCover(c *gin.Context) {
	b, err := srv.A.CatalogCover(ctx(c), c.Param("slug"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	// The address carries a version, so the picture can be kept for long.
	c.Header("Cache-Control", "public, max-age=2592000, immutable")
	c.Data(200, "image/jpeg", b)
}

// tlsAllowed answers Caddy, which asks before getting a certificate for a
// subdomain: only addresses of catalogs with plans to show get one.
func (srv *Server) tlsAllowed(c *gin.Context) {
	slug := srv.catalogHost(c.Query("domain"))
	if slug == "" {
		c.Status(http.StatusNotFound)
		return
	}
	if _, _, _, err := srv.A.PublicCatalog(ctx(c), slug); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Status(http.StatusOK)
}

// customerPlanJSON says what a customer joining today needs to know.
func (srv *Server) customerPlanJSON(p *domain.Plan) gin.H {
	out := publicPlanJSON(p)
	now := srv.A.Now()
	out["highlights"] = nonNil(p.Highlights)
	out["cycles"] = p.Cycles
	out["duration_label"] = p.DurationLabel()
	first, ok := domain.FirstDue(p, now)
	if !ok {
		return out
	}
	out["when_text"] = p.CustomerWhen(first)
	out["first_period"] = strings.ToLower(domain.PeriodLabel(p, first))
	today := domain.LocalDate(now)
	out["first_today"] = !first.After(today)
	n := 4
	if p.Cycles > 0 && p.Cycles <= 6 {
		n = p.Cycles
	}
	dates := []gin.H{}
	for i, d := range domain.Schedule(p, first, n) {
		label := domain.ShortDate(d)
		if i == 0 && !d.After(today) {
			label = "Hoje"
		}
		dates = append(dates, gin.H{"label": label, "period": strings.ToLower(domain.PeriodLabel(p, d)), "year": d.In(domain.Maputo).Year()})
	}
	out["dates"] = dates
	if p.Months != domain.AllMonths {
		out["months_off"] = domain.MonthsLabel(domain.AllMonths &^ p.Months)
	}
	return out
}

var _ = time.Second
