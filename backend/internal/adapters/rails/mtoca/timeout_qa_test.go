package mtoca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"riha/internal/core/domain"
)

// QA: the sweep, the recheck of unconfirmed payments and the check of closed
// payments call Query with the workers' context, which has no deadline. A
// status call that mtoca accepts but never answers must still end.
func TestQAQueryWithoutDeadlineDoesNotHang(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	if d, _ := New(Config{BaseURL: srv.URL, Channels: map[string]string{"mpesa": "MPESA"}}); d.cfg.QueryFor <= 0 || d.cfg.QueryFor > 30*time.Second {
		t.Fatalf("a status question waits %s by default", d.cfg.QueryFor)
	}
	c, err := New(Config{BaseURL: srv.URL, Channels: map[string]string{"mpesa": "MPESA"}, QueryFor: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		// As RunWorkers passes it: the server's root context.
		_, _, err := c.Query(context.Background(), domain.StatusQuery{Reference: "123456789012", Method: "mpesa"})
		done <- err
	}()
	select {
	case err := <-done:
		t.Logf("returned: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("mtoca Query with the workers' context is still waiting after 5s: one hung status call stops the sweep, the recheck of unconfirmed payments and session expiry for as long as mtoca holds the connection")
	}
}
