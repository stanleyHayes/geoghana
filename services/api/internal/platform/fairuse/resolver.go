// Package fairuse resolves live fair-use policy without coupling transports to
// MongoDB. The cache retains the last valid snapshot during transient failures.
package fairuse

import (
	"context"
	"sync"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
)

type Repository interface {
	CurrentPolicy(context.Context, time.Time) (*identity.FairUsePolicy, error)
	CurrentOverride(context.Context, string, time.Time) (*identity.FairUseOverride, error)
}

type Resolution struct {
	Allowance identity.Allowance
	Allowed   bool
	Source    string
}

type Resolver struct {
	repo      Repository
	ttl       time.Duration
	now       func() time.Time
	mu        sync.RWMutex
	policy    identity.FairUsePolicy
	loadedAt  time.Time
	overrides map[string]cachedOverride
}

type cachedOverride struct {
	value    *identity.FairUseOverride
	loadedAt time.Time
}

func NewResolver(repo Repository, ttl time.Duration) *Resolver {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &Resolver{repo: repo, ttl: ttl, now: time.Now, policy: identity.DefaultFairUsePolicy(), overrides: map[string]cachedOverride{}}
}

// Resolve never grants an unlimited allowance. Before Mongo is wired, or when
// no valid persisted revision has been loaded, it exactly preserves the legacy
// AllowanceFor behavior.
func (r *Resolver) Resolve(ctx context.Context, id identity.Identity, sandbox bool, cost identity.CostClass) Resolution {
	now := r.now().UTC()
	policy := r.loadPolicy(ctx, now)
	allowance := policy.Authenticated
	source := "policy:" + policy.ID
	if sandbox {
		allowance = policy.Sandbox
	} else if id.Anonymous {
		allowance = policy.Anonymous
	} else if id.Key != nil && id.Key.Elevated {
		// Migration bridge: existing documented key elevations retain their
		// established behavior until an application override is appended.
		allowance, source = identity.AllowanceFor(id, false), "legacy-elevated"
	}
	allowed := policy.AllowsCost(cost)

	if !sandbox && id.Key != nil && id.Key.ApplicationID != "" {
		if override := r.loadOverride(ctx, id.Key.ApplicationID, now); override != nil && override.Active(now) && override.Raises(policy.Authenticated, policy.CostCeilings) {
			allowance, source = override.Allowance, "override:"+override.ID
			ceiling, ok := override.CostCeilings[cost]
			allowed = ok && cost.Units() <= ceiling
		}
	}
	return Resolution{Allowance: allowance, Allowed: allowed, Source: source}
}

func (r *Resolver) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loadedAt = time.Time{}
	r.overrides = map[string]cachedOverride{}
}

func (r *Resolver) loadPolicy(ctx context.Context, now time.Time) identity.FairUsePolicy {
	r.mu.RLock()
	value, loaded := r.policy, r.loadedAt
	r.mu.RUnlock()
	if now.Sub(loaded) < r.ttl {
		return value
	}
	if r.repo == nil {
		return value
	}
	fresh, err := r.repo.CurrentPolicy(ctx, now)
	if err != nil || fresh == nil || fresh.Validate() != nil {
		return value
	}
	r.mu.Lock()
	if !fresh.EffectiveFrom.After(now) {
		r.policy, r.loadedAt = *fresh, now
	}
	value = r.policy
	r.mu.Unlock()
	return value
}

func (r *Resolver) loadOverride(ctx context.Context, appID string, now time.Time) *identity.FairUseOverride {
	r.mu.RLock()
	cached, ok := r.overrides[appID]
	r.mu.RUnlock()
	if ok && now.Sub(cached.loadedAt) < r.ttl {
		return cached.value
	}
	if r.repo == nil {
		return nil
	}
	fresh, err := r.repo.CurrentOverride(ctx, appID, now)
	if err != nil {
		if ok {
			return cached.value
		}
		return nil
	}
	if fresh != nil && fresh.Validate() != nil {
		if ok {
			return cached.value
		}
		return nil
	}
	r.mu.Lock()
	r.overrides[appID] = cachedOverride{value: fresh, loadedAt: now}
	r.mu.Unlock()
	return fresh
}
