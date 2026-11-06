package integration

import (
	"testing"

	"riha/internal/core/app"
)

// A link where the customer chooses the amount keeps it within the merchant's
// range, and a link carries its thank-you message or return address to checkout.
func TestLinkExtras(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Biblioteca Viva", "viva@example.com")
	l, err := a.CreateLink(ctx, m, false, app.LinkParams{Name: ptr("Donativo"), CustomAmount: ptr(true), Methods: []string{"mpesa"},
		Details: ptr("Cada livro custa 500 MT."), SuggestedAmount: ptr(int64(50000)), MinAmount: ptr(int64(10000)), MaxAmount: ptr(int64(500000)),
		AfterMessage: ptr("Obrigado!")}, "owner", "dashboard")
	if err != nil {
		t.Fatal(err)
	}
	for _, amount := range []int64{5000, 600000} {
		if _, err := a.CreateLinkSession(ctx, l, amount, "outro", "", ""); err == nil {
			t.Fatalf("%d outside the range was accepted", amount)
		}
	}
	cs, err := a.CreateLinkSession(ctx, l, 50000, "outro", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cs.SuccessMessage != "Obrigado!" || cs.Amount != 50000 {
		t.Fatalf("session %+v", cs)
	}
	if _, err := a.CreateLink(ctx, m, false, app.LinkParams{Name: ptr("X"), CustomAmount: ptr(true), Methods: []string{"mpesa"},
		SuggestedAmount: ptr(int64(900000)), MaxAmount: ptr(int64(500000))}, "owner", "dashboard"); err == nil {
		t.Fatal("a suggested amount above the maximum was accepted")
	}
	f, err := a.CreateLink(ctx, m, false, app.LinkParams{Name: ptr("Bolo"), Amount: ptr(int64(125000)), Methods: []string{"mpesa"},
		RedirectURL: ptr("https://loja.example.com/obrigado"), SuggestedAmount: ptr(int64(1))}, "owner", "dashboard")
	if err != nil {
		t.Fatal(err)
	}
	if f.SuggestedAmount != 0 {
		t.Fatal("a fixed amount link keeps no suggested amount")
	}
	fs, err := a.CreateLinkSession(ctx, f, 0, "outro", "", "")
	if err != nil || fs.SuccessURL != "https://loja.example.com/obrigado" || fs.Amount != 125000 {
		t.Fatalf("fixed session %+v %v", fs, err)
	}
	if _, err := a.UpdateLink(ctx, m, false, f.ID, app.LinkParams{AfterMessage: ptr("Olá")}); err == nil {
		t.Fatal("a message and a return address together were accepted")
	}
}

// Archiving switches a link off and keeps it: it takes no payment until it is
// restored and switched on again.
func TestArchivedLinkTakesNoPayment(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Loja Arquivo", "arquivo@example.com")
	l, err := a.CreateLink(ctx, m, false, app.LinkParams{Name: ptr("Bolo"), Amount: ptr(int64(125000)), Methods: []string{"mpesa"}}, "owner", "dashboard")
	if err != nil {
		t.Fatal(err)
	}
	l, err = a.UpdateLink(ctx, m, false, l.ID, app.LinkParams{Archived: ptr(true)})
	if err != nil || !l.Archived || l.Active {
		t.Fatalf("archived %+v %v", l, err)
	}
	if _, err := a.UpdateLink(ctx, m, false, l.ID, app.LinkParams{Active: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if l, _ = a.Links.ByID(ctx, l.ID); l.Active {
		t.Fatal("an archived link was switched on")
	}
	if _, err := a.CreateLinkSession(ctx, l, 0, "outro", "", ""); err == nil {
		t.Fatal("an archived link took a payment")
	}
	l, _ = a.UpdateLink(ctx, m, false, l.ID, app.LinkParams{Archived: ptr(false), Active: ptr(true)})
	if _, err := a.CreateLinkSession(ctx, l, 0, "outro", "", ""); err != nil {
		t.Fatalf("restored and switched on, it must take payments: %v", err)
	}
	if _, err := a.SearchPayments(ctx, m.ID, false, app.PaymentFilter{LinkID: l.ID, Limit: 10}); err != nil {
		t.Fatal(err)
	}
}

// The button word a merchant chooses reaches the checkout, and only known words pass.
func TestSubmitType(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Associação Ajuda", "ajuda@example.com")
	l, err := a.CreateLink(ctx, m, false, app.LinkParams{Name: ptr("Donativo"), CustomAmount: ptr(true), Methods: []string{"mpesa"}, SubmitType: ptr("donate")}, "owner", "dashboard")
	if err != nil || l.SubmitType != "donate" {
		t.Fatalf("%+v %v", l, err)
	}
	cs, err := a.CreateLinkSession(ctx, l, 50000, "outro", "", "")
	if err != nil || cs.SubmitType != "donate" {
		t.Fatalf("session %+v %v", cs, err)
	}
	if _, err := a.UpdateLink(ctx, m, false, l.ID, app.LinkParams{SubmitType: ptr("comprar")}); err == nil {
		t.Fatal("an unknown button word was accepted")
	}
	plain, _ := a.CreateLink(ctx, m, false, app.LinkParams{Name: ptr("Quota"), Amount: ptr(int64(10000)), Methods: []string{"mpesa"}}, "owner", "dashboard")
	if s, _ := a.CreateLinkSession(ctx, plain, 0, "outro", "", ""); s == nil || s.SubmitType != "pay" {
		t.Fatalf("a link without a choice pays: %+v", s)
	}
}
