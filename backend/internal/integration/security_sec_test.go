package integration

// Security regression tests. Each one began as a proof of concept of a
// weakness found in review, and now asserts that it stays fixed.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"riha/internal/adapters/httpapi"
	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// A staff member who holds only the "team" permission can't give themself
// or anyone else more, nor touch the account of someone who can do more.
func TestSecTeamPermissionNeverEscalates(t *testing.T) {
	a, ctx := setup(t)
	owner := staff(t, a, ctx, "owner@riha.co.mz")
	low, err := a.Invite(ctx, owner, app.TeamTarget{}, app.InviteInput{Name: "Low", Email: "low@riha.co.mz", Permissions: []string{domain.PermTeam}})
	if err != nil {
		t.Fatal(err)
	}
	if low.Can(domain.PermPayouts) {
		t.Fatal("setup: low must not have payouts")
	}
	if _, err := a.SetAccess(ctx, low, low.ID, app.AccessInput{Permissions: []string{domain.PermAll}}); err == nil {
		t.Fatal("staff with only the team permission gave themself every permission")
	}
	other, err := a.Invite(ctx, owner, app.TeamTarget{}, app.InviteInput{Name: "Other", Email: "other@riha.co.mz", Permissions: []string{domain.PermTeam}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetAccess(ctx, low, other.ID, app.AccessInput{Permissions: []string{domain.PermTeam, domain.PermPayouts}}); err == nil {
		t.Fatal("staff gave someone a permission they don't hold")
	}
	if _, err := a.Invite(ctx, low, app.TeamTarget{}, app.InviteInput{Name: "Friend", Email: "friend@riha.co.mz", Permissions: []string{domain.PermAll}}); err == nil {
		t.Fatal("staff invited someone with every permission")
	}
	closed := true
	if _, err := a.SetAccess(ctx, low, owner.ID, app.AccessInput{Disabled: &closed}); err == nil {
		t.Fatal("staff closed the account of someone who can do more")
	}
	// Within what they hold, they still manage the team.
	if _, err := a.SetAccess(ctx, low, other.ID, app.AccessInput{Disabled: &closed}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetAccess(ctx, owner, low.ID, app.AccessInput{Permissions: []string{domain.PermTeam, domain.PermPayouts}}); err != nil {
		t.Fatal(err)
	}
}

// Domains are verified on creation: any merchant can "own" a metadata IP or a TLD.
func TestSecDomainVerificationIsReal(t *testing.T) {
	a, ctx := setup(t)
	a.AllowPrivateNetworks = false
	a.LookupTXT = func(context.Context, string) ([]string, error) { return []string{"v=spf1 -all"}, nil }
	m := approved(t, a, ctx, "Loja D", "d@b.co.mz")
	for _, d := range []string{"169.254.169.254", "co.mz", "[::1]"} {
		if _, err := a.AddDomain(ctx, m.ID, d); err == nil {
			t.Fatalf("%s was accepted as a merchant's domain", d)
		}
	}
	d, err := a.AddDomain(ctx, m.ID, "google.com")
	if err != nil {
		t.Fatal(err)
	}
	if d.VerifiedAt != nil {
		t.Fatal("a domain was verified on being added")
	}
	if _, err := a.VerifyDomain(ctx, m.ID, d.ID); err == nil {
		t.Fatal("verified without its TXT record")
	}
	if _, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 5000, SuccessURL: "https://x.co.mz/ok", CallbackURL: "https://google.com/hook",
		Origin: domain.Origin{SourceType: domain.SourceAPI}}); err == nil {
		t.Fatal("callback to a domain not verified")
	}
}

// Concurrent API retries with one Idempotency-Key create several payouts.
func TestSecIdempotencyRaceMakesOnePayout(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Loja I", "i@b.co.mz")
	for i := 0; i < 4; i++ {
		pay(t, a, ctx, m, 100000, "841234231")
	}
	if _, err := a.SaveBankAccount(ctx, m, app.BankInput{BankName: "BCI", Holder: "Loja I", NIB: "000800004821130101135"}, app.Actor{}); err != nil {
		t.Fatal(err)
	}
	secret, _, err := a.RollSecretKey(ctx, m, false, app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpapi.New(a, httpapi.Config{}))
	defer srv.Close()
	const n = 8
	var wg, start sync.WaitGroup
	start.Add(1)
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cl := &http.Client{Transport: &http.Transport{}}
			warm, _ := http.NewRequest("GET", srv.URL+"/v1/balance", nil)
			warm.Header.Set("Authorization", "Bearer "+secret)
			if r, err := cl.Do(warm); err == nil {
				_, _ = io.Copy(io.Discard, r.Body)
				r.Body.Close()
			}
			start.Wait()
			req, _ := http.NewRequest("POST", srv.URL+"/v1/payouts", strings.NewReader(`{"amount":50000}`))
			req.Header.Set("Authorization", "Bearer "+secret)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "same-key-1")
			res, err := cl.Do(req)
			if err == nil {
				_, _ = io.Copy(io.Discard, res.Body)
				res.Body.Close()
				codes[i] = res.StatusCode
				if res.Header.Get("Idempotent-Replayed") == "true" {
					codes[i] = -res.StatusCode
				}
			}
		}(i)
	}
	time.Sleep(500 * time.Millisecond)
	start.Done()
	wg.Wait()
	f := false
	ps, _ := a.ListPayouts(ctx, ports.PayoutQuery{MerchantID: m.ID, Live: &f})
	t.Logf("status codes (negative = replayed) %v, payouts created %d", codes, len(ps))
	if len(ps) != 1 {
		t.Fatalf("one Idempotency-Key produced %d payouts", len(ps))
	}
}

