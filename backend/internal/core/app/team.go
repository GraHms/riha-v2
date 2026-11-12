package app

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"riha/internal/core/domain"
)

// InviteTTL is how long the link in an invitation lets someone set a password.
const InviteTTL = 7 * 24 * time.Hour

// TeamTarget names a team: RIHA's staff, or one partner's users.
type TeamTarget struct {
	Partner string // "" for staff
}

func (t TeamTarget) role() string {
	if t.Partner == "" {
		return domain.RoleAdmin
	}
	return domain.RolePartner
}

// canManage says whether by may manage the team t: staff with the team
// permission manage staff and every partner's users; a partner's user with the
// team permission manages only its own partner's.
func canManage(by *domain.User, t TeamTarget) bool {
	switch by.Role {
	case domain.RoleAdmin:
		return by.Can(domain.PermTeam)
	case domain.RolePartner:
		return t.Partner != "" && t.Partner == by.PartnerCode && by.Can(domain.PartnerTeam)
	}
	return false
}

func (a *App) Team(ctx context.Context, by *domain.User, t TeamTarget) ([]domain.User, error) {
	if !canManage(by, t) && !(by.Role == domain.RolePartner && by.PartnerCode == t.Partner) {
		return nil, domain.Forbidden("forbidden", "Sem acesso a esta equipa.")
	}
	if t.Partner != "" {
		if _, err := a.Integrations.Partner(ctx, t.Partner); err != nil {
			return nil, notFound(err, "O parceiro")
		}
	}
	return a.Users.Team(ctx, t.role(), t.Partner)
}

// InviteInput is someone to add to a team.
type InviteInput struct {
	Name, Email, Title string
	Permissions        []string
}

// Invite adds someone to a team with the permissions given, and emails them a
// link to set their password.
func (a *App) Invite(ctx context.Context, by *domain.User, t TeamTarget, in InviteInput) (*domain.User, error) {
	if !canManage(by, t) {
		return nil, domain.Forbidden("forbidden", "Não pode convidar para esta equipa.")
	}
	var partner *domain.Partner
	if t.Partner != "" {
		p, err := a.Integrations.Partner(ctx, t.Partner)
		if err != nil {
			return nil, notFound(err, "O parceiro")
		}
		partner = p
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if _, err := mail.ParseAddress(email); err != nil || len(email) > 200 {
		return nil, domain.Invalid("email_invalid", "email", "Email inválido.")
	}
	name := domain.Trim(in.Name, 200)
	if name == "" {
		return nil, domain.Invalid("name_required", "name", "Indique o nome.")
	}
	if taken, err := a.Users.EmailTaken(ctx, email); err != nil {
		return nil, err
	} else if taken {
		return nil, domain.Conflict("email_taken", "Já existe uma conta RIHA com este email.")
	}
	perms := domain.CleanPermissions(in.Permissions, t.Partner == "")
	if len(perms) == 0 {
		return nil, domain.Invalid("permissions_required", "permissions", "Escolha pelo menos uma permissão.")
	}
	if !holdsAll(by, t, perms) {
		return nil, domain.Forbidden("forbidden", "Só pode dar permissões que tem.")
	}
	hash, err := a.Hasher.Hash(domain.Token("", 32))
	if err != nil {
		return nil, err
	}
	now := a.Now()
	u := &domain.User{ID: domain.NewID("usr"), Email: email, PasswordHash: hash, Name: name, Role: t.role(), AdminTitle: domain.Trim(in.Title, 80),
		Permissions: perms, PartnerCode: t.Partner, CreatedAt: now}
	token := domain.Token("", 40)
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		if err := a.Users.Create(ctx, u); err != nil {
			return err
		}
		if err := a.Resets.Create(ctx, &domain.PasswordReset{TokenHash: domain.Hash(token), UserID: u.ID, ExpiresAt: now.Add(InviteTTL), CreatedAt: now}); err != nil {
			return err
		}
		return a.mailInvite(ctx, by, u, partner, token)
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, "", Actor{ID: by.ID, Role: by.Role}, "team.invited", u.ID, fmt.Sprintf("%s (%s) · %s · %s", name, email, teamName(partner), strings.Join(perms, ", ")))
	return u, nil
}

