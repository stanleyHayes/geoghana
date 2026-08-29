package identity

import "time"

// CostClass is how expensive an operation is to serve (Spec §13, Appendix D).
//
// These are FAIR-USE weights, not a price ladder. They exist so one consumer
// cannot degrade the service for everyone else. GhanaGeo is free and every
// authenticated developer gets the same allowance (agent_plan.md §24 F2).
type CostClass string

const (
	CostCheap    CostClass = "cheap"    // get by id
	CostNormal   CostClass = "normal"   // search, autocomplete
	CostSpatial  CostClass = "spatial"  // reverse, nearby
	CostGeometry CostClass = "geometry" // boundary GeoJSON
)

var costUnits = map[CostClass]int{
	CostCheap: 1, CostNormal: 2, CostSpatial: 4, CostGeometry: 8,
}

func (c CostClass) Units() int {
	if u, ok := costUnits[c]; ok {
		return u
	}
	return 1
}

// Allowance is what a caller may consume. There is exactly one allowance per
// caller category, and no way to buy a larger one.
type Allowance struct {
	// BurstUnits is the token-bucket capacity.
	BurstUnits int
	// RefillPerSecond is how fast the bucket refills, in cost units.
	RefillPerSecond float64
	// Window is the accounting period used for reporting.
	Window time.Duration
}

// Caller categories. Anonymous is deliberately generous — GhanaGeo is free and
// unauthenticated use is a supported path, not a trial.
var (
	anonymousAllowance     = Allowance{BurstUnits: 120, RefillPerSecond: 2, Window: time.Hour}
	authenticatedAllowance = Allowance{BurstUnits: 600, RefillPerSecond: 10, Window: time.Hour}
	// Sandbox is strictly tighter than production (Spec §15).
	sandboxAllowance = Allowance{BurstUnits: 60, RefillPerSecond: 1, Window: time.Hour}
	// A steward may lift a single application's ceiling on DOCUMENTED NEED —
	// research, humanitarian or government use — never on payment (§24 F4).
	elevatedAllowance = Allowance{BurstUnits: 3000, RefillPerSecond: 50, Window: time.Hour}
)

// AllowanceFor resolves what a caller may consume.
//
// It takes ONLY the identity and the environment. It has no access to
// donation, sponsorship or payment state, and it must never gain any:
// that is what makes "free" structural rather than a promise. See
// TestAllowanceIgnoresSponsorship.
func AllowanceFor(id Identity, sandbox bool) Allowance {
	if sandbox {
		return sandboxAllowance
	}
	if id.Anonymous {
		return anonymousAllowance
	}
	if id.Key != nil && id.Key.Elevated {
		return elevatedAllowance
	}
	return authenticatedAllowance
}