// A forged /callbacks/bim makes RIHA ask the operator with the caller's
// transactionID_NEP when the payment has no rail id yet, and keeps it.
func TestSecCallbackNeverChoosesRailID(t *testing.T) {
	a, ctx := setup(t)
	rail := &statusRail{}
	a.Rail = rail
	m := approved(t, a, ctx, "Loja CB", "cb@b.co.mz")
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 9_000_000, Description: "TV", Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "841234567"})
	if err != nil {
		t.Fatal(err)
	}
	if p.RailID != "" {
		t.Fatal("setup: rail id must be empty")
	}
	// Stand-in operator: says "paid" for whatever id it is asked about (as it
	// would for the attacker's own, cheaper, paid transaction).
	rail.mu.Lock()
	rail.out, rail.done = domain.Outcome{Success: true}, true
	rail.mu.Unlock()
	srv := httptest.NewServer(httpapi.New(a, httpapi.Config{}))
	defer srv.Close()
	body := fmt.Sprintf(`{"operationNumber":%q,"transactionID_NEP":"ATTACKER-OWN-TX-123"}`, p.RailReference)
	res, err := http.Post(srv.URL+"/callbacks/bim", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	var got *domain.Payment
	for i := 0; i < 100; i++ {
		got, _ = a.Payments.ByID(ctx, p.ID)
		if got.Status == domain.PaymentSucceeded {
			break
		}
		waitABit()
	}
	rail.mu.Lock()
	ids := append([]string(nil), rail.ids...)
	rail.mu.Unlock()
	t.Logf("operator was asked with rail ids %q; payment now %s, rail_id %q", ids, got.Status, got.RailID)
	for _, id := range ids {
		if id == "ATTACKER-OWN-TX-123" {
			t.Fatalf("the operator was asked about the id a callback brought")
		}
	}
	if got.RailID == "ATTACKER-OWN-TX-123" {
		t.Fatalf("the id a callback brought was kept")
	}
}

