package postgres

import (
	"context"
	"gorm.io/gorm"
	"regexp"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type Users struct{ *Store }

func (r Users) Create(ctx context.Context, u *domain.User) error {
	return create[userRecord](r.conn(ctx), u)
}
func (r Users) ByID(ctx context.Context, id string) (*domain.User, error) {
	return first[userRecord, domain.User](r.conn(ctx).Where("id = ?", id))
}
func (r Users) ByEmail(ctx context.Context, email string) (*domain.User, error) {
	return first[userRecord, domain.User](r.conn(ctx).Where("email = ?", email))
}
func (r Users) EmailTaken(ctx context.Context, email string) (bool, error) {
	return exists(r.conn(ctx).Model(&userRecord{}).Where("email = ?", email))
}
func (r Users) SetPassword(ctx context.Context, id, hash string) error {
	return r.conn(ctx).Model(&userRecord{}).Where("id = ?", id).Update("password_hash", hash).Error
}
func (r Users) TouchLogin(ctx context.Context, id string, at time.Time) error {
	return r.conn(ctx).Model(&userRecord{}).Where("id = ?", id).Updates(map[string]any{"last_login_at": at, "failed_logins": 0, "locked_until": nil}).Error
}

// LoginFailed counts a wrong password, in one statement so guesses at once
// all count, and locks the account for lock once it reaches max.
func (r Users) LoginFailed(ctx context.Context, id string, now time.Time, max int, lock time.Duration) error {
	return r.conn(ctx).Exec(`UPDATE users SET failed_logins = CASE WHEN failed_logins + 1 >= ? THEN 0 ELSE failed_logins + 1 END,
		locked_until = CASE WHEN failed_logins + 1 >= ? THEN ?::timestamptz ELSE locked_until END WHERE id = ?`, max, max, now.Add(lock), id).Error
}
func (r Users) Team(ctx context.Context, role, partnerCode string) ([]domain.User, error) {
	return find[userRecord, domain.User](r.conn(ctx).Where("role = ? AND partner_code = ?", role, partnerCode).Order("disabled_at NULLS FIRST, name"))
}
func (r Users) SetAccess(ctx context.Context, u *domain.User) error {
	return r.conn(ctx).Model(&userRecord{}).Where("id = ?", u.ID).
		Updates(map[string]any{"permissions": StringList(u.Permissions), "admin_title": u.AdminTitle, "disabled_at": u.DisabledAt, "name": u.Name}).Error
}

type AuthSessions struct{ *Store }

func (r AuthSessions) Create(ctx context.Context, s *domain.AuthSession) error {
	return create[sessionRecord](r.conn(ctx), s)
}
func (r AuthSessions) UserID(ctx context.Context, hash string, now time.Time) (string, error) {
	var rec sessionRecord
	if err := r.conn(ctx).First(&rec, "id = ? AND expires_at > ?", hash, now).Error; err != nil {
		return "", wrap(err)
	}
	return rec.UserID, nil
}
func (r AuthSessions) Delete(ctx context.Context, hash string) error {
	return r.conn(ctx).Delete(&sessionRecord{}, "id = ?", hash).Error
}
func (r AuthSessions) DeleteForUser(ctx context.Context, userID, except string) error {
	return r.conn(ctx).Where("user_id = ? AND id <> ?", userID, except).Delete(&sessionRecord{}).Error
}
func (r AuthSessions) Purge(ctx context.Context, before time.Time) error {
	return r.conn(ctx).Where("expires_at < ?", before).Delete(&sessionRecord{}).Error
}

type Merchants struct{ *Store }

func (r Merchants) Create(ctx context.Context, m *domain.Merchant) error {
	return create[merchantRecord](r.conn(ctx), m)
}
func (r Merchants) ByID(ctx context.Context, id string) (*domain.Merchant, error) {
	return first[merchantRecord, domain.Merchant](r.conn(ctx).Where("id = ?", id))
}
func (r Merchants) BySlug(ctx context.Context, slug string) (*domain.Merchant, error) {
	return first[merchantRecord, domain.Merchant](r.conn(ctx).Where("slug = ?", slug))
}
func (r Merchants) Lock(ctx context.Context, id string) (*domain.Merchant, error) {
	return first[merchantRecord, domain.Merchant](r.conn(ctx).Clauses(forUpdate).Where("id = ?", id))
}
func (r Merchants) SlugTaken(ctx context.Context, slug string) (bool, error) {
	return exists(r.conn(ctx).Model(&merchantRecord{}).Where("slug = ?", slug))
}
func (r Merchants) Many(ctx context.Context, ids []string) ([]domain.Merchant, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return find[merchantRecord, domain.Merchant](r.conn(ctx).Where("id IN ?", ids))
}

func (r Merchants) Save(ctx context.Context, m *domain.Merchant) error {
	return save[merchantRecord](r.conn(ctx), m)
}
func (r Merchants) SetFee(ctx context.Context, id string, bps *int) error {
	return r.conn(ctx).Model(&merchantRecord{}).Where("id = ?", id).Update("fee_bps", bps).Error
}
func (r Merchants) SetBeta(ctx context.Context, id string, beta bool) error {
	return r.conn(ctx).Model(&merchantRecord{}).Where("id = ?", id).Update("beta", beta).Error
}
func (r Merchants) filter(ctx context.Context, f ports.MerchantFilter) *gormDB {
	q := r.conn(ctx).Model(&merchantRecord{})
	if len(f.Statuses) > 0 {
		q = q.Where("status IN ?", f.Statuses)
	}
	if s := strings.TrimSpace(f.Query); s != "" {
		like := "%" + likeEscape(strings.ToLower(s)) + "%"
		where, args := "LOWER(name) LIKE ? OR LOWER(owner_name) LIKE ? OR nuit LIKE ? OR LOWER(email) LIKE ?", []any{like, like, like, like}
		// A phone is typed with or without spaces and the 258.
		if d := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, s); len(d) >= 3 {
			where, args = where+" OR phone LIKE ?", append(args, "%"+d+"%")
		}
		q = q.Where(where, args...)
	}
	return q
}
func (r Merchants) List(ctx context.Context, f ports.MerchantFilter) ([]domain.Merchant, error) {
	q := r.filter(ctx, f)
	if f.OrderBy != "" {
		q = q.Order(f.OrderBy)
	}
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	return find[merchantRecord, domain.Merchant](q)
}
func (r Merchants) CountMatching(ctx context.Context, f ports.MerchantFilter) (int64, error) {
	var n int64
	err := r.filter(ctx, f).Count(&n).Error
	return n, err
}
func (r Merchants) Count(ctx context.Context, statuses []string) (int64, error) {
	var n int64
	err := r.filter(ctx, ports.MerchantFilter{Statuses: statuses}).Count(&n).Error
	return n, err
}

