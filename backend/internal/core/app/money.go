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
		if e := domain.RequireApproved(m); e != nil {
			return e
		}
		bank, err := a.Banks.ByMerchant(ctx, merchantID)
		if err != nil {
			return domain.Invalid("bank_account_missing", "bank_account", "Adicione uma conta bancária em Definições antes de levantar.")
		}
		if from := bank.PayableFrom(); live && from != nil && a.Now().Before(*from) {
			return domain.Invalid("bank_account_on_hold", "bank_account", fmt.Sprintf("A conta bancária foi mudada há pouco. Recebe levantamentos a partir de %s: protege o seu dinheiro se alguém entrar na sua conta.",
				domain.EmailTime(*from)))
		}
		fee := a.TransferPricing(ctx).FeeFor(bank.BankName, bank.NIB, amount)
		if amount-fee < domain.MinPayout {
			if fee > 0 {
				return domain.Invalid("amount_too_small", "amount", fmt.Sprintf("Para %s, levante pelo menos %s MT: recebe 100 MT depois da taxa de transferência de %s MT.",
					bank.BankName, domain.FormatShort(domain.MinPayout+fee), domain.FormatShort(fee)))
			}
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
		p := &domain.Payout{ID: domain.NewID("po"), Number: num, MerchantID: merchantID, Livemode: live, Amount: amount, Fee: fee,
			Status: domain.PayoutNew, BankName: bank.BankName, Holder: bank.Holder, NIB: bank.NIB, BankChangedAt: bank.ChangedAt,
			RequestedBy: actor, RequestedAt: now, DueAt: now.Add(domain.PayoutSLA), UpdatedAt: now}
		if err := a.Payouts.Create(ctx, p); err != nil {
			return err
		}
		if err := a.ledger(ctx, merchantID, live, domain.LedgerPayout, -p.Net(), p.ID, "Levantamento "+p.Ref()); err != nil {
			return err
		}
		if fee > 0 {
			if err := a.ledger(ctx, merchantID, live, domain.LedgerTransferFee, -fee, p.ID, "Taxa de transferência para "+bank.BankName+" · "+p.Ref()); err != nil {
				return err
			}
		}
		if err := a.fund(ctx, p); err != nil {
			return err
		}
		out = p
		if err := a.emit(ctx, merchantID, live, "payout.created", p.ID, domain.PayoutResource(p), ""); err != nil {
			return err
		}
		return a.mailPayoutRequested(ctx, p, m)
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
		m, err := a.Merchants.Lock(ctx, p.MerchantID)
		if err != nil {
			return err
		}
		now := a.Now()
		t, err := p.Apply(action, reason, actor, now)
		if err != nil {
			return err
		}
		if action == domain.ActPaid {
			// Paid only once every part is: no one closes a withdrawal its
			// partners haven't all paid, or RIHA hasn't confirmed.
			if done, err := a.fundingDone(ctx, p.ID); err != nil {
				return err
			} else if !done {
				return domain.Invalid("payout_parts_open", "status", fmt.Sprintf("%s ainda tem partes por pagar ou por confirmar.", p.Ref()))
			}
		}
		if t.Reversal {
			if err := a.cancelFunding(ctx, p); err != nil {
				return err
			}
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
			if err := a.emit(ctx, p.MerchantID, p.Livemode, t.Event, p.ID, domain.PayoutResource(p), ""); err != nil {
				return err
			}
		}
		return a.mailPayoutState(ctx, p, m)
	})
	return out, err
}

func (a *App) ListPayouts(ctx context.Context, q ports.PayoutQuery) ([]domain.Payout, error) {
	return a.Payouts.Find(ctx, q)
}

func (a *App) Refund(ctx context.Context, merchantID string, live bool, paymentID string, amount int64, reason, actor string) (*domain.Refund, error) {
	if live && a.NoLiveRefunds {
		return nil, domain.Invalid("refunds_unavailable", "payment", "Os reembolsos chegam em breve. Por agora, devolva o valor ao cliente diretamente.")
	}
	var out *domain.Refund
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		// The payment, then its merchant: the order completing a payment
		// takes them in, so the two never wait on each other.
		p, err := a.Payments.Lock(ctx, paymentID)
		if err != nil || p.MerchantID != merchantID || p.Livemode != live {
			return domain.NotFound("O pagamento")
		}
		m, err := a.Merchants.Lock(ctx, merchantID)
		if err != nil {
			return err
		}
		if e := domain.RequireApproved(m); e != nil {
			return e
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
		if err := a.emit(ctx, merchantID, live, "refund.succeeded", r.ID, domain.RefundResource(r), ""); err != nil {
			return err
		}
		return a.mailRefund(ctx, r, p, m)
	})
	return out, err
}
