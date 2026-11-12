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

// Gives M-Pesa a moment to record a payment that was answered at the last second.
var mpesaTimeoutCheckDelay = 3 * time.Second

type MpesaConfig struct {
	BaseURL             string
	C2BPort             string
	QueryPort           string
	APIKey              string
	PublicKey           string
	ServiceProviderCode string
	// Trace receives what was sent to M-Pesa and what it answered, for the
	// payment's trail in Rasto.
	Trace func(domain.TrailEvent)
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

// mpesaCharge is one payment sent to M-Pesa, with what its trail needs.
type mpesaCharge struct {
	ref, paymentID, requestID string
}

func (m *Mpesa) trace(ev domain.TrailEvent) {
	if m.cfg.Trace != nil {
		m.cfg.Trace(ev)
	}
}

// answered traces M-Pesa's answer to a charge: its code and description, and
// how long it took, which is how long the customer had.
func (m *Mpesa) answered(c mpesaCharge, status int, took time.Duration, raw []byte, out mpesaResponse, title string, level string) {
	m.trace(domain.TrailEvent{PaymentID: c.paymentID, RequestID: c.requestID, Kind: domain.TrailRailResponse, Source: "operator", Method: http.MethodPost,
		Path: "M-Pesa c2bPayment/singleStage", Status: status, DurationMs: took.Milliseconds(), Response: domain.RedactBody(raw), Level: level, Title: title,
		Data: map[string]any{"reference": c.ref, "code": out.ResponseCode, "description": out.ResponseDesc, "transaction": out.TransactionID, "seconds": int(took.Seconds())}})
}

func (m *Mpesa) Initiate(ctx context.Context, req domain.ChargeRequest, onComplete func(domain.Outcome)) (domain.ChargeResult, error) {
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
	c := mpesaCharge{ref: req.Reference, paymentID: req.PaymentID, requestID: domain.RequestFrom(ctx).ID}
	m.trace(domain.TrailEvent{PaymentID: c.paymentID, RequestID: c.requestID, Kind: domain.TrailRailRequest, Source: "riha", Method: http.MethodPost,
		Path: "M-Pesa c2bPayment/singleStage", Request: domain.RedactBody(body), Data: map[string]any{"reference": c.ref},
		Title: "Pedido enviado ao M-Pesa: o cliente deve receber no telemóvel o pedido para digitar o PIN."})
	domain.Safely("mpesa charge", func() { m.send(c, body, onComplete) })
	return domain.ChargeResult{Async: true, Last4: phone[5:]}, nil
}

func (m *Mpesa) send(c mpesaCharge, body []byte, onComplete func(domain.Outcome)) {
	ref := c.ref
	ctx, cancel := context.WithTimeout(context.Background(), mpesaRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.c2bURL, bytes.NewReader(body))
	if err == nil {
		err = m.headers(req)
	}
	if err != nil {
		// Never left, so it can't have charged anyone.
		log.Printf("mpesa %s: not sent: %v", ref, err)
		m.trace(domain.TrailEvent{PaymentID: c.paymentID, RequestID: c.requestID, Kind: domain.TrailRailResponse, Source: "riha", Level: domain.TrailError,
			Title: "O pedido não chegou a sair para o M-Pesa: " + err.Error(), Data: map[string]any{"reference": ref}})
		onComplete(domain.Outcome{FailureCode: domain.FailOperatorUnavailable})
		return
	}
	start := time.Now()
	res, err := m.http.Do(req)
	took := time.Since(start)
	if err != nil {
		log.Printf("mpesa %s: no answer: %v", ref, err)
		m.trace(domain.TrailEvent{PaymentID: c.paymentID, RequestID: c.requestID, Kind: domain.TrailRailResponse, Source: "operator", Level: domain.TrailWarn,
			Method: http.MethodPost, Path: "M-Pesa c2bPayment/singleStage", DurationMs: took.Milliseconds(),
			Title: fmt.Sprintf("O M-Pesa não respondeu ao fim de %d s (%v). O cliente pode ter pago: a verificação periódica pergunta ao M-Pesa.", int(took.Seconds()), err),
			Data:  map[string]any{"reference": ref, "seconds": int(took.Seconds())}})
		return
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if err != nil {
		log.Printf("mpesa %s: answer cut short: %v", ref, err)
		m.answered(c, res.StatusCode, took, raw, mpesaResponse{}, "A resposta do M-Pesa chegou cortada: "+err.Error(), domain.TrailWarn)
		return
	}
	var out mpesaResponse
	if err := json.Unmarshal(raw, &out); err != nil || out.ResponseCode == "" {
		log.Printf("mpesa %s: answer not understood (http %d)", ref, res.StatusCode)
		m.answered(c, res.StatusCode, took, raw, out, fmt.Sprintf("Resposta do M-Pesa que não percebemos (HTTP %d)", res.StatusCode), domain.TrailWarn)
		return
	}
	o, known := mpesaOutcome(ref, res.StatusCode, out)
	secs := int(took.Seconds())
	switch {
	case known && o.Success:
		m.answered(c, res.StatusCode, took, raw, out, fmt.Sprintf("O M-Pesa confirmou o pagamento em %d s (%s, transação %s)", secs, out.ResponseCode, out.TransactionID), domain.TrailInfo)
	case known:
		m.answered(c, res.StatusCode, took, raw, out, fmt.Sprintf("O M-Pesa recusou ao fim de %d s: %s %s", secs, out.ResponseCode, out.ResponseDesc), domain.TrailWarn)
	case out.ResponseCode == "INS-9":
		m.answered(c, res.StatusCode, took, raw, out, fmt.Sprintf("O M-Pesa respondeu INS-9 (tempo esgotado) ao fim de %d s: o PIN não foi digitado a tempo, ou o pedido não chegou ao telemóvel. Perguntamos ao M-Pesa se ficou pago.", secs), domain.TrailWarn)
	default:
		m.answered(c, res.StatusCode, took, raw, out, fmt.Sprintf("Resposta do M-Pesa sem desfecho ao fim de %d s: %s %s. A verificação periódica pergunta ao M-Pesa.", secs, out.ResponseCode, out.ResponseDesc), domain.TrailWarn)
	}
	if known {
		onComplete(o)
		return
	}
	if out.ResponseCode == "INS-9" {
		m.afterTimeout(c, onComplete)
	}
}

// afterTimeout handles M-Pesa saying the customer didn't answer in time, which is
// common. It asks whether that holds, so they don't wait for the sweep to try again.
func (m *Mpesa) afterTimeout(c mpesaCharge, onComplete func(domain.Outcome)) {
	time.Sleep(mpesaTimeoutCheckDelay)
	o, known, err := m.Query(domain.WithRequest(context.Background(), domain.RequestInfo{ID: c.requestID}), domain.StatusQuery{Reference: c.ref, PaymentID: c.paymentID})
	switch {
	case err != nil:
		log.Printf("mpesa %s: query after timeout failed, the sweep will ask again: %v", c.ref, err)
	case known:
		onComplete(o)
	default:
		onComplete(domain.Outcome{FailureCode: domain.FailCustomerTimeout})
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
func (m *Mpesa) Query(ctx context.Context, sq domain.StatusQuery) (domain.Outcome, bool, error) {
	start := time.Now()
	o, known, status, out, err := m.query(ctx, sq.Reference)
	ev := domain.TrailEvent{PaymentID: sq.PaymentID, RequestID: domain.RequestFrom(ctx).ID, Kind: domain.TrailRailStatus, Source: "operator", Method: http.MethodGet,
		Path: "M-Pesa queryTransactionStatus", Status: status, DurationMs: time.Since(start).Milliseconds(),
		Data: map[string]any{"reference": sq.Reference, "code": out.ResponseCode, "status": out.TransactionStatus, "transaction": out.TransactionID}}
	switch {
	case err != nil:
		ev.Level, ev.Title = domain.TrailWarn, "Não conseguimos perguntar ao M-Pesa pelo estado: "+err.Error()
	case known && o.Success:
		ev.Title = "O M-Pesa diz que está pago (" + out.TransactionStatus + ")"
	case known:
		ev.Title = "O M-Pesa diz que não foi pago (" + out.TransactionStatus + ")"
	default:
		ev.Title = fmt.Sprintf("O M-Pesa não tem registo de pagamento (%s %s)", out.ResponseCode, domain.FirstNonEmpty(out.TransactionStatus, out.ResponseDesc))
	}
	if sq.PaymentID != "" {
		m.trace(ev)
	}
	return o, known, err
}

func (m *Mpesa) query(ctx context.Context, ref string) (domain.Outcome, bool, int, mpesaResponse, error) {
	var out mpesaResponse
	ctx, cancel := context.WithTimeout(ctx, mpesaQueryTimeout)
	defer cancel()
	q := url.Values{"input_ThirdPartyReference": {ref}, "input_QueryReference": {ref}, "input_ServiceProviderCode": {m.cfg.ServiceProviderCode}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.queryURL+"?"+q.Encode(), nil)
	if err != nil {
		return domain.Outcome{}, false, 0, out, err
	}
	if err := m.headers(req); err != nil {
		return domain.Outcome{}, false, 0, out, err
	}
	res, err := m.http.Do(req)
	if err != nil {
		return domain.Outcome{}, false, 0, out, err
	}
	defer res.Body.Close()
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<16)).Decode(&out); err != nil {
		return domain.Outcome{}, false, res.StatusCode, out, fmt.Errorf("query answer not understood (http %d)", res.StatusCode)
	}
	if out.ResponseCode == "" {
		// A refused request, such as a bad key, says nothing about the payment.
		return domain.Outcome{}, false, res.StatusCode, out, fmt.Errorf("query refused (http %d)", res.StatusCode)
	}
	switch strings.ToLower(out.TransactionStatus) {
	case "completed":
		return domain.Outcome{Success: true, OperatorReference: domain.FirstNonEmpty(out.TransactionID, ref)}, true, res.StatusCode, out, nil
	case "cancelled", "canceled":
		return domain.Outcome{FailureCode: domain.FailCustomerCancelled}, true, res.StatusCode, out, nil
	case "expired":
		return domain.Outcome{FailureCode: domain.FailCustomerTimeout}, true, res.StatusCode, out, nil
	}
	return domain.Outcome{}, false, res.StatusCode, out, nil
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

func (m Modes) pick(live bool) ports.Rail {
	if live {
		return m.Live
	}
	return m.Test
}

func (m Modes) Query(ctx context.Context, q domain.StatusQuery) (domain.Outcome, bool, error) {
	if r, ok := m.pick(q.Live).(ports.RailQuery); ok {
		return r.Query(ctx, q)
	}
	return domain.Outcome{}, false, ports.ErrNoStatusCheck
}

func (m Modes) NewReference(method string, live bool) string {
	if r, ok := m.pick(live).(ports.RailReferencer); ok {
		return r.NewReference(method, live)
	}
	return domain.RailReference()
}

func (m Modes) RouteFor(want, method string, live bool) string {
	if r, ok := m.pick(live).(ports.RailRouting); ok {
		return r.RouteFor(want, method, live)
	}
	return ""
}

func (m Modes) Routes(live bool) map[string][]string {
	if r, ok := m.pick(live).(ports.RailRouting); ok {
		return r.Routes(live)
	}
	return map[string][]string{}
}

func (m Modes) ReferenceVia(integration, method string, live bool) string {
	if r, ok := m.pick(live).(ports.RailRouting); ok {
		return r.ReferenceVia(integration, method, live)
	}
	return m.NewReference(method, live)
}
