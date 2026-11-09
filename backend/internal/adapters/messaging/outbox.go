package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"riha/internal/core/ports"
)

var ErrUndeliverable = errors.New("número sem entrega")

type Outbox struct {
	Dir string
	mu  sync.Mutex
}

// Outbox writes messages to a file instead of sending them: for development, test
// mode, and the channels no provider carries yet.
func (o *Outbox) Send(_ context.Context, m ports.OutboundMessage) (ports.SentMessage, error) {
	if strings.HasSuffix(m.To, "9999") {
		return ports.SentMessage{}, ErrUndeliverable
	}
	if o.Dir == "" {
		return ports.SentMessage{Channel: m.Channel}, nil
	}
	return ports.SentMessage{Channel: m.Channel}, o.write(m)
}

func (o *Outbox) write(m ports.OutboundMessage) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if err := os.MkdirAll(o.Dir, 0o750); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(o.Dir, "messages.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer f.Close()
	line, _ := json.Marshal(map[string]string{"at": time.Now().UTC().Format(time.RFC3339), "channel": m.Channel, "to": m.To, "from": m.Sender, "body": m.Body})
	_, err = f.Write(append(line, '\n'))
	return err
}
