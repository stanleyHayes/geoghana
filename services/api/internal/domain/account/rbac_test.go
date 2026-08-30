package account

import "testing"

// The permission matrix is transcribed from agent_plan.md Appendix C. This
// test IS the transcription check: if the table and the plan disagree, one of
// them is wrong and a steward gets a capability nobody granted.
func TestPermissionMatrixMatchesAppendixC(t *testing.T) {
	// Row per permission, in Appendix C's column order:
	// SuperAdmin, DataAdmin, DataReviewer, DataContributor, DevSupport, Auditor
	roles := []Role{
		RoleSuperAdmin, RoleDataAdmin, RoleDataReviewer,
		RoleDataContributor, RoleDeveloperSupport, RoleSecurityAuditor,
	}
	matrix := map[Permission][6]bool{
		PermViewGeography:    {true, true, true, true, true, true},
		PermEditGeography:    {true, true, false, false, false, false},
		PermEditGeometry:     {true, true, false, false, false, false},
		PermProposeChange:    {true, true, true, true, false, false},
		PermReviewChange:     {true, true, true, false, false, false},
		PermPublishRelease:   {true, true, false, false, false, false},
		PermRollbackRelease:  {true, true, false, false, false, false},
		PermViewOrganization: {true, false, false, false, true, true},
		PermSuspendKey:       {true, false, false, false, true, false},
		PermEditPlans:        {true, false, false, false, false, false},
		PermManageRoles:      {true, false, false, false, false, false},
		PermViewOperations:   {true, true, true, true, true, true},
		PermViewAudit:        {true, true, false, false, false, true},
	}
	for perm, want := range matrix {
		for i, role := range roles {
			if got := Can(role, perm); got != want[i] {
				t.Errorf("Can(%s, %s) = %v, want %v (Appendix C)", role, perm, got, want[i])
			}
		}
	}
}

// Deny by default, in both directions.
func TestUnknownRolesAndPermissionsAreDenied(t *testing.T) {
	if Can("SOME_FUTURE_ROLE", PermViewGeography) {
		t.Error("an unknown role was granted a permission")
	}
	if Can(RoleSuperAdmin, "some:invented:permission") {
		t.Error("an unknown permission was granted")
	}
	// An ordinary developer has no console access whatsoever.
	for _, p := range []Permission{PermViewGeography, PermEditGeography, PermViewOrganization, PermViewOperations, PermViewAudit} {
		if Can(RoleDeveloper, p) {
			t.Errorf("a plain developer holds %s", p)
		}
	}
}

// A security auditor that can change what it audits is not an auditor.
func TestSecurityAuditorIsReadOnly(t *testing.T) {
	for _, p := range []Permission{
		PermEditGeography, PermEditGeometry, PermProposeChange, PermReviewChange,
		PermRunImport, PermPublishRelease, PermRollbackRelease, PermSuspendKey,
		PermEditPlans, PermManageRoles,
	} {
		if Can(RoleSecurityAuditor, p) {
			t.Errorf("SECURITY_AUDITOR holds the mutating permission %s", p)
		}
	}
}

// Roles are not a hierarchy: Developer Support can view organizations and
// Data Admin cannot. Any "higher role implies lower" shortcut breaks here.
func TestRolesAreNotHierarchical(t *testing.T) {
	if Can(RoleDataAdmin, PermViewOrganization) {
		t.Error("DATA_ADMIN can view organizations — Appendix C says it cannot")
	}
	if !Can(RoleDeveloperSupport, PermViewOrganization) {
		t.Error("DEVELOPER_SUPPORT cannot view organizations")
	}
	if Can(RoleDeveloperSupport, PermEditGeography) {
		t.Error("DEVELOPER_SUPPORT can edit geography")
	}
}

func TestPermissionsListingMatchesCan(t *testing.T) {
	for _, r := range []Role{
		RoleSuperAdmin, RoleDataAdmin, RoleDataReviewer, RoleDataContributor,
		RoleDeveloperSupport, RoleSecurityAuditor, RoleDeveloper,
	} {
		for _, p := range Permissions(r) {
			if !Can(r, p) {
				t.Errorf("Permissions(%s) listed %s but Can says no", r, p)
			}
		}
	}
}
