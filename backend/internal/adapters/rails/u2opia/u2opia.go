// Package u2opia charges customers abroad, by card or with their own
// country's wallet, through U2opia Pay. The customer finishes on U2opia's page
// and U2opia says how it ended in a callback: its API has no status call, so
// nothing else can end the payment.
package u2opia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// Country is one place customers can pay from: its currency, and how much of
// it one metical is worth, since U2opia charges in the customer's currency.
type Country struct {
	Code     string // ISO 3166 alpha-2, as U2opia takes it
	Name     string
	Dial     string // the calling code, without +
	Currency string
	PerMZN   float64
}

type Config struct {
	BaseURL     string // up to and including /api-manager
	PartnerCode string
	Countries   map[string]Country
	HTTP        *http.Client
	Trace       func(domain.TrailEvent)
	Now         func() time.Time
}

type Client struct {
	cfg    Config
	mu     sync.Mutex
	tokens map[string]token
}

// A token is per country: U2opia issues it for one.
type token struct {
	access, refresh  string
	until, refreshBy time.Time
}

func New(cfg Config) (*Client, error) {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if !strings.HasPrefix(cfg.BaseURL, "https://") && !strings.HasPrefix(cfg.BaseURL, "http://127.0.0.1") && !strings.HasPrefix(cfg.BaseURL, "http://localhost") {
		return nil, errors.New("u2opia: the base URL must be https")
	}
	if cfg.PartnerCode == "" {
		return nil, errors.New("u2opia: no partner code")
	}
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Client{cfg: cfg, tokens: map[string]token{}}, nil
}

// Countries lists where customers can pay from, for the checkout.
func (c *Client) Countries() []Country {
	out := make([]Country, 0, len(c.cfg.Countries))
	for _, k := range sortedKeys(c.cfg.Countries) {
		out = append(out, c.cfg.Countries[k])
	}
	return out
}

// APIError is what U2opia answered when it refused a call.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("U2opia respondeu %d %s: %s", e.Status, e.Code, e.Message)
}

func (c *Client) call(ctx context.Context, path, country, bearer string, body any, out any) ([]byte, error) {
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+path, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
		req.Header.Set("x-partner-code", c.cfg.PartnerCode)
		req.Header.Set("x-country-id", country)
	}
	res, err := c.cfg.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var e struct {
		ResponseCode    string `json:"responseCode"`
		ResponseMessage string `json:"responseMessage"`
	}
	_ = json.Unmarshal(raw, &e)
	// U2opia also puts its own code in the body, sometimes over a 200.
	if res.StatusCode >= 300 || (e.ResponseCode != "" && e.ResponseCode != "200") {
		return raw, &APIError{Status: res.StatusCode, Code: e.ResponseCode, Message: e.ResponseMessage}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return raw, fmt.Errorf("U2opia answered something unreadable: %w", err)
		}
	}
	return raw, nil
}

type tokenAnswer struct {
	Success bool `json:"success"`
	Data    struct {
		Token                 string `json:"token"`
		RefreshToken          string `json:"refreshToken"`
		ExpiresIn             int    `json:"expiresIn"`
		RefreshTokenExpiresIn int    `json:"refreshTokenExpiresIn"`
	} `json:"data"`
}

// token returns a bearer token for a country, refreshing it before it runs
// out, and asking for a new one when the refresh can't be used.
func (c *Client) token(ctx context.Context, country string) (string, error) {
	c.mu.Lock()
	t, ok := c.tokens[country]
	c.mu.Unlock()
	now := c.cfg.Now()
	if ok && now.Before(t.until.Add(-time.Minute)) {
		return t.access, nil
	}
	var a tokenAnswer
	var err error
	if ok && t.refresh != "" && now.Before(t.refreshBy) {
		_, err = c.call(ctx, "/api/auth/refresh", "", "", map[string]string{"refreshToken": t.refresh}, &a)
	}
	if !ok || err != nil || a.Data.Token == "" {
		a = tokenAnswer{}
		if _, err = c.call(ctx, "/api/auth/token", "", "", map[string]string{"partnerCode": c.cfg.PartnerCode, "countryId": country}, &a); err != nil {
			return "", err
		}
	}
	if !a.Success || a.Data.Token == "" {
		return "", errors.New("U2opia didn't give a token")
	}
	t = token{access: a.Data.Token, refresh: a.Data.RefreshToken,
		until: now.Add(time.Duration(a.Data.ExpiresIn) * time.Second), refreshBy: now.Add(time.Duration(a.Data.RefreshTokenExpiresIn) * time.Second)}
	c.mu.Lock()
	c.tokens[country] = t
	c.mu.Unlock()
	return t.access, nil
}

