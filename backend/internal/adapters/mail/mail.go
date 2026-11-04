package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
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
	var b bytes.Buffer
	h := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	h("From", from)
	h("To", to.String())
	h("Subject", mime.QEncoding.Encode("utf-8", strings.NewReplacer("\r", " ", "\n", " ").Replace(e.Subject)))
	h("Date", time.Now().Format(time.RFC1123Z))
	h("MIME-Version", "1.0")
	h("Auto-Submitted", "auto-generated")
	if len(e.Attachments) == 0 {
		writeBody(&b, e)
		return b.Bytes(), nil
	}
	boundary := "riha-" + domain.Token("", 24)
	h("Content-Type", `multipart/mixed; boundary="`+boundary+`"`)
	b.WriteString("\r\n--" + boundary + "\r\n")
	writeBody(&b, e)
	for _, a := range e.Attachments {
		name := mime.QEncoding.Encode("utf-8", filepath.Base(a.Name))
		fmt.Fprintf(&b, "\r\n--%s\r\nContent-Type: %s; name=\"%s\"\r\nContent-Disposition: attachment; filename=\"%s\"\r\nContent-Transfer-Encoding: base64\r\n\r\n", boundary, a.ContentType, name, name)
		wrap(&b, base64.StdEncoding.EncodeToString(a.Data))
	}
	b.WriteString("\r\n--" + boundary + "--\r\n")
	return b.Bytes(), nil
}

// writeBody writes the text part, or text and HTML as alternatives of one message.
func writeBody(b *bytes.Buffer, e domain.Email) {
	part := func(contentType, body string) {
		fmt.Fprintf(b, "Content-Type: %s; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n", contentType)
		wrap(b, base64.StdEncoding.EncodeToString([]byte(body)))
	}
	if e.HTML == "" {
		part("text/plain", e.Text)
		return
	}
	boundary := "riha-alt-" + domain.Token("", 24)
	fmt.Fprintf(b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n--%s\r\n", boundary, boundary)
	part("text/plain", e.Text)
	b.WriteString("\r\n--" + boundary + "\r\n")
	part("text/html", e.HTML)
	b.WriteString("\r\n--" + boundary + "--\r\n")
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

const smtpTimeout = 30 * time.Second

// Send does what smtp.SendMail does, under a deadline: the worker sends one
// email at a time, so a stuck server must not hold up the rest of the queue.
func (s SMTP) Send(ctx context.Context, e domain.Email) error {
	msg, err := build(s.From, e)
	if err != nil {
		return err
	}
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return err
	}
	to, _ := mail.ParseAddress(e.To)
	ctx, cancel := context.WithTimeout(ctx, smtpTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Hello(from.Address[strings.LastIndex(from.Address, "@")+1:]); err != nil {
		return err
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if s.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.Username, s.Password, host)); err != nil {
			return err
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(to.Address); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
