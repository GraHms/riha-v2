package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"riha/internal/core/ports"
)

// Insights answers the questions of the admin's trail and metrics pages, straight
// from the tables, in SQL.
type Insights struct{ *Store }

func (s Insights) FindThreads(ctx context.Context, q string, limit int) ([]ports.ThreadHit, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var hits []ports.ThreadHit
	err := s.conn(ctx).Raw(`
		select p.id as payment_id, p.session_id, p.reference, p.rail_reference as operation, p.method, p.status, p.amount, p.livemode,
		       p.created_at, m.name as merchant, p.phone_prefix, p.phone_last4, coalesce(p.failure_code,'') as failure_code
		from payments p join merchants m on m.id = p.merchant_id
		where p.id = @q or upper(p.reference) = upper(@q) or p.rail_reference = @q or p.session_id = @q or upper(p.attempt_id) = upper(@q)
		   or p.rail_id = @q or (length(@q) = 4 and p.phone_last4 = @q)
		   or p.id in (select payment_id from trail_events where request_id = @q and payment_id <> '')
		   or p.session_id in (select session_id from trail_events where request_id = @q and session_id <> '')
		order by p.created_at desc limit @limit`, map[string]any{"q": q, "limit": limit}).Scan(&hits).Error
	if err != nil || len(hits) > 0 {
		return hits, err
	}
	// A checkout nobody tried to pay yet.
	err = s.conn(ctx).Raw(`
		select '' as payment_id, cs.id as session_id, '' as reference, '' as operation, '' as method, cs.status, cs.amount, cs.livemode,
		       cs.created_at, m.name as merchant, '' as phone_prefix, '' as phone_last4, '' as failure_code
		from checkout_sessions cs join merchants m on m.id = cs.merchant_id
		where cs.id = @q or cs.id in (select session_id from trail_events where request_id = @q and session_id <> '')
		limit @limit`, map[string]any{"q": q, "limit": limit}).Scan(&hits).Error
	return hits, err
}

