package mtoca

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"riha/internal/core/domain"
)

type Config struct {
	BaseURL   string
	Token     string
	Channels  map[string]string
	PollEvery time.Duration
	PollFor   time.Duration
	Debug     bool
	HTTP      *http.Client
}

type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	seen map[string]time.Time
}

func New(cfg Config) (*Client, error) {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.BaseURL == "" {
		return nil, errors.New("mtoca: base url is required")
	}
	if len(cfg.Channels) == 0 {
		return nil, errors.New("mtoca: map at least one payment method to a channel, for example mpesa=BIM")
	}
	if cfg.PollEvery <= 0 {
		cfg.PollEvery = 3 * time.Second
	}
	if cfg.PollFor <= 0 {
		cfg.PollFor = 4 * time.Minute
	}
	h := cfg.HTTP
	if h == nil {
		h = &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &Client{cfg: cfg, http: h, seen: map[string]time.Time{}}, nil
}

func (c *Client) Handles(method string) bool { return c.cfg.Channels[method] != "" }

type body struct {
	OperationNumber string          `json:"operationNumber"`
	Channel         string          `json:"channel"`
	Amount          json.RawMessage `json:"amount"`
	PhoneNumber     string          `json:"phoneNumber"`
	Description     string          `json:"description"`
}

func amount(c int64) json.RawMessage { return json.RawMessage(fmt.Sprintf("%d.%02d", c/100, c%100)) }

func description(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		s = "Pagamento Riha"
	}
	if r := []rune(s); len(r) > 100 {
		s = string(r[:100])
	}
	return s
}

type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("mtoca: http %d: %s", e.Status, e.Body) }

func (c *Client) post(ctx context.Context, path string, b body) (map[string]any, []byte, error) {
	buf, _ := json.Marshal(b)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+path, bytes.NewReader(buf))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if c.cfg.Debug {
		log.Printf("mtoca %s op=%s http=%d body=%s", path, b.OperationNumber, res.StatusCode, strings.ReplaceAll(string(raw), b.PhoneNumber, "***"))
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, raw, &HTTPError{Status: res.StatusCode, Body: truncate(string(raw), 300)}
	}
	var m map[string]any
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, raw, fmt.Errorf("mtoca: response is not json: %s", truncate(string(raw), 200))
		}
	}
	return m, raw, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func field(m map[string]any, names ...string) string {
	for _, scope := range []map[string]any{m, sub(m, "data"), sub(m, "result"), sub(m, "payment")} {
		for k, v := range scope {
			for _, n := range names {
				if strings.EqualFold(k, n) {
					switch t := v.(type) {
					case string:
						return strings.TrimSpace(t)
					case float64:
						return fmt.Sprintf("%v", t)
					}
				}
			}
		}
	}
	return ""
}

func sub(m map[string]any, k string) map[string]any {
	for key, v := range m {
		if strings.EqualFold(key, k) {
			if mm, ok := v.(map[string]any); ok {
				return mm
			}
		}
	}
	return nil
}

func Interpret(m map[string]any) (domain.Outcome, bool) {
	ref := field(m, "transactionId", "transactionReference", "reference", "externalReference", "operationId")
	switch strings.ToUpper(field(m, "paymentStatus")) {
	case "P":
		return domain.Outcome{Success: true, OperatorReference: truncate(ref, 40)}, true
	case "R":
		if field(m, "rejectionReason") == "01" {
			return domain.Outcome{FailureCode: domain.FailCustomerTimeout}, true
		}
		return domain.Outcome{FailureCode: domain.FailRejected}, true
	case "C":
		return domain.Outcome{FailureCode: domain.FailCanceled}, true
	}
	return domain.Outcome{}, false
}

func (c *Client) Initiate(ctx context.Context, req domain.ChargeRequest, onComplete func(domain.Outcome)) (domain.ChargeResult, error) {
	channel := c.cfg.Channels[req.Method]
	if channel == "" {
		return domain.ChargeResult{}, &domain.InputError{Field: "method", Msg: "Este método de pagamento não está disponível."}
	}
	phone := domain.NormalizeMZPhone(req.Phone)
	if !domain.ValidMZMobile(phone) {
		return domain.ChargeResult{}, &domain.InputError{Field: "phone", Msg: "Número inválido. Use 9 dígitos, por exemplo 84 123 4567."}
	}
	if req.Reference == "" {
		return domain.ChargeResult{}, errors.New("mtoca: missing operation number")
	}
	b := body{OperationNumber: req.Reference, Channel: channel, Amount: amount(req.Amount), PhoneNumber: "+258" + phone, Description: description(domain.FirstNonEmpty(req.Description, req.Descriptor))}
	res := domain.ChargeResult{Async: true, Last4: phone[5:]}
	m, _, err := c.post(ctx, "/api/payments", b)
	var he *HTTPError
	switch {
	case errors.As(err, &he):
		log.Printf("mtoca create op=%s refused: %v", req.Reference, err)
		return domain.ChargeResult{Outcome: domain.Outcome{FailureCode: failureFor(he.Status)}, Last4: phone[5:]}, nil
	case err != nil:
		log.Printf("mtoca create op=%s uncertain, will poll status: %v", req.Reference, err)
	default:
		if out, final := Interpret(m); final {
			return domain.ChargeResult{Outcome: out, Last4: phone[5:]}, nil
		}
	}
	q := domain.StatusQuery{PaymentID: req.PaymentID, Reference: req.Reference, Method: req.Method, Amount: req.Amount, Phone: phone, Description: domain.FirstNonEmpty(req.Description, req.Descriptor)}
	go c.poll(q, onComplete)
	return res, nil
}

func failureFor(status int) string {
	if status >= 500 || status == http.StatusTooManyRequests {
		return domain.FailOperatorUnavailable
	}
	return domain.FailRejected
}

func (c *Client) poll(q domain.StatusQuery, onComplete func(domain.Outcome)) {
	deadline := time.Now().Add(c.cfg.PollFor)
	for time.Now().Before(deadline) {
		time.Sleep(c.cfg.PollEvery)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		out, final, err := c.query(ctx, q)
		cancel()
		if err != nil {
			log.Printf("mtoca status op=%s: %v", q.Reference, err)
			continue
		}
		if final {
			onComplete(out)
			return
		}
	}
}

func (c *Client) query(ctx context.Context, q domain.StatusQuery) (domain.Outcome, bool, error) {
	channel := c.cfg.Channels[q.Method]
	phone := domain.NormalizeMZPhone(q.Phone)
	if channel == "" || len(phone) != 9 {
		return domain.Outcome{}, false, errors.New("mtoca: payment has no channel or phone for a status check")
	}
	m, _, err := c.post(ctx, "/api/payments/status", body{OperationNumber: q.Reference, Channel: channel, Amount: amount(q.Amount), PhoneNumber: "258" + phone, Description: description(q.Description)})
	if err != nil {
		return domain.Outcome{}, false, err
	}
	out, final := Interpret(m)
	if final && out.OperatorReference == "" && out.Success {
		out.OperatorReference = q.Reference
	}
	return out, final, nil
}

func (c *Client) Status(ctx context.Context, q domain.StatusQuery) (domain.Outcome, bool, error) {
	c.mu.Lock()
	if t, ok := c.seen[q.Reference]; ok && time.Since(t) < 30*time.Second {
		c.mu.Unlock()
		return domain.Outcome{}, false, nil
	}
	c.seen[q.Reference] = time.Now()
	if len(c.seen) > 10000 {
		c.seen = map[string]time.Time{q.Reference: time.Now()}
	}
	c.mu.Unlock()
	return c.query(ctx, q)
}
