package app

import (
	"context"
	"log"
	"net/mail"
	"strings"
	"time"

	"riha/internal/core/domain"
)

// queue stores an email for the worker. Inside a transaction it commits or rolls
// back with the change it reports, and it never waits on the mail server.
func (a *App) queue(ctx context.Context, kind string, n domain.Notice) error {
	addr, err := mail.ParseAddress(n.To)
	if err != nil {
		return nil
	}
	n.To = addr.Address
	e := n.Email()
	now := a.Now()
	return a.Emails.Create(ctx, &domain.QueuedEmail{ID: domain.NewID("eml"), Kind: kind, To: e.To, Subject: e.Subject, Text: e.Text, HTML: e.HTML,
		Status: domain.EmailPending, NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now})
}

// notify is queue for changes already committed: the email is not worth failing them.
func (a *App) notify(ctx context.Context, kind string, n domain.Notice) {
	if err := a.queue(ctx, kind, n); err != nil {
		log.Printf("email %s: %v", kind, err)
	}
}

func (a *App) SendDueEmails(ctx context.Context, n int) int {
	now := a.Now()
	batch, err := a.Emails.ClaimDue(ctx, now, now.Add(2*time.Minute), n)
	if err != nil {
		return 0
	}
	for i := range batch {
		a.sendQueued(ctx, &batch[i])
	}
	return len(batch)
}

func (a *App) sendQueued(ctx context.Context, q *domain.QueuedEmail) {
	err := a.Mailer.Send(ctx, domain.Email{To: q.To, Subject: q.Subject, Text: q.Text, HTML: q.HTML})
	now := a.Now()
	q.Attempts, q.UpdatedAt, q.LastError = q.Attempts+1, now, ""
	switch {
	case err == nil:
		q.Status, q.SentAt, q.NextAttemptAt, q.Text, q.HTML = domain.EmailSent, &now, nil, "", ""
	case q.Attempts > len(domain.RetrySchedule):
		q.Status, q.NextAttemptAt, q.LastError, q.Text, q.HTML = domain.EmailFailed, nil, domain.Trim(err.Error(), 500), "", ""
		log.Printf("email %s %s gave up after %d attempts: %v", q.Kind, q.ID, q.Attempts, err)
	default:
		next := now.Add(domain.RetrySchedule[q.Attempts-1])
		q.NextAttemptAt, q.LastError = &next, domain.Trim(err.Error(), 500)
		log.Printf("email %s %s attempt %d: %v", q.Kind, q.ID, q.Attempts, err)
	}
	if err := a.Emails.Save(ctx, q); err != nil {
		log.Printf("email %s %s: %v", q.Kind, q.ID, err)
	}
}

func (a *App) link(path string) string { return strings.TrimRight(a.PublicURL, "/") + path }

func hello(name string) string {
	if f := strings.Fields(name); len(f) > 0 {
		return "Olá " + f[0] + ","
	}
	return "Olá,"
}

func merchantHello(m *domain.Merchant) string {
	return hello(domain.FirstNonEmpty(m.OwnerName, m.Name))
}

func mt(c int64) string { return domain.Format(c) + " MT" }

func (a *App) adminAlert(ctx context.Context, kind string, n domain.Notice) error {
	if a.AdminNotifyEmail == "" {
		return nil
	}
	n.To = a.AdminNotifyEmail
	return a.queue(ctx, kind, n)
}

// Accounts

func (a *App) mailWelcome(ctx context.Context, m *domain.Merchant) error {
	return a.queue(ctx, "account.welcome", domain.Notice{
		To: m.Email, Subject: "Bem-vindo à RIHA", Title: "Bem-vindo à RIHA",
		Lines: []string{merchantHello(m), "A conta de " + m.Name + " está criada. Falta só verificar o negócio para começar a receber por M-Pesa, e-Mola e cartão.",
			"Precisa do tipo de negócio, do NUIT e da conta bancária. Os documentos podem ser enviados mais tarde."},
		Action: "Verificar o negócio", URL: a.link("/verificacao"),
	})
}

func (a *App) mailKycReceived(ctx context.Context, m *domain.Merchant) {
	a.notify(ctx, "account.kyc_received", domain.Notice{
		To: m.Email, Subject: "Recebemos os dados de " + m.Name, Title: "Estamos a rever a sua conta",
		Lines:  []string{merchantHello(m), "Recebemos os dados de " + m.Name + ". A equipa RIHA vai revê-los e avisamos por email assim que houver uma decisão."},
		Action: "Abrir o painel", URL: a.link("/painel"),
	})
	if err := a.adminAlert(ctx, "admin.kyc_submitted", domain.Notice{
		Subject: "Nova conta para rever: " + m.Name, Title: "Nova conta para rever",
		Rows:   []domain.NoticeRow{{Label: "Negócio", Value: m.Name}, {Label: "NUIT", Value: m.NUIT}, {Label: "Email", Value: m.Email}},
		Action: "Abrir aprovações", URL: a.link("/admin/aprovacoes"),
	}); err != nil {
		log.Printf("email admin.kyc_submitted: %v", err)
	}
}

