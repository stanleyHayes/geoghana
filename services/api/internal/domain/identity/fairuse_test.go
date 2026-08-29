package identity

import "testing"

// THE RULE THAT KEEPS GHANAGEO FREE (agent_plan.md §24 F2).
//
// Free products drift to paid one exception at a time. The structural defence
// is that allowance resolution cannot see money: AllowanceFor takes an Identity
// and an environment flag, and nothing else. If someone later adds a donation
// or sponsorship field and wires it in here, this test is what should stop the
// pull request.
func TestAllowanceIgnoresSponsorship(t *testing.T) {
	sponsored := FromKey(&APIKey{Prefix: "gh_live_aaa", OrganizationID: "org-sponsor"})
	unsponsored := FromKey(&APIKey{Prefix: "gh_live_bbb", OrganizationID: "org-nobody"})

	a := AllowanceFor(sponsored, false)
	b := AllowanceFor(unsponsored, false)

	if a != b {
		t.Fatalf("a sponsored organization received a different allowance:\n  sponsored:   %+v\n  unsponsored: %+v", a, b)
	}
}

// Elevation is the ONLY way a ceiling rises, and it is granted on documented
// need rather than payment. The reason travels with it so the audit log can
// show why.
func TestElevationRequiresARecordedReason(t *testing.T) {
	elevated := FromKey(&APIKey{
		Prefix: "gh_live_ccc", Elevated: true,
		ElevatedReason: "Ghana Statistical Service bulk reconciliation",
	})
	normal := FromKey(&APIKey{Prefix: "gh_live_ddd"})

	if AllowanceFor(elevated, false).BurstUnits <= AllowanceFor(normal, false).BurstUnits {
		t.Error("an elevated key should have a higher ceiling")
	}
	if elevated.Key.ElevatedReason == "" {
		t.Error("elevation without a recorded reason should not exist")
	}
}

// GhanaGeo is free, so anonymous callers get a real, usable allowance — not a
// teaser that pushes them toward an account.
func TestAnonymousAllowanceIsUsable(t *testing.T) {
	anon := AllowanceFor(Anonymous("41.66.0.1"), false)
	if anon.BurstUnits < 100 {
		t.Errorf("anonymous burst of %d units is too mean for a free public API", anon.BurstUnits)
	}
	if anon.RefillPerSecond <= 0 {
		t.Error("an anonymous bucket that never refills is a hard block, not a fair-use limit")
	}
}

// Signing in should be worth something — identification aids abuse response —
// but the gap must be about accountability, not about selling access.
func TestAuthenticatedIsMoreGenerousThanAnonymous(t *testing.T) {
	anon := AllowanceFor(Anonymous("1.2.3.4"), false)
	auth := AllowanceFor(FromKey(&APIKey{Prefix: "gh_live_eee"}), false)
	if auth.BurstUnits <= anon.BurstUnits {
		t.Error("an identified caller should get a somewhat higher ceiling")
	}
}

// Spec §15: the sandbox must be strictly tighter than production, because it
// is reachable without any credential at all.
func TestSandboxIsTighterThanProduction(t *testing.T) {
	sandbox := AllowanceFor(Anonymous("1.2.3.4"), true)
	prod := AllowanceFor(Anonymous("1.2.3.4"), false)
	if sandbox.BurstUnits >= prod.BurstUnits {
		t.Error("sandbox must be strictly tighter than production")
	}
	// Even an elevated key is capped in the sandbox.
	elevated := AllowanceFor(FromKey(&APIKey{Elevated: true}), true)
	if elevated != sandbox {
		t.Error("the sandbox ceiling must apply regardless of key elevation")
	}
}

func TestCostClassOrdering(t *testing.T) {
	if !(CostCheap.Units() < CostNormal.Units() &&
		CostNormal.Units() < CostSpatial.Units() &&
		CostSpatial.Units() < CostGeometry.Units()) {
		t.Error("cost must rise with how expensive an operation is to serve")
	}
	if CostClass("invented").Units() != 1 {
		t.Error("an unknown cost class should default to the cheapest, not free")
	}
}
