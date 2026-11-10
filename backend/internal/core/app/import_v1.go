package app

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// V1Account is one account from the first RIHA, as exported from its database.
type V1Account struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	BusinessName string    `json:"business_name"`
	Phone        string    `json:"phone"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

// V1Outcome says what the import did, or would do, with one old account.
type V1Outcome struct {
	V1ID       string
	Business   string
	Email      string
	Imported   bool
	Reason     string // why it was skipped
	MerchantID string
	Slug       string
}

// ImportV1 brings the first RIHA's accounts over: only the people and their
// businesses, no money, plans or links. Each keeps their password, since both
// versions hash with bcrypt, and starts as a lead, like a new signup, because
// the old approval didn't check what the new one does.
//
// Someone who already made an account here is left alone. That is decided by
// email, and also by phone or business name, since some came back with a new
// email. Without apply it changes nothing and reports what it would do.
func (a *App) ImportV1(ctx context.Context, accounts []V1Account, apply bool) ([]V1Outcome, error) {
	existing, err := a.Merchants.List(ctx, ports.MerchantFilter{})
	if err != nil {
		return nil, err
	}
	phones, names, v1 := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, m := range existing {
		if p := domain.NormalizeMZPhone(m.Phone); p != "" {
			phones[p] = true
		}
		names[nameKey(m.Name)] = true
		if m.V1ID != "" {
			v1[m.V1ID] = true
		}
	}
	emails := map[string]bool{}
	slugs := map[string]bool{}
	out := make([]V1Outcome, 0, len(accounts))
	for _, in := range accounts {
		email := strings.ToLower(strings.TrimSpace(in.Email))
		business := strings.TrimSpace(domain.FirstNonEmpty(in.BusinessName, in.Name, strings.Split(email, "@")[0]))
		phone := domain.NormalizeMZPhone(in.Phone)
		if !localPhone.MatchString(phone) {
			phone = ""
		}
		o := V1Outcome{V1ID: in.ID, Business: business, Email: email}
		skip := func(why string) { o.Reason = why; out = append(out, o) }

		switch {
		case in.ID == "":
			skip("sem id da v1")
			continue
		case v1[in.ID]:
			skip("já importada")
			continue
		case !validEmail(email):
			skip("email inválido")
			continue
		case !strings.HasPrefix(in.PasswordHash, "$2"):
			skip("palavra-passe num formato desconhecido")
			continue
		case emails[email]:
			skip("email repetido na v1")
			continue
		}
		if taken, err := a.Users.EmailTaken(ctx, email); err != nil {
			return out, err
		} else if taken {
			skip("já tem conta (mesmo email)")
			continue
		}
		if phone != "" && phones[phone] {
			skip("já tem conta (mesmo telemóvel)")
			continue
		}
		if names[nameKey(business)] {
			skip("já tem conta (mesmo nome de negócio)")
			continue
		}
		emails[email] = true

		slug := a.uniqueMerchantSlug(ctx, domain.FirstNonEmpty(domain.Slugify(business), "conta"))
		for base, i := slug, 2; slugs[slug]; i++ {
			slug = base + "-" + itoa(i)
		}
		slugs[slug] = true
		created := in.CreatedAt.UTC()
		if created.IsZero() {
			created = a.Now()
		}
		full := ""
		if phone != "" {
			full = "+258" + phone
		}
		m := &domain.Merchant{
			ID: domain.NewID("acct"), Slug: slug, Name: business, Email: email, Phone: full,
			OwnerName: strings.TrimSpace(in.Name), Status: domain.MerchantLead,
			StatementDescriptor: domain.Descriptor(business), AvatarColor: avatarColors[len(out)%len(avatarColors)],
			SigningSecretTest: domain.Token("whsec_", 32), SigningSecretLive: domain.Token("whsec_", 32),
			SMSNotifications: true, EmailNotifications: true, ImportedFrom: "v1", V1ID: in.ID, CreatedAt: created,
		}
		u := &domain.User{ID: domain.NewID("usr"), Email: email, PasswordHash: in.PasswordHash, Name: m.OwnerName, Phone: full,
			Role: domain.RoleMerchant, MerchantID: &m.ID, CreatedAt: created}
		o.Imported, o.MerchantID, o.Slug = true, m.ID, slug
		if apply {
			err := a.Tx.Do(ctx, func(ctx context.Context) error {
				if err := a.Merchants.Create(ctx, m); err != nil {
					return err
				}
				if err := a.Users.Create(ctx, u); err != nil {
					return err
				}
				return a.ensurePublishableKeys(ctx, m.ID)
			})
			if err != nil {
				return out, err
			}
		}
		// Old accounts that share a phone or a name with each other still come
		// over: only an account made here means someone came back.
		out = append(out, o)
	}
	return out, nil
}

func validEmail(e string) bool {
	addr, err := mail.ParseAddress(e)
	return err == nil && addr.Address == e
}

func nameKey(s string) string { return domain.Slugify(s) }
