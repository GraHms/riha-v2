package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"riha/internal/adapters/httpapi"
	"riha/internal/core/app"
	"riha/internal/core/domain"
)

func microcreditStatus(t *testing.T, a *app.App, ctx context.Context, url, email string) int {
	t.Helper()
	tok, _, err := a.Login(ctx, email, "segredo123", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, url+"/api/microcredit", nil)
	req.AddCookie(&http.Cookie{Name: "riha_session", Value: tok})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

// Microcrédito exists for a business only once a RIHA super admin grants it;
// nobody else can, and revoking closes it again.
func TestMicrocreditIsGrantedOnlyBySuperAdmin(t *testing.T) {
	a, ctx := setup(t)
	srv := httptest.NewServer(httpapi.New(a, httpapi.Config{}))
	defer srv.Close()
	owner := staff(t, a, ctx, "dona@riha.co.mz")
	m := approved(t, a, ctx, "Crédito Já", "credito@example.com")

	if got := microcreditStatus(t, a, ctx, srv.URL, "credito@example.com"); got != http.StatusForbidden {
		t.Fatalf("before the grant the service answered %d", got)
	}

	ops, err := a.Invite(ctx, owner, app.TeamTarget{}, app.InviteInput{Name: "Ops", Email: "ops@riha.co.mz", Permissions: []string{domain.PermMerchants}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetMerchantMicrocredit(ctx, ops, m.ID, true, "", app.Actor{}); err == nil {
		t.Fatal("an admin without every permission granted Microcrédito")
	}
	if _, err := a.CreatePartner(ctx, owner, app.PartnerInput{Code: "mt", Name: "Parceiro", Email: "p@parceiro.co.mz"}); err != nil {
		t.Fatal(err)
	}
	partner := partnerUser(t, a, ctx, owner, "mt", "gestor@parceiro.co.mz", domain.PermAll)
	if _, err := a.SetMerchantMicrocredit(ctx, partner, m.ID, true, "", app.Actor{}); err == nil {
		t.Fatal("a partner with every permission granted Microcrédito")
	}
	if _, err := a.SetMerchantMicrocredit(ctx, nil, m.ID, true, "", app.Actor{}); err == nil {
		t.Fatal("nobody granted Microcrédito")
	}

	_, lead, err := a.Signup(ctx, app.SignupInput{BusinessName: "Lead", Email: "lead@example.com", Phone: "841234568", Password: "segredo123"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetMerchantMicrocredit(ctx, owner, lead.ID, true, "", app.Actor{}); err == nil {
		t.Fatal("a lead was given Microcrédito")
	}

	got, err := a.SetMerchantMicrocredit(ctx, owner, m.ID, true, "  Contrato assinado em novembro  ", app.Actor{ID: owner.ID, Role: domain.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasMicrocredit() || got.MicrocreditBy != owner.ID || got.MicrocreditNote != "Contrato assinado em novembro" {
		t.Fatalf("grant %+v", got)
	}
	since := *got.MicrocreditAt
	second := staff(t, a, ctx, "socio@riha.co.mz")
	if got, err = a.SetMerchantMicrocredit(ctx, second, m.ID, true, "Nota nova", app.Actor{}); err != nil || !got.MicrocreditAt.Equal(since) || got.MicrocreditBy != owner.ID || got.MicrocreditNote != "Nota nova" {
		t.Fatalf("changing the note rewrote the grant: %v %+v", err, got)
	}
	if code := microcreditStatus(t, a, ctx, srv.URL, "credito@example.com"); code != http.StatusOK {
		t.Fatalf("after the grant the service answered %d", code)
	}

	if got, err = a.SetMerchantMicrocredit(ctx, owner, m.ID, false, "", app.Actor{}); err != nil || got.HasMicrocredit() || got.MicrocreditBy != "" {
		t.Fatalf("revoke %v %+v", err, got)
	}
	if code := microcreditStatus(t, a, ctx, srv.URL, "credito@example.com"); code != http.StatusForbidden {
		t.Fatalf("after the revoke the service answered %d", code)
	}
	trail, err := a.Audit.ByMerchant(ctx, m.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	for _, e := range trail {
		actions = append(actions, e.Action)
	}
	if joined := strings.Join(actions, ","); !strings.Contains(joined, "microcredit.granted") || !strings.Contains(joined, "microcredit.revoked") {
		t.Fatalf("audit trail %s", joined)
	}
}