type Documents struct{ *Store }

func (r Documents) Create(ctx context.Context, d *domain.Document) error {
	return create[documentRecord](r.conn(ctx), d)
}
func (r Documents) ByID(ctx context.Context, id string) (*domain.Document, error) {
	return first[documentRecord, domain.Document](r.conn(ctx).Where("id = ?", id))
}
func (r Documents) ByMerchant(ctx context.Context, merchantID string) ([]domain.Document, error) {
	return find[documentRecord, domain.Document](r.conn(ctx).Where("merchant_id = ?", merchantID).Order("created_at"))
}
func (r Documents) RemoveKind(ctx context.Context, merchantID, kind string) ([]domain.Document, error) {
	old, err := find[documentRecord, domain.Document](r.conn(ctx).Where("merchant_id = ? AND kind = ?", merchantID, kind))
	if err != nil {
		return nil, err
	}
	return old, r.conn(ctx).Where("merchant_id = ? AND kind = ?", merchantID, kind).Delete(&documentRecord{}).Error
}
func (r Documents) CountKinds(ctx context.Context, merchantID string, kinds []string) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&documentRecord{}).Where("merchant_id = ? AND kind IN ?", merchantID, kinds).Count(&n).Error
	return n, err
}

type Banks struct{ *Store }

func (r Banks) ByMerchant(ctx context.Context, merchantID string) (*domain.BankAccount, error) {
	return first[bankRecord, domain.BankAccount](r.conn(ctx).Where("merchant_id = ?", merchantID))
}
func (r Banks) Save(ctx context.Context, b *domain.BankAccount) error {
	found, err := exists(r.conn(ctx).Model(&bankRecord{}).Where("id = ?", b.ID))
	if err != nil {
		return err
	}
	if !found {
		return create[bankRecord](r.conn(ctx), b)
	}
	return save[bankRecord](r.conn(ctx), b)
}
func (r Banks) SetDocument(ctx context.Context, merchantID, documentID string) error {
	return r.conn(ctx).Model(&bankRecord{}).Where("merchant_id = ?", merchantID).Update("document_id", documentID).Error
}
func (r Banks) MarkVerified(ctx context.Context, merchantID string) error {
	return r.conn(ctx).Model(&bankRecord{}).Where("merchant_id = ?", merchantID).Update("verified", true).Error
}

