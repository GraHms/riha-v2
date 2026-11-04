package httpapi

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/adapters/ogcard"
	"riha/internal/core/domain"
)

// Apps that show a link preview, and most crawlers, read the HTML without running
// the site. So the server writes each page's title, description and image into it.

const (
	siteName     = "RIHA"
	siteTitle    = "RIHA: pagamentos online em Moçambique com M-Pesa"
	siteDesc     = "Receba por M-Pesa com links de pagamento por WhatsApp e SMS, código QR ou API. Sem mensalidade nem taxa de adesão. Levantamentos para o banco em até 72 horas."
	seoHeadOpen  = "<!--seo:head-->"
	seoHeadClose = "<!--/seo:head-->"
	seoBody      = "<!--seo:body-->"
)

type pageMeta struct {
	status      int
	title, desc string
	image       string // a path under /og/
	alt         string
	path        string // where the page lives, for the canonical address
	index       bool
	ld          any
	noscript    string // what a reader without JavaScript gets
}

func (srv *Server) url(path string) string {
	return strings.TrimRight(srv.A.PublicURL, "/") + path
}

func (srv *Server) metaFor(c context.Context, path string) pageMeta {
	private := func(title string) pageMeta {
		return pageMeta{status: 200, title: title + " · RIHA", desc: siteDesc, image: "/og/site.png", alt: siteTitle}
	}
	seg := strings.Split(strings.Trim(path, "/"), "/")
	switch {
	case path == "/":
		return pageMeta{status: 200, title: siteTitle, desc: siteDesc, image: "/og/site.png", alt: siteTitle, path: "/", index: true,
			ld: []any{
				domain.M{"@context": "https://schema.org", "@type": "Organization", "name": siteName, "url": srv.url("/"), "logo": srv.url("/assets/riha-icon-dark.png"),
					"contactPoint": domain.M{"@type": "ContactPoint", "telephone": "+258848255237", "contactType": "customer support", "areaServed": "MZ", "availableLanguage": "Portuguese"}},
				domain.M{"@context": "https://schema.org", "@type": "WebSite", "name": siteName, "url": srv.url("/"), "inLanguage": "pt-MZ"},
				domain.M{"@context": "https://schema.org", "@type": "Service", "name": "Pagamentos online com M-Pesa", "serviceType": "Gateway de pagamentos", "provider": domain.M{"@type": "Organization", "name": siteName}, "areaServed": domain.M{"@type": "Country", "name": "Moçambique"}},
			},
			noscript: `<h1>Pagamentos online em Moçambique com M-Pesa</h1><p>` + html.EscapeString(siteDesc) + `</p><ul><li><a href="/registo">Criar conta</a></li><li><a href="/docs">Documentação da API</a></li></ul>`}
	case path == "/docs":
		title := "API de pagamentos M-Pesa para Moçambique: documentação · RIHA"
		desc := "Integre pagamentos M-Pesa na sua loja: checkout alojado, links de pagamento, webhooks assinados e modo de teste, com exemplos em cURL, Node, PHP e Python."
		return pageMeta{status: 200, title: title, desc: desc, image: "/og/docs.png", alt: title, path: "/docs", index: true,
			ld:       domain.M{"@context": "https://schema.org", "@type": "TechArticle", "headline": title, "description": desc, "url": srv.url("/docs"), "inLanguage": "pt-MZ", "publisher": domain.M{"@type": "Organization", "name": siteName}},
			noscript: `<h1>Documentação da API RIHA</h1><p>` + html.EscapeString(desc) + `</p>`}
	case path == "/registo":
		title := "Criar conta para receber pagamentos M-Pesa · RIHA"
		return pageMeta{status: 200, title: title, desc: "Crie a conta do seu negócio em menos de 3 minutos e comece a receber por M-Pesa depois de aprovado. Sem mensalidade nem taxa de adesão.", image: "/og/site.png", alt: title, path: "/registo", index: true}
	case path == "/entrar":
		return private("Entrar")
	case seg[0] == "recuperar", seg[0] == "verificacao", seg[0] == "painel", seg[0] == "admin":
		return private("Conta")
	case seg[0] == "p" && len(seg) == 3:
		return srv.linkMeta(c, seg[1], seg[2])
	case seg[0] == "c" && len(seg) == 2:
		return srv.sessionMeta(c, seg[1])
	// These pages name a customer or belong to whoever was sent the link, so they
	// stay out of search and their previews say nothing about the person.
	case seg[0] == "i" && len(seg) == 2:
		return private("Pagamento")
	case (seg[0] == "s" && len(seg) == 3) || (seg[0] == "a" && len(seg) == 2):
		return private("Assinatura")
	}
	m := private("Página não encontrada")
	m.status = http.StatusNotFound
	return m
}

