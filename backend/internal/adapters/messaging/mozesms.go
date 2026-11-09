package messaging

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
	"riha/internal/core/ports"
)

// MozeSMS sends SMS through api.mozesms.com. A message it refuses for itself (a bad
// number) fails; a refusal about the account (no credit, a bad key), a busy or down
// service, or no answer leaves the message to be tried again later.
type MozeSMS struct {
	cfg  MozeSMSConfig
	http *http.Client

	mu     sync.Mutex
	health ports.MessengerHealth
}

type MozeSMSConfig struct {
	BaseURL   string
	APIKey    string
	APISecret string
	// Sender is the sender id used when a merchant's own isn't approved; empty
	// leaves MozeSMS's default.
	Sender string
	// Senders are the sender ids approved at MozeSMS, which merchants may use.
	Senders []string
	// LowBalance is the credit, in meticais, under which the alerts warn.
	LowBalance float64
	// Accents keeps letters such as ã and ç. Off, they become plain letters, so
	// an SMS holds 160 characters and costs one part instead of two.
	Accents bool
	Timeout time.Duration
}

func NewMozeSMS(cfg MozeSMSConfig) (*MozeSMS, error) {
	if cfg.APIKey == "" || cfg.APISecret == "" {
		return nil, errors.New("MozeSMS needs an API key and secret")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.mozesms.com"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 20 * time.Second
	}
	return &MozeSMS{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout},
		health: ports.MessengerHealth{Provider: "MozeSMS", Balance: -1, LowBalance: cfg.LowBalance}}, nil
}

type mozeReply struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Message string `json:"message"`
	Data    struct {
		ID               string   `json:"id"`
		Status           string   `json:"status"`
		Parts            int      `json:"parts"`
		Cost             float64  `json:"cost"`
		RemainingBalance *float64 `json:"remaining_balance"`
	} `json:"data"`
}

func (c *MozeSMS) Send(ctx context.Context, m ports.OutboundMessage) (ports.SentMessage, error) {
	phone := domain.NormalizeMZPhone(m.To)
	if len(phone) != 9 || phone[0] != '8' {
		return ports.SentMessage{}, errors.New("número inválido")
	}
	body := m.Body
	if !c.cfg.Accents {
		body = PlainSMS(body)
	}
	in := map[string]string{"phone": "258" + phone, "message": body}
	if s := c.sender(m.Sender); s != "" {
		in["sender_id"] = s
	}
	raw, _ := json.Marshal(in)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.BaseURL, "/")+"/sms/send", bytes.NewReader(raw))
	if err != nil {
		return ports.SentMessage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.cfg.APIKey)
	req.Header.Set("X-API-Secret", c.cfg.APISecret)
	res, err := c.http.Do(req)
	if err != nil {
		log.Printf("mozesms: to …%s: no answer: %v", phone[5:], err)
		return ports.SentMessage{}, fmt.Errorf("MozeSMS sem resposta: %w", ports.ErrRetryLater)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var out mozeReply
	_ = json.Unmarshal(b, &out)
	why := domain.Trim(domain.FirstNonEmpty(out.Error, out.Message, http.StatusText(res.StatusCode)), 150)
	switch {
	case res.StatusCode/100 == 2 && out.Success && out.Data.ID != "":
		c.took(out)
		log.Printf("mozesms: to …%s: %s, %d part(s), %.2f MT", phone[5:], out.Data.Status, out.Data.Parts, out.Data.Cost)
		return ports.SentMessage{ProviderID: out.Data.ID, Channel: domain.ChannelSMS}, nil
	case res.StatusCode == 401 || res.StatusCode == 402 || res.StatusCode == 403:
		account := "a MozeSMS recusou a conta (" + why + ")"
		if res.StatusCode == 402 {
			account = "sem crédito na MozeSMS"
		}
		c.accountError(account)
		log.Printf("mozesms: to …%s: %d %s", phone[5:], res.StatusCode, why)
		return ports.SentMessage{}, fmt.Errorf("%s: %w", account, ports.ErrRetryLater)
	case res.StatusCode == 429 || res.StatusCode >= 500:
		log.Printf("mozesms: to …%s: %d %s, will try again", phone[5:], res.StatusCode, why)
		return ports.SentMessage{}, fmt.Errorf("MozeSMS ocupada (%d): %w", res.StatusCode, ports.ErrRetryLater)
	default:
		log.Printf("mozesms: to …%s: refused %d %s", phone[5:], res.StatusCode, why)
		return ports.SentMessage{}, fmt.Errorf("MozeSMS recusou: %s", why)
	}
}