type Keys struct{ *Store }

func (r Keys) Create(ctx context.Context, k *domain.APIKey) error {
	return create[keyRecord](r.conn(ctx), k)
}
func (r Keys) ByID(ctx context.Context, id string) (*domain.APIKey, error) {
	return first[keyRecord, domain.APIKey](r.conn(ctx).Where("id = ?", id))
}
func (r Keys) Active(ctx context.Context, merchantID string, live bool, kind string) (*domain.APIKey, error) {
	return first[keyRecord, domain.APIKey](r.conn(ctx).Where("merchant_id = ? AND livemode = ? AND kind = ? AND revoked_at IS NULL", merchantID, live, kind).Order("created_at DESC"))
}
func (r Keys) BySecretHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	return first[keyRecord, domain.APIKey](r.conn(ctx).Where("hash = ? AND kind = ? AND revoked_at IS NULL", hash, domain.KeySecret))
}
func (r Keys) RevokeActive(ctx context.Context, merchantID string, live bool, kind string, at time.Time) error {
	return r.conn(ctx).Model(&keyRecord{}).Where("merchant_id = ? AND livemode = ? AND kind = ? AND revoked_at IS NULL", merchantID, live, kind).Update("revoked_at", at).Error
}
func (r Keys) Touch(ctx context.Context, id string, at time.Time) error {
	return r.conn(ctx).Model(&keyRecord{}).Where("id = ?", id).Update("last_used_at", at).Error
}

type Links struct{ *Store }

func (r Links) Create(ctx context.Context, l *domain.PaymentLink) error {
	return create[linkRecord](r.conn(ctx), l)
}
func (r Links) ByID(ctx context.Context, id string) (*domain.PaymentLink, error) {
	return first[linkRecord, domain.PaymentLink](r.conn(ctx).Where("id = ?", id))
}
func (r Links) BySlug(ctx context.Context, merchantID, slug string) (*domain.PaymentLink, error) {
	return first[linkRecord, domain.PaymentLink](r.conn(ctx).Where("merchant_id = ? AND slug = ?", merchantID, slug))
}
func (r Links) SlugTaken(ctx context.Context, merchantID, slug string) (bool, error) {
	return exists(r.conn(ctx).Model(&linkRecord{}).Where("merchant_id = ? AND slug = ?", merchantID, slug))
}
func (r Links) Save(ctx context.Context, l *domain.PaymentLink) error {
	return save[linkRecord](r.conn(ctx), l)
}
func (r Links) List(ctx context.Context, merchantID string, live bool, limit int) ([]domain.PaymentLink, error) {
	q := r.conn(ctx).Where("merchant_id = ? AND livemode = ?", merchantID, live).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	return find[linkRecord, domain.PaymentLink](q)
}
func (r Links) Stats(ctx context.Context, linkID string) (ports.LinkStats, error) {
	var st ports.LinkStats
	err := r.conn(ctx).Model(&paymentRecord{}).Where("payment_link_id = ? AND status IN ?", linkID, domain.SettledStatuses).
		Select("COUNT(*) AS count, COALESCE(SUM(amount),0) AS total, MAX(created_at) AS last").Scan(&st).Error
	return st, err
}

type CheckoutSessions struct{ *Store }

