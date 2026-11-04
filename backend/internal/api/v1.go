package api

import (
	"encoding/json"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"riha/internal/models"
	"riha/internal/service"
)

// ---------------------------------------------------------------- body parsing

var bracket = regexp.MustCompile(`\[([^\]]*)\]`)
var numeric = regexp.MustCompile(`^-?[0-9]+$`)

var stringKeys = map[string]bool{"client_reference_id": true, "description": true, "name": true, "phone": true, "email": true, "slug": true, "url": true, "reason": true}

// bind decodes JSON or form-encoded bodies (curl -d amount=125000) into dst.
// Form keys support nesting: customer[name], metadata[k], line_items[0][name].
func bind(c *gin.Context, dst any) error {
	ct := c.ContentType()
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	if ct == "application/x-www-form-urlencoded" || (ct == "" && !strings.HasPrefix(strings.TrimSpace(string(raw)), "{")) {
		vals, err := url.ParseQuery(string(raw))
		if err != nil {
			return service.Invalid("body_invalid", "", "Corpo do pedido inválido.")
		}
		tree := map[string]any{}
		for k, vs := range vals {
			setPath(tree, k, vs)
		}
		raw, _ = json.Marshal(normalizeArrays(tree))
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		var te *json.UnmarshalTypeError
		if ok := asTypeErr(err, &te); ok {
			return service.Invalid("parameter_invalid", te.Field, "Tipo inválido para "+te.Field+".")
		}
		return service.Invalid("body_invalid", "", "JSON inválido: "+err.Error())
	}
	return nil
}

func asTypeErr(err error, te **json.UnmarshalTypeError) bool {
	t, ok := err.(*json.UnmarshalTypeError)
	if ok {
		*te = t
	}
	return ok
}

