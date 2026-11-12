package httpapi

import (
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

func integrationJSON(in *domain.Integration) gin.H {
	platform := gin.H{}
	for m, bps := range in.PlatformBps {
		platform[m] = bps
	}
	return gin.H{"code": in.Code, "name": in.Name, "partner_code": in.PartnerCode, "account": in.Account, "platform_bps": platform,
		"share_bps": in.ShareBps, "note": in.Note, "updated_at": in.UpdatedAt}
}

func (srv *Server) adminIntegrations(c *gin.Context) {
	vs, err := srv.A.IntegrationViews(ctx(c))
	if err != nil {
		fail(c, err)
		return
	}
	names := srv.A.SourceNames(ctx(c))
	out := []gin.H{}
	for i := range vs {
		o := integrationJSON(&vs[i].Integration)
		o["partner_name"] = names[vs[i].Code]
		o["live_methods"], o["default_for"], o["merchants_routed"], o["merchant_deals"] = vs[i].LiveMethods, vs[i].DefaultFor, vs[i].Routed, vs[i].Deals
		out = append(out, o)
	}
	c.JSON(200, gin.H{"data": out})
}

func (srv *Server) adminEditIntegration(c *gin.Context) {
	var in struct {
		Name        *string        `json:"name"`
		PartnerCode *string        `json:"partner_code"`
		Account     *string        `json:"account"`
		Note        *string        `json:"note"`
		PlatformBps map[string]int `json:"platform_bps"`
		ShareBps    *int           `json:"share_bps"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	out, err := srv.A.EditIntegration(ctx(c), c.Param("code"), app.IntegrationEdit{Name: in.Name, PartnerCode: in.PartnerCode, Account: in.Account, Note: in.Note,
		PlatformBps: in.PlatformBps, ShareBps: in.ShareBps}, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, integrationJSON(out))
}

func (srv *Server) adminMerchantRouting(c *gin.Context) {
	r, err := srv.A.MerchantRouting(ctx(c), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	methods, deals := []gin.H{}, []gin.H{}
	for _, m := range r.Methods {
		methods = append(methods, gin.H{"method": m.Method, "options": m.Options, "default": m.Default, "chosen": m.Chosen, "via": m.Via})
	}
	for _, d := range r.Deals {
		deals = append(deals, gin.H{"integration": d.Integration, "name": d.Name, "default_bps": d.DefaultBps, "share_bps": d.ShareBps})
	}
	c.JSON(200, gin.H{"methods": methods, "deals": deals})
}

func (srv *Server) adminSetRoute(c *gin.Context) {
	var in struct {
		Integration string `json:"integration"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if err := srv.A.SetMerchantRoute(ctx(c), c.Param("id"), c.Param("method"), in.Integration, actor(c)); err != nil {
		fail(c, err)
		return
	}
	srv.adminMerchantRouting(c)
}

func (srv *Server) adminSetDeal(c *gin.Context) {
	var in struct {
		ShareBps *int `json:"share_bps"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	if err := srv.A.SetMerchantDeal(ctx(c), c.Param("id"), c.Param("code"), in.ShareBps, actor(c)); err != nil {
		fail(c, err)
		return
	}
	srv.adminMerchantRouting(c)
}

// settlementPeriod reads from and to as days, to inclusive; neither means all time.
func settlementPeriod(c *gin.Context) (from, to *time.Time) {
	if d, err := time.Parse("2006-01-02", c.Query("from")); err == nil {
		from = &d
	}
	if d, err := time.Parse("2006-01-02", c.Query("to")); err == nil {
		end := d.AddDate(0, 0, 1)
		to = &end
	}
	return from, to
}

func collectedJSON(x app.Collected) gin.H {
	return gin.H{"count": x.Count, "amount": x.Amount, "fee": x.Fee, "platform_fee": x.PlatformFee, "partner_fee": x.PartnerFee,
		"refunded": x.Refunded, "margin": x.Margin(), "due_to_riha": x.DueToRIHA()}
}

func settlementJSON(s *domain.PartnerSettlement) gin.H {
	return gin.H{"id": s.ID, "integration": s.Integration, "amount": s.Amount, "reference": s.Reference, "note": s.Note,
		"received_on": s.ReceivedOn.Format("2006-01-02"), "created_by": s.CreatedBy, "created_at": s.CreatedAt}
}

func (srv *Server) adminSettlement(c *gin.Context) {
	from, to := settlementPeriod(c)
	r, err := srv.A.Settlement(ctx(c), from, to)
	if err != nil {
		fail(c, err)
		return
	}
	partners := []gin.H{}
	for i := range r.Partners {
		p := &r.Partners[i]
		ms := []gin.H{}
		for _, m := range p.Merchants {
			o := collectedJSON(m.Collected)
			o["merchant_id"], o["name"] = m.MerchantID, m.Name
			ms = append(ms, o)
		}
		var last any
		if p.LastPaid != nil {
			last = settlementJSON(p.LastPaid)
		}
		partners = append(partners, gin.H{"integration": integrationJSON(&p.Integration), "partner_name": p.PartnerName, "all_time": collectedJSON(p.AllTime), "received": p.Received,
			"paid_out": p.PaidOut, "reserved": p.Reserved,
			"held": p.Held, "last_settlement": last, "period": collectedJSON(p.Period), "merchants": ms})
	}
	recent := []gin.H{}
	for i := range r.Recent {
		recent = append(recent, settlementJSON(&r.Recent[i]))
	}
	c.JSON(200, gin.H{"partners": partners, "period": collectedJSON(r.Period), "unassigned": collectedJSON(r.Unassigned),
		"position": gin.H{"owed_to_merchants": r.OwedToShops, "merchant_balances": r.Merchants.Balances, "payouts_in_transit": r.Merchants.InTransit,
			"paid_out": r.Merchants.PaidOut, "held_by_partners": r.HeldOutside, "at_riha": r.AtRIHA, "riha_own": r.RIHAOwn},
		"settlements": recent})
}

func mt(c int64) string { return fmt.Sprintf("%d.%02d", c/100, abs(c%100)) }

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func (srv *Server) adminSettlementExport(c *gin.Context) {
	from, to := settlementPeriod(c)
	r, err := srv.A.Settlement(ctx(c), from, to)
	if err != nil {
		fail(c, err)
		return
	}
	name := "riha-liquidacao"
	if from != nil {
		name += "-" + from.Format("20060102")
	}
	if to != nil {
		name += "-" + to.AddDate(0, 0, -1).Format("20060102")
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+name+`.csv"`)
	w := csv.NewWriter(c.Writer)
	_ = writeSafe(w, []string{"integracao", "parceiro", "comerciante", "pagamentos", "valor_mt", "taxa_riha_mt", "taxa_operador_mt", "parte_parceiro_mt", "margem_riha_mt", "reembolsado_mt", "devido_a_riha_mt"})
	for _, p := range r.Partners {
		for _, m := range p.Merchants {
			_ = writeSafe(w, []string{p.Integration.Code, p.PartnerName, strings.TrimSpace(m.Name), fmt.Sprint(m.Count), mt(m.Amount), mt(m.Fee), mt(m.PlatformFee),
				mt(m.PartnerFee), mt(m.Margin()), mt(m.Refunded), mt(m.DueToRIHA())})
		}
	}
	w.Flush()
}

func (srv *Server) adminRecordSettlement(c *gin.Context) {
	var in struct {
		Integration string `json:"integration"`
		Amount      int64  `json:"amount"`
		Reference   string `json:"reference"`
		Note        string `json:"note"`
		ReceivedOn  string `json:"received_on"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	s, err := srv.A.RecordSettlement(ctx(c), app.SettlementInput{Integration: in.Integration, Amount: in.Amount, Reference: in.Reference, Note: in.Note, ReceivedOn: in.ReceivedOn}, actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(201, settlementJSON(s))
}

func (srv *Server) adminDeleteSettlement(c *gin.Context) {
	if err := srv.A.DeleteSettlement(ctx(c), c.Param("id"), actor(c)); err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"deleted": true})
}
