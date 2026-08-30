// Package identity models developers, organizations, applications and API keys.
//
// GhanaGeo is free (agent_plan.md §24), so nothing here has a price. Keys exist
// to identify a consumer for fair-use accounting and abuse response, never to
// sell capability. In particular, no type in this package has a "plan" or
// "tier" field, and quota resolution must not be able to read donation or
// sponsorship state — see the test in fairuse_test.go.
package identity

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"
)

var (
	ErrEmptyName            = errors.New("name must not be empty")
	ErrOwnerRequired        = errors.New("organization owner is required")
	ErrOrganizationRequired = errors.New("organization is required")
	ErrApplicationRequired  = errors.New("application is required")
	ErrInvalidClass         = errors.New("invalid key class")
	ErrInvalidEnvironment   = errors.New("invalid key environment")
	ErrNotFound             = errors.New("identity resource not found")
	ErrUnknownScope         = errors.New("unknown scope")
	ErrUnsafeBrowserKey     = errors.New("unsafe scope combination for a browser key")
	ErrOriginRequired       = errors.New("a browser key requires at least one allowed origin")
	ErrKeyRevoked           = errors.New("key is revoked")
	ErrKeySuspended         = errors.New("key is suspended")
	ErrKeyExpired           = errors.New("key has expired")
	ErrInvalidCursor        = errors.New("invalid cursor")
	ErrInvalidKeyTransition = errors.New("invalid key state transition")
)

// Scope is a capability an API key may hold (Spec §12.3).
type Scope string

const (
	ScopeLocationsRead  Scope = "locations:read"
	ScopeSearchRead     Scope = "search:read"
	ScopeGeocodeRead    Scope = "geocode:read"
	ScopeBoundariesRead Scope = "boundaries:read"
	ScopeDatasetsRead   Scope = "datasets:read"
	ScopeGraphQLAccess  Scope = "graphql:access"
	ScopeGRPCAccess     Scope = "grpc:access"
)

var allScopes = map[Scope]struct{}{
	ScopeLocationsRead: {}, ScopeSearchRead: {}, ScopeGeocodeRead: {},
	ScopeBoundariesRead: {}, ScopeDatasetsRead: {}, ScopeGraphQLAccess: {},
	ScopeGRPCAccess: {},
}

// AnonymousScopes are what an unauthenticated caller gets. GhanaGeo is free,
// so this is deliberately generous: everything a browser key can do. It is a
// first-class path, not a degraded one.
var AnonymousScopes = []Scope{
	ScopeLocationsRead, ScopeSearchRead, ScopeGeocodeRead,
	ScopeBoundariesRead, ScopeDatasetsRead, ScopeGraphQLAccess,
}

func ParseScope(s string) (Scope, error) {
	sc := Scope(strings.TrimSpace(strings.ToLower(s)))
	if _, ok := allScopes[sc]; !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownScope, s)
	}
	return sc, nil
}

// KeyClass determines where a key may safely be used (Spec §32.5).
type KeyClass string

const (
	// ClassBrowser keys ship in client bundles, so they are origin-restricted
	// and cannot hold server-only scopes.
	ClassBrowser KeyClass = "BROWSER"
	// ClassServer keys stay on a server and may hold every scope.
	ClassServer KeyClass = "SERVER"
	// ClassTest keys work only against test/sandbox data.
	ClassTest KeyClass = "TEST"
)

// serverOnlyScopes may never be granted to a browser key: a key in a client
// bundle is public by construction, and gRPC access implies a backend.
var serverOnlyScopes = map[Scope]struct{}{
	ScopeGRPCAccess: {},
}

// Environment separates live traffic from test traffic.
type Environment string

const (
	EnvLive Environment = "live"
	EnvTest Environment = "test"
)

// Organization groups applications under an owner.
//
// Note what is NOT here: no plan, no tier, no billing customer id. Sponsorship
// is recorded separately and is recognition only (§24 F3).
type Organization struct {
	ID        string
	Name      string
	OwnerID   string
	Members   []OrganizationMember
	CreatedAt time.Time
}

type OrganizationRole string

const (
	OrganizationOwner      OrganizationRole = "OWNER"
	OrganizationAdmin      OrganizationRole = "ADMIN"
	OrganizationMemberRole OrganizationRole = "MEMBER"
	OrganizationViewer     OrganizationRole = "VIEWER"
)

type OrganizationMember struct {
	AccountID string
	Email     string
	Role      OrganizationRole
	JoinedAt  time.Time
}

type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "PENDING"
	InvitationAccepted InvitationStatus = "ACCEPTED"
	InvitationRevoked  InvitationStatus = "REVOKED"
)

type OrganizationInvitation struct {
	ID             string
	OrganizationID string
	Email          string
	Role           OrganizationRole
	TokenHash      string
	Status         InvitationStatus
	InvitedBy      string
	CreatedAt      time.Time
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
}

func (o Organization) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return ErrEmptyName
	}
	if strings.TrimSpace(o.OwnerID) == "" {
		return ErrOwnerRequired
	}
	return nil
}

// Application is a single integration owned by an organization.
type Application struct {
	ID             string
	OrganizationID string
	Name           string
	Description    string
	Environments   []Environment
	Domains        []string
	CallbackURL    string
	Plan           string
	CreatedAt      time.Time
}

