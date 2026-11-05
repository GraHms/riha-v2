package rails

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"riha/internal/core/domain"
)

type mpesaFake struct {
	mu     sync.Mutex
	body   mpesaBody
	auth   string
	origin string
	c2b    func(w http.ResponseWriter)
	query  func(w http.ResponseWriter, r *http.Request)
}

func reply(w http.ResponseWriter, status int, code string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"output_ResponseCode": code, "output_ResponseDesc": "x", "output_TransactionID": "TX9"})
}

func newMpesa(t *testing.T, f *mpesaFake) (*Mpesa, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	mux := http.NewServeMux()
	mux.HandleFunc("/ipg/v1x/c2bPayment/singleStage/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		json.NewDecoder(r.Body).Decode(&f.body)
		f.auth, f.origin = r.Header.Get("Authorization"), r.Header.Get("Origin")
		c := f.c2b
		f.mu.Unlock()
		c(w)
	})
	mux.HandleFunc("/ipg/v1x/queryTransactionStatus/", func(w http.ResponseWriter, r *http.Request) {
		f.query(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	i := strings.LastIndex(srv.URL, ":")
	m, err := NewMpesa(MpesaConfig{BaseURL: srv.URL[:i], C2BPort: srv.URL[i+1:], QueryPort: srv.URL[i+1:], APIKey: "chave-api", PublicKey: base64.StdEncoding.EncodeToString(der), ServiceProviderCode: "171717"})
	if err != nil {
		t.Fatal(err)
	}
	return m, priv
}

func charge(t *testing.T, m *Mpesa) <-chan domain.Outcome {
	t.Helper()
	done := make(chan domain.Outcome, 1)
	res, err := m.Initiate(context.Background(), domain.ChargeRequest{Method: domain.MethodMpesa, Amount: 12550, Phone: "+258841234567", Reference: "RIHAABCDEFGHJKLMNPQR", Live: true}, func(o domain.Outcome) { done <- o })
	if err != nil {
		t.Fatal(err)
	}
	if !res.Async || res.Last4 != "4567" {
		t.Fatalf("%+v", res)
	}
	return done
}

func TestMpesaRequest(t *testing.T) {
	f := &mpesaFake{c2b: func(w http.ResponseWriter) { reply(w, 201, "INS-0") }}
	m, priv := newMpesa(t, f)
	select {
	case o := <-charge(t, m):
		if !o.Success || o.OperatorReference != "TX9" {
			t.Fatalf("%+v", o)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no outcome")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.body.CustomerMSISDN != "258841234567" || f.body.Amount != "125.50" || f.body.ServiceProviderCode != "171717" || f.body.ThirdPartyReference != "RIHAABCDEFGHJKLMNPQR" || f.body.TransactionReference != "RIHAABCDEFGHJKLMNPQR" {
		t.Fatalf("%+v", f.body)
	}
	cipher, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(f.auth, "Bearer "))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := rsa.DecryptPKCS1v15(rand.Reader, priv, cipher)
	if err != nil || string(plain) != "chave-api" || f.origin != "*" {
		t.Fatalf("%v %q %q", err, plain, f.origin)
	}
}

func TestMpesaFailures(t *testing.T) {
	for code, want := range map[string]string{
		"INS-2006": domain.FailInsufficientFunds, "INS-2001": domain.FailPinIncorrect, "INS-5": domain.FailCustomerCancelled,
		"INS-2051": domain.FailPhoneInvalid, "INS-996": domain.FailAccountInactive, "INS-2": domain.FailOperatorUnavailable, "INS-777": domain.FailOperatorUnavailable,
	} {
		f := &mpesaFake{c2b: func(w http.ResponseWriter) { reply(w, 400, code) }}
		m, _ := newMpesa(t, f)
		select {
		case o := <-charge(t, m):
			if o.Success || o.FailureCode != want {
				t.Fatalf("%s: %+v", code, o)
			}
			if domain.FailureMessages[o.FailureCode] == "" {
				t.Fatalf("%s has no message", o.FailureCode)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("%s: no outcome", code)
		}
	}
}

// The customer may have paid, so none of these may end the payment.
func TestMpesaUnclearAnswersLeaveItProcessing(t *testing.T) {
	for name, c2b := range map[string]func(w http.ResponseWriter){
		"internal":  func(w http.ResponseWriter) { reply(w, 500, "INS-1") },
		"not json":  func(w http.ResponseWriter) { w.WriteHeader(502); w.Write([]byte("<html>")) },
		"wrong 200": func(w http.ResponseWriter) { reply(w, 200, "INS-0") },
		"dropped": func(w http.ResponseWriter) {
			c, _, _ := w.(http.Hijacker).Hijack()
			c.Close()
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := &mpesaFake{c2b: c2b}
			m, _ := newMpesa(t, f)
			select {
			case o := <-charge(t, m):
				t.Fatalf("ended with %+v", o)
			case <-time.After(300 * time.Millisecond):
			}
		})
	}
}

func TestMpesaQuery(t *testing.T) {
	for _, c := range []struct {
		status, code string
		http         int
		known        bool
		success      bool
		failure      string
	}{
		{"Completed", "INS-0", 200, true, true, ""},
		{"Cancelled", "INS-0", 200, true, false, domain.FailCustomerCancelled},
		{"Expired", "INS-0", 200, true, false, domain.FailCustomerTimeout},
		{"N/A", "INS-997", 400, false, false, ""},
	} {
		f := &mpesaFake{query: func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("input_QueryReference") != "RIHAX" || r.URL.Query().Get("input_ServiceProviderCode") != "171717" {
				t.Errorf("query: %s", r.URL.RawQuery)
			}
			w.WriteHeader(c.http)
			json.NewEncoder(w).Encode(map[string]string{"output_ResponseCode": c.code, "output_ResponseTransactionStatus": c.status})
		}}
		m, _ := newMpesa(t, f)
		o, known, err := m.Query(context.Background(), domain.StatusQuery{Reference: "RIHAX"})
		if err != nil || known != c.known || o.Success != c.success || o.FailureCode != c.failure {
			t.Fatalf("%s: %+v %v %v", c.status, o, known, err)
		}
	}
	// A bad key says nothing about the payment: it must be an error, not "no record".
	f := &mpesaFake{query: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(401); w.Write([]byte("{}")) }}
	m, _ := newMpesa(t, f)
	if _, known, err := m.Query(context.Background(), domain.StatusQuery{Reference: "RIHAX"}); err == nil || known {
		t.Fatal("a refused query must be an error")
	}
}

