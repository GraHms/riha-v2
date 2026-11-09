package messaging

import (
	"context"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// Router sends live SMS through the SMS provider and keeps everything else in the
// outbox: test-mode messages, so a test never texts a real number, and email.
// WhatsApp goes by SMS until a provider carries it, so the customer still gets it.
type Router struct {
	SMS    ports.Messenger
	Outbox ports.Messenger
}

func (r Router) Send(ctx context.Context, m ports.OutboundMessage) (ports.SentMessage, error) {
	if m.Live && r.SMS != nil && (m.Channel == domain.ChannelSMS || m.Channel == domain.ChannelWhatsApp) {
		m.Channel = domain.ChannelSMS
		return r.SMS.Send(ctx, m)
	}
	return r.Outbox.Send(ctx, m)
}

func (r Router) Health() (ports.MessengerHealth, bool) {
	if h, ok := r.SMS.(ports.HealthReporter); ok {
		return h.Health()
	}
	return ports.MessengerHealth{}, false
}
