package service

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"

	"riha/internal/ids"
	"riha/internal/models"
)

const SessionTTL = 14 * 24 * time.Hour

var avatarColors = []string{"#FFD21F", "#E2F3DE", "#FFF0D6", "#EDECE6", "#FFE3E1"}

type SignupInput struct {
	BusinessName string `json:"business_name"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Password     string `json:"password"`
}

func Slugify(s string) string {
	s = norm.NFD.String(strings.ToLower(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" {
		out = "loja"
	}
	return out
}

func (s *Service) uniqueMerchantSlug(base string) string {
	slug := base
	for i := 2; ; i++ {
		var n int64
		s.DB.Model(&models.Merchant{}).Where("slug = ?", slug).Count(&n)
		if n == 0 {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

var phoneRe = regexp.MustCompile(`^[0-9]{9}$`)

func ValidPassword(p string) *Error {
	if len(p) < 8 {
		return Invalid("password_weak", "password", "A palavra-passe precisa de pelo menos 8 caracteres.")
	}
	return nil
}

func HashPassword(p string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(b), err
}

// Signup creates a user and its merchant account (status onboarding).
func (s *Service) Signup(in SignupInput) (*models.User, *models.Merchant, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.BusinessName = strings.TrimSpace(in.BusinessName)
	if in.BusinessName == "" {
		return nil, nil, Invalid("parameter_missing", "business_name", "Indique o nome do negócio.")
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		return nil, nil, Invalid("email_invalid", "email", "Email inválido.")
	}
	phone := strings.TrimPrefix(digitsOnly(in.Phone), "258")
	if !phoneRe.MatchString(phone) {
		return nil, nil, Invalid("phone_invalid", "phone", "Telemóvel inválido. Use 9 dígitos, por exemplo 84 123 4567.")
	}
	if e := ValidPassword(in.Password); e != nil {
		return nil, nil, e
	}
	var n int64
	s.DB.Model(&models.User{}).Where("email = ?", in.Email).Count(&n)
	if n > 0 {
		return nil, nil, Conflict("email_taken", "Já existe uma conta com este email.")
	}
	hash, err := HashPassword(in.Password)
	if err != nil {
		return nil, nil, err
	}
	m := &models.Merchant{
		ID: ids.New("acct"), Slug: s.uniqueMerchantSlug(Slugify(in.BusinessName)), Name: in.BusinessName,
		Email: in.Email, Phone: "+258" + phone, OwnerName: strings.TrimSpace(in.Name), Status: models.MerchantOnboarding,
		StatementDescriptor: descriptor(in.BusinessName), AvatarColor: avatarColors[int(time.Now().UnixNano())%len(avatarColors)],
		SigningSecretTest: ids.Token("whsec_", 32), SigningSecretLive: ids.Token("whsec_", 32), SMSNotifications: true,
	}
	u := &models.User{ID: ids.New("usr"), Email: in.Email, PasswordHash: hash, Name: m.OwnerName, Phone: m.Phone, Role: models.RoleMerchant, MerchantID: &m.ID}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		if err := tx.Create(u).Error; err != nil {
			return err
		}
		return s.ensurePublishableKeys(tx, m.ID)
	})
	if err != nil {
		return nil, nil, err
	}
	return u, m, nil
}

func descriptor(name string) string {
	d := "RIHA*" + strings.ToUpper(norm.NFD.String(name))
	var b strings.Builder
	for _, r := range d {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '*') {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > 22 {
		out = out[:22]
	}
	return strings.TrimSpace(out)
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var ErrBadCredentials = &Error{Status: 401, Type: "authentication_error", Code: "invalid_credentials", Message: "Email ou palavra-passe incorretos."}

func (s *Service) Login(email, password, ip, ua string) (string, *models.User, error) {
	var u models.User
	if err := s.DB.First(&u, "email = ?", strings.ToLower(strings.TrimSpace(email))).Error; err != nil {
		bcrypt.CompareHashAndPassword([]byte("$2a$10$0000000000000000000000000000000000000000000000000000"), []byte(password))
		return "", nil, ErrBadCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", nil, ErrBadCredentials
	}
	tok, err := s.NewSession(&u, ip, ua)
	if err != nil {
		return "", nil, err
	}
	now := s.Now()
	s.DB.Model(&u).Update("last_login_at", now)
	return tok, &u, nil
}

func (s *Service) NewSession(u *models.User, ip, ua string) (string, error) {
	tok := ids.Token("", 48)
	if len(ua) > 300 {
		ua = ua[:300]
	}
	err := s.DB.Create(&models.Session{ID: ids.Hash(tok), UserID: u.ID, ExpiresAt: s.Now().Add(SessionTTL), IP: ip, UserAgent: ua}).Error
	return tok, err
}

func (s *Service) UserForSession(tok string) (*models.User, error) {
	var sess models.Session
	if err := s.DB.First(&sess, "id = ? AND expires_at > ?", ids.Hash(tok), s.Now()).Error; err != nil {
		return nil, err
	}
	var u models.User
	if err := s.DB.First(&u, "id = ?", sess.UserID).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Service) Logout(tok string) { s.DB.Delete(&models.Session{}, "id = ?", ids.Hash(tok)) }

func (s *Service) ChangePassword(u *models.User, current, next string) error {
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(current)) != nil {
		return Invalid("password_incorrect", "current_password", "A palavra-passe atual está errada.")
	}
	if e := ValidPassword(next); e != nil {
		return e
	}
	h, err := HashPassword(next)
	if err != nil {
		return err
	}
	return s.DB.Model(u).Update("password_hash", h).Error
}

// ---------------------------------------------------------------- API keys

func (s *Service) ensurePublishableKeys(tx *gorm.DB, merchantID string) error {
	for _, live := range []bool{false, true} {
		var n int64
		tx.Model(&models.APIKey{}).Where("merchant_id = ? AND livemode = ? AND kind = ? AND revoked_at IS NULL", merchantID, live, models.KeyPublishable).Count(&n)
		if n > 0 {
			continue
		}
		mode := map[bool]string{true: "live", false: "test"}[live]
		tok := ids.Token("pk_"+mode+"_", 24)
		if err := tx.Create(&models.APIKey{ID: ids.New("key"), MerchantID: merchantID, Livemode: live, Kind: models.KeyPublishable, Token: tok, Last4: tok[len(tok)-4:]}).Error; err != nil {
			return err
		}
	}
	return nil
}

// RollSecretKey revokes any existing secret key for the mode and returns the
// new secret in clear text exactly once.
func (s *Service) RollSecretKey(merchantID string, live bool, actor string) (string, *models.APIKey, error) {
	mode := map[bool]string{true: "live", false: "test"}[live]
	secret := ids.Token("sk_"+mode+"_", 32)
	k := &models.APIKey{ID: "key_" + strings.ToLower(ids.Short(4)), MerchantID: merchantID, Livemode: live, Kind: models.KeySecret,
		Hash: ids.Hash(secret), Last4: secret[len(secret)-4:], CreatedBy: actor}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		now := s.Now()
		if err := tx.Model(&models.APIKey{}).Where("merchant_id = ? AND livemode = ? AND kind = ? AND revoked_at IS NULL", merchantID, live, models.KeySecret).
			Update("revoked_at", now).Error; err != nil {
			return err
		}
		for {
			var n int64
			tx.Model(&models.APIKey{}).Where("id = ?", k.ID).Count(&n)
			if n == 0 {
				break
			}
			k.ID = "key_" + strings.ToLower(ids.Short(4))
		}
		return tx.Create(k).Error
	})
	return secret, k, err
}

var ErrBadKey = &Error{Status: 401, Type: "authentication_error", Code: "api_key_invalid", Message: "Chave API inválida ou revogada."}

// AuthenticateSecret resolves a secret key.
func (s *Service) AuthenticateSecret(secret string) (*models.APIKey, *models.Merchant, error) {
	if !strings.HasPrefix(secret, "sk_test_") && !strings.HasPrefix(secret, "sk_live_") {
		if strings.HasPrefix(secret, "pk_") {
			return nil, nil, &Error{Status: 401, Type: "authentication_error", Code: "publishable_key_not_allowed", Message: "A chave pública não pode criar pagamentos. Use a chave secreta no servidor."}
		}
		return nil, nil, ErrBadKey
	}
	var k models.APIKey
	if err := s.DB.First(&k, "hash = ? AND kind = ? AND revoked_at IS NULL", ids.Hash(secret), models.KeySecret).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrBadKey
		}
		return nil, nil, err
	}
	var m models.Merchant
	if err := s.DB.First(&m, "id = ?", k.MerchantID).Error; err != nil {
		return nil, nil, ErrBadKey
	}
	now := s.Now()
	s.DB.Model(&k).Update("last_used_at", now)
	return &k, &m, nil
}

// ---------------------------------------------------------------- KYC

// KycCheck is one automated verification shown to the operations team.
type KycCheck struct {
	Kind  string `json:"kind"` // ok | warn | bad
	Label string `json:"label"`
	Note  string `json:"note"`
}

// KycChecks runs (mock) automated checks for a merchant.
func (s *Service) KycChecks(m *models.Merchant) []KycCheck {
	out := []KycCheck{}
	nuit := digitsOnly(m.NUIT)
	switch {
	case len(nuit) != 9:
		out = append(out, KycCheck{"bad", "NUIT inválido", "O NUIT tem de ter 9 dígitos."})
	case strings.HasPrefix(nuit, "409"):
		out = append(out, KycCheck{"bad", "NUIT não encontrado", "Sem registo na Autoridade Tributária (simulado)."})
	default:
		out = append(out, KycCheck{"ok", "NUIT válido", "Confirmado na Autoridade Tributária (simulado)."})
	}
	var bank models.BankAccount
	if err := s.DB.First(&bank, "merchant_id = ?", m.ID).Error; err != nil {
		out = append(out, KycCheck{"warn", "Sem conta bancária", "O comerciante ainda não indicou o NIB."})
	} else {
		h := strings.ToLower(strings.TrimSpace(bank.Holder))
		if h == strings.ToLower(m.LegalName) || h == strings.ToLower(m.Name) || h == strings.ToLower(m.OwnerName) {
			out = append(out, KycCheck{"ok", "Titular da conta coincide", bank.BankName + ": " + bank.Holder})
		} else {
			out = append(out, KycCheck{"warn", "Titular da conta diferente", "Conta em nome de " + bank.Holder + ", não de " + firstNonEmpty(m.LegalName, m.Name)})
		}
	}
	var docs int64
	s.DB.Model(&models.KycDocument{}).Where("merchant_id = ?", m.ID).Count(&docs)
	if docs >= 2 {
		out = append(out, KycCheck{"ok", "Documentos carregados", fmt.Sprintf("%d documentos", docs)})
	} else {
		out = append(out, KycCheck{"warn", "Documentos em falta", "Precisa do BI e do alvará ou certidão comercial."})
	}
	if strings.Contains(m.ExpectedVolume, "Mais de 1") {
		out = append(out, KycCheck{"warn", "Volume elevado", "Requer revisão de dois administradores."})
	}
	return out
}

// Risk summarises checks into Baixo / Médio / Alto.
func Risk(checks []KycCheck) string {
	warn := 0
	for _, c := range checks {
		if c.Kind == "bad" {
			return "Alto"
		}
		if c.Kind == "warn" {
			warn++
		}
	}
	if warn > 0 {
		return "Médio"
	}
	return "Baixo"
}

// Decide applies an admin decision on a merchant's verification.
func (s *Service) Decide(merchantID, decision, note, actor string) (*models.Merchant, error) {
	var m models.Merchant
	if err := s.DB.First(&m, "id = ?", merchantID).Error; err != nil {
		return nil, NotFound("O comerciante")
	}
	now := s.Now()
	switch decision {
	case "approve":
		m.Status, m.ApprovedAt = models.MerchantActive, &now
		s.DB.Model(&models.BankAccount{}).Where("merchant_id = ?", m.ID).Update("verified", true)
	case "reject":
		m.Status = models.MerchantRejected
	case "info":
		m.Status = models.MerchantInfoRequested
	case "suspend":
		m.Status = models.MerchantSuspended
	case "reactivate":
		if m.ApprovedAt != nil {
			m.Status = models.MerchantActive
		} else {
			m.Status = models.MerchantInReview
		}
	default:
		return nil, Invalid("decision_invalid", "decision", "Decisão desconhecida.")
	}
	m.ReviewNote, m.ReviewedBy = note, actor
	if err := s.DB.Save(&m).Error; err != nil {
		return nil, err
	}
	for _, live := range []bool{false, true} {
		s.DB.Transaction(func(tx *gorm.DB) error {
			return s.emit(tx, m.ID, live, "account.updated", m.ID, map[string]any{"id": m.ID, "object": "account", "status": m.Status}, "")
		})
	}
	return &m, nil
}
