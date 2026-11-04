package rails

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

const (
	// M-Pesa answers when the customer does, so this outlasts their prompt. The
	// sweep starts asking about a payment only after this has run out.
	mpesaRequestTimeout = 150 * time.Second
	mpesaQueryTimeout   = 20 * time.Second
)

type MpesaConfig struct {
	BaseURL             string
	C2BPort             string
	QueryPort           string
	APIKey              string
	PublicKey           string
	ServiceProviderCode string
}

// Mpesa is the Vodacom M-Pesa rail. A request blocks until the customer answers
// the prompt, so it runs on a goroutine and reports through onComplete.
//
// A payment only fails when M-Pesa says so. A timeout or a dropped connection
// doesn't call onComplete, because the customer may have paid: the payment stays
// processing, and the sweep asks Query what happened.
type Mpesa struct {
	cfg      MpesaConfig
	key      *rsa.PublicKey
	c2bURL   string
	queryURL string
	http     *http.Client
}

func NewMpesa(cfg MpesaConfig) (*Mpesa, error) {
	der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cfg.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("mpesa: decode public key: %w", err)
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("mpesa: parse public key: %w", err)
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("mpesa: the public key is not RSA")
	}
	if cfg.APIKey == "" || cfg.ServiceProviderCode == "" {
		return nil, errors.New("mpesa: the API key and the service provider code are required")
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	return &Mpesa{cfg: cfg, key: key,
		c2bURL:   base + ":" + cfg.C2BPort + "/ipg/v1x/c2bPayment/singleStage/",
		queryURL: base + ":" + cfg.QueryPort + "/ipg/v1x/queryTransactionStatus/",
		http:     &http.Client{}}, nil
}

// headers carry the API key encrypted with M-Pesa's public key, new for each request.
func (m *Mpesa) headers(r *http.Request) error {
	out, err := rsa.EncryptPKCS1v15(rand.Reader, m.key, []byte(m.cfg.APIKey))
	if err != nil {
		return err
	}
	r.Header.Set("Authorization", "Bearer "+base64.StdEncoding.EncodeToString(out))
	r.Header.Set("Origin", "*")
	r.Header.Set("Content-Type", "application/json")
	return nil
}

type mpesaBody struct {
	TransactionReference string `json:"input_TransactionReference"`
	CustomerMSISDN       string `json:"input_CustomerMSISDN"`
	Amount               string `json:"input_Amount"`
	ThirdPartyReference  string `json:"input_ThirdPartyReference"`
	ServiceProviderCode  string `json:"input_ServiceProviderCode"`
}

type mpesaResponse struct {
	ResponseCode      string `json:"output_ResponseCode"`
	ResponseDesc      string `json:"output_ResponseDesc"`
	TransactionID     string `json:"output_TransactionID"`
	TransactionStatus string `json:"output_ResponseTransactionStatus"`
}

func (m *Mpesa) Initiate(_ context.Context, req domain.ChargeRequest, onComplete func(domain.Outcome)) (domain.ChargeResult, error) {
	if req.Method != domain.MethodMpesa {
		return domain.ChargeResult{}, &domain.InputError{Field: "method", Msg: "Por agora só aceitamos M-Pesa."}
	}
	phone := domain.NormalizeMZPhone(req.Phone)
	if len(phone) != 9 {
		return domain.ChargeResult{}, &domain.InputError{Field: "phone", Msg: "Número inválido. Use 9 dígitos, por exemplo 84 123 4567."}
	}
	if pre := phone[:2]; pre != "84" && pre != "85" {
		return domain.ChargeResult{}, &domain.InputError{Field: "phone", Msg: "Números M-Pesa começam por 84 ou 85."}
	}
	if req.Reference == "" {
		return domain.ChargeResult{}, errors.New("mpesa: the payment has no reference")
	}
	body, err := json.Marshal(mpesaBody{
		TransactionReference: req.Reference, ThirdPartyReference: req.Reference, CustomerMSISDN: "258" + phone,
		Amount: fmt.Sprintf("%d.%02d", req.Amount/100, req.Amount%100), ServiceProviderCode: m.cfg.ServiceProviderCode,
	})
	if err != nil {
		return domain.ChargeResult{}, err
	}
	go m.send(req.Reference, body, onComplete)
	return domain.ChargeResult{Async: true, Last4: phone[5:]}, nil
}

