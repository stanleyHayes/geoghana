// Package redis implements distributed fair-use limiting.
//
// A token bucket in Lua, evaluated atomically on the Redis server. Doing the
// read-modify-write in Go would race across API instances and let a burst
// through proportional to the number of replicas.
package redis

import (
	"context"
	"fmt"
	"math"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
)

// tokenBucket refills lazily from the elapsed time, so no background job is
// needed and an idle caller's bucket costs nothing to maintain.
//
// KEYS[1] bucket key
// ARGV[1] capacity  ARGV[2] refill per second
// ARGV[3] now (ms)  ARGV[4] cost  ARGV[5] ttl seconds
var tokenBucket = goredis.NewScript(`
local key      = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill   = tonumber(ARGV[2])
local now      = tonumber(ARGV[3])
local cost     = tonumber(ARGV[4])
local ttl      = tonumber(ARGV[5])

local state    = redis.call('HMGET', key, 'tokens', 'updated')
local tokens   = tonumber(state[1])
local updated  = tonumber(state[2])

if tokens == nil then
  tokens  = capacity
  updated = now
end

local elapsed = math.max(0, now - updated) / 1000.0
tokens = math.min(capacity, tokens + elapsed * refill)

local allowed = 0
if tokens >= cost then
  tokens = tokens - cost
  allowed = 1
end

redis.call('HMSET', key, 'tokens', tokens, 'updated', now)
redis.call('EXPIRE', key, ttl)

-- Seconds until enough tokens exist for another request of this cost.
local retry = 0
if allowed == 0 and refill > 0 then
  retry = math.ceil((cost - tokens) / refill)
end

return {allowed, math.floor(tokens), retry}
`)

type Limiter struct {
	client *goredis.Client
	// failOpen decides behaviour when Redis is unreachable. Read APIs fail
	// OPEN: a limiter outage must not take down a free public service that
	// people may depend on. Abuse is handled by the WAF in that window.
	failOpen bool
}

func NewLimiter(redisURL string, failOpen bool) (*Limiter, error) {
	opt, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	return &Limiter{client: goredis.NewClient(opt), failOpen: failOpen}, nil
}

func (l *Limiter) Close() error { return l.client.Close() }

func (l *Limiter) Ping(ctx context.Context) error { return l.client.Ping(ctx).Err() }

// Decision is the outcome of a limit check, shaped for response headers.
type Decision struct {
	Allowed    bool
	Remaining  int
	Limit      int
	RetryAfter int
	// Degraded reports that Redis was unreachable and the request was allowed
	// without accounting, so the caller can log it.
	Degraded bool
}

// Allow charges cost units against the caller's bucket.
func (l *Limiter) Allow(
	ctx context.Context, id identity.Identity, a identity.Allowance, cost identity.CostClass,
) (Decision, error) {
	key := "rl:" + id.RateKey
	ttl := int(math.Max(60, a.Window.Seconds()))

	res, err := tokenBucket.Run(ctx, l.client,
		[]string{key},
		a.BurstUnits, a.RefillPerSecond, time.Now().UnixMilli(), cost.Units(), ttl,
	).Int64Slice()
	if err != nil {
		if l.failOpen {
			return Decision{Allowed: true, Limit: a.BurstUnits, Degraded: true}, nil
		}
		return Decision{}, fmt.Errorf("rate limit check: %w", err)
	}
	if len(res) != 3 {
		return Decision{}, fmt.Errorf("rate limit script returned %d values, want 3", len(res))
	}
	return Decision{
		Allowed:    res[0] == 1,
		Remaining:  int(res[1]),
		Limit:      a.BurstUnits,
		RetryAfter: int(res[2]),
	}, nil
}

// Reset clears a caller's bucket. Used by tests and by the admin emergency
// controls (Spec §13).
func (l *Limiter) Reset(ctx context.Context, rateKey string) error {
	return l.client.Del(ctx, "rl:"+rateKey).Err()
}
