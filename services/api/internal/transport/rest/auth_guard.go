package rest

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

// Password hashing is intentionally memory-hard. Keep its public entrypoints
// on a small, fail-closed lane that does not depend on Redis: a cache outage
// must not turn authentication into an unbounded Argon2 work queue.
var passwordHashSlots = make(chan struct{}, 4)

var authIPBuckets = struct {
	sync.Mutex
	m         map[string]authIPBucket
	lastSweep time.Time
}{m: make(map[string]authIPBucket)}

type authIPBucket struct {
	tokens  float64
	updated time.Time
}

func allowExpensiveAuthIP(ip string, now time.Time) bool {
	const capacity, refillPerSecond = 5.0, 0.1 // five attempts, then one/10s
	ip = authRateKey(ip)
	authIPBuckets.Lock()
	defer authIPBuckets.Unlock()
	if authIPBuckets.lastSweep.IsZero() || now.Sub(authIPBuckets.lastSweep) >= time.Minute {
		for key, bucket := range authIPBuckets.m {
			if now.Sub(bucket.updated) > 10*time.Minute {
				delete(authIPBuckets.m, key)
			}
		}
		authIPBuckets.lastSweep = now
	}
	// A rotating-address attack must not turn the limiter itself into an
	// unbounded heap. Evict the oldest idle key at the hard ceiling.
	if _, exists := authIPBuckets.m[ip]; !exists && len(authIPBuckets.m) >= 4096 {
		var oldestKey string
		var oldest time.Time
		for key, bucket := range authIPBuckets.m {
			if oldestKey == "" || bucket.updated.Before(oldest) {
				oldestKey, oldest = key, bucket.updated
			}
		}
		delete(authIPBuckets.m, oldestKey)
	}
	b := authIPBuckets.m[ip]
	if b.updated.IsZero() {
		b.tokens = capacity
		b.updated = now
	}
	b.tokens += now.Sub(b.updated).Seconds() * refillPerSecond
	if b.tokens > capacity {
		b.tokens = capacity
	}
	b.updated = now
	if b.tokens < 1 {
		authIPBuckets.m[ip] = b
		return false
	}
	b.tokens--
	authIPBuckets.m[ip] = b
	return true
}

func authRateKey(remote string) string {
	host := strings.Trim(remote, "[]")
	if parsed, _, err := net.SplitHostPort(remote); err == nil {
		host = parsed
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	// Bucket IPv6 clients by /64 so address rotation inside a normal client
	// prefix cannot mint unlimited fresh authentication budgets.
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

func authExpensiveGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowExpensiveAuthIP(clientIP(r), time.Now()) {
			w.Header().Set("Retry-After", "10")
			writeErr(w, r, apierr.New(apierr.RateLimitExceeded, "Too many authentication attempts."))
			return
		}
		select {
		case passwordHashSlots <- struct{}{}:
			defer func() { <-passwordHashSlots }()
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			writeErr(w, r, apierr.New(apierr.RateLimitExceeded, "Authentication is busy. Try again shortly."))
		}
	})
}