func (c *Client) forget(country string) {
	c.mu.Lock()
	delete(c.tokens, country)
	c.mu.Unlock()
}

// LocalAmount is what a charge of cents meticais costs in a country's currency.
func LocalAmount(cents int64, co Country) string {
	return fmt.Sprintf("%.2f", math.Round(float64(cents)/100*co.PerMZN*100)/100)
}

// NationalNumber turns what the customer typed into the number without the
// calling code or a leading zero, as U2opia's example sends it.
func NationalNumber(phone string, co Country) string {
	d := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	d = strings.TrimPrefix(d, "00")
	if strings.HasPrefix(d, co.Dial) && len(d)-len(co.Dial) >= 7 {
		d = d[len(co.Dial):]
	}
	return strings.TrimLeft(d, "0")
}

type initiateAnswer struct {
	TransactionID  string `json:"transactionId"`
	ExpiresIn      int    `json:"expiresIn"`
	RedirectionURL string `json:"redirectionUrl"`
}

func (c *Client) Initiate(ctx context.Context, req domain.ChargeRequest, _ func(domain.Outcome)) (domain.ChargeResult, error) {
	co, ok := c.cfg.Countries[strings.ToUpper(req.Country)]
	if !ok {
		return domain.ChargeResult{}, &domain.InputError{Field: "country", Msg: "Ainda não aceitamos pagamentos deste país."}
	}
	msisdn := NationalNumber(req.Phone, co)
	if len(msisdn) < 6 || len(msisdn) > 12 {
		return domain.ChargeResult{}, &domain.InputError{Field: "phone", Msg: "Número inválido para " + co.Name + "."}
	}
	body := map[string]string{"msisdn": msisdn, "amount": LocalAmount(req.Amount, co), "chargingType": "Wallet"}
	var out initiateAnswer
	raw, err := c.charge(ctx, co.Code, body, &out)
	ev := domain.TrailEvent{PaymentID: req.PaymentID, RequestID: domain.RequestFrom(ctx).ID, Kind: domain.TrailRailRequest, Source: "operator", Method: "POST",
		Path: "U2opia /v1/initate-subscription", Request: domain.RedactBody(mustJSON(body)), Response: domain.RedactBody(raw),
		Data: map[string]any{"country": co.Code, "currency": co.Currency, "amount": body["amount"]}}
	if err != nil {
		ev.Level, ev.Title = domain.TrailWarn, "U2opia recusou o pedido: "+err.Error()
		c.trace(ev)
		var ae *APIError
		if errors.As(err, &ae) && ae.Status >= 400 && ae.Status < 500 && ae.Status != 401 {
			return domain.ChargeResult{}, &domain.InputError{Field: "phone", Msg: "O operador recusou o pedido: " + domain.FirstNonEmpty(ae.Message, "verifique o número e o país.")}
		}
		return domain.ChargeResult{}, err
	}
	if out.TransactionID == "" || !c.safePage(out.RedirectionURL) {
		ev.Level, ev.Title = domain.TrailWarn, "U2opia não devolveu a página de pagamento"
		c.trace(ev)
		return domain.ChargeResult{}, errors.New("U2opia didn't return its payment page")
	}
	ev.Title = fmt.Sprintf("Pedido aceite pela U2opia: %s %s, transação %s", body["amount"], co.Currency, out.TransactionID)
	c.trace(ev)
	if req.OnRailID != nil {
		req.OnRailID(out.TransactionID)
	}
	return domain.ChargeResult{Async: true, RedirectURL: out.RedirectionURL}, nil
}

