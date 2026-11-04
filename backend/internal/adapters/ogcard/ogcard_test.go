package ogcard

import (
	"bytes"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestRendersTheSharingSize(t *testing.T) {
	for _, c := range []Card{
		{},
		{Kicker: "Pagar a Café Maputo", Title: "Bolo de chocolate, 1 kg", Big: "1 250,00 MT", Foot: "rihagateway.com"},
		{Title: strings.Repeat("Um nome de negócio muito longo ", 20), Body: strings.Repeat("Uma descrição longa ", 40)},
		{Title: strings.Repeat("x", 500)},
		{Title: "🎉 Łódź 中文 <script>", Body: "\x00\n\t"},
	} {
		b, err := Render(c)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != Width || img.Bounds().Dy() != Height {
			t.Fatalf("size %v", img.Bounds())
		}
		if len(b) > 300<<10 {
			t.Fatalf("WhatsApp may drop an image of %d KB", len(b)>>10)
		}
		if p := os.Getenv("OGCARD_PREVIEW"); p != "" && c.Big != "" {
			os.WriteFile(p, b, 0o644)
		}
	}
}

func TestCleanKeepsPortuguese(t *testing.T) {
	if got := clean(bodyFont, "Moçambique: São João, Açúcar"); got != "Moçambique: São João, Açúcar" {
		t.Fatal(got)
	}
	if got := clean(bodyFont, "Loja 🎉"); got != "Loja" {
		t.Fatalf("%q", got)
	}
}
