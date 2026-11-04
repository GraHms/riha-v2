// Package service holds Riha's business logic: checkout sessions, payments,
// the balance ledger, payouts, refunds and event emission.
package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"riha/internal/config"
	"riha/internal/ids"
	"riha/internal/models"
	"riha/internal/money"
	"riha/internal/providers"
)

// Error is a stable, client-facing error (maps to the public API error object).
type Error struct {
	Status  int    `json:"-"`
	Type    string `json:"type"`
	Code    string `json:"code"`
	Param   string `json:"param,omitempty"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

func Invalid(code, param, msg string) *Error {
	return &Error{Status: 400, Type: "invalid_request_error", Code: code, Param: param, Message: msg}
}
func NotFound(what string) *Error {
	return &Error{Status: 404, Type: "invalid_request_error", Code: "resource_missing", Message: what + " não existe neste modo."}
}
func Forbidden(code, msg string) *Error {
	return &Error{Status: 403, Type: "permission_error", Code: code, Message: msg}
}
func Conflict(code, msg string) *Error {
	return &Error{Status: 409, Type: "invalid_request_error", Code: code, Message: msg}
}

type Service struct {
	DB       *gorm.DB
	Cfg      config.Config
	Provider providers.Provider
	Now      func() time.Time
}

func New(db *gorm.DB, cfg config.Config, p providers.Provider) *Service {
	return &Service{DB: db, Cfg: cfg, Provider: p, Now: func() time.Time { return time.Now().UTC() }}
}

// ---------------------------------------------------------------- settings

const SettingDefaultFee = "default_fee_bps"

func (s *Service) DefaultFeeBps() int {
	var st models.PlatformSetting
	if err := s.DB.First(&st, "key = ?", SettingDefaultFee).Error; err == nil {
		if v, err := strconv.Atoi(st.Value); err == nil && v >= 0 {
			return v
		}
	}
	return money.DefaultFeeBps
}

func (s *Service) SetDefaultFeeBps(bps int) error {
	if bps < 0 || bps > 2000 {
		return Invalid("parameter_invalid", "default_fee_bps", "A taxa tem de estar entre 0% e 20%.")
	}
	return s.DB.Save(&models.PlatformSetting{Key: SettingDefaultFee, Value: strconv.Itoa(bps), UpdatedAt: s.Now()}).Error
}

// FeeBpsFor returns the merchant's negotiated fee or the platform default.
func (s *Service) FeeBpsFor(m *models.Merchant) int {
	if m.FeeBps != nil {
		return *m.FeeBps
	}
	return s.DefaultFeeBps()
}

// ---------------------------------------------------------------- ledger

func Balance(tx *gorm.DB, merchantID string, live bool) int64 {
	var sum int64
	tx.Model(&models.LedgerEntry{}).Where("merchant_id = ? AND livemode = ?", merchantID, live).
		Select("COALESCE(SUM(amount),0)").Scan(&sum)
	return sum
}

func (s *Service) Balance(merchantID string, live bool) int64 { return Balance(s.DB, merchantID, live) }

// InTransit is the sum of payouts not yet paid (they are already debited).
func (s *Service) InTransit(merchantID string, live bool) int64 {
	var sum int64
	s.DB.Model(&models.Payout{}).Where("merchant_id = ? AND livemode = ? AND status IN ?", merchantID, live,
		[]string{models.PayoutNew, models.PayoutReview, models.PayoutSent}).Select("COALESCE(SUM(amount),0)").Scan(&sum)
	return sum
}

func ledger(tx *gorm.DB, merchantID string, live bool, typ string, amount int64, ref, desc string) error {
	return tx.Create(&models.LedgerEntry{ID: ids.New("le"), MerchantID: merchantID, Livemode: live, Type: typ, Amount: amount, ReferenceID: ref, Description: desc}).Error
}

// lockMerchant serialises balance-changing operations for one merchant.
func lockMerchant(tx *gorm.DB, merchantID string) (*models.Merchant, error) {
	var m models.Merchant
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, "id = ?", merchantID).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Audit writes an audit log entry (best effort).
func (s *Service) Audit(merchantID, actorID, role, action, target, details, ip string) {
	s.DB.Create(&models.AuditLog{ID: ids.New("aud"), MerchantID: merchantID, ActorID: actorID, ActorRole: role, Action: action, Target: target, Details: details, IP: ip})
}

// ---------------------------------------------------------------- refunds

func (s *Service) Refund(merchantID string, live bool, paymentID string, amount int64, reason, actor string) (*models.Refund, error) {
	var out *models.Refund
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if _, err := lockMerchant(tx, merchantID); err != nil {
			return err
		}
		var p models.Payment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&p, "id = ? AND merchant_id = ? AND livemode = ?", paymentID, merchantID, live).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return NotFound("O pagamento")
			}
			return err
		}
		if p.Status != models.PaymentSucceeded && p.Status != models.PaymentPartiallyRefunded {
			return Invalid("payment_not_refundable", "payment", "Só pagamentos concluídos podem ser reembolsados.")
		}
		remaining := p.Amount - p.AmountRefunded
		if amount == 0 {
			amount = remaining
		}
		if amount <= 0 || amount > remaining {
			return Invalid("amount_too_large", "amount", fmt.Sprintf("O valor máximo a reembolsar é %s MT.", money.Format(remaining)))
		}
		// The fee is not returned on refunds (same as most gateways); the
		// merchant balance is debited the full refunded amount.
		if bal := Balance(tx, merchantID, live); bal < amount {
			return Invalid("balance_insufficient", "amount", "Saldo insuficiente para este reembolso.")
		}
		r := &models.Refund{ID: ids.New("re"), PaymentID: p.ID, MerchantID: merchantID, Livemode: live, Amount: amount, Reason: reason, Status: "succeeded", CreatedBy: actor}
		if err := tx.Create(r).Error; err != nil {
			return err
		}
		p.AmountRefunded += amount
		if p.AmountRefunded >= p.Amount {
			p.Status = models.PaymentRefunded
		} else {
			p.Status = models.PaymentPartiallyRefunded
		}
		if err := tx.Save(&p).Error; err != nil {
			return err
		}
		if err := ledger(tx, merchantID, live, models.LedgerRefund, -amount, r.ID, "Reembolso "+p.Reference); err != nil {
			return err
		}
		if err := s.emit(tx, merchantID, live, "refund.succeeded", r.ID, RefundObject(r), ""); err != nil {
			return err
		}
		out = r
		return nil
	})
	return out, err
}
