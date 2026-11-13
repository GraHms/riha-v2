package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type CreditRequestInput struct {
	ProductID string            `json:"product_id"`
	Amount    int64             `json:"amount"`
	Term      int               `json:"term"`
	Day       int               `json:"day"`
	Name      string            `json:"name"`
	Phone     string            `json:"phone"`
	IDNumber  string            `json:"id_number"`
	NUIT      string            `json:"nuit"`
	Address   string            `json:"address"`
	Activity  string            `json:"activity"`
	Purpose   string            `json:"purpose"`
	Income    int64             `json:"income"`
	Extra     map[string]string `json:"extra"`
	IP        string            `json:"-"`
}

var standardFields = map[string]bool{domain.FieldID: true, domain.FieldActivity: true, domain.FieldIncome: true, domain.FieldPurpose: true}

func clean(s string, n int) string { return domain.Trim(strings.Join(strings.Fields(s), " "), n) }

// StartCreditRequest takes a customer's request from the institution's link and
// sends a code to confirm the number. The institution sees it once confirmed.
func (a *App) StartCreditRequest(ctx context.Context, slug string, in CreditRequestInput) (*domain.CreditRequest, string, error) {
	m, products, live, err := a.PublicCreditProducts(ctx, slug)
	if err != nil {
		return nil, "", err
	}
	var p *domain.CreditProduct
	for i := range products {
		if products[i].ID == in.ProductID {
			p = &products[i]
		}
	}
	if p == nil {
		return nil, "", domain.Invalid("product_invalid", "product_id", "Escolha um dos produtos desta página.")
	}
	day := in.Day
	if !p.ClientPicksDay() {
		day = domain.DefaultDay(a.Now())
	}
	if e := p.CheckTerms(in.Amount, in.Term, day); e != nil {
		return nil, "", e
	}
	r := &domain.CreditRequest{ID: domain.NewID("creq"), MerchantID: m.ID, Livemode: live, ProductID: p.ID, Token: domain.Token("mc_", 32),
		Name: clean(in.Name, 120), Phone: domain.NormalizeMZPhone(in.Phone), Amount: in.Amount, Term: in.Term, Day: day,
		Status: domain.RequestVerifying, Source: "link", IP: domain.Trim(in.IP, 64), Extra: map[string]string{}}
	r.Wallet = domain.WalletFor(r.Phone)
	switch {
	case len([]rune(r.Name)) < 3 || !strings.Contains(r.Name, " "):
		return nil, "", domain.Invalid("parameter_invalid", "name", "Escreva o nome completo.")
	case !domain.ValidMZMobile(r.Phone):
		return nil, "", domain.Invalid("parameter_invalid", "phone", "Use um número de telemóvel moçambicano, de 9 algarismos.")
	}
	if p.Asks(domain.FieldID) {
		r.IDNumber = strings.ToUpper(strings.ReplaceAll(clean(in.IDNumber, 30), " ", ""))
		if len(r.IDNumber) < 6 || len(r.IDNumber) > 20 || strings.Trim(r.IDNumber, "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
			return nil, "", domain.Invalid("parameter_invalid", "id_number", "Escreva o número do BI.")
		}
	}
	if p.Asks(domain.FieldActivity) {
		if r.Activity = clean(in.Activity, 200); r.Activity == "" {
			return nil, "", domain.Invalid("parameter_invalid", "activity", "Diga o que faz.")
		}
	}
	if p.Asks(domain.FieldPurpose) {
		if r.Purpose = clean(in.Purpose, 300); r.Purpose == "" {
			return nil, "", domain.Invalid("parameter_invalid", "purpose", "Diga para que é o dinheiro.")
		}
	}
	r.NUIT, r.Address = domain.Digits(domain.Trim(in.NUIT, 20)), clean(in.Address, 200)
	for _, f := range p.Fields {
		if standardFields[f] {
			continue
		}
		v := clean(in.Extra[f], 200)
		if v == "" {
			return nil, "", domain.Invalid("parameter_invalid", "extra."+f, "Preencha "+strings.ToLower(f)+".")
		}
		r.Extra[f] = v
	}
	q := p.Quote(r.Amount, r.Term, r.Day, in.Income, a.Now())
	r.Instalment = q.Instalment
	if p.Asks(domain.FieldIncome) {
		if in.Income <= 0 || in.Income > 100000000 {
			return nil, "", domain.Invalid("parameter_invalid", "income", "Diga quanto ganha por mês.")
		}
		r.Income, r.EffortPct = in.Income, q.EffortPct
		if q.EffortPct > p.EffortPct {
			return nil, "", domain.Invalid("effort_too_high", "income", fmt.Sprintf("A prestação é %d%% do que ganha por mês. O limite é %d%%. Peça menos dinheiro ou escolha mais tempo.", q.EffortPct, p.EffortPct))
		}
	}
	if n, _ := a.Credit.RecentRequests(ctx, r.Phone, a.Now().Add(-24*time.Hour)); n >= 3 {
		return nil, "", domain.Conflict("too_many_requests", "Este número já fez vários pedidos hoje. Tente de novo amanhã.")
	}
	open, _ := a.Credit.Requests(ctx, ports.CreditRequestQuery{MerchantID: m.ID, Live: live, Statuses: []string{domain.RequestNew, domain.RequestReview}, Search: r.Phone, Limit: 5})
	for _, o := range open {
		if o.Phone == r.Phone {
			return nil, "", domain.Conflict("request_open", "Já tem um pedido em análise na "+m.Name+". Espere pela resposta por SMS.")
		}
	}
	now := a.Now()
	r.CreatedAt, r.UpdatedAt = now, now
	if err := a.Credit.CreateRequest(ctx, r); err != nil {
		return nil, "", err
	}
	code, err := a.sendCreditCode(ctx, m, live, r.ID, domain.CodeVerifyPhone, r.Phone, "confirmar o seu pedido de crédito")
	return r, code, err
}