// holdsAll tells whether by may give perms to someone on team t: only the
// permissions it holds itself, and "*" only if it holds "*". Staff who manage
// the team give partners' users their partner permissions, which are another
// set, about the partner's own account.
func holdsAll(by *domain.User, t TeamTarget, perms []string) bool {
	if by.Role == domain.RoleAdmin && t.Partner != "" {
		return by.Can(domain.PermTeam)
	}
	for _, p := range perms {
		if p == domain.PermAll {
			if !domain.Contains(by.Permissions, domain.PermAll) {
				return false
			}
		} else if !by.Can(p) {
			return false
		}
	}
	return true
}

func teamName(p *domain.Partner) string {
	if p == nil {
		return "equipa RIHA"
	}
	return p.Name
}

// AccessInput changes a member's permissions, title or whether the account is
// open; nil leaves a field as it is.
type AccessInput struct {
	Permissions []string
	Title       *string
	Disabled    *bool
}

func (a *App) SetAccess(ctx context.Context, by *domain.User, userID string, in AccessInput) (*domain.User, error) {
	u, err := a.Users.ByID(ctx, userID)
	if err != nil || (u.Role != domain.RoleAdmin && u.Role != domain.RolePartner) {
		return nil, domain.NotFound("A pessoa")
	}
	t := TeamTarget{Partner: u.PartnerCode}
	if !canManage(by, t) {
		return nil, domain.NotFound("A pessoa")
	}
	before := describeAccess(u)
	// Nobody changes their own permissions, and nobody changes or closes the
	// account of someone who can do more than they can.
	if in.Permissions != nil && u.ID == by.ID {
		return nil, domain.Forbidden("own_access", "Não pode mudar as suas próprias permissões. Peça a outra pessoa da equipa.")
	}
	if u.ID != by.ID && !holdsAll(by, t, u.Permissions) {
		return nil, domain.Forbidden("forbidden", "Esta pessoa tem permissões que não tem. Só quem as tem pode mudar o seu acesso.")
	}
	if in.Permissions != nil {
		perms := domain.CleanPermissions(in.Permissions, u.Role == domain.RoleAdmin)
		if len(perms) == 0 {
			return nil, domain.Invalid("permissions_required", "permissions", "Escolha pelo menos uma permissão, ou feche a conta.")
		}
		if !holdsAll(by, t, perms) {
			return nil, domain.Forbidden("forbidden", "Só pode dar permissões que tem.")
		}
		u.Permissions = perms
	}
	if in.Title != nil {
		u.AdminTitle = domain.Trim(*in.Title, 80)
	}
	if in.Disabled != nil {
		if *in.Disabled && u.DisabledAt == nil {
			now := a.Now()
			u.DisabledAt = &now
		} else if !*in.Disabled {
			u.DisabledAt = nil
		}
	}
	// Nobody locks themselves out, and the team is never left without someone
	// who can manage it.
	if u.ID == by.ID && (u.DisabledAt != nil || !u.Can(teamPerm(u))) {
		return nil, domain.Invalid("own_access", "permissions", "Não pode tirar a si próprio o acesso à equipa. Peça a outra pessoa.")
	}
	if !u.Can(teamPerm(u)) {
		team, err := a.Users.Team(ctx, u.Role, u.PartnerCode)
		if err != nil {
			return nil, err
		}
		left := 0
		for i := range team {
			if team[i].ID != u.ID && team[i].Can(teamPerm(u)) {
				left++
			}
		}
		if left == 0 && u.Role == domain.RoleAdmin {
			return nil, domain.Invalid("last_manager", "permissions", "É a única pessoa que gere a equipa. Dê essa permissão a outra pessoa primeiro.")
		}
	}
	if err := a.Users.SetAccess(ctx, u); err != nil {
		return nil, err
	}
	if u.DisabledAt != nil {
		if err := a.AuthSessions.DeleteForUser(ctx, u.ID, ""); err != nil {
			return nil, err
		}
	}
	a.Record(ctx, "", Actor{ID: by.ID, Role: by.Role}, "team.access", u.ID, u.Email+": "+before+" → "+describeAccess(u))
	return u, nil
}

func teamPerm(u *domain.User) string {
	if u.Role == domain.RolePartner {
		return domain.PartnerTeam
	}
	return domain.PermTeam
}

func describeAccess(u *domain.User) string {
	if u.DisabledAt != nil {
		return "fechada"
	}
	return strings.Join(u.Permissions, ", ")
}

