// Package perf holds the p95 acceptance gate for the public read paths
// (Spec §22.4, story GEO-12.6).
//
// It is a TEST rather than a benchmark because the targets are pass/fail
// acceptance criteria, not numbers to compare across commits. It is skipped
// unless PERF_TARGET_URL is set, so `go test ./...` stays fast and hermetic
// while CI can run it against a live stack.
//
//	PERF_TARGET_URL=http://localhost:8180 \
//	PERF_API_KEY=gh_live_... go test ./perf/ -v
//
// An API key is effectively required. Fair-use limiting applies to this
// traffic like any other, and an anonymous caller gets 120 burst units — a
// load test exhausts that in under a second and then measures the limiter
// rather than the API. Use a key with an elevated ceiling, which is exactly
// the documented-need case §24 F4 describes.
package perf

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// scenario is one hot path and the p95 it must meet.
type scenario struct {
	name   string
	method string
	path   string
	body   string
	// targetMS is the p95 ceiling from Spec §22.4.
	targetMS float64
}

var scenarios = []scenario{
	{"id lookup (region)", "GET", "/v1/regions/01KDVDNA00JR6256MY7B23EX4J", "", 150},
	{"id lookup (place)", "GET", "/v1/places/01KDVDNA00N6BFFK8VF5K8YXPW", "", 150},
	{"autocomplete", "GET", "/v1/autocomplete?q=kum", "", 200},
	{"search", "GET", "/v1/search?q=kumsai", "", 350},
	{"search (multi-token)", "GET", "/v1/search?q=tema%20comm", "", 350},
	{"reverse", "GET", "/v1/reverse?lat=5.6037&lng=-0.1870", "", 400},
	{"nearby", "GET", "/v1/nearby?lat=5.6037&lng=-0.1870&radius=5000", "", 500},
	{"list districts (page)", "GET", "/v1/districts?limit=50", "", 350},
	{
		name: "graphql (ordinary query)", method: "POST", path: "/graphql",
		body:     `{"query":"{ regions(first:5){ nodes { id name capital } } }"}`,
		targetMS: 500,
	},
}

const (
	defaultRequests    = 200
	defaultConcurrency = 8
	warmupRequests     = 10
)

func TestP95Targets(t *testing.T) {
	base := strings.TrimRight(os.Getenv("PERF_TARGET_URL"), "/")
	if base == "" {
		t.Skip("PERF_TARGET_URL not set — skipping the p95 acceptance gate")
	}

	n := envInt("PERF_REQUESTS", defaultRequests)
	c := envInt("PERF_CONCURRENCY", defaultConcurrency)

	key := os.Getenv("PERF_API_KEY")
	client := &http.Client{
		Timeout: 20 * time.Second,
		// A generous pool: the default 2 idle connections per host would
		// serialise a concurrent run and measure connection setup.
		Transport: &http.Transport{MaxIdleConnsPerHost: 64, MaxConnsPerHost: 64},
	}
	requireReachable(t, client, base)

	auth := "anonymous"
	if key != "" {
		auth = "authenticated"
	}
	t.Logf("target %s · %d requests · concurrency %d · %s", base, n, c, auth)
	t.Logf("%-26s %8s %8s %8s %8s   %s", "SCENARIO", "p50", "p95", "p99", "max", "TARGET")

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			// Warm up so the first measured request does not pay for a cold
			// connection pool, a lazy index load or a JIT-cold code path.
			for i := 0; i < warmupRequests; i++ {
				_, _ = doOnce(client, base, s, key)
			}
			t.Cleanup(func() {}) // keep scenarios independent

			lat, errs := measure(client, base, s, key, n, c)
			if len(errs) > 0 {
				// A failing request is not a slow one; reporting a p95 over a
				// set that includes errors would flatter the result.
				if strings.Contains(errs[0].Error(), "HTTP 429") {
					t.Fatalf("%d/%d requests were rate limited. The gate is measuring "+
						"fair-use limiting, not latency. Set PERF_API_KEY to a key with "+
						"an elevated ceiling, or lower PERF_CONCURRENCY. First: %v",
						len(errs), n, errs[0])
				}
				t.Fatalf("%d/%d requests failed, first: %v", len(errs), n, errs[0])
			}

			p50, p95, p99, max := percentile(lat, 50), percentile(lat, 95), percentile(lat, 99), lat[len(lat)-1]
			status := "ok"
			if p95 > s.targetMS {
				status = "FAIL"
			}
			t.Logf("%-26s %7.1f %7.1f %7.1f %7.1f   <%.0f ms %s",
				s.name, p50, p95, p99, max, s.targetMS, status)

			if p95 > s.targetMS {
				t.Errorf("p95 %.1f ms exceeds the %0.f ms target for %s (Spec §22.4)",
					p95, s.targetMS, s.name)
			}
		})
	}
}

