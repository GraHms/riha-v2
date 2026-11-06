package app

import (
	"bytes"
	"context"
	"io"

	"riha/internal/core/domain"
)

// SaveLinkImage keeps the picture shown with a payment link, redrawn as a JPEG.
func (a *App) SaveLinkImage(ctx context.Context, m *domain.Merchant, live bool, id string, r io.Reader) (*domain.PaymentLink, error) {
	if e := domain.RequireApproved(m); e != nil {
		return nil, e
	}
	l, err := a.merchantLink(ctx, m.ID, live, id)
	if err != nil {
		return nil, err
	}
	out, _, _, err := redraw(r, 200, 200, 1200)
	if err != nil {
		return nil, err
	}
	path, err := a.Files.Save(ctx, "links/"+m.ID, domain.NewID("img")+".jpg", "image/jpeg", bytes.NewReader(out))
	if err != nil {
		return nil, err
	}
	old := l.Image
	l.Image = path
	if err := a.Links.Save(ctx, l); err != nil {
		return nil, err
	}
	if old != "" {
		_ = a.Files.Remove(ctx, old)
	}
	return l, nil
}

func (a *App) RemoveLinkImage(ctx context.Context, m *domain.Merchant, live bool, id string) (*domain.PaymentLink, error) {
	l, err := a.merchantLink(ctx, m.ID, live, id)
	if err != nil {
		return nil, err
	}
	if l.Image == "" {
		return l, nil
	}
	old := l.Image
	l.Image = ""
	if err := a.Links.Save(ctx, l); err != nil {
		return nil, err
	}
	_ = a.Files.Remove(ctx, old)
	return l, nil
}

// LinkImage reads the picture of a public payment link.
func (a *App) LinkImage(ctx context.Context, merchantSlug, slug string) ([]byte, error) {
	l, _, err := a.PublicLink(ctx, merchantSlug, slug)
	if err != nil || l.Image == "" {
		return nil, domain.NotFound("A imagem")
	}
	return a.Files.Read(ctx, l.Image)
}
