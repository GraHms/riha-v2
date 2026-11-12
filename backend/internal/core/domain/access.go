package domain

import "time"

// RolePartner is a user of a partner's account: the people at mtoca or 92
// Interactions who see what their integrations collected and pay withdrawals.
const RolePartner = "partner"

// PermAll gives every permission of the user's kind.
const PermAll = "*"

// Staff permissions: each opens one area of the backoffice. The overview is
// open to all staff.
const (
	PermApprovals    = "approvals"
	PermMerchants    = "merchants"
	PermPricing      = "pricing"
	PermPayouts      = "payouts"
	PermTransactions = "transactions"
	PermSettlement   = "settlement"
	PermMetrics      = "metrics"
	PermTeam         = "team"
)

// Partner permissions: what a partner's user can see and do in their account.
const (
	PartnerMoney   = "money"
	PartnerPayouts = "payouts"
	PartnerTeam    = "team"
)

type Permission struct {
	Key   string
	Label string
	Help  string
}

var StaffPermissions = []Permission{
	{PermApprovals, "Aprovações", "Rever e decidir pedidos de adesão e documentos."},
	{PermMerchants, "Comerciantes", "Ver comerciantes, suspender e reativar, programa beta."},
	{PermPricing, "Taxas e acordos", "Taxas dos comerciantes, acordos com parceiros e por onde entra o dinheiro."},
	{PermPayouts, "Levantamentos", "Aprovar, recusar e concluir levantamentos."},
	{PermTransactions, "Transações e rasto", "Ver pagamentos, o rasto e perguntar ao operador."},
	{PermSettlement, "Liquidação", "Ver onde está o dinheiro e registar recebimentos dos parceiros."},
	{PermMetrics, "Métricas", "Ver as métricas da plataforma."},
	{PermTeam, "Equipa e parceiros", "Convidar pessoas, mudar permissões e gerir contas de parceiros."},
}

var PartnerPermissions = []Permission{
	{PartnerMoney, "Dinheiro e acordo", "O acordo, o que foi recebido, a parte do parceiro e o que falta pagar à RIHA."},
	{PartnerPayouts, "Levantamentos", "Ver os levantamentos a pagar e marcá-los como pagos com o comprovativo."},
	{PartnerTeam, "Equipa", "Convidar colegas e mudar o que cada um vê."},
}

func permissionKeys(ps []Permission) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Key
	}
	return out
}

// CleanPermissions keeps the known permissions of a kind, in their order.
func CleanPermissions(in []string, staff bool) []string {
	known := PartnerPermissions
	if staff {
		known = StaffPermissions
	}
	if contains(in, PermAll) {
		return []string{PermAll}
	}
	out := []string{}
	for _, k := range permissionKeys(known) {
		if contains(in, k) {
			out = append(out, k)
		}
	}
	return out
}

// Can says whether a user has a permission; a merchant's user has none.
func (u *User) Can(perm string) bool {
	if u == nil || u.DisabledAt != nil || (u.Role != RoleAdmin && u.Role != RolePartner) {
		return false
	}
	return contains(u.Permissions, PermAll) || contains(u.Permissions, perm)
}

// A Partner owns integrations: the account customers' money lands in, until
// it pays RIHA or the merchants' withdrawals from it.
type Partner struct {
	Code      string
	Name      string
	Email     string
	CreatedAt time.Time
}

const (
	PartnerRIHA           = "riha"
	PartnerMtoca          = "mtoca"
	PartnerInteractions92 = "interactions92"
)

func DefaultPartners() []Partner {
	return []Partner{
		{Code: PartnerMtoca, Name: "Equipa mtoca"},
		{Code: PartnerInteractions92, Name: "92 Interactions"},
		{Code: PartnerRIHA, Name: "RIHA"},
	}
}
