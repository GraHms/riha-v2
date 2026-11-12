package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

func permissionsJSON(ps []domain.Permission) []gin.H {
	out := []gin.H{}
	for _, p := range ps {
		out = append(out, gin.H{"key": p.Key, "label": p.Label, "help": p.Help})
	}
	return out
}

func memberJSON(u *domain.User) gin.H {
	perms := u.Permissions
	if perms == nil {
		perms = []string{}
	}
	return gin.H{"id": u.ID, "name": u.Name, "email": u.Email, "title": u.AdminTitle, "permissions": perms, "partner_code": u.PartnerCode,
		"disabled": u.DisabledAt != nil, "last_login_at": iso(u.LastLoginAt), "created_at": u.CreatedAt}
}

func membersJSON(us []domain.User) []gin.H {
	out := []gin.H{}
	for i := range us {
		out = append(out, memberJSON(&us[i]))
	}
	return out
}

type inviteBody struct {
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Title       string   `json:"title"`
	Permissions []string `json:"permissions"`
}

func (srv *Server) invite(c *gin.Context, t app.TeamTarget) {
	var in inviteBody
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	u, err := srv.A.Invite(ctx(c), user(c), t, app.InviteInput{Name: in.Name, Email: in.Email, Title: in.Title, Permissions: in.Permissions})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(201, memberJSON(u))
}

func (srv *Server) setAccess(c *gin.Context) {
	var in struct {
		Permissions []string `json:"permissions"`
		Title       *string  `json:"title"`
		Disabled    *bool    `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	u, err := srv.A.SetAccess(ctx(c), user(c), c.Param("id"), app.AccessInput{Permissions: in.Permissions, Title: in.Title, Disabled: in.Disabled})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, memberJSON(u))
}

func (srv *Server) resendInvite(c *gin.Context) {
	if err := srv.A.ResendInvite(ctx(c), user(c), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"sent": true})
}

// Staff

func (srv *Server) adminTeam(c *gin.Context) {
	us, err := srv.A.Team(ctx(c), user(c), app.TeamTarget{})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"data": membersJSON(us), "permissions": permissionsJSON(domain.StaffPermissions), "partner_permissions": permissionsJSON(domain.PartnerPermissions)})
}

func (srv *Server) adminInvite(c *gin.Context) { srv.invite(c, app.TeamTarget{}) }

func (srv *Server) adminInvitePartnerUser(c *gin.Context) {
	srv.invite(c, app.TeamTarget{Partner: c.Param("code")})
}

func (srv *Server) adminSetAccess(c *gin.Context)    { srv.setAccess(c) }
func (srv *Server) adminResendInvite(c *gin.Context) { srv.resendInvite(c) }

func (srv *Server) adminPartners(c *gin.Context) {
	ps, err := srv.A.PartnerViews(ctx(c))
	if err != nil {
		fail(c, err)
		return
	}
	out := []gin.H{}
	for i := range ps {
		ins := []gin.H{}
		for j := range ps[i].Integrations {
			ins = append(ins, gin.H{"code": ps[i].Integrations[j].Code, "name": ps[i].Integrations[j].Name, "account": ps[i].Integrations[j].Account})
		}
		out = append(out, gin.H{"code": ps[i].Code, "name": ps[i].Name, "email": ps[i].Email, "integrations": ins, "users": membersJSON(ps[i].Users)})
	}
	c.JSON(200, gin.H{"data": out, "permissions": permissionsJSON(domain.PartnerPermissions)})
}

func (srv *Server) adminCreatePartner(c *gin.Context) {
	var in app.PartnerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	p, err := srv.A.CreatePartner(ctx(c), user(c), in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(201, gin.H{"code": p.Code, "name": p.Name, "email": p.Email})
}

func (srv *Server) adminRenamePartner(c *gin.Context) {
	var in app.PartnerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	p, err := srv.A.RenamePartner(ctx(c), user(c), c.Param("code"), in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"code": p.Code, "name": p.Name, "email": p.Email})
}

// Withdrawal parts

func fundingJSON(f *domain.PayoutFunding) gin.H {
	return gin.H{"id": f.ID, "source": f.Source, "amount": f.Amount, "send": f.Send, "status": f.Status, "reference": f.Reference,
		"has_receipt": f.HasReceipt(), "paid_at": iso(f.PaidAt), "confirmed_at": iso(f.ConfirmedAt), "warning": f.Warning}
}

// fundingRowJSON is a part of a withdrawal with what its payer needs to send
// it: the merchant's bank account in full.
func fundingRowJSON(r *app.FundingRow) gin.H {
	o := fundingJSON(&r.PayoutFunding)
	p := &r.Payout
	o["payout"] = gin.H{"id": p.ID, "reference": p.Ref(), "status": p.Status, "amount": p.Amount, "fee": p.Fee, "requested_at": isoT(p.RequestedAt),
		"due_at": isoT(p.DueAt), "bank_name": p.BankName, "holder": p.Holder, "nib": p.NIB}
	o["merchant_name"], o["source_name"] = r.MerchantName, r.SourceName
	return o
}

func receiptUpload(c *gin.Context) (app.Upload, func(), bool) {
	fh, err := c.FormFile("receipt")
	if err != nil {
		bad(c, "Junte o comprovativo da transferência.")
		return app.Upload{}, nil, false
	}
	f, err := fh.Open()
	if err != nil {
		bad(c, "Não foi possível ler o comprovativo.")
		return app.Upload{}, nil, false
	}
	head := make([]byte, 8)
	n, _ := io.ReadFull(f, head)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		fail(c, err)
		return app.Upload{}, nil, false
	}
	return app.Upload{FileName: fh.Filename, Size: fh.Size, Head: head[:n], Body: f}, func() { _ = f.Close() }, true
}

func (srv *Server) payFunding(c *gin.Context, partner string) {
	up, done, ok := receiptUpload(c)
	if !ok {
		return
	}
	defer done()
	f, err := srv.A.PayFunding(ctx(c), c.Param("id"), partner, c.PostForm("reference"), up, user(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, fundingJSON(f))
}

func (srv *Server) fundingReceipt(c *gin.Context, partner string) {
	att, err := srv.A.FundingReceipt(ctx(c), c.Param("id"), partner)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, strings.ReplaceAll(att.Name, `"`, "")))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, att.ContentType, att.Data)
}

func (srv *Server) adminPayFunding(c *gin.Context) { srv.payFunding(c, "") }

// adminConfirmFunding is staff seeing partners' transfers on the statement.
func (srv *Server) adminConfirmFunding(c *gin.Context) {
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || len(in.IDs) > 200 {
		bad(c, "Pedido inválido.")
		return
	}
	fs, err := srv.A.ConfirmFunding(ctx(c), in.IDs, user(c).ID)
	out := make([]gin.H, 0, len(fs))
	for i := range fs {
		out = append(out, fundingJSON(&fs[i]))
	}
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"data": out})
}

