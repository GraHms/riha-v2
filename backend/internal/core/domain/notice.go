package domain

import (
	"bytes"
	"html/template"
	"strings"
	"time"
)

const (
	EmailPending = "pending"
	EmailSent    = "sent"
	EmailFailed  = "failed"
)

// QueuedEmail is one email waiting for the worker. Its body is cleared once
// sent, because a password reset body holds a live link.
type QueuedEmail struct {
	ID            string
	Kind          string
	To            string
	Subject       string
	Text          string
	HTML          string
	Status        string
	Attempts      int
	NextAttemptAt *time.Time
	LastError     string
	SentAt        *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type PasswordReset struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type NoticeRow struct{ Label, Value string }

// Notice is the content of one email: everything is rendered through the same
// layout, as HTML and as plain text.
type Notice struct {
	To      string
	Subject string
	Title   string
	Lines   []string
	Rows    []NoticeRow
	Note    string
	Action  string
	URL     string
	Small   string
}

const footerLine = "RIHA · Maputo, Moçambique"
const footerAuto = "Este é um email automático. Por favor, não responda."

var maputoTime = time.FixedZone("CAT", 2*3600)

func EmailTime(t time.Time) string { return t.In(maputoTime).Format("02/01/2006 15:04") }

func MethodName(method string) string {
	switch method {
	case MethodMpesa:
		return "M-Pesa"
	case MethodEmola:
		return "e-Mola"
	case MethodCard:
		return "Cartão"
	}
	return method
}

func (n Notice) Email() Email {
	var html bytes.Buffer
	if err := noticeHTML.Execute(&html, n); err != nil {
		html.Reset()
	}
	return Email{To: n.To, Subject: n.Subject, Text: n.text(), HTML: html.String()}
}

func (n Notice) text() string {
	var b strings.Builder
	b.WriteString(n.Title + "\n\n")
	for _, l := range n.Lines {
		b.WriteString(l + "\n\n")
	}
	for _, r := range n.Rows {
		b.WriteString(r.Label + ": " + r.Value + "\n")
	}
	if len(n.Rows) > 0 {
		b.WriteString("\n")
	}
	if n.Note != "" {
		b.WriteString(n.Note + "\n\n")
	}
	if n.URL != "" {
		b.WriteString(n.Action + ":\n" + n.URL + "\n\n")
	}
	if n.Small != "" {
		b.WriteString(n.Small + "\n\n")
	}
	b.WriteString("--\n" + footerLine + "\n" + footerAuto + "\n")
	return b.String()
}

// Email clients strip style blocks and ignore most layout CSS, hence the tables
// and inline styles.
var noticeHTML = template.Must(template.New("notice").Funcs(template.FuncMap{
	"footerLine": func() string { return footerLine },
	"footerAuto": func() string { return footerAuto },
	"preview": func(n Notice) string {
		if len(n.Lines) > 0 {
			return n.Lines[0]
		}
		return n.Title
	},
}).Parse(`<!doctype html>
<html lang="pt"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light"><title>{{.Subject}}</title></head>
<body style="margin:0;padding:0;background:#F4F3EE;">
<div style="display:none;max-height:0;overflow:hidden;opacity:0;">{{preview .}}</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#F4F3EE;"><tr><td align="center" style="padding:32px 16px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;color:#0B120E;">
<tr><td style="padding:0 4px 18px;"><table role="presentation" cellpadding="0" cellspacing="0"><tr>
<td width="5" height="22" style="background:#0A8205;font-size:0;line-height:0;">&nbsp;</td><td width="3" style="font-size:0;">&nbsp;</td>
<td width="5" height="22" style="background:#FF0F15;font-size:0;line-height:0;">&nbsp;</td><td width="9" style="font-size:0;">&nbsp;</td>
<td style="font-size:17px;font-weight:800;letter-spacing:3px;color:#0B120E;">RIHA</td>
</tr></table></td></tr>
<tr><td style="background:#FFFFFF;border:1px solid #E8E6DF;border-radius:16px;padding:32px 28px;">
<h1 style="margin:0 0 16px;font-size:23px;line-height:1.25;font-weight:700;letter-spacing:-0.3px;color:#0B120E;">{{.Title}}</h1>
{{range .Lines}}<p style="margin:0 0 14px;font-size:15px;line-height:1.55;color:#3B4540;">{{.}}</p>{{end}}
{{if .Rows}}<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:8px 0 18px;border-top:1px solid #E8E6DF;">
{{range .Rows}}<tr><td style="padding:11px 12px 11px 0;border-bottom:1px solid #E8E6DF;font-size:14px;color:#6B7570;vertical-align:top;">{{.Label}}</td><td align="right" style="padding:11px 0;border-bottom:1px solid #E8E6DF;font-size:14px;font-weight:600;color:#0B120E;vertical-align:top;">{{.Value}}</td></tr>
{{end}}</table>{{end}}
{{if .Note}}<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:6px 0 18px;"><tr><td style="background:#FFF6D1;border-radius:10px;padding:13px 15px;font-size:14px;line-height:1.5;color:#5B4600;">{{.Note}}</td></tr></table>{{end}}
{{if .URL}}<table role="presentation" cellpadding="0" cellspacing="0" style="margin:10px 0 6px;"><tr><td style="background:#0A8205;border-radius:11px;"><a href="{{.URL}}" style="display:inline-block;padding:13px 24px;font-size:15px;font-weight:600;color:#FFFFFF;text-decoration:none;">{{.Action}}</a></td></tr></table>{{end}}
{{if .Small}}<p style="margin:16px 0 0;font-size:13px;line-height:1.5;color:#6B7570;">{{.Small}}</p>{{end}}
</td></tr>
<tr><td style="padding:18px 4px 0;font-size:12.5px;line-height:1.6;color:#6B7570;">{{footerLine}}<br>{{footerAuto}}</td></tr>
</table></td></tr></table></body></html>
`))