func (a *App) creditRequestByToken(ctx context.Context, token string) (*domain.CreditRequest, *domain.Merchant, error) {
	r, err := a.Credit.RequestByToken(ctx, token)
	if err != nil {
		return nil, nil, domain.NotFound("Este pedido")
	}
	m, err := a.Merchants.ByID(ctx, r.MerchantID)
	if err != nil || !m.HasMicrocredit() {
		return nil, nil, domain.NotFound("Este pedido")
	}
	return r, m, nil
}

func (a *App) ResendRequestCode(ctx context.Context, token string) (string, error) {
	r, m, err := a.creditRequestByToken(ctx, token)
	if err != nil {
		return "", err
	}
	if r.Status != domain.RequestVerifying {
		return "", domain.Invalid("already_confirmed", "code", "O número já está confirmado.")
	}
	return a.sendCreditCode(ctx, m, r.Livemode, r.ID, domain.CodeVerifyPhone, r.Phone, "confirmar o seu pedido de crédito")
}

func (a *App) ConfirmCreditRequest(ctx context.Context, token, code string) (*domain.CreditRequest, error) {
	r, m, err := a.creditRequestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if r.Status != domain.RequestVerifying {
		return r, nil
	}
	if err := a.checkCreditCode(ctx, r.ID, domain.CodeVerifyPhone, code); err != nil {
		return nil, err
	}
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		l, err := a.Credit.LockRequest(ctx, r.ID)
		if err != nil {
			return err
		}
		if l.Status != domain.RequestVerifying {
			r = l
			return nil
		}
		now := a.Now()
		l.Status, l.PhoneOKAt, l.UpdatedAt = domain.RequestNew, &now, now
		r = l
		return a.Credit.SaveRequest(ctx, l)
	})
	if err != nil {
		return nil, err
	}
	if r.PhoneOKAt != nil && a.Now().Sub(*r.PhoneOKAt) < time.Minute {
		_ = a.creditText(ctx, m, r.Livemode, r.ID, "", r.Phone, "credit_received",
			fmt.Sprintf("%s recebeu o seu pedido de %s MT. Avisamos por SMS quando houver resposta. Acompanhe em %s", m.Name, domain.FormatShort(r.Amount), a.creditURL(r.Token)), true)
		a.SendSoon(ctx)
	}
	return r, nil
}

var creditDocKinds = map[string]string{"id_front": "BI · frente", "id_back": "BI · verso", "other": "Outro documento"}

func CreditDocLabel(kind string) string { return creditDocKinds[kind] }