func requireReachable(t *testing.T, c *http.Client, base string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", base+"/health", nil)
	res, err := c.Do(req)
	if err != nil {
		t.Fatalf("target %s is unreachable: %v", base, err)
	}
	_ = res.Body.Close()
}

// measure runs n requests at concurrency c and returns sorted latencies in ms.
func measure(c *http.Client, base string, s scenario, key string, n, conc int) ([]float64, []error) {
	var (
		mu   sync.Mutex
		lat  = make([]float64, 0, n)
		errs []error
		wg   sync.WaitGroup
	)
	jobs := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		jobs <- struct{}{}
	}
	close(jobs)

	for w := 0; w < conc; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				d, err := doOnce(c, base, s, key)
				mu.Lock()
				if err != nil {
					errs = append(errs, err)
				} else {
					lat = append(lat, float64(d.Microseconds())/1000)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	sort.Float64s(lat)
	return lat, errs
}

// maxRateLimitRetries bounds how long a single request will wait out fair-use
// limiting before the gate gives up and reports it.
const maxRateLimitRetries = 8

// doOnce issues one request and returns its latency.
//
// A 429 is WAITED OUT rather than counted. Fair-use limiting is deliberate
// behaviour with its own tests; this gate measures how fast the service
// answers, and letting the limiter contaminate the sample would either fail
// the build spuriously or — worse — inflate the p95 with queueing time and
// send someone optimising a query that was never slow.
func doOnce(c *http.Client, base string, s scenario, key string) (time.Duration, error) {
	for attempt := 0; ; attempt++ {
		d, retryAfter, err := attemptOnce(c, base, s, key)
		if retryAfter <= 0 {
			return d, err
		}
		if attempt >= maxRateLimitRetries {
			return 0, fmt.Errorf("%s %s: HTTP 429 after %d retries", s.method, s.path, attempt)
		}
		time.Sleep(retryAfter)
	}
}

// attemptOnce returns a non-zero retryAfter when the request was rate limited.
func attemptOnce(
	c *http.Client, base string, s scenario, key string,
) (time.Duration, time.Duration, error) {
	var body io.Reader
	if s.body != "" {
		body = strings.NewReader(s.body)
	}
	req, err := http.NewRequest(s.method, base+s.path, body)
	if err != nil {
		return 0, 0, err
	}
	if s.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	start := time.Now()
	res, err := c.Do(req)
	if err != nil {
		return 0, 0, err
	}
	// The body must be drained and closed, or the connection is not returned
	// to the pool and every later request pays for a fresh TCP handshake —
	// which would measure the harness, not the API.
	n, _ := io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
	d := time.Since(start)

	if res.StatusCode == http.StatusTooManyRequests {
		// Honour the server's own guidance; fall back to a short pause when
		// the header is absent or unparseable.
		wait := 250 * time.Millisecond
		if ra := res.Header.Get("Retry-After"); ra != "" {
			if secs, perr := strconv.Atoi(ra); perr == nil && secs > 0 {
				wait = time.Duration(secs) * time.Second
			}
		}
		return 0, wait, nil
	}
	if res.StatusCode >= 400 {
		return 0, 0, fmt.Errorf("%s %s: HTTP %d", s.method, s.path, res.StatusCode)
	}
	if n == 0 {
		return 0, 0, fmt.Errorf("%s %s: empty body", s.method, s.path)
	}
	return d, 0, nil
}

// percentile uses nearest-rank on a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(p / 100 * float64(len(sorted)))
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
