package mail

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	netmail "net/mail"
	"strings"
	"testing"

	"riha/internal/core/domain"
)

func parts(t *testing.T, ct string, body io.Reader) map[string]string {
	t.Helper()
	media, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	if !strings.HasPrefix(media, "multipart/") {
		raw, _ := io.ReadAll(body)
		dec, err := base64.StdEncoding.DecodeString(strings.NewReplacer("\r", "", "\n", "").Replace(string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		out[media] = string(dec)
		return out
	}
	r := multipart.NewReader(body, params["boundary"])
	for {
		p, err := r.NextRawPart()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range parts(t, p.Header.Get("Content-Type"), p) {
			out[k] = v
		}
	}
}

func TestBuildAlternativesAndAttachment(t *testing.T) {
	raw, err := build("RIHA <no-reply@rihagateway.com>", domain.Email{To: "a@b.co.mz", Subject: "Olá\r\nBcc: x@y.z", Text: "texto", HTML: "<p>html</p>",
		Attachments: []domain.Attachment{{Name: "a.pdf", ContentType: "application/pdf", Data: []byte("pdf")}}})
	if err != nil {
		t.Fatal(err)
	}
	m, err := netmail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if m.Header.Get("Bcc") != "" {
		t.Fatal("subject injected a header")
	}
	got := parts(t, m.Header.Get("Content-Type"), m.Body)
	if got["text/plain"] != "texto" || got["text/html"] != "<p>html</p>" || got["application/pdf"] != "pdf" {
		t.Fatalf("parts: %v", got)
	}
}

func TestBuildWithoutAttachmentsOrHTML(t *testing.T) {
	raw, err := build("RIHA <no-reply@rihagateway.com>", domain.Email{To: "a@b.co.mz", Subject: "x", Text: "só texto"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := netmail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got := parts(t, m.Header.Get("Content-Type"), m.Body); got["text/plain"] != "só texto" || len(got) != 1 {
		t.Fatalf("parts: %v", got)
	}
}