func (a *App) AddCreditDocument(ctx context.Context, token string, up Upload) (*domain.CreditDocument, error) {
	r, _, err := a.creditRequestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if r.Status != domain.RequestVerifying && r.Status != domain.RequestNew {
		return nil, domain.Invalid("request_closed", "file", "Já não é possível juntar documentos a este pedido.")
	}
	if _, ok := creditDocKinds[up.Kind]; !ok {
		return nil, domain.Invalid("parameter_invalid", "kind", "Tipo de documento inválido.")
	}
	if docs, _ := a.Credit.Documents(ctx, r.ID); len(docs) >= 6 {
		return nil, domain.Invalid("too_many_documents", "file", "Já juntou documentos suficientes.")
	}
	if up.Size <= 0 || up.Size > 8<<20 {
		return nil, domain.Invalid("parameter_invalid", "file", "O ficheiro tem de ter menos de 8 MB.")
	}
	ext := strings.ToLower(filepath.Ext(up.FileName))
	ct, ok := docTypes[ext]
	if !ok {
		return nil, domain.Invalid("parameter_invalid", "file", "Use uma foto JPG ou PNG, ou um PDF.")
	}
	if !sniff(ct, up.Head) {
		return nil, domain.Invalid("parameter_invalid", "file", "O conteúdo do ficheiro não corresponde à extensão.")
	}
	id := domain.NewID("cdoc")
	path, err := a.Files.Save(ctx, r.MerchantID, "credit/"+id+ext, ct, up.Body)
	if err != nil {
		return nil, err
	}
	d := &domain.CreditDocument{ID: id, RequestID: r.ID, MerchantID: r.MerchantID, Kind: up.Kind, FileName: domain.Trim(filepath.Base(up.FileName), 200),
		StoredPath: path, ContentType: ct, Size: up.Size, CreatedAt: a.Now()}
	return d, a.Credit.AddDocument(ctx, d)
}

func (a *App) CreditDocumentContent(ctx context.Context, m *domain.Merchant, id string) (*domain.CreditDocument, []byte, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, nil, e
	}
	d, err := a.Credit.Document(ctx, m.ID, id)
	if err != nil {
		return nil, nil, domain.NotFound("O documento")
	}
	b, err := a.Files.Read(ctx, d.StoredPath)
	if err != nil {
		return nil, nil, domain.NotFound("O documento")
	}
	return d, b, nil
}

var requestFilters = map[string][]string{
	"decidir": {domain.RequestNew, domain.RequestReview},
	"enviado": {domain.RequestOffered},
	"todos":   nil,
}

func (a *App) CreditRequests(ctx context.Context, m *domain.Merchant, live bool, filter, productID, search string) ([]domain.CreditRequest, map[string]int64, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, nil, e
	}
	st, ok := requestFilters[filter]
	if !ok {
		st = requestFilters["decidir"]
	}
	rs, err := a.Credit.Requests(ctx, ports.CreditRequestQuery{MerchantID: m.ID, Live: live, Statuses: st, ProductID: productID, Search: search, Limit: 500})
	if err != nil {
		return nil, nil, err
	}
	counts, _ := a.Credit.CountRequests(ctx, m.ID, live)
	return rs, counts, nil
}

type CreditCheck struct {
	OK    bool   `json:"ok"`
	Title string `json:"title"`
	Note  string `json:"note"`
}

type CreditRequestDetail struct {
	Request   *domain.CreditRequest
	Product   *domain.CreditProduct
	Documents []domain.CreditDocument
	History   []domain.Loan
	Checks    []CreditCheck
	Quote     domain.Quote
	Loan      *domain.Loan
}