func (r CheckoutSessions) Create(ctx context.Context, cs *domain.CheckoutSession) error {
	return create[sessionCheckoutRecord](r.conn(ctx), cs)
}
func (r CheckoutSessions) ByID(ctx context.Context, id string) (*domain.CheckoutSession, error) {
	return first[sessionCheckoutRecord, domain.CheckoutSession](r.conn(ctx).Where("id = ?", id))
}
func (r CheckoutSessions) Lock(ctx context.Context, id string) (*domain.CheckoutSession, error) {
	return first[sessionCheckoutRecord, domain.CheckoutSession](r.conn(ctx).Clauses(forUpdate).Where("id = ?", id))
}
func (r CheckoutSessions) Save(ctx context.Context, cs *domain.CheckoutSession) error {
	return save[sessionCheckoutRecord](r.conn(ctx), cs)
}
func (r CheckoutSessions) List(ctx context.Context, merchantID string, live bool, clientRef string, limit int) ([]domain.CheckoutSession, error) {
	q := r.conn(ctx).Where("merchant_id = ? AND livemode = ?", merchantID, live).Order("created_at DESC").Limit(limit)
	if clientRef != "" {
		q = q.Where("client_reference_id = ?", clientRef)
	}
	return find[sessionCheckoutRecord, domain.CheckoutSession](q)
}
func (r CheckoutSessions) ExpiredOpen(ctx context.Context, now time.Time, limit int) ([]domain.CheckoutSession, error) {
	return find[sessionCheckoutRecord, domain.CheckoutSession](r.conn(ctx).Where("status = ? AND expires_at < ?", domain.SessionOpen, now).Limit(limit))
}

type Payments struct{ *Store }

func (r Payments) Create(ctx context.Context, p *domain.Payment) error {
	return create[paymentRecord](r.conn(ctx), p)
}
func (r Payments) Delete(ctx context.Context, id string) error {
	return r.conn(ctx).Delete(&paymentRecord{}, "id = ?", id).Error
}
func (r Payments) Save(ctx context.Context, p *domain.Payment) error {
	return save[paymentRecord](r.conn(ctx), p)
}
func (r Payments) ByID(ctx context.Context, id string) (*domain.Payment, error) {
	return first[paymentRecord, domain.Payment](r.conn(ctx).Where("id = ?", id))
}
func (r Payments) Lock(ctx context.Context, id string) (*domain.Payment, error) {
	return first[paymentRecord, domain.Payment](r.conn(ctx).Clauses(forUpdate).Where("id = ?", id))
}
func (r Payments) BySessionAttempt(ctx context.Context, sessionID, attemptID string) (*domain.Payment, error) {
	return first[paymentRecord, domain.Payment](r.conn(ctx).Where("session_id = ? AND attempt_id = ?", sessionID, attemptID))
}
func (r Payments) ByRailReference(ctx context.Context, ref string) (*domain.Payment, error) {
	return first[paymentRecord, domain.Payment](r.conn(ctx).Where("rail_reference = ?", ref))
}
func (r Payments) ByReference(ctx context.Context, ref string) (*domain.Payment, error) {
	return first[paymentRecord, domain.Payment](r.conn(ctx).Where("reference = ?", ref))
}
func (r Payments) LatestBySession(ctx context.Context, sessionID string) (*domain.Payment, error) {
	return first[paymentRecord, domain.Payment](r.conn(ctx).Where("session_id = ?", sessionID).Order("created_at DESC, retry DESC"))
}
func (r Payments) CountCustomerFailures(ctx context.Context, sessionID string) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&paymentRecord{}).Where("session_id = ? AND status = ? AND retry = 0", sessionID, domain.PaymentFailed).Count(&n).Error
	return n, err
}
func (r Payments) CountBySession(ctx context.Context, sessionID, status string) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&paymentRecord{}).Where("session_id = ? AND status = ?", sessionID, status).Count(&n).Error
	return n, err
}
func (r Payments) CountSettledByLink(ctx context.Context, linkID string) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&paymentRecord{}).Where("payment_link_id = ? AND status IN ?", linkID, domain.SettledStatuses).Count(&n).Error
	return n, err
}
func (r Payments) query(ctx context.Context, q ports.PaymentQuery) *gormDB {
	db := r.conn(ctx).Model(&paymentRecord{})
	if q.MerchantID != "" {
		db = db.Where("merchant_id = ?", q.MerchantID)
	}
	if q.Live != nil {
		db = db.Where("livemode = ?", *q.Live)
	}
	if len(q.Statuses) > 0 {
		db = db.Where("status IN ?", q.Statuses)
	}
	if q.Source != "" {
		db = db.Where("source_type = ?", q.Source)
	}
	if q.SessionID != "" {
		db = db.Where("session_id = ?", q.SessionID)
	}
	if q.LinkID != "" {
		db = db.Where("payment_link_id = ?", q.LinkID)
	}
	if q.Method != "" {
		db = db.Where("method = ?", q.Method)
	}
	if q.PaidLate {
		db = db.Where("paid_late")
	}
	if q.Duplicates {
		db = db.Where("duplicate_of <> ''")
	}
	if s := strings.TrimSpace(q.Search); s != "" {
		db = searchPayments(db, s)
	}
	if q.From != nil {
		db = db.Where("created_at >= ?", *q.From)
	}
	if q.To != nil {
		db = db.Where("created_at < ?", *q.To)
	}
	if q.StartingAfter != nil {
		db = db.Where("(created_at, id) < (?, ?)", q.StartingAfter.CreatedAt, q.StartingAfter.ID)
	}
	return db
}
func (r Payments) Find(ctx context.Context, q ports.PaymentQuery) ([]domain.Payment, error) {
	db := r.query(ctx, q).Order("created_at DESC, id DESC").Offset(q.Offset)
	if q.Limit > 0 {
		db = db.Limit(q.Limit)
	}
	return find[paymentRecord, domain.Payment](db)
}
func (r Payments) Count(ctx context.Context, q ports.PaymentQuery) (int64, error) {
	var n int64
	err := r.query(ctx, q).Count(&n).Error
	return n, err
}
func (r Payments) RecentlyTimedOut(ctx context.Context, since time.Time, limit int) ([]domain.Payment, error) {
	return find[paymentRecord, domain.Payment](r.conn(ctx).Where("livemode AND status = ? AND failure_code IN ? AND rail_reference <> '' AND failed_at >= ?", domain.PaymentFailed, []string{domain.FailCustomerTimeout, domain.FailOperatorTimeout, domain.FailNotCompleted}, since).Order("failed_at DESC").Limit(limit))
}

