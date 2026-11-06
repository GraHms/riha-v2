package postgres

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"riha/internal/adapters/netinfo"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type AnyMap map[string]any

func (m AnyMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(map[string]any(m))
	return string(b), err
}

func (m *AnyMap) Scan(v any) error {
	out := map[string]any{}
	if err := scanJSON(v, &out); err != nil {
		return err
	}
	*m = out
	return nil
}

type trailRecord struct {
	ID         string `gorm:"primaryKey;size:40"`
	MerchantID string `gorm:"size:40;index:trail_merchant,priority:1"`
	Livemode   bool
	SessionID  string `gorm:"size:60;index"`
	PaymentID  string `gorm:"size:40;index"`
	RequestID  string `gorm:"size:40;index"`
	Kind       string `gorm:"size:40;index"`
	Source     string `gorm:"size:20"`
	Level      string `gorm:"size:10"`
	Title      string `gorm:"size:300"`
	Method     string `gorm:"size:10"`
	Path       string `gorm:"size:300"`
	Status     int
	DurationMs int64
	IP         string    `gorm:"size:64"`
	UserAgent  string    `gorm:"size:300"`
	Network    string    `gorm:"size:120"`
	Request    string    `gorm:"type:text"`
	Response   string    `gorm:"type:text"`
	Data       AnyMap    `gorm:"type:jsonb"`
	CreatedAt  time.Time `gorm:"index:trail_merchant,priority:2;index"`
}

func (trailRecord) TableName() string { return "trail_events" }

// Trail keeps the trail. Add never waits: events go to a queue that is written in
// batches, and if the queue is full the event is dropped and counted, because a
// payment must never wait on its own record.
type Trail struct {
	*Store
	queue   chan domain.TrailEvent
	dropped atomic.Int64
	written atomic.Int64
	done    chan struct{}
	mu      sync.Mutex
	owners  map[string]owner
}

type owner struct {
	merchant, session string
	live              bool
}

func NewTrail(s *Store) *Trail {
	return &Trail{Store: s, queue: make(chan domain.TrailEvent, 20000), done: make(chan struct{}), owners: map[string]owner{}}
}