func (a *App) CreditRequestDetail(ctx context.Context, m *domain.Merchant, live bool, id string) (*CreditRequestDetail, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, e
	}
	r, err := a.Credit.Request(ctx, m.ID, id)
	if err != nil || r.Livemode != live || r.Status == domain.RequestVerifying {
		return nil, domain.NotFound("O pedido")
	}
	p, err := a.Credit.Product(ctx, m.ID, r.ProductID)
	if err != nil {
		return nil, domain.NotFound("O produto")
	}
	d := &CreditRequestDetail{Request: r, Product: p}
	d.Documents, _ = a.Credit.Documents(ctx, r.ID)
	hist, _ := a.Credit.LoansByPhone(ctx, m.ID, live, r.Phone)
	for _, l := range hist {
		if l.RequestID == r.ID {
			lc := l
			d.Loan = &lc
			continue
		}
		if l.Status != domain.LoanCanceled && l.Status != domain.LoanOffered {
			d.History = append(d.History, l)
		}
	}
	d.Quote = p.Quote(r.Amount, r.Term, r.Day, r.Income, a.Now())
	d.Checks = append(d.Checks, CreditCheck{r.PhoneOKAt != nil, "Número confirmado por código SMS", domain.FormatPhone(r.Phone) + " · " + domain.MethodNames[r.Wallet]})
	if p.Asks(domain.FieldID) {
		front, back := false, false
		for _, doc := range d.Documents {
			front = front || doc.Kind == "id_front"
			back = back || doc.Kind == "id_back"
		}
		note := "Falta " + map[bool]string{true: "o verso", false: "a frente"}[front]
		if front && back {
			note = "BI " + r.IDNumber
		}
		d.Checks = append(d.Checks, CreditCheck{front && back, "BI enviado, frente e verso", note})
	}
	if p.Asks(domain.FieldIncome) && r.Income > 0 {
		d.Checks = append(d.Checks, CreditCheck{r.EffortPct <= p.EffortPct, fmt.Sprintf("Esforço de %d%%", r.EffortPct),
			fmt.Sprintf("Prestação de %s MT sobre %s MT. Limite: %d%%.", domain.FormatShort(r.Instalment), domain.FormatShort(r.Income), p.EffortPct)})
	}
	active := 0
	var last *domain.Loan
	for i, l := range d.History {
		if l.Running() {
			active++
		}
		if l.Status == domain.LoanSettled && (last == nil || l.ClosedAt != nil && last.ClosedAt != nil && l.ClosedAt.After(*last.ClosedAt)) {
			last = &d.History[i]
		}
	}
	switch {
	case active > 0:
		d.Checks = append(d.Checks, CreditCheck{false, fmt.Sprintf("%d %s na %s", active, map[bool]string{true: "empréstimo ativo", false: "empréstimos ativos"}[active == 1], m.Name), "Veja o histórico abaixo"})
	case last != nil && last.ClosedAt != nil:
		d.Checks = append(d.Checks, CreditCheck{true, "Sem empréstimo ativo na " + m.Name, "Último liquidado em " + strings.ToLower(domain.MonthName(last.ClosedAt.In(domain.Maputo).Month()))})
	default:
		d.Checks = append(d.Checks, CreditCheck{true, "Sem empréstimo ativo na " + m.Name, "Primeiro pedido"})
	}
	return d, nil
}

// ReviewCreditRequest marks a new request as being looked at.
func (a *App) ReviewCreditRequest(ctx context.Context, m *domain.Merchant, live bool, id string, actor Actor) error {
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		r, err := a.lockRequest(ctx, m, live, id)
		if err != nil {
			return err
		}
		if r.Status != domain.RequestNew {
			return nil
		}
		r.Status, r.UpdatedAt = domain.RequestReview, a.Now()
		return a.Credit.SaveRequest(ctx, r)
	})
}

func (a *App) lockRequest(ctx context.Context, m *domain.Merchant, live bool, id string) (*domain.CreditRequest, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, e
	}
	r, err := a.Credit.LockRequest(ctx, id)
	if err != nil || r.MerchantID != m.ID || r.Livemode != live || r.Status == domain.RequestVerifying {
		return nil, domain.NotFound("O pedido")
	}
	return r, nil
}

func (a *App) DeclineCreditRequest(ctx context.Context, m *domain.Merchant, live bool, id, note string, actor Actor) (*domain.CreditRequest, error) {
	var out *domain.CreditRequest
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		r, err := a.lockRequest(ctx, m, live, id)
		if err != nil {
			return err
		}
		if !r.Open() {
			return domain.Invalid("request_decided", "id", "Este pedido já foi decidido.")
		}
		now := a.Now()
		r.Status, r.DeclineNote, r.DecidedBy, r.DecidedAt, r.UpdatedAt = domain.RequestDeclined, clean(note, 300), actor.ID, &now, now
		if err := a.Credit.SaveRequest(ctx, r); err != nil {
			return err
		}
		out = r
		return a.creditText(ctx, m, live, r.ID, "", r.Phone, "credit_declined",
			fmt.Sprintf("%s analisou o seu pedido de %s MT e não o pode aprovar agora. Para saber mais, fale com a %s.", m.Name, domain.FormatShort(r.Amount), m.Name), true)
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.request_declined", id, out.DeclineNote)
	a.SendSoon(ctx)
	return out, nil
}
