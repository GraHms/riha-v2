package domain

import "testing"

func TestWalletNumbers(t *testing.T) {
	for _, tc := range []struct {
		method, phone string
		ok            bool
	}{
		{MethodMpesa, "+258 84 123 4567", true}, {MethodMpesa, "861234567", false},
		{MethodEmola, "871234567", true}, {MethodEmola, "851234567", false},
		{MethodBim, "821234567", true}, {MethodBim, "861234567", true}, {MethodBim, "211234567", false}, {MethodBim, "8412", false},
	} {
		if got := WalletNumberError(tc.method, tc.phone) == nil; got != tc.ok {
			t.Fatalf("%s %s: ok=%v", tc.method, tc.phone, got)
		}
	}
	if !IsWallet(MethodBim) || IsWallet(MethodCard) {
		t.Fatal("IsWallet")
	}
}
