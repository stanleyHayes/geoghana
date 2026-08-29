// Package auth resolves the caller and enforces scope and fair-use limits.
//
// Applied once as middleware to every transport, so REST, GraphQL and gRPC
// share one implementation and cannot drift (Spec §6).
package auth

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

type ctxKey int

const identityKey ctxKey = iota

// KeyLookup resolves a public prefix to a stored key. Returns nil when unknown.
type KeyLookup interface {
	ByPrefix(ctx context.Context, prefix string) (*identity.APIKey, error)
	MarkUsed(ctx context.Context, prefix string, at time.Time) error
}

// Limiter charges a request against the caller's fair-use bucket.
type Limiter interface {
	Allow(ctx context.Context, id identity.Identity, a identity.Allowance, cost identity.CostClass) (Decision, error)
}

type Decision struct {
	Allowed    bool
	Remaining  int
	Limit      int
	RetryAfter int
	Degraded   bool
}

type Authenticator struct {
	keys    KeyLookup
	limiter Limiter
	sandbox bool
	now     func() time.Time
}

func New(keys KeyLookup, limiter Limiter, sandbox bool) *Authenticator {
	return &Authenticator{keys: keys, limiter: limiter, sandbox: sandbox, now: time.Now}
}

// FromContext returns the resolved caller. It always succeeds after the
// middleware has run: anonymous is an identity, not an absence.
func FromContext(ctx context.Context) identity.Identity {
	if id, ok := ctx.Value(identityKey).(identity.Identity); ok {
		return id
	}
	return identity.Anonymous("unknown")
}

// Resolve turns an Authorization header into an Identity.
//
// A missing header is NOT an error: GhanaGeo is free and anonymous access is a
// supported path. A malformed or invalid key IS an error, because the caller
// clearly intended to authenticate and silently downgrading them to anonymous
// would hide the mistake until they hit an unexpected limit.
func (a *Authenticator) Resolve(ctx context.Context, header, ip, origin string) (identity.Identity, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return identity.Anonymous(ip), nil
	}

	parsed, err := identity.ParseKey(header)
	if err != nil {
		return identity.Identity{}, apierr.New(apierr.Unauthenticated,
			"The Authorization header is not a valid GhanaGeo API key.")
	}

	key, err := a.keys.ByPrefix(ctx, parsed.Prefix)
	if err != nil {
		return identity.Identity{}, apierr.Wrap(apierr.Internal, "Could not verify the API key.", err)
	}
	// An unknown prefix and a wrong secret return the SAME error, so the
	// response cannot be used to enumerate which prefixes exist.
	if key == nil {
		return identity.Identity{}, apierr.New(apierr.Unauthenticated, "Invalid API key.")
	}
	if err := identity.VerifySecret(parsed.Secret, key.SecretHash); err != nil {
		return identity.Identity{}, apierr.New(apierr.Unauthenticated, "Invalid API key.")
	}
	if err := key.Usable(a.now()); err != nil {
		return identity.Identity{}, apierr.New(apierr.KeyRevoked,
			"This API key is no longer valid.")
	}
	if origin != "" && !key.OriginAllowed(origin) {
		return identity.Identity{}, apierr.
			New(apierr.OriginNotAllowed, "This origin is not on the key's allow-list.").
			WithDetail("origin", origin)
	}

	// Best-effort: a failure to record last-used must never fail the request.
	_ = a.keys.MarkUsed(ctx, key.Prefix, a.now())

	return identity.FromKey(key), nil
}

// Middleware resolves the caller, charges the fair-use bucket and attaches the
// identity to the request context.
func (a *Authenticator) Middleware(costOf func(*http.Request) identity.CostClass) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := a.Resolve(r.Context(), r.Header.Get("Authorization"), clientIP(r), r.Header.Get("Origin"))
			if err != nil {
				writeAuthErr(w, r, err)
				return
			}

			allowance := identity.AllowanceFor(id, a.sandbox)
			cost := costOf(r)

			d, lerr := a.limiter.Allow(r.Context(), id, allowance, cost)
			if lerr != nil {
				writeAuthErr(w, r, apierr.Wrap(apierr.Internal, "Could not apply fair-use limits.", lerr))
				return
			}

			// Standard limit headers on every response, not only on a 429, so a
			// well-behaved client can slow down before being refused.
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(d.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(max(0, d.Remaining)))
			w.Header().Set("X-RateLimit-Cost", strconv.Itoa(cost.Units()))

			if !d.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(max(1, d.RetryAfter)))
				writeAuthErr(w, r, apierr.
					New(apierr.RateLimitExceeded, "Request rate exceeded.").
					WithDetail("retryAfterSeconds", max(1, d.RetryAfter)).
					WithDetail("limit", d.Limit))
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey, id)))
		})
	}
}

// RequireScope rejects a caller lacking the scope an operation needs.
func RequireScope(s identity.Scope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !FromContext(r.Context()).HasScope(s) {
				writeAuthErr(w, r, apierr.
					New(apierr.PermissionDenied, "This key lacks the required scope.").
					WithDetail("requiredScope", string(s)))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// writeAuthErr is set by the transport so this package stays free of
// transport-specific rendering.
var writeAuthErr = func(w http.ResponseWriter, r *http.Request, err error) {
	e := apierr.From(err)
	http.Error(w, e.Message, e.Code.HTTPStatus())
}

// SetErrorWriter lets the REST transport install its Spec §19 envelope.
func SetErrorWriter(f func(w http.ResponseWriter, r *http.Request, err error)) {
	writeAuthErr = f
}

func clientIP(r *http.Request) string {
	// chi's RealIP middleware has already normalised X-Forwarded-For.
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return strings.Trim(host, "[]")
}
