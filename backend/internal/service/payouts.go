package service

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"riha/internal/ids"
	"riha/internal/models"
	"riha/internal/money"
)

const PayoutSLA = 72 * time.Hour
const MinPayout = 10000 // 100,00 MT

var nibRe = regexp.MustCompile(`^[0-9]{21}$`)

func ValidNIB(n string) bool { return nibRe.MatchString(n) }

// RequestPayout debits the balance immediately and queues a bank transfer
// that must be completed within 72 hours.
func (s *Service) RequestPayout(merchantID string, live bool, amount int64, actor string) (*models.Payout, error) {
	var out *models.Payout
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		m, err := lockMerchant(tx, merchantID)
		if err != nil {
			return err
		}
		if live && m.Status != models.MerchantActive {
			return Forbidden("account_not_verified", "A conta tem de estar verificada antes do primeiro levantamento.")
		}
		if m.Status == models.MerchantSuspended {
			return Forbidden("account_suspended", "Esta conta está suspensa.")
		}
		var bank models.BankAccount
		if err := tx.First(&bank, "merchant_id = ?", merchantID).Error; err != nil {
			return Invalid("bank_account_missing", "bank_account", "Adicione uma conta bancária em Definições antes de levantar.")
		}
		if amount < MinPayout {
			return Invalid("amount_too_small", "amount", "O levantamento mínimo é 100,00 MT.")
		}
		bal := Balance(tx, merchantID, live)
		if amount > bal {
			return Invalid("balance_insufficient", "amount", fmt.Sprintf("Acima do saldo disponível (%s MT).", money.Format(bal)))
		}
		var num int64
		if err := tx.Raw("SELECT nextval('payout_number_seq')").Scan(&num).Error; err != nil {
			return err
		}
		now := s.Now()
		p := &models.Payout{ID: ids.New("po"), Number: num, MerchantID: merchantID, Livemode: live, Amount: amount,
			Status: models.PayoutNew, BankName: bank.BankName, Holder: bank.Holder, NIB: bank.NIB,
			RequestedBy: actor, RequestedAt: now, DueAt: now.Add(PayoutSLA)}
		if err := tx.Create(p).Error; err != nil {
			return err
		}
		if err := ledger(tx, merchantID, live, models.LedgerPayout, -amount, p.ID, "Levantamento "+PayoutRef(p)); err != nil {
			return err
		}
		out = p
		return s.emit(tx, merchantID, live, "payout.created", p.ID, PayoutObject(p), "")
	})
	return out, err
}

func PayoutRef(p *models.Payout) string { return fmt.Sprintf("LV-%04d", p.Number) }

// Payout admin actions.
const (
	ActReview  = "review"  // new → review
	ActApprove = "approve" // new/review → sent
	ActPaid    = "paid"    // sent → paid
	ActReject  = "reject"  // new/review → rejected (balance restored)
	ActFail    = "fail"    // sent → failed, e.g. wrong NIB (balance restored)
)

// TransitionPayout applies an operations action.
func (s *Service) TransitionPayout(id, action, reason, actor string) (*models.Payout, error) {
	var p models.Payout
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&p, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return NotFound("O levantamento")
			}
			return err
		}
		if _, err := lockMerchant(tx, p.MerchantID); err != nil {
			return err
		}
		now := s.Now()
		pending := p.Status == models.PayoutNew || p.Status == models.PayoutReview
		bad := Invalid("payout_state", "status", fmt.Sprintf("Não é possível %s um levantamento %s.", action, p.Status))
		evt := ""
		switch action {
		case ActReview:
			if p.Status != models.PayoutNew {
				return bad
			}
			p.Status, p.ReviewedAt = models.PayoutReview, &now
		case ActApprove:
			if !pending {
				return bad
			}
			p.Status, p.SentAt = models.PayoutSent, &now
			if p.ReviewedAt == nil {
				p.ReviewedAt = &now
			}
		case ActPaid:
			if p.Status != models.PayoutSent {
				return bad
			}
			p.Status, p.PaidAt, p.ClosedAt = models.PayoutPaid, &now, &now
			evt = "payout.paid"
		case ActReject, ActFail:
			if action == ActReject && !pending {
				return bad
			}
			if action == ActFail && p.Status != models.PayoutSent {
				return bad
			}
			p.Status, p.ClosedAt, p.FailureReason = models.PayoutRejected, &now, reason
			if action == ActFail {
				p.Status = models.PayoutFailed
				if reason == "" {
					p.FailureReason = "Transferência devolvida pelo banco (NIB inválido)."
				}
			}
			if err := ledger(tx, p.MerchantID, p.Livemode, models.LedgerPayoutReversal, p.Amount, p.ID, "Devolução "+PayoutRef(&p)); err != nil {
				return err
			}
			evt = "payout.failed"
		default:
			return Invalid("action_invalid", "action", "Ação desconhecida.")
		}
		p.ReviewedBy = actor
		if err := tx.Save(&p).Error; err != nil {
			return err
		}
		if evt != "" {
			return s.emit(tx, p.MerchantID, p.Livemode, evt, p.ID, PayoutObject(&p), "")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}