func money(c int64) string { return domain.Format(c) + " MT" }

func paymentDesc(what, merchant string) string {
	if what != "" {
		what += ". "
	}
	return what + "Pague a " + merchant + " por M-Pesa: sem app e sem criar conta, confirma com o seu PIN."
}

// Payment links stay out of search results: they belong to whoever was sent one.
func (srv *Server) linkMeta(c context.Context, merchant, slug string) pageMeta {
	l, m, err := srv.A.PublicLink(c, merchant, slug)
	if err != nil {
		missing := pageMeta{status: http.StatusNotFound, title: "Link não encontrado · RIHA", desc: "Este link de pagamento não existe. Confirme-o com quem o enviou.", image: "/og/site.png", alt: siteTitle}
		return missing
	}
	if !l.Active || (l.ExpiresAt != nil && l.ExpiresAt.Before(time.Now())) {
		return pageMeta{status: 200, title: "Link de pagamento indisponível · " + m.Name, desc: "Este link de pagamento de " + m.Name + " já não está ativo. Peça um novo.", image: "/og/site.png", alt: siteTitle}
	}
	title := "Pagar a " + m.Name
	if !l.CustomAmount {
		title = "Pagar " + money(l.Amount) + " a " + m.Name
	}
	img := fmt.Sprintf("/og/p/%s/%s.png?v=%s", m.Slug, l.Slug, version(m.Name, l.Name, fmt.Sprint(l.Amount, l.CustomAmount)))
	return pageMeta{status: 200, title: title, desc: paymentDesc(l.Name, m.Name), image: img, alt: title}
}

func (srv *Server) sessionMeta(c context.Context, id string) pageMeta {
	cs, err := srv.A.Session(c, id)
	if err != nil {
		return pageMeta{status: http.StatusNotFound, title: "Pagamento não encontrado · RIHA", desc: "Este pagamento não existe. Confirme o link com a loja.", image: "/og/site.png", alt: siteTitle}
	}
	m, err := srv.A.Merchants.ByID(c, cs.MerchantID)
	if err != nil {
		return pageMeta{status: http.StatusNotFound, title: "Pagamento não encontrado · RIHA", desc: siteDesc, image: "/og/site.png", alt: siteTitle}
	}
	title := "Pagar " + money(cs.Amount) + " a " + m.Name
	return pageMeta{status: 200, title: title, desc: paymentDesc(cs.Description, m.Name), image: "/og/c/" + cs.ID + ".png?v=" + version(m.Name, cs.Description, fmt.Sprint(cs.Amount)), alt: title}
}

// version changes the image's address when its content does, because WhatsApp
// keeps an image by its address.
func version(parts ...string) string {
	s := sha1.Sum([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(s[:4])
}

func (m pageMeta) head(srv *Server) string {
	e := html.EscapeString
	var b strings.Builder
	tag := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	tag(`<title>%s</title>`, e(m.title))
	tag(`<meta name="description" content="%s" />`, e(trimDesc(m.desc)))
	robots := "noindex,nofollow"
	if m.index {
		robots = "index,follow"
		tag(`<link rel="canonical" href="%s" />`, e(srv.url(m.path)))
	}
	tag(`<meta name="robots" content="%s" />`, robots)
	tag(`<meta property="og:site_name" content="%s" />`, siteName)
	tag(`<meta property="og:locale" content="pt_MZ" />`)
	tag(`<meta property="og:type" content="website" />`)
	tag(`<meta property="og:title" content="%s" />`, e(m.title))
	tag(`<meta property="og:description" content="%s" />`, e(trimDesc(m.desc)))
	if m.path != "" {
		tag(`<meta property="og:url" content="%s" />`, e(srv.url(m.path)))
	}
	tag(`<meta property="og:image" content="%s" />`, e(srv.url(m.image)))
	tag(`<meta property="og:image:type" content="image/png" />`)
	tag(`<meta property="og:image:width" content="%d" />`, ogcard.Width)
	tag(`<meta property="og:image:height" content="%d" />`, ogcard.Height)
	tag(`<meta property="og:image:alt" content="%s" />`, e(m.alt))
	tag(`<meta name="twitter:card" content="summary_large_image" />`)
	if m.ld != nil {
		if j, err := json.Marshal(m.ld); err == nil {
			// In a script tag, "</script>" in a name would end it early.
			tag(`<script type="application/ld+json">%s</script>`, strings.ReplaceAll(string(j), "<", `<`))
		}
	}
	return b.String()
}

func trimDesc(s string) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) <= 200 {
		return string(r)
	}
	return strings.TrimSpace(string(r[:199])) + "…"
}