func (m *Mpesa) send(ref string, body []byte, onComplete func(domain.Outcome)) {
	ctx, cancel := context.WithTimeout(context.Background(), mpesaRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.c2bURL, bytes.NewReader(body))
	if err == nil {
		err = m.headers(req)
	}
	if err != nil {
		// Never left, so it can't have charged anyone.
		log.Printf("mpesa %s: not sent: %v", ref, err)
		onComplete(domain.Outcome{FailureCode: domain.FailOperatorUnavailable})
		return
	}
	res, err := m.http.Do(req)
	if err != nil {
		log.Printf("mpesa %s: no answer: %v", ref, err)
		return
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if err != nil {
		log.Printf("mpesa %s: answer cut short: %v", ref, err)
		return
	}
	var out mpesaResponse
	if err := json.Unmarshal(raw, &out); err != nil || out.ResponseCode == "" {
		log.Printf("mpesa %s: answer not understood (http %d)", ref, res.StatusCode)
		return
	}
	if o, known := mpesaOutcome(ref, res.StatusCode, out); known {
		onComplete(o)
	}
}

// mpesaOutcome reports false for answers that don't say whether the customer paid.
func mpesaOutcome(ref string, status int, out mpesaResponse) (domain.Outcome, bool) {
	code := out.ResponseCode
	if code == "INS-0" && status == http.StatusCreated {
		return domain.Outcome{Success: true, OperatorReference: out.TransactionID}, true
	}
	if code == "INS-0" || code == "INS-1" || code == "INS-9" || code == "INS-23" {
		log.Printf("mpesa %s: outcome unclear: %s (http %d)", ref, code, status)
		return domain.Outcome{}, false
	}
	failure, known := mpesaFailures[code]
	if !known {
		// The customer can't act on these: a bad key or short code is ours to fix.
		log.Printf("mpesa %s: refused: %s: %s (http %d)", ref, code, out.ResponseDesc, status)
		failure = domain.FailOperatorUnavailable
	}
	return domain.Outcome{FailureCode: failure}, true
}

var mpesaFailures = map[string]string{
	"INS-4":    domain.FailAccountInactive,
	"INS-5":    domain.FailCustomerCancelled,
	"INS-6":    domain.FailPaymentDeclined,
	"INS-10":   domain.FailPaymentDeclined,
	"INS-16":   domain.FailOperatorUnavailable,
	"INS-995":  domain.FailAccountInactive,
	"INS-996":  domain.FailAccountInactive,
	"INS-2001": domain.FailPinIncorrect,
	"INS-2002": domain.FailPhoneInvalid,
	"INS-2006": domain.FailInsufficientFunds,
	"INS-2051": domain.FailPhoneInvalid,
}

// Query asks M-Pesa about a reference. It returns false when M-Pesa has no answer.
func (m *Mpesa) Query(ctx context.Context, ref string) (domain.Outcome, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, mpesaQueryTimeout)
	defer cancel()
	q := url.Values{"input_ThirdPartyReference": {ref}, "input_QueryReference": {ref}, "input_ServiceProviderCode": {m.cfg.ServiceProviderCode}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.queryURL+"?"+q.Encode(), nil)
	if err != nil {
		return domain.Outcome{}, false, err
	}
	if err := m.headers(req); err != nil {
		return domain.Outcome{}, false, err
	}
	res, err := m.http.Do(req)
	if err != nil {
		return domain.Outcome{}, false, err
	}
	defer res.Body.Close()
	var out mpesaResponse
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<16)).Decode(&out); err != nil {
		return domain.Outcome{}, false, fmt.Errorf("query answer not understood (http %d)", res.StatusCode)
	}
	if out.ResponseCode == "" {
		// A refused request, such as a bad key, says nothing about the payment.
		return domain.Outcome{}, false, fmt.Errorf("query refused (http %d)", res.StatusCode)
	}
	switch strings.ToLower(out.TransactionStatus) {
	case "completed":
		return domain.Outcome{Success: true, OperatorReference: domain.FirstNonEmpty(out.TransactionID, ref)}, true, nil
	case "cancelled", "canceled":
		return domain.Outcome{FailureCode: domain.FailCustomerCancelled}, true, nil
	case "expired":
		return domain.Outcome{FailureCode: domain.FailCustomerTimeout}, true, nil
	}
	return domain.Outcome{}, false, nil
}

// Modes keeps live and test money apart: test mode never reaches a real operator.
type Modes struct {
	Test, Live ports.Rail
}

func (m Modes) Initiate(ctx context.Context, req domain.ChargeRequest, onComplete func(domain.Outcome)) (domain.ChargeResult, error) {
	if req.Live {
		return m.Live.Initiate(ctx, req, onComplete)
	}
	return m.Test.Initiate(ctx, req, onComplete)
}

func (m Modes) Query(ctx context.Context, ref string) (domain.Outcome, bool, error) {
	if q, ok := m.Live.(ports.RailQuery); ok {
		return q.Query(ctx, ref)
	}
	return domain.Outcome{}, false, nil
}
