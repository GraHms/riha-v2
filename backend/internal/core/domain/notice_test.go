package domain

import (
	"strings"
	"testing"
)

func TestNoticeEscapesWhatPeopleType(t *testing.T) {
	e := Notice{To: "a@b.co.mz", Subject: "Olá", Title: "Olá <b>x</b>", Lines: []string{"<script>alert(1)</script>"},
		Rows: []NoticeRow{{"Cliente", "<img src=x>"}}, Note: "<i>nota</i>", Action: "Abrir", URL: "https://rihagateway.com/painel?a=1&b=2"}.Email()
	for _, bad := range []string{"<script>", "<img src=x>", "<i>nota</i>", "<b>x</b>"} {
		if strings.Contains(e.HTML, bad) {
			t.Fatalf("html holds %q", bad)
		}
	}
	if !strings.Contains(e.HTML, "Abrir</a>") || !strings.Contains(e.HTML, "a=1&amp;b=2") {
		t.Fatalf("action missing: %s", e.HTML)
	}
	if !strings.Contains(e.Text, "<script>alert(1)</script>") || !strings.Contains(e.Text, "https://rihagateway.com/painel?a=1&b=2") {
		t.Fatalf("plain text must stay literal: %s", e.Text)
	}
}

func TestNoticeRefusesScriptLinks(t *testing.T) {
	e := Notice{Title: "x", Action: "Abrir", URL: "javascript:alert(1)"}.Email()
	if strings.Contains(e.HTML, "javascript:") {
		t.Fatalf("script url reached the html: %s", e.HTML)
	}
}

func TestEmailsSayWhereToGetHelp(t *testing.T) {
	e := Notice{Title: "x"}.Email()
	if !strings.Contains(e.Text, "support@rihagateway.com") || !strings.Contains(e.HTML, "support@rihagateway.com") {
		t.Fatalf("%s", e.Text)
	}
}
