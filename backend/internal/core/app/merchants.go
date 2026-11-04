package app

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"riha/internal/core/domain"
)

type ProfileInput struct {
	Name                *string `json:"name"`
	LegalName           *string `json:"legal_name"`
	Phone               *string `json:"phone"`
	City                *string `json:"city"`
	OwnerName           *string `json:"owner_name"`
	StatementDescriptor *string `json:"statement_descriptor"`
	SMSNotifications    *bool   `json:"sms_notifications"`
	EmailNotifications  *bool   `json:"email_notifications"`
}

func set(dst *string, v *string, n int) {
	if v != nil {
		*dst = domain.Trim(*v, n)
	}
}

func (a *App) UpdateProfile(ctx context.Context, m *domain.Merchant, in ProfileInput) error {
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		return domain.Invalid("parameter_invalid", "name", "O nome comercial não pode ficar vazio.")
	}
	set(&m.Name, in.Name, 200)
	set(&m.LegalName, in.LegalName, 200)
	set(&m.Phone, in.Phone, 40)
	set(&m.City, in.City, 80)
	set(&m.OwnerName, in.OwnerName, 200)
	if in.StatementDescriptor != nil {
		d := strings.ToUpper(strings.TrimSpace(*in.StatementDescriptor))
		if len(d) > 22 {
			return domain.Invalid("parameter_invalid", "statement_descriptor", "O nome no extrato tem no máximo 22 caracteres.")
		}
		m.StatementDescriptor = d
	}
	if in.SMSNotifications != nil {
		m.SMSNotifications = *in.SMSNotifications
	}
	if in.EmailNotifications != nil {
		m.EmailNotifications = *in.EmailNotifications
	}
	return a.Merchants.Save(ctx, m)
}

type KycInput struct {
	BusinessType   string `json:"business_type"`
	NUIT           string `json:"nuit"`
	LegalName      string `json:"legal_name"`
	City           string `json:"city"`
	ExpectedVolume string `json:"expected_volume"`
	OwnerName      string `json:"owner_name"`
}

func (a *App) SaveKyc(ctx context.Context, m *domain.Merchant, in KycInput) error {
	if m.Status == domain.MerchantActive || m.Status == domain.MerchantSuspended {
		return domain.Forbidden("kyc_locked", "A verificação já está concluída. Para alterar, contacte o suporte: +258 84 825 5237.")
	}
	if in.BusinessType != "individual" && in.BusinessType != "company" {
		return domain.Invalid("parameter_invalid", "business_type", "Escolha o tipo de negócio.")
	}
	nuit := domain.Digits(in.NUIT)
	if len(nuit) != 9 || len(strings.Join(strings.Fields(in.NUIT), "")) != 9 {
		return domain.Invalid("nuit_invalid", "nuit", "O NUIT tem 9 dígitos.")
	}
	m.BusinessType, m.NUIT = in.BusinessType, nuit
	m.LegalName = domain.FirstNonEmpty(strings.TrimSpace(in.LegalName), m.LegalName, m.Name)
	m.City = domain.FirstNonEmpty(strings.TrimSpace(in.City), m.City)
	m.ExpectedVolume = domain.FirstNonEmpty(in.ExpectedVolume, m.ExpectedVolume)
	m.OwnerName = domain.FirstNonEmpty(strings.TrimSpace(in.OwnerName), m.OwnerName)
	return a.Merchants.Save(ctx, m)
}

var DocKinds = map[string]bool{"id_front": true, "id_back": true, "registration": true, "nib_proof": true}

