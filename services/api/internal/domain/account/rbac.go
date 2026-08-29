package account

// Role-based access control (Spec §17.1, agent_plan.md Appendix C).
//
// Permissions are enforced HERE and checked by the API. UI gating is
// presentation only: hiding a button is a courtesy to the user, never a
// security control, and an admin console that relies on it is one crafted
// request away from an unauthorised write.

// Permission is a single capability from the Appendix C matrix.
type Permission string

const (
	PermViewGeography    Permission = "geography:view"
	PermEditGeography    Permission = "geography:edit"
	PermEditGeometry     Permission = "geometry:edit"
	PermProposeChange    Permission = "change:propose"
	PermReviewChange     Permission = "change:review"
	PermRunImport        Permission = "import:run"
	PermPublishRelease   Permission = "release:publish"
	PermRollbackRelease  Permission = "release:rollback"
	PermViewOrganization Permission = "organization:view"
	PermSuspendKey       Permission = "key:suspend"
	PermEditPlans        Permission = "plan:edit"
	PermManageRoles      Permission = "role:manage"
)

// permissions is Appendix C transcribed exactly.
//
// Written as an explicit allow-list per role rather than derived from a
// hierarchy: Ghana's steward roles are not neatly nested — a Developer
// Support user can view organizations that a Data Admin cannot — so any
// "higher role implies lower" shortcut would quietly grant the wrong thing.
var permissions = map[Role]map[Permission]bool{
	RoleSuperAdmin: {
		PermViewGeography: true, PermEditGeography: true, PermEditGeometry: true,
		PermProposeChange: true, PermReviewChange: true, PermRunImport: true,
		PermPublishRelease: true, PermRollbackRelease: true,
		PermViewOrganization: true, PermSuspendKey: true,
		PermEditPlans: true, PermManageRoles: true,
	},
	RoleDataAdmin: {
		PermViewGeography: true, PermEditGeography: true, PermEditGeometry: true,
		PermProposeChange: true, PermReviewChange: true, PermRunImport: true,
		PermPublishRelease: true, PermRollbackRelease: true,
	},
	RoleDataReviewer: {
		PermViewGeography: true, PermProposeChange: true, PermReviewChange: true,
	},
	RoleDataContributor: {
		PermViewGeography: true, PermProposeChange: true,
		// "Run import job (mapping only)" in Appendix C. Mapping-only is a
		// narrower capability than the import permission grants, so it is NOT
		// granted here; it lands with the import screens that can express the
		// distinction. Granting the broad permission now would be a quiet
		// privilege escalation.
	},
	RoleDeveloperSupport: {
		PermViewGeography: true, PermViewOrganization: true, PermSuspendKey: true,
	},
	RoleSecurityAuditor: {
		// Read-only by definition. An auditor that can change what it audits
		// is not an auditor.
		PermViewGeography: true, PermViewOrganization: true,
	},
	RoleDeveloper: {
		// An ordinary developer has no admin console access at all.
	},
}

// Can reports whether a role holds a permission.
//
// Unknown roles and unknown permissions both return false. Denying by default
// means a role added without updating this table can read nothing rather than
// inheriting everything.
func Can(r Role, p Permission) bool {
	return permissions[r][p]
}

// Permissions lists what a role holds, for the UI to gate on. The API checks
// Can independently — this is only so the console does not show a control
// that would be refused.
func Permissions(r Role) []Permission {
	held := permissions[r]
	out := make([]Permission, 0, len(held))
	for _, p := range []Permission{
		PermViewGeography, PermEditGeography, PermEditGeometry,
		PermProposeChange, PermReviewChange, PermRunImport,
		PermPublishRelease, PermRollbackRelease, PermViewOrganization,
		PermSuspendKey, PermEditPlans, PermManageRoles,
	} {
		if held[p] {
			out = append(out, p)
		}
	}
	return out
}
