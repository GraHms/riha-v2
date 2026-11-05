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
	"regexp"
	"strings"
	"sync"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type Config struct {
	BaseURL   string
	Token     string
	Channels  map[string]string
	PollEvery time.Duration
	PollFor   time.Duration
	// CreateFor is how long to wait for the create call, which mtoca only answers
	// when bim does. It must stay under the sweep's 5 minutes for a payment with no id.
	CreateFor time.Duration
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
		return nil, errors.New("mtoca: map at least one payment method to a channel, for example mpesa=MPESA")
	}
	if cfg.PollEvery <= 0 {
		cfg.PollEvery = 3 * time.Second
	}
	if cfg.PollFor <= 0 {
		cfg.PollFor = 4 * time.Minute
	}
	if cfg.CreateFor <= 0 {
		cfg.CreateFor = 4 * time.Minute
	}
	h := cfg.HTTP
	if h == nil {
		h = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &Client{cfg: cfg, http: h, seen: map[string]time.Time{}}, nil
}

func (c *Client) Handles(method string) bool { return c.cfg.Channels[method] != "" }

type createBody struct {
	OperationNumber string          `json:"operationNumber"`
	Channel         string          `json:"channel"`
	Amount          json.RawMessage `json:"amount"`
	PhoneNumber     string          `json:"phoneNumber"`
	Description     string          `json:"description"`
}

// statusBody is all the status call takes: it refuses any other field.
type statusBody struct {
	OperationNumber string `json:"operationNumber"`
	TransactionID   string `json:"transactionID_NEP"`
}

func amount(c int64) json.RawMessage { return json.RawMessage(fmt.Sprintf("%d.%02d", c/100, c%100)) }

func description(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		s = "Pagamento RIHA"
	}
	if r := []rune(s); len(r) > 100 {
		s = string(r[:100])
	}
	return s
}

var msisdn = regexp.MustCompile(`\+?258\d{9}`)

type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("mtoca: http %d: %s", e.Status, truncate(e.Body, 300))
}

func (c *Client) post(ctx context.Context, path, op string, b any) (map[string]any, []byte, error) {
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
		log.Printf("mtoca %s op=%s http=%d body=%s", path, op, res.StatusCode, msisdn.ReplaceAllString(string(raw), "***"))
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, raw, &HTTPError{Status: res.StatusCode, Body: string(raw)}
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

// rejectionFailures maps mtoca's rejectionReason to what we tell the customer. A
// reason not listed here reads as a plain rejection.
var rejectionFailures = map[string]string{
	"01": domain.FailCustomerTimeout,
	"02": domain.FailRejected,
}

func Interpret(m map[string]any) (domain.Outcome, bool) {
	ref := field(m, "transactionID_NEP", "vendorTransactionId_NEP")
	switch strings.ToUpper(field(m, "paymentStatus")) {
	case "P":
		return domain.Outcome{Success: true, OperatorReference: truncate(ref, 40)}, true
	case "R":
		if code := rejectionFailures[field(m, "rejectionReason")]; code != "" {
			return domain.Outcome{FailureCode: code}, true
		}
		return domain.Outcome{FailureCode: domain.FailRejected}, true
	case "C":
		return domain.Outcome{FailureCode: domain.FailCanceled}, true
	}
	return domain.Outcome{}, false
}

// Initiate returns at once and calls mtoca in the background, because mtoca only
// answers once bim does, and the checkout has to tell the customer to look at their phone.
func (c *Client) Initiate(_ context.Context, req domain.ChargeRequest, onComplete func(domain.Outcome)) (domain.ChargeResult, error) {
	channel := c.cfg.Channels[req.Method]
	if channel == "" {
		return domain.ChargeResult{}, &domain.InputError{Field: "method", Msg: "Este método de pagamento não está disponível."}
	}
	if e := domain.WalletNumberError(req.Method, req.Phone); e != nil {
		return domain.ChargeResult{}, e
	}
	if req.Reference == "" {
		return domain.ChargeResult{}, errors.New("mtoca: missing operation number")
	}
	phone := domain.NormalizeMZPhone(req.Phone)
	b := createBody{OperationNumber: req.Reference, Channel: channel, Amount: amount(req.Amount), PhoneNumber: "258" + phone, Description: description(domain.FirstNonEmpty(req.Description, req.Descriptor))}
	go c.create(b, req, onComplete)
	return domain.ChargeResult{Async: true, Last4: phone[5:]}, nil
}

func (c *Client) create(b createBody, req domain.ChargeRequest, onComplete func(domain.Outcome)) {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.CreateFor)
	defer cancel()
	m, _, err := c.post(ctx, "/api/payments", b.OperationNumber, b)
	var he *HTTPError
	switch {
	case errors.As(err, &he) && he.Status < 500:
		code := failureFor(he)
		log.Printf("mtoca create op=%s refused, read as %s: %v", b.OperationNumber, code, err)
		if code == domain.FailOperatorUnavailable {
			log.Printf("mtoca create op=%s may have reached the operator, CHECK BY HAND whether the customer paid", b.OperationNumber)
		}
		onComplete(domain.Outcome{FailureCode: code})
		return
	case err != nil:
		// With no id the status call can't be asked, so the sweep closes this one.
		log.Printf("mtoca create op=%s lost its answer, CHECK BY HAND whether the customer paid: %v", b.OperationNumber, err)
		return
	}
	id := field(m, "vendorTransactionId_NEP", "transactionID_NEP")
	if id == "" {
		log.Printf("mtoca create op=%s answered with no transaction id, CHECK BY HAND whether the customer paid", b.OperationNumber)
		return
	}
	if req.OnRailID != nil {
		req.OnRailID(id)
	}
	c.poll(domain.StatusQuery{PaymentID: req.PaymentID, Reference: b.OperationNumber, RailID: id, Method: req.Method}, onComplete)
}