var docTypes = map[string]string{".pdf": "application/pdf", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png"}

func sniff(ct string, head []byte) bool {
	switch ct {
	case "application/pdf":
		return strings.HasPrefix(string(head), "%PDF")
	case "image/jpeg":
		return len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF
	case "image/png":
		return strings.HasPrefix(string(head), "\x89PNG")
	}
	return false
}

type Upload struct {
	Kind     string
	FileName string
	Size     int64
	Head     []byte
	Body     io.Reader
}

func (a *App) UploadDocument(ctx context.Context, m *domain.Merchant, up Upload) (*domain.Document, error) {
	if !DocKinds[up.Kind] {
		return nil, domain.Invalid("parameter_invalid", "kind", "Tipo de documento inválido.")
	}
	if up.Size > 8<<20 {
		return nil, domain.Invalid("parameter_invalid", "file", "O ficheiro tem de ter menos de 8 MB.")
	}
	ext := strings.ToLower(filepath.Ext(up.FileName))
	ct, ok := docTypes[ext]
	if !ok {
		return nil, domain.Invalid("parameter_invalid", "file", "Use PDF, JPG ou PNG.")
	}
	if !sniff(ct, up.Head) {
		return nil, domain.Invalid("parameter_invalid", "file", "O conteúdo do ficheiro não corresponde à extensão.")
	}
	id := domain.NewID("doc")
	path, err := a.Files.Save(ctx, m.ID, id+ext, up.Body)
	if err != nil {
		return nil, err
	}
	old, _ := a.Documents.RemoveKind(ctx, m.ID, up.Kind)
	for _, o := range old {
		if o.StoredPath != "" {
			_ = a.Files.Remove(o.StoredPath)
		}
	}
	d := &domain.Document{ID: id, MerchantID: m.ID, Kind: up.Kind, FileName: domain.Trim(filepath.Base(up.FileName), 200), StoredPath: path, Size: up.Size, ContentType: ct, CreatedAt: a.Now()}
	if err := a.Documents.Create(ctx, d); err != nil {
		return nil, err
	}
	if up.Kind == "nib_proof" {
		_ = a.Banks.SetDocument(ctx, m.ID, id)
	}
	return d, nil
}

func (a *App) SubmitKyc(ctx context.Context, m *domain.Merchant, actor Actor) error {
	if m.NUIT == "" || m.BusinessType == "" {
		return domain.Invalid("kyc_incomplete", "nuit", "Preencha o tipo de negócio e o NUIT.")
	}
	switch m.Status {
	case domain.MerchantLead, domain.MerchantInfoRequested, domain.MerchantRejected:
		now := a.Now()
		m.Status, m.KycSubmittedAt = domain.MerchantInReview, &now
		if err := a.Merchants.Save(ctx, m); err != nil {
			return err
		}
		a.Record(ctx, m.ID, actor, "kyc.submitted", m.ID, "")
		a.mailKycReceived(ctx, m)
	}
	return nil
}

func (a *App) DocumentsOf(ctx context.Context, merchantID string) []domain.Document {
	d, _ := a.Documents.ByMerchant(ctx, merchantID)
	return d
}

func (a *App) Document(ctx context.Context, id string) (*domain.Document, error) {
	d, err := a.Documents.ByID(ctx, id)
	return d, notFound(err, "O documento")
}

var Banks = []string{"BCI", "Millennium bim", "Standard Bank", "Absa", "Moza Banco", "FNB", "Access Bank", "Nedbank", "Ecobank", "Letshego", "Banco Único"}

func (a *App) BankAccount(ctx context.Context, merchantID string) *domain.BankAccount {
	b, err := a.Banks.ByMerchant(ctx, merchantID)
	if err != nil {
		return nil
	}
	return b
}

type BankInput struct {
	BankName string `json:"bank_name"`
	Holder   string `json:"holder"`
	NIB      string `json:"nib"`
}

func (a *App) SaveBankAccount(ctx context.Context, m *domain.Merchant, in BankInput, actor Actor) (*domain.BankAccount, error) {
	nib := strings.Join(strings.Fields(in.NIB), "")
	if !domain.ValidNIB(nib) {
		return nil, domain.Invalid("nib_invalid", "nib", "O NIB tem 21 dígitos.")
	}
	if !domain.Contains(Banks, in.BankName) {
		return nil, domain.Invalid("bank_invalid", "bank_name", "Escolha um banco da lista.")
	}
	if strings.TrimSpace(in.Holder) == "" {
		return nil, domain.Invalid("parameter_missing", "holder", "Indique o titular da conta.")
	}
	b := a.BankAccount(ctx, m.ID)
	replaced := b != nil && b.NIB != nib
	changed := b == nil || b.NIB != nib
	if b == nil {
		b = &domain.BankAccount{ID: domain.NewID("ba"), MerchantID: m.ID}
		for _, d := range a.DocumentsOf(ctx, m.ID) {
			if d.Kind == "nib_proof" {
				id := d.ID
				b.DocumentID = &id
			}
		}
	}
	b.BankName, b.Holder, b.NIB, b.UpdatedAt = in.BankName, strings.TrimSpace(in.Holder), nib, a.Now()
	if changed {
		b.Verified = m.Status == domain.MerchantActive
	}
	if err := a.Banks.Save(ctx, b); err != nil {
		return nil, err
	}
	if changed {
		a.Record(ctx, m.ID, actor, "bank_account.changed", b.ID, b.BankName+" •••• "+nib[16:])
	}
	if replaced {
		a.mailBankChanged(ctx, m, b)
	}
	return b, nil
}

func (a *App) Decide(ctx context.Context, merchantID, decision, note, actor string) (*domain.Merchant, error) {
	m, err := a.Merchants.ByID(ctx, merchantID)
	if err != nil {
		return nil, notFound(err, "O comerciante")
	}
	now := a.Now()
	before := m.Status
	switch decision {
	case "approve":
		m.Status, m.ApprovedAt = domain.MerchantActive, &now
		_ = a.Banks.MarkVerified(ctx, m.ID)
	case "reject":
		m.Status = domain.MerchantRejected
	case "info":
		m.Status = domain.MerchantInfoRequested
	case "suspend":
		m.Status = domain.MerchantSuspended
	case "reactivate":
		m.Status = domain.MerchantInReview
		if m.ApprovedAt != nil {
			m.Status = domain.MerchantActive
		}
	default:
		return nil, domain.Invalid("decision_invalid", "decision", "Decisão desconhecida.")
	}
	m.ReviewNote, m.ReviewedBy = note, actor
	if err := a.Merchants.Save(ctx, m); err != nil {
		return nil, err
	}
	for _, live := range []bool{false, true} {
		_ = a.Tx.Do(ctx, func(ctx context.Context) error {
			return a.emit(ctx, m.ID, live, "account.updated", m.ID, domain.AccountResource(m), "")
		})
	}
	if m.Status != before {
		a.mailDecision(ctx, m, decision, note)
	}
	return m, nil
}

func (a *App) SetMerchantFee(ctx context.Context, merchantID string, bps *int, actor Actor) error {
	m, err := a.Merchants.ByID(ctx, merchantID)
	if err != nil {
		return notFound(err, "O comerciante")
	}
	if bps != nil && (*bps < 0 || *bps > 2000) {
		return domain.Invalid("fee_invalid", "fee_bps", "A taxa tem de estar entre 0% e 20%.")
	}
	old := domain.FormatBps(a.FeeBpsFor(ctx, m))
	if err := a.Merchants.SetFee(ctx, merchantID, bps); err != nil {
		return err
	}
	m.FeeBps = bps
	a.Record(ctx, m.ID, actor, "fee.changed", m.ID, old+" → "+domain.FormatBps(a.FeeBpsFor(ctx, m)))
	return nil
}
