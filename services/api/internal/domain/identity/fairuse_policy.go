package identity

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const DefaultFairUsePolicyID = "public-default"

var (
	ErrInvalidFairUsePolicy   = errors.New("invalid fair-use policy")
	ErrInvalidFairUseOverride = errors.New("invalid fair-use override")
)

// FairUsePolicy is an immutable revision of the public fair-use configuration.
// A new revision supersedes the previous one; published revisions are never
// edited. Nothing in this model can represent payment, donations, or plans.
type FairUsePolicy struct {
	ID            string
	Revision      int64
	Anonymous     Allowance
	Authenticated Allowance
	Sandbox       Allowance
	CostCeilings  map[CostClass]int
	Reason        string
	ActorID       string
	CreatedAt     time.Time
	EffectiveFrom time.Time
}

// FairUseOverride is an immutable application-specific decision. Revocation
// or replacement is represented by a later record, retaining a complete audit
// history. Enabled grants must expire; permanent exemptions are not allowed.
type FairUseOverride struct {
	ID            string
	ApplicationID string
	Allowance     Allowance
	CostCeilings  map[CostClass]int
	Enabled       bool
	Reason        string
	ActorID       string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	SupersedesID  string
}

func DefaultFairUsePolicy() FairUsePolicy {
	return FairUsePolicy{
		ID: DefaultFairUsePolicyID, Revision: 1,
		Anonymous: anonymousAllowance, Authenticated: authenticatedAllowance,
		Sandbox: sandboxAllowance,
		CostCeilings: map[CostClass]int{
			CostCheap: 1, CostNormal: 2, CostSpatial: 4, CostGeometry: 8,
		},
		Reason: "built-in migration fallback", ActorID: "system",
	}
}

func (p FairUsePolicy) Validate() error {
	if p.ID != DefaultFairUsePolicyID || p.Revision < 1 {
		return fmt.Errorf("%w: public-default id and positive revision are required", ErrInvalidFairUsePolicy)
	}
	if err := validateAllowanceSet(p.Anonymous, p.Authenticated, p.Sandbox); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidFairUsePolicy, err)
	}
	if p.Sandbox.BurstUnits > p.Authenticated.BurstUnits ||
		p.Sandbox.RefillPerSecond > p.Authenticated.RefillPerSecond ||
		p.Sandbox.Window > p.Authenticated.Window {
		return fmt.Errorf("%w: sandbox allowance cannot exceed authenticated allowance", ErrInvalidFairUsePolicy)
	}
	if err := validateCeilings(p.CostCeilings); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidFairUsePolicy, err)
	}
	if strings.TrimSpace(p.Reason) == "" || strings.TrimSpace(p.ActorID) == "" || p.CreatedAt.IsZero() || p.EffectiveFrom.IsZero() {
		return fmt.Errorf("%w: reason, actor and timestamps are required", ErrInvalidFairUsePolicy)
	}
	return nil
}

func (o FairUseOverride) Validate() error {
	if strings.TrimSpace(o.ID) == "" || strings.TrimSpace(o.ApplicationID) == "" {
		return fmt.Errorf("%w: id and application id are required", ErrInvalidFairUseOverride)
	}
	if strings.TrimSpace(o.Reason) == "" || strings.TrimSpace(o.ActorID) == "" || o.CreatedAt.IsZero() {
		return fmt.Errorf("%w: reason, actor and created timestamp are required", ErrInvalidFairUseOverride)
	}
	if err := validateAllowance(o.Allowance); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidFairUseOverride, err)
	}
	if err := validateCeilings(o.CostCeilings); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidFairUseOverride, err)
	}
	if !o.ExpiresAt.After(o.CreatedAt) {
		return fmt.Errorf("%w: override must expire after creation", ErrInvalidFairUseOverride)
	}
	return nil
}

func (p FairUsePolicy) AllowsCost(cost CostClass) bool {
	ceiling, ok := p.CostCeilings[cost]
	return ok && cost.Units() <= ceiling
}

func (o FairUseOverride) Active(at time.Time) bool {
	return o.Enabled && !at.Before(o.CreatedAt) && at.Before(o.ExpiresAt)
}

// Raises reports whether an exemption is a monotonic increase over the public
// policy. An override is not a second policy surface and cannot quietly make
// one application worse off or reshape its cost classes.
func (o FairUseOverride) Raises(base Allowance, ceilings map[CostClass]int) bool {
	if o.Allowance.BurstUnits < base.BurstUnits || o.Allowance.RefillPerSecond < base.RefillPerSecond || o.Allowance.Window < base.Window {
		return false
	}
	raised := o.Allowance.BurstUnits > base.BurstUnits || o.Allowance.RefillPerSecond > base.RefillPerSecond || o.Allowance.Window > base.Window
	for _, class := range []CostClass{CostCheap, CostNormal, CostSpatial, CostGeometry} {
		if o.CostCeilings[class] < ceilings[class] {
			return false
		}
		raised = raised || o.CostCeilings[class] > ceilings[class]
	}
	return raised
}

func validateAllowanceSet(values ...Allowance) error {
	for _, value := range values {
		if err := validateAllowance(value); err != nil {
			return err
		}
	}
	return nil
}

func validateAllowance(a Allowance) error {
	if a.BurstUnits <= 0 || a.RefillPerSecond <= 0 || a.Window <= 0 {
		return errors.New("allowance values must be positive")
	}
	return nil
}

func validateCeilings(values map[CostClass]int) error {
	for _, class := range []CostClass{CostCheap, CostNormal, CostSpatial, CostGeometry} {
		ceiling, ok := values[class]
		if !ok || ceiling < 0 {
			return fmt.Errorf("%s ceiling must be zero or greater", class)
		}
	}
	return nil
}
