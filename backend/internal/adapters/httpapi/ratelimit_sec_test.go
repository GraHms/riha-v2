package httpapi

import (
	"fmt"
	"testing"
)

// Filling the limiter with 100 001 keys must not wipe an exhausted login
// bucket.
func TestSecRateLimiterFlush(t *testing.T) {
	l := newRateLimiter(10, 10.0/60)
	for i := 0; i < 10; i++ {
		l.allow("login|1.2.3.4")
	}
	if l.allow("login|1.2.3.4") {
		t.Fatal("setup: bucket should be empty")
	}
	for i := 0; i < 100002; i++ {
		l.allow(fmt.Sprintf("login|10.%d", i))
	}
	if l.allow("login|1.2.3.4") {
		t.Fatal("a flood of other keys reset an exhausted login bucket")
	}
	if len(l.b) > 100002 {
		t.Fatalf("%d buckets kept", len(l.b))
	}
}
