package redis

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
)

func testLimiter(t *testing.T) *Limiter {
	t.Helper()
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "redis://localhost:6679"
	}
	l, err := NewLimiter(url, false)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := l.Ping(ctx); err != nil {
		t.Skipf("redis unavailable at %s: %v", url, err)
	}
	return l
}

func TestBucketAllowsThenDenies(t *testing.T) {
	l := testLimiter(t)
	defer l.Close()
	ctx := context.Background()

	id := identity.Anonymous("test-allow-deny")
	if err := l.Reset(ctx, id.RateKey); err != nil {
		t.Fatal(err)
	}
	// A small bucket that refills slowly, so the test is deterministic.
	a := identity.Allowance{BurstUnits: 5, RefillPerSecond: 0.001, Window: time.Minute}

	for i := 0; i < 5; i++ {
		d, err := l.Allow(ctx, id, a, identity.CostCheap)
		if err != nil {
			t.Fatal(err)
		}
		if !d.Allowed {
			t.Fatalf("request %d denied while the bucket should still hold tokens", i+1)
		}
	}
	d, err := l.Allow(ctx, id, a, identity.CostCheap)
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed {
		t.Fatal("the sixth request should have exhausted a 5-unit bucket")
	}
	if d.RetryAfter <= 0 {
		t.Error("a denial must tell the caller when to retry")
	}
}

// An expensive operation must consume proportionally more of the bucket, or
// the cost classes are decorative.
func TestExpensiveOperationsCostMore(t *testing.T) {
	l := testLimiter(t)
	defer l.Close()
	ctx := context.Background()
	a := identity.Allowance{BurstUnits: 8, RefillPerSecond: 0.001, Window: time.Minute}

	cheap := identity.Anonymous("test-cost-cheap")
	_ = l.Reset(ctx, cheap.RateKey)
	geometry := identity.Anonymous("test-cost-geometry")
	_ = l.Reset(ctx, geometry.RateKey)

	dc, _ := l.Allow(ctx, cheap, a, identity.CostCheap)
	dg, _ := l.Allow(ctx, geometry, a, identity.CostGeometry)

	if dc.Remaining <= dg.Remaining {
		t.Errorf("a geometry call left %d tokens and a cheap call left %d — cost is not being applied",
			dg.Remaining, dc.Remaining)
	}
}

func TestBucketRefills(t *testing.T) {
	l := testLimiter(t)
	defer l.Close()
	ctx := context.Background()

	id := identity.Anonymous("test-refill")
	_ = l.Reset(ctx, id.RateKey)
	// Refills fast enough to recover within the test's sleep.
	a := identity.Allowance{BurstUnits: 2, RefillPerSecond: 20, Window: time.Minute}

	for i := 0; i < 2; i++ {
		if d, _ := l.Allow(ctx, id, a, identity.CostCheap); !d.Allowed {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}
	if d, _ := l.Allow(ctx, id, a, identity.CostCheap); d.Allowed {
		t.Fatal("the bucket should be empty")
	}
	time.Sleep(250 * time.Millisecond)
	if d, _ := l.Allow(ctx, id, a, identity.CostCheap); !d.Allowed {
		t.Error("the bucket should have refilled after 250ms at 20 units/second")
	}
}

// Callers must not share a bucket, or one heavy user throttles everyone.
func TestCallersAreIsolated(t *testing.T) {
	l := testLimiter(t)
	defer l.Close()
	ctx := context.Background()
	a := identity.Allowance{BurstUnits: 1, RefillPerSecond: 0.001, Window: time.Minute}

	one := identity.Anonymous("test-isolate-1")
	two := identity.Anonymous("test-isolate-2")
	_ = l.Reset(ctx, one.RateKey)
	_ = l.Reset(ctx, two.RateKey)

	if d, _ := l.Allow(ctx, one, a, identity.CostCheap); !d.Allowed {
		t.Fatal("first caller's first request should pass")
	}
	if d, _ := l.Allow(ctx, one, a, identity.CostCheap); d.Allowed {
		t.Fatal("first caller should now be exhausted")
	}
	if d, _ := l.Allow(ctx, two, a, identity.CostCheap); !d.Allowed {
		t.Error("a second caller must be unaffected by the first's usage")
	}
}

// A limiter outage must not take down a free public API that people depend on.
func TestFailOpenWhenRedisIsUnreachable(t *testing.T) {
	l, err := NewLimiter("redis://127.0.0.1:59998", true)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	d, err := l.Allow(ctx, identity.Anonymous("x"), identity.Allowance{BurstUnits: 1}, identity.CostCheap)
	if err != nil {
		t.Fatalf("fail-open should not surface an error: %v", err)
	}
	if !d.Allowed {
		t.Error("with failOpen the request must be allowed")
	}
	if !d.Degraded {
		t.Error("the decision must report that accounting was skipped, so it can be logged")
	}
}

func TestFailClosedWhenConfigured(t *testing.T) {
	l, err := NewLimiter("redis://127.0.0.1:59998", false)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := l.Allow(ctx, identity.Anonymous("x"), identity.Allowance{BurstUnits: 1}, identity.CostCheap); err == nil {
		t.Error("with failOpen disabled an outage must surface as an error")
	}
}
