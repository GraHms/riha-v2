package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func (a *App) payoutSheet(ctx context.Context, p *domain.Payout, withReceipt bool) (domain.PayoutSheet, error) {
	m, err := a.Merchants.ByID(ctx, p.MerchantID)
	if err != nil {
		return domain.PayoutSheet{}, err
	}
	s := domain.PayoutSheet{Payout: *p, Merchant: *m}
	if withReceipt && p.HasReceipt() {
		if data, err := a.Files.Read(p.ReceiptPath); err == nil {
			s.Receipt = &domain.Attachment{Name: p.ReceiptName, ContentType: p.ReceiptType, Data: data}
		}
	}
	return s, nil
}

func (a *App) payoutFor(ctx context.Context, id, merchantID string) (*domain.Payout, error) {
	ps, err := a.Payouts.Find(ctx, ports.PayoutQuery{IDs: []string{id}, MerchantID: merchantID, Limit: 1})
	if err != nil || len(ps) == 0 {
		return nil, domain.NotFound("O levantamento")
	}
	return &ps[0], nil
}

func (a *App) PayoutInstructionsPDF(ctx context.Context, ids []string) ([]byte, error) {
	live := true
	q := ports.PayoutQuery{Live: &live, IDs: ids, OrderBy: "due_at"}
	if len(ids) == 0 {
		q.Statuses = domain.OpenPayoutStatuses
	}
	ps, err := a.Payouts.Find(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(ps) == 0 {
		return nil, domain.NotFound("O levantamento")
	}
	sheets := make([]domain.PayoutSheet, 0, len(ps))
	for i := range ps {
		s, err := a.payoutSheet(ctx, &ps[i], false)
		if err != nil {
			return nil, err
		}
		sheets = append(sheets, s)
	}
	return a.Renderer.PayoutInstructions(sheets)
}

var receiptTypes = map[string]string{".pdf": "application/pdf", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png"}

func (a *App) CompletePayouts(ctx context.Context, ids []string, up Upload, actor string) ([]domain.Payout, error) {
	if len(ids) == 0 {
		return nil, domain.Invalid("parameter_missing", "ids", "Escolha pelo menos um levantamento.")
	}
	if up.Size == 0 || up.Size > 8<<20 {
		return nil, domain.Invalid("receipt_invalid", "receipt", "O comprovativo tem de ter entre 1 byte e 8 MB.")
	}
	ext := strings.ToLower(filepath.Ext(up.FileName))
	ct, ok := receiptTypes[ext]
	if !ok || !sniff(ct, up.Head) {
		return nil, domain.Invalid("receipt_invalid", "receipt", "Use um comprovativo em PDF, JPG ou PNG.")
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(up.Body); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(buf.Bytes())
	hash := hex.EncodeToString(sum[:])
	name := domain.Trim(filepath.Base(up.FileName), 200)
	path, err := a.Files.Save(ctx, "payout-receipts", domain.NewID("rcpt")+ext, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return nil, err
	}
	out := []domain.Payout{}
	for _, id := range ids {
		err := a.Tx.Do(ctx, func(ctx context.Context) error {
			p, err := a.Payouts.Lock(ctx, id)
			if err != nil {
				return notFound(err, "O levantamento")
			}
			if p.Status != domain.PayoutSent {
				return domain.Invalid("payout_state", "status", fmt.Sprintf("%s tem de estar enviado ao banco antes de concluir.", p.Ref()))
			}
			p.ReceiptPath, p.ReceiptName, p.ReceiptType, p.ReceiptSize, p.ReceiptHash = path, name, ct, int64(buf.Len()), hash
			return a.Payouts.Save(ctx, p)
		})
		if err != nil {
			return out, err
		}
		p, err := a.TransitionPayout(ctx, id, domain.ActPaid, "", actor)
		if err != nil {
			return out, err
		}
		out = append(out, *p)
	}
	return out, nil
}

func (a *App) PayoutStatementPDF(ctx context.Context, id, merchantID string) ([]byte, *domain.Payout, error) {
	p, err := a.payoutFor(ctx, id, merchantID)
	if err != nil {
		return nil, nil, err
	}
	if p.Status != domain.PayoutPaid {
		return nil, nil, domain.Invalid("payout_state", "status", "O comprovativo completo só existe para levantamentos pagos.")
	}
	s, err := a.payoutSheet(ctx, p, true)
	if err != nil {
		return nil, nil, err
	}
	pdf, err := a.Renderer.PayoutStatement(s)
	return pdf, p, err
}

func (a *App) PayoutReceipt(ctx context.Context, id, merchantID string) (*domain.Attachment, error) {
	p, err := a.payoutFor(ctx, id, merchantID)
	if err != nil {
		return nil, err
	}
	if !p.HasReceipt() {
		return nil, domain.NotFound("O comprovativo bancário")
	}
	data, err := a.Files.Read(p.ReceiptPath)
	if err != nil {
		return nil, domain.NotFound("O comprovativo bancário")
	}
	return &domain.Attachment{Name: p.ReceiptName, ContentType: p.ReceiptType, Data: data}, nil
}

func (a *App) EmailPayoutStatement(ctx context.Context, id string, actor Actor) (string, error) {
	pdf, p, err := a.PayoutStatementPDF(ctx, id, "")
	if err != nil {
		return "", err
	}
	m, err := a.Merchants.ByID(ctx, p.MerchantID)
	if err != nil {
		return "", err
	}
	if m.Email == "" {
		return "", domain.Invalid("email_missing", "email", "O comerciante não tem email registado.")
	}
	atts := []domain.Attachment{{Name: "riha-" + strings.ToLower(p.Ref()) + ".pdf", ContentType: "application/pdf", Data: pdf}}
	if r, err := a.PayoutReceipt(ctx, id, ""); err == nil {
		atts = append(atts, *r)
	}
	email := domain.Notice{To: m.Email, Subject: "RIHA · Levantamento " + p.Ref() + " transferido", Title: "O comprovativo do seu levantamento",
		Lines: []string{merchantHello(m), fmt.Sprintf("O levantamento %s de %s foi transferido para %s (NIB terminado em %s).", p.Ref(), mt(p.Amount), p.BankName, p.NIB[len(p.NIB)-4:]),
			"Segue em anexo o comprovativo completo da RIHA e o comprovativo bancário."}}.Email()
	email.Attachments = atts
	if err := a.Mailer.Send(ctx, email); err != nil {
		return "", err
	}
	now := a.Now()
	_ = a.Tx.Do(ctx, func(ctx context.Context) error {
		x, err := a.Payouts.Lock(ctx, id)
		if err != nil {
			return err
		}
		x.EmailedAt = &now
		return a.Payouts.Save(ctx, x)
	})
	a.Record(ctx, m.ID, actor, "payout.statement_emailed", p.ID, m.Email)
	return m.Email, nil
}
