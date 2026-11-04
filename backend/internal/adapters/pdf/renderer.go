package pdf

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"riha/internal/core/domain"
)

//go:embed logo.png
var logo []byte

type Renderer struct{ Now func() time.Time }

func New() Renderer { return Renderer{Now: func() time.Time { return time.Now() }} }

var maputo = time.FixedZone("CAT", 2*3600)

func stamp(t time.Time) string { return t.In(maputo).Format("02/01/2006 15:04") }

func stampPtr(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return stamp(*t)
}

func groupNIB(n string) string {
	var b strings.Builder
	for i, r := range n {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

var statusPT = map[string]string{
	domain.PayoutNew: "Novo", domain.PayoutReview: "Em análise", domain.PayoutSent: "Enviado ao banco",
	domain.PayoutPaid: "Pago", domain.PayoutRejected: "Rejeitado", domain.PayoutFailed: "Falhado",
}

type doc struct {
	f  *fpdf.Fpdf
	tr func(string) string
}

func newDoc(title string) *doc {
	f := fpdf.New("P", "mm", "A4", "")
	f.SetMargins(18, 18, 18)
	f.SetAutoPageBreak(true, 20)
	f.SetTitle(title, true)
	f.SetAuthor("RIHA", true)
	f.SetCreator("RIHA", true)
	f.RegisterImageOptionsReader("logo", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(logo))
	d := &doc{f: f, tr: f.UnicodeTranslatorFromDescriptor("")}
	f.SetFooterFunc(func() {
		f.SetY(-14)
		f.SetFont("Helvetica", "", 8)
		f.SetTextColor(138, 147, 142)
		f.CellFormat(0, 5, d.tr(fmt.Sprintf("RIHA · Maputo, Moçambique · Documento gerado automaticamente · Página %d", f.PageNo())), "", 0, "C", false, 0, "")
	})
	return d
}

func (d *doc) header(title, ref string, generated time.Time) {
	f := d.f
	f.AddPage()
	f.ImageOptions("logo", 18, 16, 10, 10, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
	f.SetXY(30, 17)
	f.SetFont("Helvetica", "B", 13)
	f.SetTextColor(11, 18, 14)
	f.CellFormat(40, 8, "RIHA", "", 0, "L", false, 0, "")
	f.SetFont("Helvetica", "", 9)
	f.SetTextColor(94, 104, 98)
	f.SetXY(110, 17)
	f.CellFormat(82, 4, d.tr("Gerado em "+stamp(generated)), "", 2, "R", false, 0, "")
	f.CellFormat(82, 4, d.tr(ref), "", 0, "R", false, 0, "")
	f.SetFillColor(10, 130, 5)
	f.Rect(18, 30, 87, 1.2, "F")
	f.SetFillColor(255, 15, 21)
	f.Rect(105, 30, 87, 1.2, "F")
	f.SetXY(18, 38)
	f.SetFont("Helvetica", "B", 18)
	f.SetTextColor(11, 18, 14)
	f.CellFormat(0, 9, d.tr(title), "", 1, "L", false, 0, "")
	f.Ln(2)
}

func (d *doc) section(title string) {
	f := d.f
	f.Ln(3)
	f.SetFont("Helvetica", "B", 8.5)
	f.SetTextColor(94, 104, 98)
	f.CellFormat(0, 6, d.tr(strings.ToUpper(title)), "", 1, "L", false, 0, "")
	f.SetDrawColor(232, 230, 223)
	f.Line(18, f.GetY(), 192, f.GetY())
	f.Ln(1.5)
}

func (d *doc) row(k, v string, mono bool) {
	f := d.f
	f.SetFont("Helvetica", "", 10)
	f.SetTextColor(94, 104, 98)
	f.CellFormat(55, 7, d.tr(k), "", 0, "L", false, 0, "")
	if mono {
		f.SetFont("Courier", "B", 10.5)
	} else {
		f.SetFont("Helvetica", "B", 10)
	}
	f.SetTextColor(11, 18, 14)
	f.MultiCell(0, 7, d.tr(v), "", "L", false)
}

func (d *doc) amount(label string, amount int64) {
	f := d.f
	f.SetFillColor(11, 18, 14)
	y := f.GetY() + 2
	f.Rect(18, y, 174, 22, "F")
	f.SetXY(24, y+3)
	f.SetFont("Helvetica", "", 9)
	f.SetTextColor(169, 181, 174)
	f.CellFormat(0, 5, d.tr(label), "", 2, "L", false, 0, "")
	f.SetFont("Helvetica", "B", 20)
	f.SetTextColor(255, 255, 255)
	f.CellFormat(0, 10, d.tr(domain.Format(amount)+" MT"), "", 0, "L", false, 0, "")
	f.SetY(y + 26)
}

func (d *doc) bank(s *domain.PayoutSheet) {
	p, m := s.Payout, s.Merchant
	d.section("Beneficiário")
	d.row("Comerciante", m.Name, false)
	if m.LegalName != "" && m.LegalName != m.Name {
		d.row("Razão social", m.LegalName, false)
	}
	if m.NUIT != "" {
		d.row("NUIT", m.NUIT, true)
	}
	if m.Email != "" {
		d.row("Email", m.Email, false)
	}
	d.section("Dados bancários")
	d.row("Banco", p.BankName, false)
	d.row("Titular", p.Holder, false)
	d.row("NIB", groupNIB(p.NIB), true)
	d.row("Descritivo", "RIHA "+p.Ref(), true)
}

func (d *doc) bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := d.f.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r Renderer) PayoutInstructions(sheets []domain.PayoutSheet) ([]byte, error) {
	now := r.Now()
	d := newDoc("Ordens de transferência RIHA")
	if len(sheets) > 1 {
		d.header("Ordens de transferência", fmt.Sprintf("%d levantamentos", len(sheets)), now)
		f := d.f
		f.SetFont("Helvetica", "B", 9)
		f.SetFillColor(250, 250, 247)
		f.SetTextColor(94, 104, 98)
		for _, h := range []struct {
			t string
			w float64
		}{{"Ref.", 22}, {"Comerciante", 52}, {"Banco", 30}, {"NIB", 46}, {"Valor (MT)", 24}} {
			f.CellFormat(h.w, 8, d.tr(h.t), "B", 0, "L", true, 0, "")
		}
		f.Ln(-1)
		var total int64
		for _, s := range sheets {
			p := s.Payout
			total += p.Amount
			f.SetTextColor(11, 18, 14)
			f.SetFont("Courier", "", 8.5)
			f.CellFormat(22, 7, p.Ref(), "B", 0, "L", false, 0, "")
			f.SetFont("Helvetica", "", 8.5)
			f.CellFormat(52, 7, d.tr(domain.Trim(s.Merchant.Name, 32)), "B", 0, "L", false, 0, "")
			f.CellFormat(30, 7, d.tr(p.BankName), "B", 0, "L", false, 0, "")
			f.SetFont("Courier", "", 8)
			f.CellFormat(46, 7, p.NIB, "B", 0, "L", false, 0, "")
			f.SetFont("Helvetica", "B", 8.5)
			f.CellFormat(24, 7, d.tr(domain.Format(p.Amount)), "B", 1, "R", false, 0, "")
		}
		f.SetFont("Helvetica", "B", 10)
		f.CellFormat(150, 9, "Total", "", 0, "R", false, 0, "")
		f.CellFormat(24, 9, d.tr(domain.Format(total)), "", 1, "R", false, 0, "")
	}
	for i := range sheets {
		s := &sheets[i]
		p := s.Payout
		d.header("Ordem de transferência", p.Ref(), now)
		d.amount("Valor a transferir", p.Amount)
		d.bank(s)
		d.section("Prazos")
		d.row("Pedido em", stamp(p.RequestedAt), false)
		d.row("Transferir até", stamp(p.DueAt)+" (72 h)", false)
		d.row("Estado", statusPT[p.Status], false)
		d.section("Assinaturas")
		d.f.Ln(14)
		d.f.SetDrawColor(180, 180, 180)
		d.f.Line(18, d.f.GetY(), 95, d.f.GetY())
		d.f.Line(115, d.f.GetY(), 192, d.f.GetY())
		d.f.SetFont("Helvetica", "", 8.5)
		d.f.SetTextColor(94, 104, 98)
		d.f.CellFormat(97, 5, d.tr("Preparado por (Operações)"), "", 0, "L", false, 0, "")
		d.f.CellFormat(77, 5, d.tr("Aprovado por"), "", 1, "L", false, 0, "")
	}
	return d.bytes()
}

func (r Renderer) PayoutStatement(s domain.PayoutSheet) ([]byte, error) {
	p := s.Payout
	d := newDoc("Comprovativo de levantamento " + p.Ref())
	d.header("Comprovativo de levantamento", p.Ref(), r.Now())
	d.amount("Valor transferido", p.Amount)
	d.bank(&s)
	d.section("Cronologia")
	d.row("Pedido", stamp(p.RequestedAt), false)
	d.row("Aprovado e enviado ao banco", stampPtr(p.SentAt), false)
	d.row("Transferência confirmada", stampPtr(p.PaidAt), false)
	d.row("Estado", statusPT[p.Status], false)
	d.section("Comprovativo bancário")
	if p.HasReceipt() {
		d.row("Ficheiro", p.ReceiptName, false)
		d.row("SHA-256", p.ReceiptHash, true)
	} else {
		d.row("Ficheiro", "Não anexado", false)
	}
	if s.Receipt != nil && (s.Receipt.ContentType == "image/jpeg" || s.Receipt.ContentType == "image/png") {
		typ := "JPG"
		if s.Receipt.ContentType == "image/png" {
			typ = "PNG"
		}
		d.f.AddPage()
		d.section("Anexo · comprovativo bancário")
		opt := fpdf.ImageOptions{ImageType: typ, ReadDpi: true}
		d.f.RegisterImageOptionsReader("receipt", opt, bytes.NewReader(s.Receipt.Data))
		if d.f.Ok() {
			d.f.ImageOptions("receipt", 18, d.f.GetY()+2, 174, 0, false, opt, 0, "")
		} else {
			d.f.ClearError()
			d.row("Imagem", "Não foi possível incorporar a imagem; o ficheiro original segue em anexo.", false)
		}
	} else if s.Receipt != nil {
		d.row("Anexo", "O comprovativo em PDF é enviado como ficheiro separado.", false)
	}
	return d.bytes()
}