// safePage is a payment page the customer can be sent to: https, or a local
// page when U2opia itself is a local stand-in, for development.
func (c *Client) safePage(u string) bool {
	if strings.HasPrefix(u, "https://") {
		return true
	}
	local := strings.HasPrefix(c.cfg.BaseURL, "http://127.0.0.1") || strings.HasPrefix(c.cfg.BaseURL, "http://localhost")
	return local && (strings.HasPrefix(u, "http://127.0.0.1") || strings.HasPrefix(u, "http://localhost"))
}

// charge starts the payment, once more with a fresh token if U2opia turned
// the cached one down.
func (c *Client) charge(ctx context.Context, country string, body map[string]string, out *initiateAnswer) ([]byte, error) {
	for try := 0; ; try++ {
		tok, err := c.token(ctx, country)
		if err != nil {
			return nil, err
		}
		raw, err := c.call(ctx, "/v1/initate-subscription", country, tok, body, out)
		var ae *APIError
		if try == 0 && errors.As(err, &ae) && ae.Status == http.StatusUnauthorized {
			c.forget(country)
			continue
		}
		return raw, err
	}
}

// Query can't ask: U2opia has no status call. The payment stays open until
// its callback comes, and is left unconfirmed if it doesn't.
func (c *Client) Query(context.Context, domain.StatusQuery) (domain.Outcome, bool, error) {
	return domain.Outcome{}, false, fmt.Errorf("%w: U2opia only answers by callback", ports.ErrStillPending)
}

func (c *Client) trace(ev domain.TrailEvent) {
	if c.cfg.Trace != nil {
		c.cfg.Trace(ev)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func sortedKeys(m map[string]Country) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && m[out[j]].Name < m[out[j-1]].Name; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Known are the countries U2opia maps for RIHA, with their names and calling
// codes. Which are offered, and at what rate, comes from the configuration.
var Known = map[string]Country{
	"AE": {Code: "AE", Name: "Emirados Árabes Unidos", Dial: "971", Currency: "AED"},
	"BH": {Code: "BH", Name: "Barém", Dial: "973", Currency: "BHD"},
	"CI": {Code: "CI", Name: "Costa do Marfim", Dial: "225", Currency: "XOF"},
	"CM": {Code: "CM", Name: "Camarões", Dial: "237", Currency: "XAF"},
	"EG": {Code: "EG", Name: "Egito", Dial: "20", Currency: "EGP"},
	"GH": {Code: "GH", Name: "Gana", Dial: "233", Currency: "GHS"},
	"IN": {Code: "IN", Name: "Índia", Dial: "91", Currency: "INR"},
	"KE": {Code: "KE", Name: "Quénia", Dial: "254", Currency: "KES"},
	"KW": {Code: "KW", Name: "Kuwait", Dial: "965", Currency: "KWD"},
	"NA": {Code: "NA", Name: "Namíbia", Dial: "264", Currency: "NAD"},
	"NG": {Code: "NG", Name: "Nigéria", Dial: "234", Currency: "NGN"},
	"OM": {Code: "OM", Name: "Omã", Dial: "968", Currency: "OMR"},
	"QA": {Code: "QA", Name: "Catar", Dial: "974", Currency: "QAR"},
	"RW": {Code: "RW", Name: "Ruanda", Dial: "250", Currency: "RWF"},
	"SA": {Code: "SA", Name: "Arábia Saudita", Dial: "966", Currency: "SAR"},
	"SN": {Code: "SN", Name: "Senegal", Dial: "221", Currency: "XOF"},
	"TZ": {Code: "TZ", Name: "Tanzânia", Dial: "255", Currency: "TZS"},
	"UG": {Code: "UG", Name: "Uganda", Dial: "256", Currency: "UGX"},
	"ZA": {Code: "ZA", Name: "África do Sul", Dial: "27", Currency: "ZAR"},
}