func (r Payments) DueRechecks(ctx context.Context, now time.Time, limit int) ([]domain.Payment, error) {
	return find[paymentRecord, domain.Payment](r.conn(ctx).Where("status = ? AND recheck_at <= ?", domain.PaymentUnconfirmed, now).Order("recheck_at").Limit(limit))
}

func (r Payments) SetRecheck(ctx context.Context, id string, at time.Time, rechecks int) error {
	return r.conn(ctx).Model(&paymentRecord{}).Where("id = ? AND status = ?", id, domain.PaymentUnconfirmed).
		Updates(map[string]any{"recheck_at": at, "rechecks": rechecks}).Error
}

func (r Payments) LaterTwin(ctx context.Context, p *domain.Payment) (*domain.Payment, error) {
	if p.PhoneLast4 == "" {
		return nil, nil
	}
	got, err := find[paymentRecord, domain.Payment](r.conn(ctx).Where(
		"merchant_id = ? AND livemode = ? AND amount = ? AND phone_prefix = ? AND phone_last4 = ? AND id <> ? AND created_at > ? AND created_at < ? AND status IN ?",
		p.MerchantID, p.Livemode, p.Amount, p.PhonePrefix, p.PhoneLast4, p.ID, p.CreatedAt, p.CreatedAt.Add(48*time.Hour), domain.SettledStatuses).Order("created_at").Limit(1))
	if err != nil || len(got) == 0 {
		return nil, err
	}
	return &got[0], nil
}

func (r Payments) StaleProcessing(ctx context.Context, before time.Time, limit int) ([]domain.Payment, error) {
	return find[paymentRecord, domain.Payment](r.conn(ctx).Where("status = ? AND created_at < ?", domain.PaymentProcessing, before).Limit(limit))
}

type Refunds struct{ *Store }

func (r Refunds) Create(ctx context.Context, x *domain.Refund) error {
	return create[refundRecord](r.conn(ctx), x)
}
func (r Refunds) ByPayment(ctx context.Context, paymentID string) ([]domain.Refund, error) {
	return find[refundRecord, domain.Refund](r.conn(ctx).Where("payment_id = ?", paymentID).Order("created_at"))
}

type Ledger struct{ *Store }

func (r Ledger) Add(ctx context.Context, e *domain.LedgerEntry) error {
	return create[ledgerRecord](r.conn(ctx), e)
}
func (r Ledger) Balance(ctx context.Context, merchantID string, live bool) (int64, error) {
	// The merchant_balances rollup, kept by the ledger's trigger in the same
	// transaction as each entry: inside a transaction it already counts
	// that transaction's entries.
	var sum int64
	err := r.conn(ctx).Model(&merchantBalanceRecord{}).Where("merchant_id = ? AND livemode = ?", merchantID, live).Select("COALESCE(SUM(balance),0)").Scan(&sum).Error
	return sum, err
}

type Payouts struct{ *Store }

