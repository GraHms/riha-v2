package app

import (
	"context"
	"errors"
	"log"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"riha/internal/core/domain"
)

const (
	SessionTTL       = 14 * 24 * time.Hour
	PasswordResetTTL = time.Hour
	MaxResetsPerHour = 3
)

var avatarColors = []string{"#FFD21F", "#E2F3DE", "#FFF0D6", "#EDECE6", "#FFE3E1"}
var localPhone = regexp.MustCompile(`^[0-9]{9}$`)

type SignupInput struct {
	BusinessName string `json:"business_name"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Password     string `json:"password"`
}

func validPassword(p string) error {
	if len(p) < 8 {
		return domain.Invalid("password_weak", "password", "A palavra-passe precisa de pelo menos 8 caracteres.")
	}
	return nil
}

func (a *App) uniqueMerchantSlug(ctx context.Context, base string) string {
	slug := base
	for i := 2; ; i++ {
		taken, _ := a.Merchants.SlugTaken(ctx, slug)
		// A catalog's address answers before a merchant's slug, so a new account
		// can't take one.
		if _, err := a.SubSettings.ByCatalogSlug(ctx, slug); err == nil {
			taken = true
		}
		if !taken {
			return slug
		}
		slug = base + "-" + itoa(i)
	}
}

func (a *App) Signup(ctx context.Context, in SignupInput) (*domain.User, *domain.Merchant, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.BusinessName = strings.TrimSpace(in.BusinessName)
	if in.BusinessName == "" {
		return nil, nil, domain.Invalid("parameter_missing", "business_name", "Indique o nome do negócio.")
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		return nil, nil, domain.Invalid("email_invalid", "email", "Email inválido.")
	}
	phone := domain.NormalizeMZPhone(in.Phone)
	if !localPhone.MatchString(phone) {
		return nil, nil, domain.Invalid("phone_invalid", "phone", "Telemóvel inválido. Use 9 dígitos, por exemplo 84 123 4567.")
	}
	if err := validPassword(in.Password); err != nil {
		return nil, nil, err
	}
	if taken, _ := a.Users.EmailTaken(ctx, in.Email); taken {
		return nil, nil, domain.Conflict("email_taken", "Já existe uma conta com este email.")
	}
	hash, err := a.Hasher.Hash(in.Password)
	if err != nil {
		return nil, nil, err
	}
	now := a.Now()
	m := &domain.Merchant{
		ID: domain.NewID("acct"), Slug: a.uniqueMerchantSlug(ctx, domain.Slugify(in.BusinessName)), Name: in.BusinessName,
		Email: in.Email, Phone: "+258" + phone, OwnerName: strings.TrimSpace(in.Name), Status: domain.MerchantLead,
		StatementDescriptor: domain.Descriptor(in.BusinessName), AvatarColor: avatarColors[int(now.UnixNano())%len(avatarColors)],
		SigningSecretTest: domain.Token("whsec_", 32), SigningSecretLive: domain.Token("whsec_", 32), SMSNotifications: true, EmailNotifications: true, CreatedAt: now,
	}
	u := &domain.User{ID: domain.NewID("usr"), Email: in.Email, PasswordHash: hash, Name: m.OwnerName, Phone: m.Phone, Role: domain.RoleMerchant, MerchantID: &m.ID, CreatedAt: now}
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		if err := a.Merchants.Create(ctx, m); err != nil {
			return err
		}
		if err := a.Users.Create(ctx, u); err != nil {
			return err
		}
		if err := a.ensurePublishableKeys(ctx, m.ID); err != nil {
			return err
		}
		return a.mailWelcome(ctx, m)
	})
	if err != nil {
		return nil, nil, err
	}
	return u, m, nil
}

func (a *App) CreateAdmin(ctx context.Context, email, password, name, title string) error {
	hash, err := a.Hasher.Hash(password)
	if err != nil {
		return err
	}
	return a.Users.Create(ctx, &domain.User{ID: domain.NewID("usr"), Email: email, PasswordHash: hash, Name: name, Role: domain.RoleAdmin, AdminTitle: title, CreatedAt: a.Now()})
}

func (a *App) Login(ctx context.Context, email, password, ip, ua string) (string, *domain.User, error) {
	u, err := a.Users.ByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		a.Hasher.Matches("", password)
		return "", nil, domain.ErrBadCredentials
	}
	if !a.Hasher.Matches(u.PasswordHash, password) {
		return "", nil, domain.ErrBadCredentials
	}
	tok, err := a.NewSession(ctx, u, ip, ua)
	if err != nil {
		return "", nil, err
	}
	_ = a.Users.TouchLogin(ctx, u.ID, a.Now())
	return tok, u, nil
}

func (a *App) NewSession(ctx context.Context, u *domain.User, ip, ua string) (string, error) {
	tok := domain.Token("", 48)
	err := a.AuthSessions.Create(ctx, &domain.AuthSession{TokenHash: domain.Hash(tok), UserID: u.ID, ExpiresAt: a.Now().Add(SessionTTL), IP: ip, UserAgent: domain.Trim(ua, 300)})
	return tok, err
}

func (a *App) UserForSession(ctx context.Context, tok string) (*domain.User, error) {
	id, err := a.AuthSessions.UserID(ctx, domain.Hash(tok), a.Now())
	if err != nil {
		return nil, err
	}
	return a.Users.ByID(ctx, id)
}

func (a *App) Logout(ctx context.Context, tok string) {
	_ = a.AuthSessions.Delete(ctx, domain.Hash(tok))
}

func (a *App) ChangePassword(ctx context.Context, u *domain.User, current, next, keepToken string) error {
	if !a.Hasher.Matches(u.PasswordHash, current) {
		return domain.Invalid("password_incorrect", "current_password", "A palavra-passe atual está errada.")
	}
	if err := validPassword(next); err != nil {
		return err
	}
	h, err := a.Hasher.Hash(next)
	if err != nil {
		return err
	}
	if err := a.Users.SetPassword(ctx, u.ID, h); err != nil {
		return err
	}
	if err := a.AuthSessions.DeleteForUser(ctx, u.ID, domain.Hash(keepToken)); err != nil {
		return err
	}
	if err := a.mailPasswordChanged(ctx, u); err != nil {
		log.Printf("email security.password_changed: %v", err)
	}
	return nil
}

// RequestPasswordReset answers the same whether or not the account exists, so
// the form can't be used to find out who has one.
func (a *App) RequestPasswordReset(ctx context.Context, email string) error {
	u, err := a.Users.ByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil
	}
	now := a.Now()
	if n, err := a.Resets.CountSince(ctx, u.ID, now.Add(-time.Hour)); err != nil || n >= MaxResetsPerHour {
		return err
	}
	token := domain.Token("", 40)
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		if err := a.Resets.Create(ctx, &domain.PasswordReset{TokenHash: domain.Hash(token), UserID: u.ID, ExpiresAt: now.Add(PasswordResetTTL), CreatedAt: now}); err != nil {
			return err
		}
		return a.mailPasswordReset(ctx, u, token)
	})
}

// ResetPassword spends the token and signs out every device, in case someone
// else was in the account.
func (a *App) ResetPassword(ctx context.Context, token, next string) error {
	if err := validPassword(next); err != nil {
		return err
	}
	h, err := a.Hasher.Hash(next)
	if err != nil {
		return err
	}
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		now := a.Now()
		id, err := a.Resets.Take(ctx, domain.Hash(token), now)
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Invalid("reset_invalid", "token", "Este link já não é válido. Peça um novo.")
		}
		if err != nil {
			return err
		}
		u, err := a.Users.ByID(ctx, id)
		if err != nil {
			return err
		}
		if err := a.Users.SetPassword(ctx, id, h); err != nil {
			return err
		}
		if err := a.Resets.SpendAll(ctx, id, now); err != nil {
			return err
		}
		if err := a.AuthSessions.DeleteForUser(ctx, id, ""); err != nil {
			return err
		}
		return a.mailPasswordChanged(ctx, u)
	})
}

func (a *App) EnsureAdmin(ctx context.Context, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil
	}
	if taken, err := a.Users.EmailTaken(ctx, email); err != nil || taken {
		return err
	}
	if len(password) < 12 {
		return domain.Invalid("password_weak", "RIHA_ADMIN_PASSWORD", "A palavra-passe do administrador precisa de pelo menos 12 caracteres.")
	}
	return a.CreateAdmin(ctx, email, password, "Administrador", "Operações")
}

func (a *App) MerchantForUser(ctx context.Context, u *domain.User) (*domain.Merchant, error) {
	if u.MerchantID == nil {
		return nil, domain.Forbidden("forbidden", "Esta conta não tem negócio associado.")
	}
	return a.Merchants.ByID(ctx, *u.MerchantID)
}

func (a *App) ensurePublishableKeys(ctx context.Context, merchantID string) error {
	for _, live := range []bool{false, true} {
		if _, err := a.Keys.Active(ctx, merchantID, live, domain.KeyPublishable); err == nil {
			continue
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		tok := domain.Token("pk_"+domain.Mode(live)+"_", 24)
		if err := a.Keys.Create(ctx, &domain.APIKey{ID: domain.NewID("key"), MerchantID: merchantID, Livemode: live, Kind: domain.KeyPublishable, Token: tok, Last4: tok[len(tok)-4:], CreatedAt: a.Now()}); err != nil {
			return err
		}
	}
	return nil
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
