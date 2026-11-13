package pdf

import (
	"fmt"

	"riha/internal/core/domain"
)

var agreementStatus = map[string]string{
	domain.AgreementProposed: "Por aceitar", domain.AgreementAccepted: "Aceite", domain.AgreementSuperseded: "Substituído por um acordo mais recente",
	domain.AgreementDeclined: "Cancelado", domain.AgreementExpired: "Terminado",
}

func (r Renderer) CreditAgreement(x domain.AgreementDoc) ([]byte, error) {
	l, g := x.Loan, x.Agreement
	d := newDoc("Acordo de pagamento " + l.Reference)
	d.header(fmt.Sprintf("Acordo de pagamento · v%d", g.Version), l.Reference, x.Generated)
	f := d.f
	d.section("Partes")
	d.row("Instituição", x.Institution, false)
	d.row("Cliente", l.Name, false)
	d.row("Número", "+258 "+domain.FormatPhone(l.Phone)+" · "+domain.MethodNames[l.Wallet], false)
	if x.IDNumber != "" {
		d.row("BI", x.IDNumber, true)
	}
	d.section("Crédito")
	if x.Product != "" {
		d.row("Produto", x.Product, false)
	}
	d.row("Montante", domain.Format(l.Amount)+" MT", false)
	d.row("Taxa", domain.RateLabel(l.RateBP, l.Freq)+", sobre o montante emprestado", false)
	if g.MoraWaived > 0 {
		d.row("Mora perdoada", domain.Format(g.MoraWaived)+" MT", false)
	}
	d.amount("Total a pagar", g.Total)
	d.section("Prestações")
	f.SetFont("Helvetica", "", 10)
	for i, dt := range g.Dates {
		f.SetTextColor(94, 104, 98)
		f.CellFormat(12, 6.5, fmt.Sprint(i+1), "", 0, "L", false, 0, "")
		f.SetTextColor(11, 18, 14)
		f.CellFormat(100, 6.5, d.tr(domain.LongDate(dt)), "", 0, "L", false, 0, "")
		f.CellFormat(0, 6.5, d.tr(domain.Format(g.Amounts[i])+" MT"), "", 1, "R", false, 0, "")
	}
	d.section("Termos aceites")
	f.SetFont("Helvetica", "", 10)
	f.SetTextColor(11, 18, 14)
	f.MultiCell(0, 5.5, d.tr(g.Text), "", "L", false)
	d.section("Aceitação")
	d.row("Estado", agreementStatus[g.Status], false)
	d.row("Enviado", stamp(g.SentAt), false)
	if g.AcceptedAt != nil {
		d.row("Aceite", stamp(*g.AcceptedAt)+" com código SMS enviado para +258 "+domain.FormatPhone(g.Phone), false)
	}
	f.Ln(4)
	f.SetFont("Helvetica", "", 8.5)
	f.SetTextColor(94, 104, 98)
	f.MultiCell(0, 4.5, d.tr("A RIHA não empresta. O crédito, a taxa e a decisão são da instituição. A RIHA envia o pedido, o acordo e o link de cada prestação, e guarda cada versão do acordo com a data, a hora e o número que a aceitou."), "", "L", false)
	return d.bytes()
}
