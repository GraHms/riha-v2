package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"riha/internal/core/domain"
)

const (
	MaxCoverBytes = 6 << 20
	coverMaxWidth = 1600
)

// SaveCatalogCover keeps a merchant's catalog cover. The picture is redrawn as
// a JPEG no wider than 1600 pixels, so what is kept is only the picture.
func (a *App) SaveCatalogCover(ctx context.Context, m *domain.Merchant, r io.Reader, actor Actor) (domain.SubscriptionSettings, error) {
	if e := domain.RequireApproved(m); e != nil {
		return domain.SubscriptionSettings{}, e
	}
	raw, err := io.ReadAll(io.LimitReader(r, MaxCoverBytes+1))
	if err != nil {
		return domain.SubscriptionSettings{}, err
	}
	if len(raw) > MaxCoverBytes {
		return domain.SubscriptionSettings{}, domain.Invalid("file_too_large", "file", "A imagem tem de ter no máximo 6 MB.")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return domain.SubscriptionSettings{}, domain.Invalid("file_invalid", "file", "Envie uma imagem JPG, PNG ou WebP.")
	}
	if cfg.Width < 600 || cfg.Height < 200 {
		return domain.SubscriptionSettings{}, domain.Invalid("file_invalid", "file", "A imagem é pequena demais. Use uma com pelo menos 600 por 200 pixels.")
	}
	if cfg.Width*cfg.Height > 40_000_000 {
		return domain.SubscriptionSettings{}, domain.Invalid("file_invalid", "file", "A imagem é grande demais.")
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return domain.SubscriptionSettings{}, domain.Invalid("file_invalid", "file", "Não foi possível ler a imagem.")
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > coverMaxWidth {
		w, h = coverMaxWidth, h*coverMaxWidth/w
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 82}); err != nil {
		return domain.SubscriptionSettings{}, err
	}
	path, err := a.Files.Save(ctx, "catalog/"+m.ID, domain.NewID("cover")+".jpg", "image/jpeg", &out)
	if err != nil {
		return domain.SubscriptionSettings{}, err
	}
	s := a.SubSettingsFor(ctx, m)
	old := s.CatalogCover
	s.CatalogCover, s.UpdatedAt = path, a.Now()
	if err := a.SubSettings.Save(ctx, &s); err != nil {
		return s, err
	}
	if old != "" {
		_ = a.Files.Remove(ctx, old)
	}
	a.Record(ctx, m.ID, actor, "catalog.cover", m.ID, fmt.Sprintf("%dx%d", w, h))
	return s, nil
}

func (a *App) RemoveCatalogCover(ctx context.Context, m *domain.Merchant, actor Actor) (domain.SubscriptionSettings, error) {
	s := a.SubSettingsFor(ctx, m)
	if s.CatalogCover == "" {
		return s, nil
	}
	old := s.CatalogCover
	s.CatalogCover, s.UpdatedAt = "", a.Now()
	if err := a.SubSettings.Save(ctx, &s); err != nil {
		return s, err
	}
	_ = a.Files.Remove(ctx, old)
	a.Record(ctx, m.ID, actor, "catalog.cover", m.ID, "removida")
	return s, nil
}

// CatalogCover reads the cover of the catalog at this address.
func (a *App) CatalogCover(ctx context.Context, slug string) ([]byte, error) {
	m, err := a.publicMerchant(ctx, slug)
	if err != nil {
		return nil, err
	}
	s := a.SubSettingsFor(ctx, m)
	if s.CatalogCover == "" {
		return nil, domain.NotFound("A imagem")
	}
	return a.Files.Read(ctx, s.CatalogCover)
}