func (a *App) mailDecision(ctx context.Context, m *domain.Merchant, decision, note string) {
	n := domain.Notice{To: m.Email, Action: "Abrir o painel", URL: a.link("/painel")}
	greet := merchantHello(m)
	switch decision {
	case "approve":
		n.Subject, n.Title = "A conta de "+m.Name+" foi aprovada", "A sua conta foi aprovada"
		n.Lines = []string{greet, "Já pode receber pagamentos: crie links de pagamento, gere chaves API e peça levantamentos para a sua conta bancária."}
		n.Action = "Começar a receber"
	case "reject":
		n.Subject, n.Title = "Não foi possível aprovar a conta de "+m.Name, "Não foi possível aprovar a sua conta"
		n.Lines = []string{greet, "Depois de rever os dados de " + m.Name + ", não conseguimos aprovar a conta. Pode corrigir os dados e submeter de novo."}
		n.Action, n.URL = "Rever os dados", a.link("/verificacao")
	case "info":
		n.Subject, n.Title = "Precisamos de mais informação sobre "+m.Name, "Precisamos de mais informação"
		n.Lines = []string{greet, "Para concluir a verificação de " + m.Name + ", a equipa RIHA pediu mais documentos."}
		n.Note = note
		n.Action, n.URL = "Enviar documentos", a.link("/verificacao")
	case "suspend":
		n.Subject, n.Title = "A conta de "+m.Name+" foi suspensa", "A sua conta foi suspensa"
		n.Lines = []string{greet, "A conta de " + m.Name + " está suspensa. Enquanto assim for, não é possível receber pagamentos, fazer reembolsos nem pedir levantamentos."}
	case "reactivate":
		n.Subject, n.Title = "A conta de "+m.Name+" foi reativada", "A sua conta foi reativada"
		n.Lines = []string{greet, "A suspensão foi levantada."}
		if m.Status == domain.MerchantActive {
			n.Lines = append(n.Lines, "Já pode voltar a receber pagamentos.")
		} else {
			n.Lines = append(n.Lines, "A conta volta a estar em revisão, e avisamos quando houver uma decisão.")
		}
	default:
		return
	}
	a.notify(ctx, "account."+decision, n)
}

func (a *App) mailBankChanged(ctx context.Context, m *domain.Merchant, b *domain.BankAccount) {
	a.notify(ctx, "security.bank_changed", domain.Notice{
		To: m.Email, Subject: "A conta bancária da RIHA foi alterada", Title: "A conta bancária foi alterada",
		Lines:  []string{merchantHello(m), "Os levantamentos de " + m.Name + " passam a ir para esta conta:"},
		Rows:   []domain.NoticeRow{{Label: "Banco", Value: b.BankName}, {Label: "Titular", Value: b.Holder}, {Label: "NIB", Value: "•••• " + b.NIB[len(b.NIB)-4:]}},
		Note:   "Se não foi você, altere já a palavra-passe e reponha a conta bancária.",
		Action: "Rever a conta bancária", URL: a.link("/painel/definicoes/banco"),
	})
}

func (a *App) mailKeyRolled(ctx context.Context, m *domain.Merchant, k *domain.APIKey) {
	a.notify(ctx, "security.key_rolled", domain.Notice{
		To: m.Email, Subject: "Foi gerada uma nova chave API", Title: "Foi gerada uma nova chave API",
		Lines:  []string{merchantHello(m), "A chave secreta de produção de " + m.Name + " foi substituída. A anterior deixou de funcionar."},
		Rows:   []domain.NoticeRow{{Label: "Termina em", Value: k.Last4}, {Label: "Data", Value: domain.EmailTime(k.CreatedAt)}},
		Note:   "Se não foi você, altere já a palavra-passe e gere outra chave.",
		Action: "Abrir as chaves API", URL: a.link("/painel/definicoes/api"),
	})
}

// Passwords

func (a *App) mailPasswordReset(ctx context.Context, u *domain.User, token string) error {
	return a.queue(ctx, "security.password_reset", domain.Notice{
		To: u.Email, Subject: "Reponha a sua palavra-passe da RIHA", Title: "Reponha a sua palavra-passe",
		Lines: []string{hello(u.Name), "Recebemos um pedido para repor a palavra-passe da sua conta RIHA. O link é válido durante 1 hora e só funciona uma vez."},
		// The token goes in the fragment, so it never reaches a server or its logs.
		Action: "Repor a palavra-passe", URL: a.link("/recuperar/nova#" + token),
		Small: "Se não foi você, ignore este email: a sua palavra-passe não muda.",
	})
}