func TestMpesaRefusesWhatItCannotTake(t *testing.T) {
	m, _ := newMpesa(t, &mpesaFake{})
	for _, r := range []domain.ChargeRequest{
		{Method: domain.MethodEmola, Amount: 100, Phone: "861234567", Reference: "R"},
		{Method: domain.MethodCard, Amount: 100, Reference: "R"},
		{Method: domain.MethodMpesa, Amount: 100, Phone: "861234567", Reference: "R"},
		{Method: domain.MethodMpesa, Amount: 100, Phone: "84", Reference: "R"},
		{Method: domain.MethodMpesa, Amount: 100, Phone: "841234567"},
	} {
		if _, err := m.Initiate(context.Background(), r, func(domain.Outcome) { t.Error("completed") }); err == nil {
			t.Fatalf("accepted %+v", r)
		}
	}
}

type recordRail struct{ got []domain.ChargeRequest }

func (r *recordRail) Initiate(_ context.Context, q domain.ChargeRequest, _ func(domain.Outcome)) (domain.ChargeResult, error) {
	r.got = append(r.got, q)
	return domain.ChargeResult{}, nil
}

func TestTestModeNeverReachesTheRealRail(t *testing.T) {
	test, live := &recordRail{}, &recordRail{}
	modes := Modes{Test: test, Live: live}
	modes.Initiate(context.Background(), domain.ChargeRequest{Live: false}, nil)
	if len(live.got) != 0 || len(test.got) != 1 {
		t.Fatalf("test mode went live: %+v", live.got)
	}
	modes.Initiate(context.Background(), domain.ChargeRequest{Live: true}, nil)
	if len(live.got) != 1 || len(test.got) != 1 {
		t.Fatalf("live: %+v", live.got)
	}
}

func TestMpesaTimeoutIsSettledByAskingMpesa(t *testing.T) {
	mpesaTimeoutCheckDelay = 0
	for name, c := range map[string]struct {
		status  string
		http    int
		success bool
		failure string
	}{
		"no record":               {"N/A", 400, false, domain.FailCustomerTimeout},
		"paid at the last second": {"Completed", 200, true, ""},
	} {
		t.Run(name, func(t *testing.T) {
			f := &mpesaFake{c2b: func(w http.ResponseWriter) { reply(w, 408, "INS-9") }, query: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(c.http)
				json.NewEncoder(w).Encode(map[string]string{"output_ResponseCode": "INS-997", "output_ResponseTransactionStatus": c.status})
			}}
			m, _ := newMpesa(t, f)
			select {
			case o := <-charge(t, m):
				if o.Success != c.success || o.FailureCode != c.failure {
					t.Fatalf("%+v", o)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("no outcome")
			}
		})
	}
	t.Run("M-Pesa unreachable leaves it for the sweep", func(t *testing.T) {
		f := &mpesaFake{c2b: func(w http.ResponseWriter) { reply(w, 408, "INS-9") }, query: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(401); w.Write([]byte("{}")) }}
		m, _ := newMpesa(t, f)
		select {
		case o := <-charge(t, m):
			t.Fatalf("ended with %+v", o)
		case <-time.After(300 * time.Millisecond):
		}
	})
}