func (s Insights) Metrics(ctx context.Context, q ports.MetricsQuery) (ports.Metrics, error) {
	var out ports.Metrics
	// Counts and sums of payments come from the payment_days rollup, by
	// whole Maputo days; timings and the trail are read within the bounds.
	args := map[string]any{"from": q.From, "to": q.To, "live": q.Live, "merchant": q.MerchantID,
		"fromday": maputoDay(q.From), "today": maputoDay(q.To)}
	mf := ""
	if q.MerchantID != "" {
		mf = " and p.merchant_id = @merchant"
	}
	db := s.conn(ctx)
	steps := []struct {
		sql  string
		dest any
	}{
		{`select to_char(p.day, 'YYYY-MM-DD') as day, p.method,
		         sum(p.n) as total, coalesce(sum(p.n) filter (where p.status in (` + settledList() + `)), 0) as paid,
		         coalesce(sum(p.n) filter (where p.status = 'failed'), 0) as failed,
		         coalesce(sum(p.amount) filter (where p.status in (` + settledList() + `)), 0) as amount
		  from payment_days p where p.livemode = @live and p.day >= @fromday and p.day <= @today` + mf + `
		  group by 1, 2 having sum(p.n) <> 0 order by 1, 2`, &out.Daily},
		{`select coalesce(nullif(p.failure_code,''), 'desconhecido') as code, p.method, sum(p.n) as n
		  from payment_days p where p.livemode = @live and p.status = 'failed' and p.day >= @fromday and p.day <= @today` + mf + `
		  group by 1, 2 having sum(p.n) <> 0 order by 3 desc`, &out.Failures},
		{`select p.method,
		         count(*) filter (where p.status in ('succeeded','refunded')) as paid, count(*) filter (where p.status = 'failed') as failed,
		         coalesce(percentile_cont(0.5) within group (order by extract(epoch from p.succeeded_at - p.created_at)) filter (where p.succeeded_at is not null), 0) as p50_seconds,
		         coalesce(percentile_cont(0.9) within group (order by extract(epoch from p.succeeded_at - p.created_at)) filter (where p.succeeded_at is not null), 0) as p90_seconds
		  from payments p where p.livemode = @live and p.created_at >= @from and p.created_at < @to` + mf + `
		  group by 1 order by 1`, &out.Methods},
		{`select p.method, count(*) as n,
		         coalesce(percentile_cont(0.5) within group (order by t.duration_ms), 0) as p50_ms,
		         coalesce(percentile_cont(0.9) within group (order by t.duration_ms), 0) as p90_ms,
		         count(*) filter (where t.level <> 'info') as errors
		  from trail_events t join payments p on p.id = t.payment_id
		  where t.kind = 'rail.response' and p.livemode = @live and t.created_at >= @from and t.created_at < @to` + mf + `
		  group by 1 order by 1`, &out.Operator},
		{`select coalesce(nullif(t.network,''), 'desconhecida') as network, count(*) as total,
		         count(*) filter (where p.status in ('succeeded','refunded')) as paid,
		         count(*) filter (where p.status = 'failed') as failed
		  from trail_events t join payments p on p.id = t.payment_id
		  where t.kind = 'payment.submitted' and p.livemode = @live and t.created_at >= @from and t.created_at < @to` + mf + `
		  group by 1 order by 2 desc limit 12`, &out.Networks},
		{`select coalesce(t.data->>'wallet', 'desconhecida') as wallet, count(*) as payments,
		         coalesce(sum((t.data->>'commission')::numeric), 0) as commission, coalesce(sum((t.data->>'amount')::numeric), 0) as amount
		  from trail_events t join payments p on p.id = t.payment_id
		  where t.kind = 'rail.callback' and (t.data->>'commission') is not null and p.livemode = @live and t.created_at >= @from and t.created_at < @to` + mf + `
		  group by 1 order by 1`, &out.Commission},
		{`select count(*) as opened,
		         count(*) filter (where exists (select 1 from payments x where x.session_id = cs.id)) as tried,
		         count(*) filter (where cs.status = 'complete') as paid
		  from checkout_sessions cs where cs.livemode = @live and cs.created_at >= @from and cs.created_at < @to` + strings.ReplaceAll(mf, "p.", "cs."),
			&out.Funnel},
		{`select count(*) as total, count(*) filter (where d.status = 'succeeded') as delivered, count(*) filter (where d.status = 'failed') as failed,
		         count(*) filter (where d.status = 'pending') as pending,
		         coalesce(percentile_cont(0.9) within group (order by d.last_duration_ms) filter (where d.attempts > 0), 0) as p90_ms
		  from webhook_deliveries d where d.livemode = @live and d.created_at >= @from and d.created_at < @to` + strings.ReplaceAll(mf, "p.", "d."),
			&out.Webhooks},
		{`select count(*) as total, count(*) filter (where r.status >= 400 and r.status < 500) as client_errors,
		         count(*) filter (where r.status >= 500) as server_errors,
		         coalesce(percentile_cont(0.9) within group (order by r.duration_ms), 0) as p90_ms
		  from api_request_logs r where r.livemode = @live and r.created_at >= @from and r.created_at < @to` + strings.ReplaceAll(mf, "p.", "r."),
			&out.API},
		{`select count(*) from payments p where p.livemode = @live and p.status = 'processing' and p.created_at < now() - interval '15 minutes'` + mf,
			&out.Stuck},
	}
	for i, st := range steps {
		if err := db.Raw(st.sql, args).Scan(st.dest).Error; err != nil {
			return out, fmt.Errorf("metrics query %d: %w", i, err)
		}
	}
	return out, nil
}

// Health is what the watchdog looks at: problems of the last minutes.
func (s Insights) Health(ctx context.Context, since time.Time) (ports.Health, error) {
	var h ports.Health
	db := s.conn(ctx)
	steps := []struct {
		sql  string
		dest any
	}{
		{`select count(*) from payments where status = 'processing' and created_at < now() - interval '15 minutes'`, &h.StuckPayments},
		{`select count(*) from webhook_deliveries where status = 'failed' and updated_at >= @since`, &h.FailedWebhooks},
		{`select count(*) from trail_events where kind = 'rail.response' and level <> 'info' and created_at >= @since`, &h.OperatorErrors},
		{`select count(*) from api_request_logs where status >= 500 and created_at >= @since`, &h.APIServerErrors},
		{`select count(*) from trail_events where kind = 'log' and level = 'error' and created_at >= @since`, &h.ErrorLogs},
		{`select coalesce(max(created_at), 'epoch') from trail_events`, &h.LastTrail},
	}
	for _, st := range steps {
		var args []any
		if strings.Contains(st.sql, "@since") {
			args = append(args, map[string]any{"since": since})
		}
		if err := db.Raw(st.sql, args...).Scan(st.dest).Error; err != nil {
			return h, err
		}
	}
	return h, nil
}