func (a *App) mailPasswordChanged(ctx context.Context, u *domain.User) error {
	return a.queue(ctx, "security.password_changed", domain.Notice{
		To: u.Email, Subject: "A sua palavra-passe da RIHA foi alterada", Title: "A sua palavra-passe foi alterada",
		Lines: []string{hello(u.Name), "A palavra-passe da sua conta foi alterada em " + domain.EmailTime(a.Now()) + "."},
		Note:  "Se não foi você, reponha já a palavra-passe.", Action: "Repor a palavra-passe", URL: a.link("/recuperar"),
	})
}

// Payments

func customerReceipt(a *App, p *domain.Payment, m *domain.Merchant) domain.Notice {
	rows := []domain.NoticeRow{{Label: "Comerciante", Value: m.Name}, {Label: "Valor", Value: mt(p.Amount)}, {Label: "Método", Value: domain.MethodName(p.Method)}}
	if p.Description != "" {
		rows = append(rows, domain.NoticeRow{Label: "Descrição", Value: p.Description})
	}
	when := p.CreatedAt
	if p.SucceededAt != nil {
		when = *p.SucceededAt
	}
	rows = append(rows, domain.NoticeRow{Label: "Referência", Value: p.Reference}, domain.NoticeRow{Label: "Data", Value: domain.EmailTime(when)})
	n := domain.Notice{To: p.CustomerEmail, Subject: "Recibo do pagamento a " + m.Name, Title: "Pagamento confirmado",
		Lines: []string{hello(p.CustomerName), "O seu pagamento de " + mt(p.Amount) + " a " + m.Name + " foi concluído."}, Rows: rows,
		Small: "Para dúvidas sobre a compra, contacte " + m.Name + " e indique a referência."}
	if !p.Livemode {
		n.Subject, n.Note = "[Teste] "+n.Subject, "Este pagamento foi feito em modo de teste. Não houve movimento de dinheiro."
	}
	return n
}

func (a *App) mailPaymentSucceeded(ctx context.Context, p *domain.Payment, m *domain.Merchant) error {
	if !p.Livemode {
		return nil
	}
	if p.CustomerEmail != "" {
		if err := a.queue(ctx, "payment.receipt", customerReceipt(a, p, m)); err != nil {
			return err
		}
	}
	if !m.EmailNotifications {
		return nil
	}
	rows := []domain.NoticeRow{{Label: "Valor", Value: mt(p.Amount)}, {Label: "Taxa RIHA", Value: mt(p.Fee)}, {Label: "Líquido", Value: mt(p.Net)},
		{Label: "Método", Value: domain.MethodName(p.Method)}, {Label: "Referência", Value: p.Reference}}
	if p.CustomerName != "" {
		rows = append(rows, domain.NoticeRow{Label: "Cliente", Value: p.CustomerName})
	}
	return a.queue(ctx, "payment.received", domain.Notice{
		To: m.Email, Subject: "Recebeu " + mt(p.Amount), Title: "Recebeu um pagamento",
		Lines: []string{merchantHello(m), "O pagamento de " + mt(p.Amount) + " foi concluído e o valor líquido já está no seu saldo."}, Rows: rows,
		Action: "Ver a transação", URL: a.link("/painel/transacoes/" + p.ID),
		Small: "Pode desligar estes avisos em Definições, no painel.",
	})
}

func (a *App) mailRefund(ctx context.Context, r *domain.Refund, p *domain.Payment, m *domain.Merchant) error {
	if !p.Livemode {
		return nil
	}
	if p.CustomerEmail != "" {
		if err := a.queue(ctx, "refund.customer", domain.Notice{
			To: p.CustomerEmail, Subject: "Reembolso de " + m.Name, Title: "O seu reembolso foi processado",
			Lines: []string{hello(p.CustomerName), m.Name + " devolveu " + mt(r.Amount) + " do seu pagamento."},
			Rows:  []domain.NoticeRow{{Label: "Valor devolvido", Value: mt(r.Amount)}, {Label: "Método", Value: domain.MethodName(p.Method)}, {Label: "Referência", Value: p.Reference}},
			Small: "Para dúvidas sobre o reembolso, contacte " + m.Name + " e indique a referência.",
		}); err != nil {
			return err
		}
	}
	return a.queue(ctx, "refund.merchant", domain.Notice{
		To: m.Email, Subject: "Reembolso de " + mt(r.Amount) + " concluído", Title: "Reembolso concluído",
		Lines:  []string{merchantHello(m), "O valor foi debitado do seu saldo. A taxa RIHA não é devolvida."},
		Rows:   []domain.NoticeRow{{Label: "Valor", Value: mt(r.Amount)}, {Label: "Pagamento", Value: p.Reference}},
		Action: "Ver a transação", URL: a.link("/painel/transacoes/" + p.ID),
	})
}

