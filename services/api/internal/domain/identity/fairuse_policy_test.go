package identity

import (
	"testing"
	"time"
)

func TestFairUseOverrideRequiresDocumentedTemporaryGrant(t *testing.T) {
	now := time.Now().UTC()
	valid := FairUseOverride{ID: "ovr-1", ApplicationID: "app-1", Allowance: elevatedAllowance, CostCeilings: DefaultFairUsePolicy().CostCeilings, Enabled: true, Reason: "national census reconciliation", ActorID: "admin-1", CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid override rejected: %v", err)
	}
	for name, mutate := range map[string]func(*FairUseOverride){
		"reason": func(o *FairUseOverride) { o.Reason = "" },
		"actor":  func(o *FairUseOverride) { o.ActorID = "" },
		"expiry": func(o *FairUseOverride) { o.ExpiresAt = o.CreatedAt },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if candidate.Validate() == nil {
				t.Fatal("invalid override accepted")
			}
		})
	}
}

func TestFairUsePolicyRequiresEveryCostClass(t *testing.T) {
	now := time.Now().UTC()
	p := DefaultFairUsePolicy()
	p.CreatedAt = now
	p.EffectiveFrom = now
	p.Reason = "initial policy"
	delete(p.CostCeilings, CostGeometry)
	if p.Validate() == nil {
		t.Fatal("policy without geometry ceiling accepted")
	}
}

func TestFairUsePolicyKeepsSandboxNoMorePermissive(t *testing.T) {
	now := time.Now().UTC()
	p := DefaultFairUsePolicy()
	p.CreatedAt = now
	p.EffectiveFrom = now
	p.Sandbox.BurstUnits = p.Authenticated.BurstUnits + 1
	if p.Validate() == nil {
		t.Fatal("policy allowed sandbox to exceed authenticated allowance")
	}
}

func TestFairUseModelContainsNoCommercialInput(t *testing.T) {
	_ = FairUsePolicy{Anonymous: Allowance{}, Authenticated: Allowance{}, Sandbox: Allowance{}}
	_ = FairUseOverride{ApplicationID: "app", Reason: "operational need", ActorID: "operator"}
}