func (srv *Server) adminRejectClaim(c *gin.Context) {
	var in struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "Pedido inválido.")
		return
	}
	f, err := srv.A.RejectClaim(ctx(c), c.Param("id"), in.Reason, user(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, fundingJSON(f))
}
func (srv *Server) adminFundingReceipt(c *gin.Context) { srv.fundingReceipt(c, "") }

// The partner's account

func partnerOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := user(c)
		if u.Role != domain.RolePartner || u.PartnerCode == "" {
			c.AbortWithStatusJSON(403, gin.H{"error": gin.H{"code": "forbidden", "message": "Só para contas de parceiro."}})
			return
		}
		c.Next()
	}
}

func partnerCan(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !user(c).Can(perm) {
			c.AbortWithStatusJSON(403, gin.H{"error": gin.H{"code": "forbidden", "message": "Não tem permissão para esta área. Peça a quem gere a sua equipa."}})
			return
		}
		c.Next()
	}
}

func (srv *Server) partnerRoutes(g *gin.RouterGroup) {
	g.GET("/home", partnerCan(domain.PartnerMoney), srv.partnerHome)
	g.GET("/payouts", partnerCan(domain.PartnerPayouts), srv.partnerPayouts)
	g.POST("/payouts/:id/paid", partnerCan(domain.PartnerPayouts), func(c *gin.Context) { srv.payFunding(c, user(c).PartnerCode) })
	g.GET("/payouts/:id/receipt", partnerCan(domain.PartnerPayouts), func(c *gin.Context) { srv.fundingReceipt(c, user(c).PartnerCode) })
	g.GET("/team", srv.partnerTeam)
	g.POST("/team", partnerCan(domain.PartnerTeam), func(c *gin.Context) { srv.invite(c, app.TeamTarget{Partner: user(c).PartnerCode}) })
	g.PATCH("/team/:id", partnerCan(domain.PartnerTeam), srv.setAccess)
	g.POST("/team/:id/invite", partnerCan(domain.PartnerTeam), srv.resendInvite)
}