func (a Application) Validate() error {
	if strings.TrimSpace(a.Name) == "" {
		return ErrEmptyName
	}
	if strings.TrimSpace(a.OrganizationID) == "" {
		return ErrOrganizationRequired
	}
	if a.Plan != "" && a.Plan != "free" {
		return errors.New("GhanaGeo applications use the free plan")
	}
	return nil
}

// APIKey is the stored record. The secret itself is never stored: only a
// non-secret prefix for lookup and display, plus a one-way digest (Spec §12.2).
type APIKey struct {
	ID             string
	ApplicationID  string
	OrganizationID string
	Name           string
	Class          KeyClass
	Environment    Environment
	// Prefix is the public, greppable portion, e.g. "gh_live_7f3a9c2b".
	Prefix string
	// SecretHash is argon2id over the secret. The secret is shown once and
	// then unrecoverable.
	SecretHash      string
	Scopes          []Scope
	AllowedOrigins  []string
	AllowedIPs      []string
	CreatedAt       time.Time
	ExpiresAt       *time.Time
	LastUsedAt      *time.Time
	RevokedAt       *time.Time
	RevokedReason   string
	SuspendedAt     *time.Time
	SuspendedReason string

	// Elevated lifts this key's fair-use ceiling. A steward sets it on
	// DOCUMENTED NEED — research, humanitarian or government use — and the
	// reason is recorded in the audit log. It is never granted for payment
	// (agent_plan.md §24 F4), and there is deliberately no amount, invoice or
	// sponsor field anywhere near it.
	Elevated       bool
	ElevatedReason string
}

// Validate enforces the safety rules that make a browser key safe to publish.
func (k APIKey) Validate() error {
	if strings.TrimSpace(k.Name) == "" {
		return ErrEmptyName
	}
	if strings.TrimSpace(k.ApplicationID) == "" {
		return ErrApplicationRequired
	}
	if strings.TrimSpace(k.OrganizationID) == "" {
		return ErrOrganizationRequired
	}
	switch k.Class {
	case ClassBrowser, ClassServer, ClassTest:
	default:
		return ErrInvalidClass
	}
	switch k.Environment {
	case EnvLive, EnvTest:
	default:
		return ErrInvalidEnvironment
	}
	if k.Class == ClassTest && k.Environment != EnvTest {
		return ErrInvalidEnvironment
	}
	for _, s := range k.Scopes {
		if _, ok := allScopes[s]; !ok {
			return fmt.Errorf("%w: %q", ErrUnknownScope, s)
		}
	}
	if k.Class == ClassBrowser {
		for _, s := range k.Scopes {
			if _, bad := serverOnlyScopes[s]; bad {
				return fmt.Errorf("%w: %q cannot be granted to a browser key", ErrUnsafeBrowserKey, s)
			}
		}
		if len(k.AllowedOrigins) == 0 {
			return ErrOriginRequired
		}
	}
	return nil
}

// Usable reports whether the key may authenticate right now.
func (k APIKey) Usable(now time.Time) error {
	if k.RevokedAt != nil {
		return ErrKeyRevoked
	}
	if k.SuspendedAt != nil {
		return ErrKeySuspended
	}
	if k.ExpiresAt != nil && now.After(*k.ExpiresAt) {
		return ErrKeyExpired
	}
	return nil
}

func (k APIKey) HasScope(s Scope) bool {
	for _, have := range k.Scopes {
		if have == s {
			return true
		}
	}
	return false
}

// OriginAllowed checks a browser key's origin allow-list. A non-browser key is
// not origin-restricted, and an empty list on a browser key denies everything —
// failing closed, since Validate should have prevented that state.
func (k APIKey) OriginAllowed(origin string) bool {
	if k.Class != ClassBrowser {
		return true
	}
	for _, o := range k.AllowedOrigins {
		if strings.EqualFold(strings.TrimRight(o, "/"), strings.TrimRight(origin, "/")) {
			return true
		}
	}
	return false
}

// IPAllowed enforces an optional exact-IP or CIDR allow-list. An empty list
// means unrestricted; an invalid stored rule fails closed.
func (k APIKey) IPAllowed(ip string) bool {
	if len(k.AllowedIPs) == 0 {
		return true
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}
	for _, rule := range k.AllowedIPs {
		rule = strings.TrimSpace(rule)
		if prefix, err := netip.ParsePrefix(rule); err == nil && prefix.Contains(addr) {
			return true
		}
		if allowed, err := netip.ParseAddr(rule); err == nil && allowed == addr {
			return true
		}
	}
	return false
}

// Identity is the resolved caller: either an authenticated key or the
// deliberate anonymous identity. Anonymous is an identity, not an absence
// (Spec §1.2).
type Identity struct {
	Anonymous      bool
	Key            *APIKey
	OrganizationID string
	Scopes         []Scope
	// RateKey is the hierarchical bucket this caller is limited against.
	RateKey string
}

func Anonymous(ip string) Identity {
	return Identity{Anonymous: true, Scopes: AnonymousScopes, RateKey: "ip:" + ip}
}

func FromKey(k *APIKey) Identity {
	return Identity{
		Key:            k,
		OrganizationID: k.OrganizationID,
		Scopes:         k.Scopes,
		RateKey:        "key:" + k.Prefix,
	}
}

func (i Identity) HasScope(s Scope) bool {
	for _, have := range i.Scopes {
		if have == s {
			return true
		}
	}
	return false
}