func (t *Trail) Add(ev domain.TrailEvent) {
	if ev.ID == "" {
		ev.ID = domain.NewID("trl")
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	// The alerts count everything above info, so an event must say its level.
	if ev.Level == "" {
		ev.Level = domain.TrailInfo
	}
	select {
	case t.queue <- ev:
	default:
		t.dropped.Add(1)
	}
}

func (t *Trail) Dropped() int64 { return t.dropped.Load() }
func (t *Trail) Written() int64 { return t.written.Load() }

// Run writes the queue until ctx ends, then writes what is left.
func (t *Trail) Run(ctx context.Context) {
	defer close(t.done)
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	var batch []domain.TrailEvent
	flush := func() {
		if len(batch) > 0 {
			t.write(batch)
			batch = batch[:0]
		}
	}
	for {
		select {
		case ev := <-t.queue:
			batch = append(batch, ev)
			if len(batch) >= 200 {
				flush()
			}
		case <-tick.C:
			flush()
		case <-ctx.Done():
			for {
				select {
				case ev := <-t.queue:
					batch = append(batch, ev)
				default:
					flush()
					return
				}
			}
		}
	}
}

// Wait returns once Run has written what was left, or after d.
func (t *Trail) Wait(d time.Duration) {
	select {
	case <-t.done:
	case <-time.After(d):
	}
}

func (t *Trail) write(batch []domain.TrailEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rs := make([]*trailRecord, 0, len(batch))
	for i := range batch {
		ev := batch[i]
		t.fill(ctx, &ev)
		r := toRecord[trailRecord](&ev)
		r.Data = AnyMap(ev.Data)
		rs = append(rs, r)
	}
	if err := t.db.WithContext(ctx).CreateInBatches(rs, 100).Error; err != nil {
		log.Printf("trail: %d events not written: %v", len(rs), err)
		t.dropped.Add(int64(len(rs)))
		return
	}
	t.written.Add(int64(len(rs)))
}

// fill completes an event from what it names: the merchant and checkout of its
// payment, and the network of its address.
func (t *Trail) fill(ctx context.Context, ev *domain.TrailEvent) {
	if ev.PaymentID != "" && (ev.MerchantID == "" || ev.SessionID == "") {
		if o, ok := t.ownerOf(ctx, ev.PaymentID); ok {
			ev.MerchantID = domain.FirstNonEmpty(ev.MerchantID, o.merchant)
			ev.SessionID = domain.FirstNonEmpty(ev.SessionID, o.session)
			ev.Livemode = o.live
		}
	}
	if ev.IP != "" && ev.Network == "" {
		if n, ok := netinfo.Lookup(ev.IP); ok {
			ev.Network = n.Label
			if ev.Data == nil {
				ev.Data = map[string]any{}
			}
			ev.Data["network"] = n
		}
	}
	ev.UserAgent = domain.Trim(ev.UserAgent, 300)
	ev.Title = domain.Trim(ev.Title, 300)
	ev.Path = domain.Trim(ev.Path, 300)
}

func (t *Trail) ownerOf(ctx context.Context, paymentID string) (owner, bool) {
	t.mu.Lock()
	o, ok := t.owners[paymentID]
	t.mu.Unlock()
	if ok {
		return o, true
	}
	var r struct {
		MerchantID string
		SessionID  string
		Livemode   bool
	}
	if err := t.db.WithContext(ctx).Model(&paymentRecord{}).Select("merchant_id, session_id, livemode").Where("id = ?", paymentID).Take(&r).Error; err != nil {
		return owner{}, false
	}
	o = owner{r.MerchantID, r.SessionID, r.Livemode}
	t.mu.Lock()
	if len(t.owners) > 5000 {
		t.owners = map[string]owner{}
	}
	t.owners[paymentID] = o
	t.mu.Unlock()
	return o, true
}

// Thread is everything about a checkout and its payments, oldest first.
func (t *Trail) Thread(ctx context.Context, sessionID string, paymentIDs []string) ([]domain.TrailEvent, error) {
	q := t.conn(ctx).Where("session_id = ?", sessionID)
	if len(paymentIDs) > 0 {
		q = t.conn(ctx).Where("session_id = ? OR payment_id IN ?", sessionID, paymentIDs)
	}
	var rs []trailRecord
	if err := q.Order("created_at, id").Limit(2000).Find(&rs).Error; err != nil {
		return nil, err
	}
	return trailEvents(rs), nil
}

func (t *Trail) ByRequest(ctx context.Context, requestID string) ([]domain.TrailEvent, error) {
	var rs []trailRecord
	err := t.conn(ctx).Where("request_id = ?", requestID).Order("created_at").Limit(200).Find(&rs).Error
	return trailEvents(rs), err
}

func (t *Trail) Recent(ctx context.Context, q ports.TrailQuery) ([]domain.TrailEvent, error) {
	db := t.conn(ctx).Model(&trailRecord{})
	if q.MerchantID != "" {
		db = db.Where("merchant_id = ?", q.MerchantID)
	}
	if len(q.Kinds) > 0 {
		db = db.Where("kind IN ?", q.Kinds)
	}
	if q.Level != "" {
		db = db.Where("level = ?", q.Level)
	}
	if q.Since != nil {
		db = db.Where("created_at >= ?", *q.Since)
	}
	if s := strings.TrimSpace(q.Search); s != "" {
		like := "%" + s + "%"
		db = db.Where("title ILIKE ? OR path ILIKE ? OR request_id = ? OR payment_id = ? OR session_id = ?", like, like, s, s, s)
	}
	limit := q.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rs []trailRecord
	err := db.Order("created_at DESC").Limit(limit).Find(&rs).Error
	return trailEvents(rs), err
}

// Prune empties the bodies of old events and deletes very old ones.
func (t *Trail) Prune(ctx context.Context, bodiesBefore, eventsBefore time.Time) (int64, int64, error) {
	r1 := t.conn(ctx).Model(&trailRecord{}).Where("created_at < ? AND (request <> '' OR response <> '')", bodiesBefore).
		Updates(map[string]any{"request": "", "response": ""})
	if r1.Error != nil {
		return 0, 0, r1.Error
	}
	r2 := t.conn(ctx).Where("created_at < ?", eventsBefore).Delete(&trailRecord{})
	return r1.RowsAffected, r2.RowsAffected, r2.Error
}

func trailEvents(rs []trailRecord) []domain.TrailEvent {
	out := make([]domain.TrailEvent, len(rs))
	for i := range rs {
		copyInto(&out[i], &rs[i])
		out[i].Data = map[string]any(rs[i].Data)
	}
	return out
}
