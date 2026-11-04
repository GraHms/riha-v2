package filestore

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	types   map[string]string
	headers map[string]http.Header
	t       *testing.T
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=AKIATEST/") || !strings.Contains(auth, "/auto/s3/aws4_request") {
		f.t.Errorf("unsigned or wrong scope: %q", auth)
		w.WriteHeader(403)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := r.URL.Path
	switch r.Method {
	case http.MethodPut:
		b, _ := io.ReadAll(r.Body)
		f.objects[key], f.types[key], f.headers[key] = b, r.Header.Get("Content-Type"), r.Header.Clone()
		w.Header().Set("ETag", `"x"`)
	case http.MethodGet:
		b, ok := f.objects[key]
		if !ok {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>NoSuchKey</Code></Error>`))
			return
		}
		w.Header().Set("Content-Type", f.types[key])
		_, _ = w.Write(b)
	case http.MethodDelete:
		delete(f.objects, key)
		w.WriteHeader(204)
	case http.MethodHead:
		if key != "/riha-docs" {
			w.WriteHeader(404)
		}
	}
}

func TestR2RoundTrip(t *testing.T) {
	f := &fakeS3{objects: map[string][]byte{}, types: map[string]string{}, headers: map[string]http.Header{}, t: t}
	srv := httptest.NewServer(f)
	defer srv.Close()
	r2, err := NewR2(R2Config{Endpoint: srv.URL, AccessKey: "AKIATEST", SecretKey: "secret", Bucket: "riha-docs", Prefix: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := r2.Check(ctx); err != nil {
		t.Fatalf("head bucket: %v", err)
	}
	pdf := []byte("%PDF-1.4 test")
	p, err := r2.Save(ctx, "../../acct_1", "doc_1.pdf", "application/pdf", bytes.NewReader(pdf))
	if err != nil {
		t.Fatal(err)
	}
	if p != "r2:prod/acct_1/doc_1.pdf" {
		t.Fatalf("path %q", p)
	}
	h := f.headers["/riha-docs/prod/acct_1/doc_1.pdf"]
	if f.types["/riha-docs/prod/acct_1/doc_1.pdf"] != "application/pdf" || h.Get("Cache-Control") != "private, no-store" || h.Get("Content-Disposition") != "attachment" {
		t.Fatalf("object headers %v", h)
	}
	got, err := r2.Read(ctx, p)
	if err != nil || !bytes.Equal(got, pdf) {
		t.Fatalf("read %q %v", got, err)
	}
	for _, bad := range []string{"r2:other/acct_1/doc_1.pdf", "r2:prod/../x", "/etc/passwd", "r2:"} {
		if _, err := r2.Read(ctx, bad); err == nil {
			t.Fatalf("%q must be refused", bad)
		}
	}
	router := Router{Primary: r2, Remote: r2, Local: Local{Root: t.TempDir()}}
	if _, err := router.Read(ctx, "/tmp/legacy.pdf"); err == nil {
		t.Fatal("legacy path outside the local root must be refused")
	}
	if err := router.Remove(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := r2.Read(ctx, p); err == nil {
		t.Fatal("removed object must be gone")
	}
}
