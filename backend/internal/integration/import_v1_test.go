package integration

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

// The first RIHA's accounts come over with their passwords and as leads, and
// anyone who already came back on their own is left alone.
func TestImportV1(t *testing.T) {
	a, ctx := setup(t)
	if _, _, err := a.Signup(ctx, app.SignupInput{BusinessName: "Já Voltou", Email: "voltou@example.com", Phone: "841111111", Password: "segredo123"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Signup(ctx, app.SignupInput{BusinessName: "Outro Email", Email: "novo@example.com", Phone: "842222222", Password: "segredo123"}); err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("antiga123"), 4)
	old := time.Date(2025, 11, 3, 9, 0, 0, 0, time.UTC)
	accounts := []app.V1Account{
		{ID: "v1-a", Email: "Loja@Example.com", Name: "Ana Mabote", BusinessName: "Loja da Ana", Phone: "+258 84 333 3333", PasswordHash: string(hash), CreatedAt: old},
		{ID: "v1-b", Email: "voltou@example.com", BusinessName: "Já Voltou", PasswordHash: string(hash)},
		{ID: "v1-c", Email: "velho@example.com", BusinessName: "Telefone Igual", Phone: "842222222", PasswordHash: string(hash)},
		{ID: "v1-d", Email: "nome@example.com", BusinessName: "outro email", PasswordHash: string(hash)},
		{ID: "v1-e", Email: "sem-negocio@example.com", Name: "Só Nome", Phone: "123", PasswordHash: string(hash)},
	}

	dry, err := a.ImportV1(ctx, accounts, false)
	if err != nil {
		t.Fatal(err)
	}
	if n := countImported(dry); n != 2 {
		t.Fatalf("the dry run must find 2 to import, found %d: %+v", n, dry)
	}
	if taken, _ := a.Users.EmailTaken(ctx, "loja@example.com"); taken {
		t.Fatal("the dry run wrote to the database")
	}

	out, err := a.ImportV1(ctx, accounts, true)
	if err != nil {
		t.Fatal(err)
	}
	if n := countImported(out); n != 2 {
		t.Fatalf("imported %d: %+v", n, out)
	}
	tok, u, err := a.Login(ctx, "loja@example.com", "antiga123", "", "")
	if err != nil || tok == "" {
		t.Fatalf("the old password must work: %v", err)
	}
	m, err := a.Merchants.ByID(ctx, *u.MerchantID)
	if err != nil {
		t.Fatal(err)
	}
	if m.ImportedFrom != "v1" || m.V1ID != "v1-a" || m.Status != domain.MerchantLead || m.Phone != "+258843333333" || m.OwnerName != "Ana Mabote" || !m.CreatedAt.Equal(old) {
		t.Fatalf("merchant %+v", m)
	}
	if _, err := a.Keys.Active(ctx, m.ID, true, domain.KeyPublishable); err != nil {
		t.Fatalf("the imported account needs its publishable keys: %v", err)
	}
	if _, u, err := a.Login(ctx, "sem-negocio@example.com", "antiga123", "", ""); err != nil || u.Phone != "" {
		t.Fatalf("a bad phone is dropped, not imported: %v %+v", err, u)
	}

	again, err := a.ImportV1(ctx, accounts, true)
	if err != nil {
		t.Fatal(err)
	}
	if n := countImported(again); n != 0 {
		t.Fatalf("a second run must import nothing, imported %d", n)
	}
}

func countImported(out []app.V1Outcome) int {
	n := 0
	for _, o := range out {
		if o.Imported {
			n++
		}
	}
	return n
}
