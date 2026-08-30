package identity

import (
	"strings"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
)

// AdminListFilter is the stable, cursor-based contract used by the developer
// support console. Cursor values are opaque to HTTP clients and are always
// applied after the search filters, preventing page drift and tenant leakage.
type AdminListFilter struct {
	Cursor         string
	Query          string
	OrganizationID string
	ApplicationID  string
	State          string
	Limit          int
}

func (f AdminListFilter) Normalize() AdminListFilter {
	f.Cursor = strings.TrimSpace(f.Cursor)
	f.Query = strings.TrimSpace(f.Query)
	f.OrganizationID = strings.TrimSpace(f.OrganizationID)
	f.ApplicationID = strings.TrimSpace(f.ApplicationID)
	f.State = strings.ToLower(strings.TrimSpace(f.State))
	if f.Limit <= 0 {
		f.Limit = 25
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	return f
}

type AdminPage[T any] struct {
	Data       []T
	NextCursor string
	Total      int64
}

// DeveloperAccount intentionally excludes credentials, MFA material and
// session state. Admin search must never become a credential export path.
type DeveloperAccount struct {
	ID            string
	Email         string
	EmailVerified bool
	Role          account.Role
	Disabled      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// AdminKey is the support-safe representation of an API key. In particular,
// SecretHash is absent: even a one-way digest is authentication material and
// has no operational value in the console.
type AdminKey struct {
	ID, ApplicationID, OrganizationID, Name string
	Class                                   KeyClass
	Environment                             Environment
	Prefix                                  string
	Scopes                                  []Scope
	AllowedOrigins, AllowedIPs              []string
	CreatedAt                               time.Time
	ExpiresAt, LastUsedAt                   *time.Time
	RevokedAt, SuspendedAt                  *time.Time
	RevokedReason, SuspendedReason          string
}

func RedactKey(k APIKey) AdminKey {
	return AdminKey{
		ID: k.ID, ApplicationID: k.ApplicationID, OrganizationID: k.OrganizationID,
		Name: k.Name, Class: k.Class, Environment: k.Environment, Prefix: k.Prefix,
		Scopes:         append([]Scope(nil), k.Scopes...),
		AllowedOrigins: append([]string(nil), k.AllowedOrigins...), AllowedIPs: append([]string(nil), k.AllowedIPs...),
		CreatedAt: k.CreatedAt, ExpiresAt: k.ExpiresAt, LastUsedAt: k.LastUsedAt,
		RevokedAt: k.RevokedAt, SuspendedAt: k.SuspendedAt,
		RevokedReason: k.RevokedReason, SuspendedReason: k.SuspendedReason,
	}
}