func (r Payouts) NextNumber(ctx context.Context) (int64, error) {
	var n int64
	err := r.conn(ctx).Raw("SELECT nextval('payout_number_seq')").Scan(&n).Error
	return n, err
}
func (r Payouts) Create(ctx context.Context, p *domain.Payout) error {
	return create[payoutRecord](r.conn(ctx), p)
}
func (r Payouts) Save(ctx context.Context, p *domain.Payout) error {
	return save[payoutRecord](r.conn(ctx), p)
}
func (r Payouts) Lock(ctx context.Context, id string) (*domain.Payout, error) {
	return first[payoutRecord, domain.Payout](r.conn(ctx).Clauses(forUpdate).Where("id = ?", id))
}
func (r Payouts) query(ctx context.Context, q ports.PayoutQuery) *gormDB {
	db := r.conn(ctx).Model(&payoutRecord{})
	if q.MerchantID != "" {
		db = db.Where("merchant_id = ?", q.MerchantID)
	}
	if q.Live != nil {
		db = db.Where("livemode = ?", *q.Live)
	}
	if len(q.Statuses) > 0 {
		db = db.Where("status IN ?", q.Statuses)
	}
	if len(q.IDs) > 0 {
		db = db.Where("id IN ?", q.IDs)
	}
	if q.DueBefore != nil {
		db = db.Where("due_at < ?", *q.DueBefore)
	}
	return db
}
func (r Payouts) Find(ctx context.Context, q ports.PayoutQuery) ([]domain.Payout, error) {
	db := r.query(ctx, q)
	if q.OrderBy != "" {
		db = db.Order(q.OrderBy)
	}
	if q.Limit > 0 {
		db = db.Limit(q.Limit)
	}
	return find[payoutRecord, domain.Payout](db)
}
func (r Payouts) Sum(ctx context.Context, q ports.PayoutQuery) (int64, int64, error) {
	var s struct{ Count, Total int64 }
	err := r.query(ctx, q).Select("COUNT(*) AS count, COALESCE(SUM(amount),0) AS total").Scan(&s).Error
	return s.Count, s.Total, err
}
func (r Payouts) PaidBetween(ctx context.Context, merchantID string, live bool, from, to time.Time) (int64, int64, error) {
	var s struct{ Count, Total int64 }
	err := r.conn(ctx).Model(&payoutRecord{}).Where("merchant_id = ? AND livemode = ? AND status = ? AND paid_at >= ? AND paid_at < ?", merchantID, live, domain.PayoutPaid, from, to).
		Select("COUNT(*) AS count, COALESCE(SUM(amount),0) AS total").Scan(&s).Error
	return s.Count, s.Total, err
}
func (r Payouts) TestModeToSettle(ctx context.Context, before time.Time, limit int) ([]domain.Payout, error) {
	return find[payoutRecord, domain.Payout](r.conn(ctx).Where("livemode = ? AND status IN ? AND updated_at < ?", false, []string{domain.PayoutNew, domain.PayoutSent}, before).Limit(limit))
}

// likeEscape makes % and _ in a search mean themselves, not wildcards.
// searchPayments looks where what was typed can be: an id, a RIHA reference,
// an operation number or the last digits of a phone each go to their own
// index. Only free text reads names and the merchant's own references.
func searchPayments(db *gorm.DB, s string) *gorm.DB {
	up := strings.ToUpper(s)
	digits := strings.Trim(s, "0123456789") == ""
	switch {
	case strings.HasPrefix(s, "pay_"):
		return db.Where("id = ?", s)
	case strings.HasPrefix(up, "RH-"):
		return db.Where("reference LIKE ?", likeEscape(up)+"%")
	case digits && len(s) == 4:
		return db.Where("phone_last4 = ?", s)
	case digits && (len(s) == 12 || len(s) == 20):
		return db.Where("rail_reference = ? OR rail_id = ?", s, s)
	case attemptShape.MatchString(up):
		return db.Where("attempt_id = ? OR operator_reference = ? OR rail_id = ?", up, s, s)
	}
	like := "%" + likeEscape(strings.ToLower(s)) + "%"
	return db.Where("LOWER(customer_name) LIKE ? OR LOWER(client_reference_id) LIKE ? OR operator_reference = ? OR rail_id = ?", like, like, s, s)
}

var attemptShape = regexp.MustCompile(`^[A-Z2-9-]{9,20}$`)

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(s)
}
