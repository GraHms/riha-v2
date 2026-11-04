package mail

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/mail"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"riha/internal/core/domain"
)

func build(from string, e domain.Email) ([]byte, error) {
	to, err := mail.ParseAddress(e.To)
	if err != nil {
		return nil, fmt.Errorf("invalid recipient: %w", err)
	}
	boundary := "riha-" + domain.Token("", 24)
	var b bytes.Buffer
	h := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	h("From", from)
	h("To", to.String())
	h("Subject", mime.QEncoding.Encode("utf-8", strings.NewReplacer("\r", " ", "\n", " ").Replace(e.Subject)))
	h("Date", time.Now().Format(time.RFC1123Z))
	h("MIME-Version", "1.0")
	h("Content-Type", `multipart/mixed; boundary="`+boundary+`"`)
	b.WriteString("\r\n--" + boundary + "\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	wrap(&b, base64.StdEncoding.EncodeToString([]byte(e.Text)))
	for _, a := range e.Attachments {
		name := mime.QEncoding.Encode("utf-8", filepath.Base(a.Name))
		fmt.Fprintf(&b, "\r\n--%s\r\nContent-Type: %s; name=\"%s\"\r\nContent-Disposition: attachment; filename=\"%s\"\r\nContent-Transfer-Encoding: base64\r\n\r\n", boundary, a.ContentType, name, name)
		wrap(&b, base64.StdEncoding.EncodeToString(a.Data))
	}
	b.WriteString("\r\n--" + boundary + "--\r\n")
	return b.Bytes(), nil
}

func wrap(b *bytes.Buffer, s string) {
	for len(s) > 76 {
		b.WriteString(s[:76] + "\r\n")
		s = s[76:]
	}
	b.WriteString(s + "\r\n")
}

type Outbox struct {
	Dir  string
	From string
}

func (o Outbox) Send(_ context.Context, e domain.Email) error {
	msg, err := build(o.From, e)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(o.Dir, 0o750); err != nil {
		return err
	}
	name := time.Now().UTC().Format("20060102-150405") + "-" + domain.Token("", 6) + ".eml"
	return os.WriteFile(filepath.Join(o.Dir, name), msg, 0o640)
}

type SMTP struct {
	Addr, Username, Password, From string
}

func (s SMTP) Send(_ context.Context, e domain.Email) error {
	msg, err := build(s.From, e)
	if err != nil {
		return err
	}
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return err
	}
	to, _ := mail.ParseAddress(e.To)
	var auth smtp.Auth
	if s.Username != "" {
		host := strings.Split(s.Addr, ":")[0]
		auth = smtp.PlainAuth("", s.Username, s.Password, host)
	}
	return smtp.SendMail(s.Addr, auth, from.Address, []string{to.Address}, msg)
}