func (srv *Server) partnerHome(c *gin.Context) {
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -29)
	if d, err := time.Parse("2006-01-02", c.Query("from")); err == nil {
		from = d
	}
	if d, err := time.Parse("2006-01-02", c.Query("to")); err == nil {
		to = d
	}
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	to = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	h, err := srv.A.PartnerHome(ctx(c), user(c).PartnerCode, from, to)
	if err != nil {
		fail(c, err)
		return
	}
	ins := []gin.H{}
	for i := range h.Integrations {
		pi := &h.Integrations[i]
		deals := []gin.H{}
		for _, d := range pi.Deals {
			deals = append(deals, gin.H{"merchant_name": d.MerchantName, "share_bps": d.ShareBps})
		}
		platform := gin.H{}
		for m, bps := range pi.Integration.PlatformBps {
			platform[m] = bps
		}
		ins = append(ins, gin.H{"code": pi.Integration.Code, "name": pi.Integration.Name, "account": pi.Integration.Account, "share_bps": pi.Integration.ShareBps,
			"platform_bps": platform, "note": pi.Integration.Note, "methods": pi.Methods, "deals": deals, "all_time": collectedJSON(pi.AllTime),
			"received": pi.Received, "paid_out": pi.PaidOut, "reserved": pi.Reserved, "held": pi.Held})
	}
	days := []gin.H{}
	for _, d := range h.Days {
		days = append(days, gin.H{"day": d.Day, "count": d.Count, "amount": d.Amount, "partner_fee": d.PartnerFee})
	}
	ms := []gin.H{}
	for _, m := range h.Merchants {
		o := collectedJSON(m.Collected)
		o["name"] = m.Name
		delete(o, "fee")
		delete(o, "margin")
		ms = append(ms, o)
	}
	ss := []gin.H{}
	for i := range h.Settlements {
		s := &h.Settlements[i]
		ss = append(ss, gin.H{"id": s.ID, "integration": s.Integration, "amount": s.Amount, "reference": s.Reference, "received_on": s.ReceivedOn.Format("2006-01-02")})
	}
	period := collectedJSON(h.Period)
	delete(period, "fee")
	delete(period, "margin")
	c.JSON(200, gin.H{"partner": gin.H{"code": h.Partner.Code, "name": h.Partner.Name}, "integrations": ins, "held": h.Held, "reserved": h.Reserved,
		"to_pay": h.ToPay, "to_pay_count": h.ToPayCount, "from": from.Format("2006-01-02"), "to": to.Format("2006-01-02"), "days": days, "period": period,
		"merchants": ms, "settlements": ss})
}

func (srv *Server) partnerPayouts(c *gin.Context) {
	tab := c.Query("tab")
	if tab != "waiting" && tab != "paid" {
		tab = "to_pay"
	}
	rows, err := srv.A.PartnerFunding(ctx(c), app.PartnerFundingQuery{Partner: user(c).PartnerCode, Tab: tab})
	if err != nil {
		fail(c, err)
		return
	}
	out := []gin.H{}
	for i := range rows {
		out = append(out, fundingRowJSON(&rows[i]))
	}
	c.JSON(200, gin.H{"data": out, "tab": tab})
}

func (srv *Server) partnerTeam(c *gin.Context) {
	u := user(c)
	us, err := srv.A.Team(ctx(c), u, app.TeamTarget{Partner: u.PartnerCode})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"data": membersJSON(us), "permissions": permissionsJSON(domain.PartnerPermissions), "can_manage": u.Can(domain.PartnerTeam)})
}
