package app

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"math/big"
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
	return a.Users.Create(ctx, &domain.User{ID: domain.NewID("usr"), Email: email, PasswordHash: hash, Name: name, Role: domain.RoleAdmin, AdminTitle: title, Permissions: []string{domain.PermAll}, CreatedAt: a.Now()})
}

const (
	// MaxFailedLogins wrong passwords in a row lock an account for LoginLock.
	MaxFailedLogins = 8
	LoginLock       = 15 * time.Minute
	LoginCodeTTL    = 10 * time.Minute
	// DeviceTTL is how long a device that entered a login code is known.
	DeviceTTL = 30 * 24 * time.Hour
)

func (a *App) Login(ctx context.Context, email, password, ip, ua string) (string, *domain.User, error) {
	r, err := a.SignIn(ctx, LoginInput{Email: email, Password: password, IP: ip, UA: ua})
	return r.Token, r.User, err
}

// LoginInput is a sign in: Device is the token of a device that entered a
// login code before, and Code the one emailed now.
type LoginInput struct {
	Email, Password, Code, Device, IP, UA string
}

type LoginResult struct {
	Token string
	User  *domain.User
	// Device is set when this sign in made the device known.
	Device string
}

// SignIn checks the password, with guessing slowed by a lock after too many
// wrong ones. Staff and partners, who can move money, also enter a code
// emailed to them on a device RIHA doesn't know: a stolen password alone
// doesn't open the backoffice.
func (a *App) SignIn(ctx context.Context, in LoginInput) (LoginResult, error) {
	var out LoginResult
	u, err := a.Users.ByEmail(ctx, strings.ToLower(strings.TrimSpace(in.Email)))
	if err != nil {
		a.Hasher.Matches("", in.Password)
		return out, domain.ErrBadCredentials
	}
	now := a.Now()
	if u.LockedUntil != nil && now.Before(*u.LockedUntil) {
		return out, domain.ErrLoginLocked
	}
	if !a.Hasher.Matches(u.PasswordHash, in.Password) || u.DisabledAt != nil {
		if u.DisabledAt == nil {
			_ = a.Users.LoginFailed(ctx, u.ID, now, MaxFailedLogins, LoginLock)
		}
		return out, domain.ErrBadCredentials
	}
	if a.LoginCodes && (u.Role == domain.RoleAdmin || u.Role == domain.RolePartner) {
		known := false
		if in.Device != "" {
			if known, err = a.Resets.DeviceTrusted(ctx, u.ID, domain.Hash(in.Device), now); err != nil {
				return out, err
			}
		}
		if !known {
			code := strings.Join(strings.Fields(in.Code), "")
			if code == "" {
				if err := a.sendLoginCode(ctx, u, in); err != nil {
					return out, err
				}
				return out, domain.ErrLoginCode
			}
			c, err := a.Resets.LoginCode(ctx, u.ID)
			if err != nil || now.After(c.ExpiresAt) || c.Attempts >= 5 {
				return out, domain.ErrLoginCodeWrong
			}
			if err := a.Resets.TryLoginCode(ctx, u.ID); err != nil {
				return out, err
			}
			if subtle.ConstantTimeCompare([]byte(c.CodeHash), []byte(domain.Hash(u.ID+":"+code))) != 1 {
				return out, domain.ErrLoginCodeWrong
			}
			if err := a.Resets.SpendLoginCode(ctx, u.ID); err != nil {
				return out, err
			}
			out.Device = domain.Token("", 32)
			if err := a.Resets.TrustDevice(ctx, &domain.TrustedDevice{TokenHash: domain.Hash(out.Device), UserID: u.ID, ExpiresAt: now.Add(DeviceTTL), CreatedAt: now}); err != nil {
				return out, err
			}
			a.Record(ctx, "", Actor{ID: u.ID, Role: u.Role, IP: in.IP}, "auth.device_added", u.ID, domain.Trim(in.UA, 160))
		}
	}
	tok, err := a.NewSession(ctx, u, in.IP, in.UA)
	if err != nil {
		return out, err
	}
	_ = a.Users.TouchLogin(ctx, u.ID, now)
	out.Token, out.User = tok, u
	return out, nil
}

// sendLoginCode emails a new code, unless one went out in the last minute.
func (a *App) sendLoginCode(ctx context.Context, u *domain.User, in LoginInput) error {
	now := a.Now()
	if c, err := a.Resets.LoginCode(ctx, u.ID); err == nil && now.Sub(c.CreatedAt) < time.Minute && now.Before(c.ExpiresAt) {
		return nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		if err := a.Resets.SaveLoginCode(ctx, &domain.LoginCode{UserID: u.ID, CodeHash: domain.Hash(u.ID + ":" + code), ExpiresAt: now.Add(LoginCodeTTL), CreatedAt: now}); err != nil {
			return err
		}
		return a.mailLoginCode(ctx, u, code, in.IP)
	})
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
	u, err := a.Users.ByID(ctx, id)
	if err == nil && u.DisabledAt != nil {
		return nil, domain.ErrNotFound
	}
	return u, err
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
	if err != nil || u.DisabledAt != nil {
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
