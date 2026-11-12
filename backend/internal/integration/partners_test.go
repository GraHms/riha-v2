package integration

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

func receipt() app.Upload {
	pdf := []byte("%PDF-1.4\n% comprovativo\n")
	return app.Upload{FileName: "comprovativo.pdf", Size: int64(len(pdf)), Head: pdf[:8], Body: bytes.NewReader(pdf)}
}

func staff(t *testing.T, a *app.App, ctx context.Context, email string) *domain.User {
	t.Helper()
	if err := a.CreateAdmin(ctx, email, "segredo-admin-123", "Dona", "Direção"); err != nil {
		t.Fatal(err)
	}
	u, err := a.Users.ByEmail(ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func partnerUser(t *testing.T, a *app.App, ctx context.Context, by *domain.User, partner, email string, perms ...string) *domain.User {
	t.Helper()
	u, err := a.Invite(ctx, by, app.TeamTarget{Partner: partner}, app.InviteInput{Name: email, Email: email, Permissions: perms})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// A merchant paid through both wallets: its withdrawal is split between the
// partners holding its money, each pays its part, and then it's paid.
func TestWithdrawalsArePaidFromWhereTheMoneyIs(t *testing.T) {
	a, ctx := settlementApp(t)
	owner := staff(t, a, ctx, "dona@riha.co.mz")
	m := approved(t, a, ctx, "MyEvents", "events@example.com")
	payLive(t, a, ctx, m, 100000, "mpesa", "841234231") // direct: 940 MT for the merchant
	payLive(t, a, ctx, m, 200000, "emola", "861234231") // mtoca: 1 880 MT
	if _, err := a.SaveBankAccount(ctx, m, app.BankInput{BankName: "BCI", Holder: "MyEvents", NIB: "000800004821130101135"}, app.Actor{}); err != nil {
		t.Fatal(err)
	}

	// 500 MT: mtoca holds it all, so mtoca alone pays, less the 120 MT fee.
	small, err := a.RequestPayout(ctx, m.ID, true, 50000, "u")
	if err != nil {
		t.Fatal(err)
	}
	parts := a.PayoutFunding(ctx, []string{small.ID})[small.ID]
	if len(parts) != 1 || parts[0].Source != domain.IntegrationMtoca || parts[0].Amount != 50000 || parts[0].Send != 50000-small.Fee {
		t.Fatalf("small %+v", parts)
	}
	// 2 000 MT: mtoca has 1 380 MT left for this merchant, 92 Interactions the rest.
	big, err := a.RequestPayout(ctx, m.ID, true, 200000, "u")
	if err != nil {
		t.Fatal(err)
	}
	parts = a.PayoutFunding(ctx, []string{big.ID})[big.ID]
	if len(parts) != 2 || parts[0].Source != domain.IntegrationMtoca || parts[0].Amount != 138000 || parts[1].Source != domain.IntegrationMpesaDirect || parts[1].Amount != 62000 {
		t.Fatalf("big %+v", parts)
	}

	mt := partnerUser(t, a, ctx, owner, domain.PartnerMtoca, "pagamentos@mtoca.co.mz", domain.PartnerPayouts)
	i92 := partnerUser(t, a, ctx, owner, domain.PartnerInteractions92, "contas@92i.co.mz", domain.PartnerPayouts, domain.PartnerMoney)

	// Before RIHA approves, the partner sees it waiting and can't pay it.
	waiting, _ := a.PartnerFunding(ctx, app.PartnerFundingQuery{Partner: domain.PartnerMtoca, Tab: "waiting"})
	if len(waiting) != 2 {
		t.Fatalf("mtoca waiting %d", len(waiting))
	}
	var de *domain.Error
	if _, err := a.PayFunding(ctx, parts[0].ID, domain.PartnerMtoca, "TRF-1", receipt(), mt.ID); !errors.As(err, &de) || de.Code != "payout_state" {
		t.Fatalf("paying before approval: %v", err)
	}
	if _, err := a.TransitionPayout(ctx, big.ID, domain.ActApprove, "", owner.ID); err != nil {
		t.Fatal(err)
	}
	// A partner can't pay another's part, nor see it.
	if _, err := a.PayFunding(ctx, parts[1].ID, domain.PartnerMtoca, "TRF-1", receipt(), mt.ID); !errors.As(err, &de) || de.Code != "resource_missing" {
		t.Fatalf("mtoca paying 92 Interactions' part: %v", err)
	}
	toPay, _ := a.PartnerFunding(ctx, app.PartnerFundingQuery{Partner: domain.PartnerInteractions92, Tab: "to_pay"})
	if len(toPay) != 1 || toPay[0].Send != 62000 || toPay[0].Payout.NIB != "000800004821130101135" || toPay[0].MerchantName != "MyEvents" {
		t.Fatalf("92 Interactions to pay %+v", toPay)
	}
	if _, err := a.PayFunding(ctx, parts[0].ID, domain.PartnerMtoca, "TRF-MT-1", receipt(), mt.ID); err != nil {
		t.Fatal(err)
	}
	if p, _ := a.Payouts.Lock(ctx, big.ID); p.Status != domain.PayoutSent {
		t.Fatalf("one part paid of two: %s", p.Status)
	}
	if _, err := a.PayFunding(ctx, parts[1].ID, domain.PartnerInteractions92, "TRF-92-1", receipt(), i92.ID); err != nil {
		t.Fatal(err)
	}
	if p, _ := a.Payouts.Lock(ctx, big.ID); p.Status != domain.PayoutPaid || !p.HasReceipt() {
		t.Fatalf("both parts paid: %s", p.Status)
	}

	// What partners paid merchants is no longer theirs to hold.
	rep, err := a.Settlement(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range rep.Partners {
		switch p.Integration.Code {
		case domain.IntegrationMtoca:
			// 2 000 − 36 partner share = 1 964 due, 1 380 paid out, 500 reserved.
			if p.PaidOut != 138000 || p.Reserved != 50000 || p.Held != 196400-138000 {
				t.Fatalf("mtoca %+v", p)
			}
		case domain.IntegrationMpesaDirect:
			if p.PaidOut != 62000 || p.Held != 97000-62000 {
				t.Fatalf("92 Interactions %+v", p)
			}
		}
	}
	if margin := rep.HeldOutside + rep.AtRIHA - rep.OwedToShops; margin != rep.RIHAOwn {
		t.Fatalf("own %d", rep.RIHAOwn)
	}

	// A refused withdrawal frees what it reserved.
	if _, err := a.TransitionPayout(ctx, small.ID, domain.ActReject, "teste", owner.ID); err != nil {
		t.Fatal(err)
	}
	if fs := a.PayoutFunding(ctx, []string{small.ID})[small.ID]; fs[0].Status != domain.FundingCancelled {
		t.Fatalf("after refusal %+v", fs)
	}
	// 92 Interactions' account sees its money, never mtoca's.
	home, err := a.PartnerHome(ctx, domain.PartnerInteractions92, rep.Partners[0].Integration.UpdatedAt.AddDate(0, 0, -1), rep.Partners[0].Integration.UpdatedAt.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(home.Integrations) != 1 || home.Integrations[0].Integration.Code != domain.IntegrationMpesaDirect || home.Period.Amount != 100000 || home.Held != 35000 {
		t.Fatalf("92 Interactions home %+v", home)
	}
}

// Each person sees and does only what their permissions allow, and a team
// can't be left without someone to manage it.
func TestTeamsAndPermissions(t *testing.T) {
	a, ctx := settlementApp(t)
	owner := staff(t, a, ctx, "dona@riha.co.mz")
	finance, err := a.Invite(ctx, owner, app.TeamTarget{}, app.InviteInput{Name: "Finanças", Email: "fin@riha.co.mz", Permissions: []string{"payouts", "settlement", "made-up"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(finance.Permissions) != 2 || !finance.Can(domain.PermPayouts) || finance.Can(domain.PermTeam) {
		t.Fatalf("finance %v", finance.Permissions)
	}
	var de *domain.Error
	if _, err := a.Invite(ctx, finance, app.TeamTarget{}, app.InviteInput{Name: "X", Email: "x@riha.co.mz", Permissions: []string{"payouts"}}); !errors.As(err, &de) || de.Code != "forbidden" {
		t.Fatalf("finance inviting: %v", err)
	}
	if _, err := a.Invite(ctx, owner, app.TeamTarget{}, app.InviteInput{Name: "Y", Email: "FIN@riha.co.mz", Permissions: []string{"payouts"}}); !errors.As(err, &de) || de.Code != "email_taken" {
		t.Fatalf("same email: %v", err)
	}

	// mtoca's lead manages mtoca's team, and gives only what they have.
	lead := partnerUser(t, a, ctx, owner, domain.PartnerMtoca, "lead@mtoca.co.mz", domain.PartnerTeam, domain.PartnerPayouts)
	if _, err := a.Invite(ctx, lead, app.TeamTarget{Partner: domain.PartnerMtoca}, app.InviteInput{Name: "C", Email: "c@mtoca.co.mz", Permissions: []string{domain.PartnerMoney}}); !errors.As(err, &de) || de.Code != "forbidden" {
		t.Fatalf("giving what it hasn't: %v", err)
	}
	clerk, err := a.Invite(ctx, lead, app.TeamTarget{Partner: domain.PartnerMtoca}, app.InviteInput{Name: "C", Email: "c@mtoca.co.mz", Permissions: []string{domain.PartnerPayouts, domain.PermApprovals}})
	if err != nil || len(clerk.Permissions) != 1 || clerk.Role != domain.RolePartner || clerk.PartnerCode != domain.PartnerMtoca {
		t.Fatalf("clerk %+v %v", clerk, err)
	}
	if _, err := a.Invite(ctx, lead, app.TeamTarget{Partner: domain.PartnerInteractions92}, app.InviteInput{Name: "D", Email: "d@x.co.mz", Permissions: []string{domain.PartnerPayouts}}); !errors.As(err, &de) || de.Code != "forbidden" {
		t.Fatalf("inviting to another partner: %v", err)
	}
	if _, err := a.SetAccess(ctx, lead, finance.ID, app.AccessInput{Permissions: []string{"payouts"}}); err == nil {
		t.Fatal("a partner can't touch staff")
	}
	if team, err := a.Team(ctx, clerk, app.TeamTarget{Partner: domain.PartnerMtoca}); err != nil || len(team) != 2 {
		t.Fatalf("mtoca's team as the clerk sees it: %d %v", len(team), err)
	}
	if _, err := a.Team(ctx, clerk, app.TeamTarget{Partner: domain.PartnerInteractions92}); err == nil {
		t.Fatal("a partner can't see another's team")
	}

	// Nobody shuts themselves out; the last manager of the staff stays.
	if _, err := a.SetAccess(ctx, owner, owner.ID, app.AccessInput{Permissions: []string{"payouts"}}); !errors.As(err, &de) || de.Code != "own_access" {
		t.Fatalf("own access: %v", err)
	}
	closed := true
	if _, err := a.SetAccess(ctx, owner, clerk.ID, app.AccessInput{Disabled: &closed}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Login(ctx, "c@mtoca.co.mz", "anything", "", ""); !errors.Is(err, domain.ErrBadCredentials) {
		t.Fatalf("a closed account can't sign in: %v", err)
	}
	if _, _, err := a.Login(ctx, "dona@riha.co.mz", "segredo-admin-123", "", ""); err != nil {
		t.Fatalf("owner signs in: %v", err)
	}
}