// An active merchant's bank account change is verified at once and a payout
// to the new NIB can be requested immediately, with no re-authentication.
func TestSecBankChangeNeedsPasswordAndWaits(t *testing.T) {
	a, ctx := settlementApp(t)
	m := approved(t, a, ctx, "Loja B", "b@b.co.mz")
	owner, err := a.Users.ByEmail(ctx, "b@b.co.mz")
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Now()
	a.Now = func() time.Time { return clock }
	payLive(t, a, ctx, m, 200000, "mpesa", "841234231")
	who := app.Actor{ID: owner.ID, Role: domain.RoleMerchant}
	if _, err := a.SaveBankAccount(ctx, m, app.BankInput{BankName: "BCI", Holder: "Loja B", NIB: "000800004821130101135"}, who); err != nil {
		t.Fatal(err)
	}
	m, _ = a.Merchants.ByID(ctx, m.ID)
	// A stolen session alone can't move the money.
	if _, err := a.SaveBankAccount(ctx, m, app.BankInput{BankName: "Moza Banco", Holder: "Atacante", NIB: "999999999999999999999"}, who); err == nil {
		t.Fatal("the bank account was replaced without the password")
	}
	b, err := a.SaveBankAccount(ctx, m, app.BankInput{BankName: "Moza Banco", Holder: "Atacante", NIB: "999999999999999999999", Password: "segredo123"}, who)
	if err != nil {
		t.Fatal(err)
	}
	if b.ChangedAt == nil {
		t.Fatal("no hold on the new account")
	}
	if _, err := a.RequestPayout(ctx, m.ID, true, 100000, owner.ID); err == nil {
		t.Fatal("a live withdrawal went to the new account at once")
	}
	clock = clock.Add(domain.BankChangeHold + time.Minute)
	po, err := a.RequestPayout(ctx, m.ID, true, 100000, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	rows := a.PayoutRows(ctx, ports.PayoutQuery{IDs: []string{po.ID}})
	if len(rows) != 1 || rows[0].BankChangedAt == nil || !rows[0].HolderDiffers {
		t.Fatalf("staff aren't told: %+v", rows)
	}
}

// Failing a withdrawal after a partner paid its part would credit the
// merchant the full amount back, paying that part twice: it is refused.
func TestSecFailAfterPartPaidIsRefused(t *testing.T) {
	a, ctx := settlementApp(t)
	owner := staff(t, a, ctx, "dona@riha.co.mz")
	m := approved(t, a, ctx, "MyEvents", "events@example.com")
	payLive(t, a, ctx, m, 100000, "mpesa", "841234231")
	payLive(t, a, ctx, m, 200000, "emola", "861234231")
	if _, err := a.SaveBankAccount(ctx, m, app.BankInput{BankName: "BCI", Holder: "MyEvents", NIB: "000800004821130101135"}, app.Actor{}); err != nil {
		t.Fatal(err)
	}
	before := a.Balance(ctx, m.ID, true)
	big, err := a.RequestPayout(ctx, m.ID, true, 200000, "u")
	if err != nil {
		t.Fatal(err)
	}
	parts := a.PayoutFunding(ctx, []string{big.ID})[big.ID]
	if len(parts) != 2 {
		t.Fatalf("parts %+v", parts)
	}
	mt := partnerUser(t, a, ctx, owner, domain.PartnerMtoca, "pag@mtoca.co.mz", domain.PartnerPayouts)
	if _, err := a.TransitionPayout(ctx, big.ID, domain.ActApprove, "", owner.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PayFunding(ctx, parts[0].ID, domain.PartnerMtoca, "TRF-MT-1", receipt(), mt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.TransitionPayout(ctx, big.ID, domain.ActFail, "", owner.ID); err == nil {
		t.Fatal("a withdrawal was failed after a partner paid part of it")
	}
	if after := a.Balance(ctx, m.ID, true); after != before-200000 {
		t.Fatalf("balance %d, want %d: nothing goes back while a part is paid", after, before-200000)
	}
	// RIHA rejects the claim (the transfer isn't on the statement): then the
	// withdrawal can fail, and the merchant gets it all back.
	if _, err := a.RejectClaim(ctx, parts[0].ID, "Não aparece no extrato", owner.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.TransitionPayout(ctx, big.ID, domain.ActFail, "", owner.ID); err != nil {
		t.Fatal(err)
	}
	if after := a.Balance(ctx, m.ID, true); after != before {
		t.Fatalf("balance %d, want %d", after, before)
	}
}

func waitABit() { time.Sleep(50 * time.Millisecond) }

var _ = context.Background

// Anyone who knows a member's phone and name can use the public subscribe
// form: the signup must not change that member nor show their bills.
func TestSecPublicSubscribeNeverTakesOverMember(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Escola Sec", "escola@sec.co.mz")
	p1 := schoolPlan(t, a, ctx, m, "Propina", 250000, domain.MissedDebt)
	p2 := schoolPlan(t, a, ctx, m, "Transporte", 100000, domain.MissedDebt)
	victim, err := a.InviteMember(ctx, m, false, app.MemberInput{PlanIDs: []string{p1.ID}, Phone: "84 123 4567", PayerName: "Tânia Mabunda", Email: "tania@example.com",
		Fields: map[string]string{"Nome do aluno": "Kelvin Mabunda", "Turma": "3ª B"}}, app.Actor{ID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	tok, err := a.Subscribe(ctx, m.Slug, app.SubscribeInput{Accept: true, MemberInput: app.MemberInput{PlanIDs: []string{p2.ID}, Phone: "841234567",
		PayerName: "Atacante", Email: "x@evil.example", Fields: map[string]string{"Nome do aluno": "kelvin  mabunda", "Turma": "HACKED"}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := a.Invoice(ctx, tok, false)
	if err != nil {
		t.Fatal(err)
	}
	mem, _ := a.Member(ctx, m.ID, false, victim.ID)
	if v.Member.ID == victim.ID {
		t.Fatal("the signup's bill is the existing member's")
	}
	if mem.PayerName != "Tânia Mabunda" || mem.Email != "tania@example.com" || mem.Fields["Turma"] != "3ª B" {
		t.Fatalf("the existing member was changed: payer %q email %q fields %v", mem.PayerName, mem.Email, mem.Fields)
	}
	if len(v.Lines) != 1 || v.Member.DuplicateOf != victim.ID {
		t.Fatalf("lines %d, duplicate of %q", len(v.Lines), v.Member.DuplicateOf)
	}
	for _, ag := range v.Agreements {
		if strings.Contains(ag, "4567") && strings.Contains(ag, "123") {
			t.Fatalf("the agreement shows the whole number: %s", ag)
		}
	}
	// A signup left incomplete carries on as the same member when retried.
	tok2, err := a.Subscribe(ctx, m.Slug, app.SubscribeInput{Accept: true, MemberInput: app.MemberInput{PlanIDs: []string{p2.ID}, Phone: "841234567",
		PayerName: "Atacante", Fields: map[string]string{"Nome do aluno": "kelvin  mabunda", "Turma": "HACKED"}}})
	if err != nil {
		t.Fatal(err)
	}
	if v2, _ := a.Invoice(ctx, tok2, false); v2 == nil || v2.Member.ID != v.Member.ID {
		t.Fatal("a retried signup made yet another member")
	}
}

// Guessing a password locks the account for a while; staff on a new device
// also enter a code emailed to them, and the device is then known.
func TestSecLoginLockAndDeviceCode(t *testing.T) {
	a, ctx := setup(t)
	a.LoginCodes = true
	clock := time.Now()
	a.Now = func() time.Time { return clock }
	staff(t, a, ctx, "dona@riha.co.mz")
	for i := 0; i < app.MaxFailedLogins; i++ {
		if _, err := a.SignIn(ctx, app.LoginInput{Email: "dona@riha.co.mz", Password: "errada"}); !errors.Is(err, domain.ErrBadCredentials) {
			t.Fatalf("try %d: %v", i, err)
		}
	}
	if _, err := a.SignIn(ctx, app.LoginInput{Email: "dona@riha.co.mz", Password: "segredo-admin-123"}); !errors.Is(err, domain.ErrLoginLocked) {
		t.Fatalf("the right password after %d wrong ones: %v", app.MaxFailedLogins, err)
	}
	clock = clock.Add(app.LoginLock + time.Second)
	subjects(t, a, ctx)
	if _, err := a.SignIn(ctx, app.LoginInput{Email: "dona@riha.co.mz", Password: "segredo-admin-123"}); !errors.Is(err, domain.ErrLoginCode) {
		t.Fatalf("a new device got in without a code: %v", err)
	}
	var code string
	for _, s := range subjects(t, a, ctx) {
		if strings.HasPrefix(s, "dona@riha.co.mz | ") && strings.Contains(s, "código") {
			code = strings.Fields(strings.TrimPrefix(s, "dona@riha.co.mz | "))[0]
		}
	}
	if len(code) != 6 {
		t.Fatalf("no code emailed")
	}
	if _, err := a.SignIn(ctx, app.LoginInput{Email: "dona@riha.co.mz", Password: "segredo-admin-123", Code: "000000"}); !errors.Is(err, domain.ErrLoginCodeWrong) && code != "000000" {
		t.Fatalf("a wrong code: %v", err)
	}
	r, err := a.SignIn(ctx, app.LoginInput{Email: "dona@riha.co.mz", Password: "segredo-admin-123", Code: code})
	if err != nil || r.Device == "" {
		t.Fatalf("the emailed code: %v", err)
	}
	if _, err := a.SignIn(ctx, app.LoginInput{Email: "dona@riha.co.mz", Password: "segredo-admin-123", Code: code}); !errors.Is(err, domain.ErrLoginCodeWrong) {
		t.Fatalf("a code works once: %v", err)
	}
	if _, err := a.SignIn(ctx, app.LoginInput{Email: "dona@riha.co.mz", Password: "segredo-admin-123", Device: r.Device}); err != nil {
		t.Fatalf("the known device: %v", err)
	}
	// Merchants sign in with their password alone.
	approved(t, a, ctx, "Loja L", "l@loja.co.mz")
	if _, err := a.SignIn(ctx, app.LoginInput{Email: "l@loja.co.mz", Password: "segredo123"}); err != nil {
		t.Fatal(err)
	}
}