// servePage sends the app's HTML with this page's details written into it.
func (srv *Server) servePage(c *gin.Context) {
	srv.shellOnce.Do(func() {
		b, err := os.ReadFile(filepath.Join(srv.cfg.StaticDir, "index.html"))
		if err == nil {
			srv.shell = string(b)
		}
	})
	m := srv.metaFor(ctx(c), c.Request.URL.Path)
	out := srv.shell
	if i, j := strings.Index(out, seoHeadOpen), strings.Index(out, seoHeadClose); i >= 0 && j > i {
		out = out[:i] + m.head(srv) + out[j+len(seoHeadClose):]
	}
	body := ""
	if m.noscript != "" {
		body = "<noscript>" + m.noscript + "</noscript>"
	}
	out = strings.Replace(out, seoBody, body, 1)
	c.Header("Cache-Control", "no-cache")
	if !m.index {
		c.Header("X-Robots-Tag", "noindex, nofollow")
	}
	c.Data(m.status, "text/html; charset=utf-8", []byte(out))
}

func (srv *Server) robots(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=3600")
	c.String(200, "User-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /painel\nDisallow: /verificacao\nDisallow: /i/\nDisallow: /api/\nDisallow: /v1/\n\nSitemap: %s\n", srv.url("/sitemap.xml"))
}

func (srv *Server) sitemap(c *gin.Context) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range []string{"/", "/docs", "/registo"} {
		fmt.Fprintf(&b, "<url><loc>%s</loc></url>\n", html.EscapeString(srv.url(p)))
	}
	b.WriteString("</urlset>\n")
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(200, "application/xml; charset=utf-8", []byte(b.String()))
}

// Drawing takes milliseconds, but a shared link can be fetched in bursts.
type cardCache struct {
	mu    sync.Mutex
	items map[string][]byte
}

func (k *cardCache) get(key string, draw func() ogcard.Card) ([]byte, error) {
	k.mu.Lock()
	b, ok := k.items[key]
	k.mu.Unlock()
	if ok {
		return b, nil
	}
	b, err := ogcard.Render(draw())
	if err != nil {
		return nil, err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.items == nil || len(k.items) >= 200 {
		k.items = map[string][]byte{}
	}
	k.items[key] = b
	return b, nil
}

const payFoot = "rihagateway.com · Pagamento seguro por M-Pesa"

func (srv *Server) ogImage(c *gin.Context) {
	path := strings.TrimSuffix(strings.Trim(c.Param("rest"), "/"), ".png")
	seg := strings.Split(path, "/")
	var card ogcard.Card
	switch {
	case path == "site":
		card = ogcard.Card{Title: "Receba de todo Moçambique, por M-Pesa", Body: "Links de pagamento, código QR e API. Sem mensalidade.", Foot: "rihagateway.com"}
	case path == "docs":
		card = ogcard.Card{Kicker: "Documentação", Title: "API de pagamentos M-Pesa", Body: "Checkout alojado, links de pagamento e webhooks assinados.", Foot: "rihagateway.com/docs"}
	case len(seg) == 3 && seg[0] == "p":
		l, m, err := srv.A.PublicLink(ctx(c), seg[1], seg[2])
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		card = ogcard.Card{Kicker: "Pagar a " + m.Name, Title: l.Name, Foot: payFoot, Big: money(l.Amount)}
		if l.CustomAmount {
			card.Big = "Valor à escolha"
		}
	case len(seg) == 2 && seg[0] == "c":
		cs, err := srv.A.Session(ctx(c), seg[1])
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		m, err := srv.A.Merchants.ByID(ctx(c), cs.MerchantID)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		card = ogcard.Card{Kicker: "Pagar a " + m.Name, Title: domain.FirstNonEmpty(cs.Description, m.Name), Foot: payFoot, Big: money(cs.Amount)}
	default:
		c.Status(http.StatusNotFound)
		return
	}
	key := path + "|" + card.Kicker + "|" + card.Title + "|" + card.Big
	b, err := srv.cards.get(key, func() ogcard.Card { return card })
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(200, "image/png", b)
}