func setPath(tree map[string]any, key string, vs []string) {
	base := key
	var parts []string
	if i := strings.Index(key, "["); i > 0 {
		base = key[:i]
		for _, m := range bracket.FindAllStringSubmatch(key[i:], -1) {
			parts = append(parts, m[1])
		}
	}
	path := append([]string{base}, parts...)
	cur := tree
	for i, p := range path {
		last := i == len(path)-1
		if last {
			var v any = vs[len(vs)-1]
			isMeta := len(path) > 1 && path[0] == "metadata"
			if !isMeta && !stringKeys[p] && numeric.MatchString(vs[len(vs)-1]) {
				n, _ := strconv.ParseInt(vs[len(vs)-1], 10, 64)
				v = n
			} else if vs[len(vs)-1] == "true" || vs[len(vs)-1] == "false" {
				if !isMeta {
					v = vs[len(vs)-1] == "true"
				}
			}
			if p == "" { // list
				lst := []any{}
				for _, x := range vs {
					lst = append(lst, x)
				}
				cur["__list"] = lst
				return
			}
			if base == "payment_method_types" || base == "enabled_events" {
				if len(path) == 1 {
					lst := []any{}
					for _, x := range vs {
						for _, y := range strings.Split(x, ",") {
							lst = append(lst, strings.TrimSpace(y))
						}
					}
					cur[p] = lst
					return
				}
			}
			cur[p] = v
			return
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
}

// normalizeArrays turns {"0":{..},"1":{..}} and {"__list":[..]} into arrays.
func normalizeArrays(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	if l, ok := m["__list"]; ok && len(m) == 1 {
		return l
	}
	allIdx := len(m) > 0
	for k := range m {
		if _, err := strconv.Atoi(k); err != nil {
			allIdx = false
		}
		m[k] = normalizeArrays(m[k])
	}
	if allIdx {
		arr := make([]any, len(m))
		for k, x := range m {
			i, _ := strconv.Atoi(k)
			if i < len(arr) {
				arr[i] = x
			}
		}
		return arr
	}
	return m
}

func list(items any, hasMore bool) gin.H {
	return gin.H{"object": "list", "data": items, "has_more": hasMore}
}

// ---------------------------------------------------------------- sessions

func (srv *Server) v1CreateSession(c *gin.Context) {
	var p service.SessionParams
	if err := bind(c, &p); err != nil {
		apiErr(c, err)
		return
	}
	k := apiKey(c)
	p.SourceType = models.SourceAPI
	p.APIKeyID = &k.ID
	p.OriginIP = c.ClientIP()
	p.OriginUA = c.GetHeader("User-Agent")
	p.OriginDomain = originDomain(c, p.SuccessURL)
	cs, err := srv.S.CreateSession(merchant(c), k.Livemode, p)
	if err != nil {
		apiErr(c, err)
		return
	}
	c.JSON(200, service.SessionObject(cs, srv.S.Cfg.PublicURL))
}

func originDomain(c *gin.Context, successURL string) string {
	for _, h := range []string{c.GetHeader("Origin"), c.GetHeader("Referer"), successURL} {
		if u, err := url.Parse(h); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}
	return ""
}

func (srv *Server) v1GetSession(c *gin.Context) {
	var cs models.CheckoutSession
	if err := srv.S.DB.First(&cs, "id = ? AND merchant_id = ? AND livemode = ?", c.Param("id"), merchant(c).ID, live(c)).Error; err != nil {
		apiErr(c, service.NotFound("A sessão "+c.Param("id")))
		return
	}
	c.JSON(200, service.SessionObject(&cs, srv.S.Cfg.PublicURL))
}

func (srv *Server) v1ListSessions(c *gin.Context) {
	var rows []models.CheckoutSession
	lim := clampLimit(c)
	q := srv.S.DB.Where("merchant_id = ? AND livemode = ?", merchant(c).ID, live(c)).Order("created_at DESC").Limit(lim + 1)
	if ref := c.Query("client_reference_id"); ref != "" {
		q = q.Where("client_reference_id = ?", ref)
	}
	q.Find(&rows)
	out := []any{}
	for i := range rows {
		if i < lim {
			out = append(out, service.SessionObject(&rows[i], srv.S.Cfg.PublicURL))
		}
	}
	c.JSON(200, list(out, len(rows) > lim))
}

func (srv *Server) v1ExpireSession(c *gin.Context) {
	cs, err := srv.S.ExpireSession(merchant(c).ID, live(c), c.Param("id"))
	if err != nil {
		apiErr(c, err)
		return
	}
	c.JSON(200, service.SessionObject(cs, srv.S.Cfg.PublicURL))
}

func clampLimit(c *gin.Context) int {
	l := qInt(c, "limit", 10)
	if l < 1 {
		l = 1
	}
	if l > 100 {
		l = 100
	}
	return l
}

// ---------------------------------------------------------------- payments

func (srv *Server) v1ListPayments(c *gin.Context) {
	lim := clampLimit(c)
	q := srv.S.DB.Where("merchant_id = ? AND livemode = ?", merchant(c).ID, live(c)).Order("created_at DESC, id DESC").Limit(lim + 1)
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if sa := c.Query("starting_after"); sa != "" {
		var p models.Payment
		if srv.S.DB.First(&p, "id = ?", sa).Error == nil {
			q = q.Where("(created_at, id) < (?, ?)", p.CreatedAt, p.ID)
		}
	}
	var rows []models.Payment
	q.Find(&rows)
	out := []any{}
	for i := range rows {
		if i < lim {
			out = append(out, service.PaymentObject(&rows[i]))
		}
	}
	c.JSON(200, list(out, len(rows) > lim))
}

func (srv *Server) v1GetPayment(c *gin.Context) {
	var p models.Payment
	if err := srv.S.DB.First(&p, "(id = ? OR reference = ?) AND merchant_id = ? AND livemode = ?", c.Param("id"), c.Param("id"), merchant(c).ID, live(c)).Error; err != nil {
		apiErr(c, service.NotFound("O pagamento "+c.Param("id")))
		return
	}
	c.JSON(200, service.PaymentObject(&p))
}

func (srv *Server) v1CreateRefund(c *gin.Context) {
	var in struct {
		Payment string `json:"payment"`
		Amount  int64  `json:"amount"`
		Reason  string `json:"reason"`
	}
	if err := bind(c, &in); err != nil {
		apiErr(c, err)
		return
	}
	if in.Payment == "" {
		apiErr(c, service.Invalid("parameter_missing", "payment", "Falta o parâmetro payment."))
		return
	}
	r, err := srv.S.Refund(merchant(c).ID, live(c), in.Payment, in.Amount, in.Reason, apiKey(c).ID)
	if err != nil {
		apiErr(c, err)
		return
	}
	c.JSON(200, service.RefundObject(r))
}

// ---------------------------------------------------------------- links

type linkInput struct {
	Name               *string  `json:"name"`
	Amount             *int64   `json:"amount"`
	CustomAmount       *bool    `json:"custom_amount"`
	Reusable           *bool    `json:"reusable"`
	PaymentMethodTypes []string `json:"payment_method_types"`
	Active             *bool    `json:"active"`
	ExpiresAt          *int64   `json:"expires_at"`
	Slug               *string  `json:"slug"`
	Type               string   `json:"type"`
}

func (srv *Server) v1ListLinks(c *gin.Context) {
	var rows []models.PaymentLink
	m := merchant(c)
	srv.S.DB.Where("merchant_id = ? AND livemode = ?", m.ID, live(c)).Order("created_at DESC").Limit(clampLimit(c)).Find(&rows)
	out := []any{}
	for i := range rows {
		out = append(out, service.LinkObject(&rows[i], srv.S.Cfg.PublicURL, m.Slug))
	}
	c.JSON(200, list(out, false))
}

func (srv *Server) v1CreateLink(c *gin.Context) {
	var in linkInput
	if err := bind(c, &in); err != nil {
		apiErr(c, err)
		return
	}
	m := merchant(c)
	l, err := srv.S.CreateLink(m, live(c), toLinkParams(in), apiKey(c).ID, "api")
	if err != nil {
		apiErr(c, err)
		return
	}
	c.JSON(200, service.LinkObject(l, srv.S.Cfg.PublicURL, m.Slug))
}

func (srv *Server) v1UpdateLink(c *gin.Context) {
	var in linkInput
	if err := bind(c, &in); err != nil {
		apiErr(c, err)
		return
	}
	m := merchant(c)
	l, err := srv.S.UpdateLink(m, live(c), c.Param("id"), toLinkParams(in))
	if err != nil {
		apiErr(c, err)
		return
	}
	c.JSON(200, service.LinkObject(l, srv.S.Cfg.PublicURL, m.Slug))
}

func toLinkParams(in linkInput) service.LinkParams {
	return service.LinkParams{Name: in.Name, Amount: in.Amount, CustomAmount: in.CustomAmount, Reusable: in.Reusable,
		Methods: in.PaymentMethodTypes, Active: in.Active, ExpiresAt: in.ExpiresAt, Slug: in.Slug, Kind: in.Type}
}

// ---------------------------------------------------------------- money

func (srv *Server) v1Balance(c *gin.Context) {
	m := merchant(c)
	c.JSON(200, gin.H{"object": "balance", "available": srv.S.Balance(m.ID, live(c)), "in_transit": srv.S.InTransit(m.ID, live(c)), "currency": "MZN", "livemode": live(c)})
}

func (srv *Server) v1ListPayouts(c *gin.Context) {
	var rows []models.Payout
	srv.S.DB.Where("merchant_id = ? AND livemode = ?", merchant(c).ID, live(c)).Order("requested_at DESC").Limit(clampLimit(c)).Find(&rows)
	out := []any{}
	for i := range rows {
		out = append(out, service.PayoutObject(&rows[i]))
	}
	c.JSON(200, list(out, false))
}

func (srv *Server) v1CreatePayout(c *gin.Context) {
	var in struct {
		Amount int64 `json:"amount"`
	}
	if err := bind(c, &in); err != nil {
		apiErr(c, err)
		return
	}
	p, err := srv.S.RequestPayout(merchant(c).ID, live(c), in.Amount, apiKey(c).ID)
	if err != nil {
		apiErr(c, err)
		return
	}
	c.JSON(200, service.PayoutObject(p))
}

// ---------------------------------------------------------------- webhooks

func (srv *Server) v1ListEndpoints(c *gin.Context) {
	var rows []models.WebhookEndpoint
	srv.S.DB.Where("merchant_id = ? AND livemode = ?", merchant(c).ID, live(c)).Order("created_at").Find(&rows)
	out := []any{}
	for i := range rows {
		out = append(out, service.EndpointObject(&rows[i]))
	}
	c.JSON(200, list(out, false))
}

func (srv *Server) v1CreateEndpoint(c *gin.Context) {
	var in struct {
		URL           string   `json:"url"`
		EnabledEvents []string `json:"enabled_events"`
	}
	if err := bind(c, &in); err != nil {
		apiErr(c, err)
		return
	}
	e, err := srv.S.CreateEndpoint(merchant(c).ID, live(c), in.URL, in.EnabledEvents)
	if err != nil {
		apiErr(c, err)
		return
	}
	o := service.EndpointObject(e)
	o["secret"] = merchant(c).SigningSecret(live(c))
	c.JSON(200, o)
}

func (srv *Server) v1DeleteEndpoint(c *gin.Context) {
	res := srv.S.DB.Where("id = ? AND merchant_id = ? AND livemode = ?", c.Param("id"), merchant(c).ID, live(c)).Delete(&models.WebhookEndpoint{})
	if res.RowsAffected == 0 {
		apiErr(c, service.NotFound("O endpoint"))
		return
	}
	c.JSON(200, gin.H{"id": c.Param("id"), "object": "webhook_endpoint", "deleted": true})
}

func (srv *Server) v1GetEvent(c *gin.Context) {
	var ev models.Event
	if err := srv.S.DB.First(&ev, "id = ? AND merchant_id = ? AND livemode = ?", c.Param("id"), merchant(c).ID, live(c)).Error; err != nil {
		apiErr(c, service.NotFound("O evento"))
		return
	}
	c.JSON(200, gin.H{"id": ev.ID, "object": "event", "type": ev.Type, "created": ev.CreatedAt.Unix(), "livemode": ev.Livemode, "data": gin.H{"object": json.RawMessage(ev.Data)}})
}