// SendReceipt sends the receipt again from the dashboard. An address given here
// is kept on the payment, so a later refund notice reaches the customer too.
func (a *App) SendReceipt(ctx context.Context, merchantID string, live bool, paymentID, email string) (string, error) {
	email = strings.TrimSpace(email)
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return "", domain.Invalid("email_invalid", "email", "Email inválido.")
		}
	}
	var to string
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		p, err := a.Payments.Lock(ctx, paymentID)
		if err != nil || p.MerchantID != merchantID || p.Livemode != live {
			return domain.NotFound("O pagamento")
		}
		if !domain.Contains(domain.SettledStatuses, p.Status) {
			return domain.Invalid("payment_not_settled", "payment", "Só pagamentos concluídos têm recibo.")
		}
		if email != "" && email != p.CustomerEmail {
			p.CustomerEmail = email
			if err := a.Payments.Save(ctx, p); err != nil {
				return err
			}
		}
		if p.CustomerEmail == "" {
			return domain.Invalid("customer_email_missing", "email", "Indique o email do cliente.")
		}
		m, err := a.Merchants.ByID(ctx, merchantID)
		if err != nil {
			return err
		}
		to = p.CustomerEmail
		return a.queue(ctx, "payment.receipt", customerReceipt(a, p, m))
	})
	return to, err
}

// Payouts

func (a *App) mailPayoutRequested(ctx context.Context, p *domain.Payout, m *domain.Merchant) error {
	if !p.Livemode {
		return nil
	}
	if err := a.queue(ctx, "payout.requested", domain.Notice{
		To: m.Email, Subject: "Pedido de levantamento " + p.Ref(), Title: "Recebemos o seu pedido de levantamento",
		Lines:  []string{merchantHello(m), "O valor saiu do saldo disponível e será transferido até " + domain.EmailTime(p.DueAt) + "."},
		Rows:   payoutRows(p),
		Action: "Ver levantamentos", URL: a.link("/painel/saldo"),
	}); err != nil {
		return err
	}
	return a.adminAlert(ctx, "admin.payout_requested", domain.Notice{
		Subject: "Novo levantamento " + p.Ref() + ": transferir " + mt(p.Net()), Title: "Novo pedido de levantamento",
		Rows:   append([]domain.NoticeRow{{Label: "Comerciante", Value: m.Name}}, payoutRows(p)...),
		Action: "Abrir levantamentos", URL: a.link("/admin/levantamentos"),
	})
}

func payoutRows(p *domain.Payout) []domain.NoticeRow {
	rows := []domain.NoticeRow{{Label: "Referência", Value: p.Ref()}, {Label: "Valor", Value: mt(p.Amount)}}
	if p.Fee > 0 {
		rows = append(rows, domain.NoticeRow{Label: "Taxa de transferência", Value: "−" + mt(p.Fee)}, domain.NoticeRow{Label: "Recebe", Value: mt(p.Net())})
	}
	return append(rows, domain.NoticeRow{Label: "Banco", Value: p.BankName}, domain.NoticeRow{Label: "NIB", Value: "•••• " + p.NIB[len(p.NIB)-4:]})
}

func (a *App) mailPayoutState(ctx context.Context, p *domain.Payout, m *domain.Merchant) error {
	if !p.Livemode {
		return nil
	}
	n := domain.Notice{To: m.Email, Rows: payoutRows(p), Action: "Ver levantamentos", URL: a.link("/painel/saldo")}
	greet := merchantHello(m)
	switch p.Status {
	case domain.PayoutSent:
		n.Subject, n.Title = "Levantamento "+p.Ref()+" enviado ao banco", "O levantamento foi enviado ao banco"
		n.Lines = []string{greet, "A ordem de transferência seguiu para o banco. Avisamos quando estiver concluída."}
	case domain.PayoutPaid:
		n.Subject, n.Title = "Levantamento "+p.Ref()+" transferido", "O levantamento foi transferido"
		n.Lines = []string{greet, "O valor foi transferido para a sua conta bancária. O comprovativo está disponível no painel."}
		n.Action = "Ver o comprovativo"
	case domain.PayoutRejected, domain.PayoutFailed:
		n.Subject, n.Title = "Levantamento "+p.Ref()+" não concluído", "O levantamento não foi concluído"
		n.Lines = []string{greet, "O levantamento não foi concluído e o valor voltou ao seu saldo."}
		if p.FailureReason != "" {
			n.Note = "Motivo: " + p.FailureReason
		}
	default:
		return nil
	}
	return a.queue(ctx, "payout."+p.Status, n)
}