// failureFor reads a create call that mtoca refused. mtoca answers 409 to bim's
// refusals and to its own lost connections alike, and only the message tells them apart.
func failureFor(he *HTTPError) string {
	switch he.Status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return domain.FailRejected
	case http.StatusConflict:
		var body struct {
			Message string `json:"message"`
		}
		if json.Unmarshal([]byte(he.Body), &body) != nil || lost.MatchString(body.Message) {
			return domain.FailOperatorUnavailable
		}
		return domain.FailRejected
	}
	return domain.FailOperatorUnavailable
}

var lost = regexp.MustCompile(`(?i)\bEOF\b|timeout|deadline|connection|no such host|invalid character|\btls\b|broken pipe`)

func (c *Client) poll(q domain.StatusQuery, onComplete func(domain.Outcome)) {
	deadline := time.Now().Add(c.cfg.PollFor)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		out, final, err := c.query(ctx, q)
		cancel()
		switch {
		case err != nil:
			log.Printf("mtoca status op=%s: %v", q.Reference, err)
		case final:
			onComplete(out)
			return
		}
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(c.cfg.PollEvery)
	}
}

func (c *Client) query(ctx context.Context, q domain.StatusQuery) (domain.Outcome, bool, error) {
	m, _, err := c.post(ctx, "/api/payments/status", q.Reference, statusBody{OperationNumber: q.Reference, TransactionID: q.RailID})
	if err != nil {
		return domain.Outcome{}, false, err
	}
	// An empty status can mean bim doesn't know the payment or that its answer was
	// unreadable, because mtoca reports both the same way. So it stays pending.
	out, final := Interpret(m)
	if final && out.Success && out.OperatorReference == "" {
		out.OperatorReference = q.RailID
	}
	return out, final, nil
}

// NewReference: mtoca takes a 12 digit operation number.
func (c *Client) NewReference(string, bool) string { return domain.OperationNumber() }

// Query answers the sweep. Pending is an error, not "no record": mtoca knows the
// payment, so the customer can still confirm it. With no id nothing can be asked,
// because the create call is still waiting or its answer is gone.
func (c *Client) Query(ctx context.Context, q domain.StatusQuery) (domain.Outcome, bool, error) {
	if q.RailID == "" {
		return domain.Outcome{}, false, nil
	}
	c.mu.Lock()
	if t, ok := c.seen[q.Reference]; ok && time.Since(t) < 30*time.Second {
		c.mu.Unlock()
		return domain.Outcome{}, false, ports.ErrStillPending
	}
	c.seen[q.Reference] = time.Now()
	if len(c.seen) > 10000 {
		c.seen = map[string]time.Time{q.Reference: time.Now()}
	}
	c.mu.Unlock()
	out, final, err := c.query(ctx, q)
	if err == nil && !final {
		return out, false, ports.ErrStillPending
	}
	return out, final, err
}