// ResendInvite sends a new link to someone who hasn't set a password yet, or
// lost it.
func (a *App) ResendInvite(ctx context.Context, by *domain.User, userID string) error {
	u, err := a.Users.ByID(ctx, userID)
	if err != nil || (u.Role != domain.RoleAdmin && u.Role != domain.RolePartner) || !canManage(by, TeamTarget{Partner: u.PartnerCode}) {
		return domain.NotFound("A pessoa")
	}
	if u.DisabledAt != nil {
		return domain.Invalid("account_closed", "user", "A conta está fechada. Reabra-a primeiro.")
	}
	var partner *domain.Partner
	if u.PartnerCode != "" {
		partner, _ = a.Integrations.Partner(ctx, u.PartnerCode)
	}
	now := a.Now()
	token := domain.Token("", 40)
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		if err := a.Resets.Create(ctx, &domain.PasswordReset{TokenHash: domain.Hash(token), UserID: u.ID, ExpiresAt: now.Add(InviteTTL), CreatedAt: now}); err != nil {
			return err
		}
		return a.mailInvite(ctx, by, u, partner, token)
	})
}

func (a *App) mailInvite(ctx context.Context, by, u *domain.User, partner *domain.Partner, token string) error {
	where := "na equipa da RIHA"
	if partner != nil {
		where = "na conta de parceiro de " + partner.Name + " na RIHA"
	}
	return a.queue(ctx, "team.invite", domain.Notice{
		To: u.Email, Subject: "Foi convidado para a RIHA", Title: "Foi convidado para a RIHA",
		Lines:  []string{hello(u.Name), fmt.Sprintf("%s adicionou-o %s. Escolha uma palavra-passe para entrar. O link é válido durante 7 dias e só funciona uma vez.", domain.FirstNonEmpty(by.Name, "A RIHA"), where)},
		Action: "Escolher a palavra-passe", URL: a.link("/recuperar/nova#" + token),
		Small: "Se não esperava este convite, ignore este email.",
	})
}

// PartnerInput creates or renames a partner.
type PartnerInput struct {
	Code, Name, Email string
}

func (a *App) CreatePartner(ctx context.Context, by *domain.User, in PartnerInput) (*domain.Partner, error) {
	if !by.Can(domain.PermTeam) || by.Role != domain.RoleAdmin {
		return nil, domain.Forbidden("forbidden", "Sem acesso.")
	}
	code := strings.ToLower(strings.TrimSpace(in.Code))
	if !domain.ValidIntegrationCode(code) || code == domain.FundRIHA {
		return nil, domain.Invalid("code_invalid", "code", "O código usa letras minúsculas, números e _, e começa por uma letra.")
	}
	if _, err := a.Integrations.Partner(ctx, code); err == nil {
		return nil, domain.Conflict("code_taken", "Já existe um parceiro com este código.")
	}
	name := domain.Trim(in.Name, 120)
	if name == "" {
		return nil, domain.Invalid("name_required", "name", "Indique o nome do parceiro.")
	}
	p := &domain.Partner{Code: code, Name: name, Email: domain.Trim(strings.ToLower(in.Email), 200), CreatedAt: a.Now()}
	if err := a.Integrations.CreatePartner(ctx, p); err != nil {
		return nil, err
	}
	a.Record(ctx, "", Actor{ID: by.ID, Role: by.Role}, "partner.created", code, name)
	return p, nil
}

func (a *App) RenamePartner(ctx context.Context, by *domain.User, code string, in PartnerInput) (*domain.Partner, error) {
	if !by.Can(domain.PermTeam) || by.Role != domain.RoleAdmin {
		return nil, domain.Forbidden("forbidden", "Sem acesso.")
	}
	p, err := a.Integrations.Partner(ctx, code)
	if err != nil {
		return nil, notFound(err, "O parceiro")
	}
	if name := domain.Trim(in.Name, 120); name != "" {
		p.Name = name
	}
	p.Email = domain.Trim(strings.ToLower(in.Email), 200)
	if err := a.Integrations.SavePartner(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// PartnerView is a partner as staff see it: its integrations and its people.
type PartnerView struct {
	domain.Partner
	Integrations []domain.Integration
	Users        []domain.User
}

func (a *App) PartnerViews(ctx context.Context) ([]PartnerView, error) {
	ps, err := a.Integrations.Partners(ctx)
	if err != nil {
		return nil, err
	}
	out := []PartnerView{}
	for _, p := range ps {
		if p.Code == domain.PartnerRIHA {
			continue
		}
		users, _ := a.Users.Team(ctx, domain.RolePartner, p.Code)
		out = append(out, PartnerView{Partner: p, Integrations: a.integrationsOf(ctx, p.Code), Users: users})
	}
	return out, nil
}
