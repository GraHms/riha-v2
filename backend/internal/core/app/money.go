package app

import (
	"context"
	"fmt"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func (a *App) RequestPayout(ctx context.Context, merchantID string, live bool, amount int64, actor string) (*domain.Payout, error) {
	var out *domain.Payout
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		m, err := a.Merchants.Lock(ctx, merchantID)
		if err != nil {
			return err
		}
		if live && m.Status != domain.MerchantActive {
			return domain.Forbidden("account_not_verified", "A conta tem de estar verificada antes do primeiro levantamento.")
		}
		if m.Status == domain.MerchantSuspended {
			return domain.Forbidden("account_suspended", "Esta conta está suspensa.")
		}
		bank, err := a.Banks.ByMerchant(ctx, merchantID)
		if err != nil {
			return domain.Invalid("bank_account_missing", "bank_account", "Adicione uma conta bancária em Definições antes de levantar.")
		}
		if amount < domain.MinPayout {
			return domain.Invalid("amount_too_small", "amount", "O levantamento mínimo é 100,00 MT.")
		}
		bal, err := a.Ledger.Balance(ctx, merchantID, live)
		if err != nil {
			return err
		}
		if amount > bal {
			return domain.Invalid("balance_insufficient", "amount", fmt.Sprintf("Acima do saldo disponível (%s MT).", domain.Format(bal)))
		}
		num, err := a.Payouts.NextNumber(ctx)
		if err != nil {
			return err
		}
		now := a.Now()
		p := &domain.Payout{ID: domain.NewID("po"), Number: num, MerchantID: merchantID, Livemode: live, Amount: amount,
			Status: domain.PayoutNew, BankName: bank.BankName, Holder: bank.Holder, NIB: bank.NIB,
			RequestedBy: actor, RequestedAt: now, DueAt: now.Add(domain.PayoutSLA), UpdatedAt: now}
		if err := a.Payouts.Create(ctx, p); err != nil {
			return err
		}
		if err := a.ledger(ctx, merchantID, live, domain.LedgerPayout, -amount, p.ID, "Levantamento "+p.Ref()); err != nil {
			return err
		}
		out = p
		return a.emit(ctx, merchantID, live, "payout.created", p.ID, domain.PayoutResource(p), "")
	})
	return out, err
}

func (a *App) TransitionPayout(ctx context.Context, id, action, reason, actor string) (*domain.Payout, error) {
	var out *domain.Payout
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		p, err := a.Payouts.Lock(ctx, id)
		if err != nil {
			return notFound(err, "O levantamento")
		}
		if _, err := a.Merchants.Lock(ctx, p.MerchantID); err != nil {
			return err
		}
		now := a.Now()
		t, err := p.Apply(action, reason, actor, now)
		if err != nil {
			return err
		}
		if t.Reversal {
			if err := a.ledger(ctx, p.MerchantID, p.Livemode, domain.LedgerPayoutReversal, p.Amount, p.ID, "Devolução "+p.Ref()); err != nil {
				return err
			}
		}
		p.UpdatedAt = now
		if err := a.Payouts.Save(ctx, p); err != nil {
			return err
		}
		out = p
		if t.Event != "" {
			return a.emit(ctx, p.MerchantID, p.Livemode, t.Event, p.ID, domain.PayoutResource(p), "")
		}
		return nil
	})
	return out, err
}

func (a *App) ListPayouts(ctx context.Context, q ports.PayoutQuery) ([]domain.Payout, error) {
	return a.Payouts.Find(ctx, q)
}

func (a *App) Refund(ctx context.Context, merchantID string, live bool, paymentID string, amount int64, reason, actor string) (*domain.Refund, error) {
	var out *domain.Refund
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		if _, err := a.Merchants.Lock(ctx, merchantID); err != nil {
			return err
		}
		p, err := a.Payments.Lock(ctx, paymentID)
		if err != nil || p.MerchantID != merchantID || p.Livemode != live {
			return domain.NotFound("O pagamento")
		}
		remaining := p.Refundable()
		if remaining == 0 {
			return domain.Invalid("payment_not_refundable", "payment", "Só pagamentos concluídos podem ser reembolsados.")
		}
		if amount == 0 {
			amount = remaining
		}
		if amount <= 0 || amount > remaining {
			return domain.Invalid("amount_too_large", "amount", fmt.Sprintf("O valor máximo a reembolsar é %s MT.", domain.Format(remaining)))
		}
		bal, err := a.Ledger.Balance(ctx, merchantID, live)
		if err != nil {
			return err
		}
		if bal < amount {
			return domain.Invalid("balance_insufficient", "amount", "Saldo insuficiente para este reembolso.")
		}
		r := &domain.Refund{ID: domain.NewID("re"), PaymentID: p.ID, MerchantID: merchantID, Livemode: live, Amount: amount, Reason: reason, Status: "succeeded", CreatedBy: actor, CreatedAt: a.Now()}
		if err := a.Refunds.Create(ctx, r); err != nil {
			return err
		}
		p.AmountRefunded += amount
		p.Status = domain.PaymentPartiallyRefunded
		if p.AmountRefunded >= p.Amount {
			p.Status = domain.PaymentRefunded
		}
		if err := a.Payments.Save(ctx, p); err != nil {
			return err
		}
		if err := a.ledger(ctx, merchantID, live, domain.LedgerRefund, -amount, r.ID, "Reembolso "+p.Reference); err != nil {
			return err
		}
		out = r
		return a.emit(ctx, merchantID, live, "refund.succeeded", r.ID, domain.RefundResource(r), "")
	})
	return out, err
}