// sender is the merchant's sender id when MozeSMS approved it, else RIHA's.
func (c *MozeSMS) sender(want string) string {
	for _, s := range c.cfg.Senders {
		if want != "" && strings.EqualFold(s, want) {
			return s
		}
	}
	return c.cfg.Sender
}

func (c *MozeSMS) took(out mozeReply) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.health.AccountError, c.health.AccountErrorAt = "", time.Time{}
	if out.Data.RemainingBalance != nil {
		c.health.Balance, c.health.BalanceAt = *out.Data.RemainingBalance, time.Now()
	}
}

func (c *MozeSMS) accountError(why string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.health.AccountError, c.health.AccountErrorAt = why, time.Now()
}

func (c *MozeSMS) Health() (ports.MessengerHealth, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.health, true
}

// MozeSMSReport is a delivery report from MozeSMS's webhook.
type MozeSMSReport struct {
	Event string `json:"event"`
	Data  struct {
		MessageID     string `json:"message_id"`
		Status        string `json:"status"`
		GatewayStatus string `json:"gateway_status"`
		ErrorCode     string `json:"error_code"`
		DeliveryTime  string `json:"delivery_time"`
	} `json:"data"`
}

// Outcome is what a report means for the message: delivered, failed, or nothing
// final yet (accepted, pending, unknown).
func (r MozeSMSReport) Outcome() (status, why string) {
	g := strings.ToUpper(strings.TrimSpace(r.Data.GatewayStatus))
	ev := strings.TrimPrefix(strings.ToLower(r.Event), "sms.")
	switch {
	case g == "DELIVERED" || g == "" && ev == "delivered":
		return domain.MessageDelivered, ""
	case g == "UNDELIVERED" || g == "FAILED" || g == "EXPIRED" || g == "UNDELIVERABLE" || g == "REJECTED" || g == "DELETED",
		g == "" && (ev == "failed" || ev == "rejected" || ev == "expired" || ev == "undelivered" || ev == "undeliverable" || ev == "cancelled"):
		why := map[string]string{"EXPIRED": "expirou sem chegar ao telefone", "REJECTED": "rejeitada pela operadora", "DELETED": "cancelada",
			"UNDELIVERABLE": "número sem entrega"}[g]
		if why == "" {
			why = "não chegou ao telefone"
		}
		if r.Data.ErrorCode != "" {
			why += " (código " + domain.Trim(r.Data.ErrorCode, 20) + ")"
		}
		return domain.MessageFailed, why
	}
	return "", ""
}

// gsm is the GSM 7-bit alphabet, with its extension, which an SMS carries at 160
// characters a part. One character outside it makes the whole SMS Unicode: 70.
const gsm = "@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞÆæßÉ !\"#¤%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà^{}\\[~]|€\f"

var plain = map[rune]string{
	'á': "a", 'â': "a", 'ã': "a", 'ç': "c", 'ê': "e", 'í': "i", 'ó': "o", 'ô': "o", 'õ': "o", 'ú': "u", 'ï': "i", 'ë': "e",
	'Á': "A", 'Â': "A", 'Ã': "A", 'À': "A", 'Ê': "E", 'È': "E", 'Í': "I", 'Ì': "I", 'Ó': "O", 'Ô': "O", 'Õ': "O", 'Ò': "O", 'Ú': "U", 'Ù': "U",
	'‘': "'", '’': "'", '“': "\"", '”': "\"", '–': "-", '—': "-", '…': "...", '·': "-", '•': "-", ' ': " ", 'º': "o", 'ª': "a", '\t': " ",
}

// PlainSMS turns Portuguese letters outside the GSM alphabet into plain ones (não
// becomes nao), so the SMS stays GSM. If something can't be turned, such as an
// emoji, the text is left as it is: better a dearer SMS than a wrong one.
func PlainSMS(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case strings.ContainsRune(gsm, r):
			b.WriteRune(r)
		case plain[r] != "":
			b.WriteString(plain[r])
		default:
			return s
		}
	}
	return b.String()
}
