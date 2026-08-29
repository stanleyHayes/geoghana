# Security review — 2026-08-29

Scope: `services/api` (Go), the four Next.js apps, and production npm
dependencies. Run locally against the current tree.

**Result: no unresolved critical or high findings.** Two genuine issues were
found and fixed; everything else is triaged below with the reason it is not a
finding.

## Tools

| Tool | Scope | Result |
|---|---|---|
| `govulncheck` (golang.org/x/vuln) | Go, reachability-aware | **0 reachable** |
| `gosec` | Go static analysis, 84 files / 19,965 lines | 27 raised, 0 unresolved |
| `pnpm audit --prod` | production npm dependencies | **0 vulnerabilities** |
| `go test ./...` | the §22.3 suite | pass |

## Fixed

### 1. Unbounded argon2 parameters read from a stored digest — HIGH

`VerifyPassword` and `VerifySecret` parsed `m=`, `t=` and `p=` out of the
stored hash string and passed them straight to `argon2.IDKey`. A corrupted or
tampered row carrying `m=4294967295` would request a **four-terabyte
allocation**, and a single sign-in attempt — or, for API keys, a single
authenticated request — would take the process down.

Both now bound every parameter before use. The ceilings are generous relative
to what we write (1 GiB against 19 MiB), so raising the work factor later
still verifies existing hashes, but nothing absurd gets through. Covered by
`TestVerifyPasswordBoundsStoredParameters` and
`TestVerifySecretBoundsStoredParameters`, including that a *raised* cost still
verifies — a bound tight enough to break future rehashing would be its own bug.

### 2. `go.mongodb.org/mongo-driver` GSSAPI heap out-of-bounds read (GO-2026-5327)

Pulled in transitively by `paulmach/orb` and `paulmach/osm` for BSON encoding.
Not reachable from our code — we use the v2 driver — but required at module
level. Bumped to the patched **v1.17.9**, which clears the advisory rather than
leaving it to be re-triaged by whoever reads the next scan.

## Triaged, not findings

**`golang.org/x/crypto/openpgp` unmaintained (GO-2026-5932)** — module-level
only. `go list -deps` confirms **zero** openpgp packages in the build graph; we
require `x/crypto` for argon2. There is no fixed version because the advisory
is that the package is unmaintained.

**G115 integer conversions (5 remaining)** — `totp.go` converts a Unix
timestamp to a time step; `accounts.go` converts a stored `int64` step already
guarded against negatives; the two in the argon2 verifiers convert a key length
now bounded to ≤1024 by fix 1. gosec cannot see the guards.

**G124 cookie attributes (2)** — false positive. The session cookie sets
`HttpOnly`, `SameSite=Lax` and `Secure`, the last of which is set from the
request scheme so local HTTP still works. gosec does not evaluate the
conditional.

**G304 file inclusion via variable (10)** — nine are `os.Open` on an operator
supplied `--file` flag in the admin CLI, which is not untrusted input. The
tenth, `dataset/usecases.go`, IS request-reachable and is already guarded
twice: the filename is validated on write and on read, and the resolved path
is confirmed to sit under the export directory afterwards, so a symlink inside
it cannot escape either.

**G301/G306 permissions** — export directory `0755`, files `0644`. These are
public downloads.

**G104 unhandled errors (5)** — deferred `Close` on cursors and files.

## Not covered here

Hosted scanning is blocked on account state, not on code:

- **GitHub Actions** run `33272922624` was rejected before runner allocation
  because account payments failed or the spending limit was reached.
- **CodeQL** is not enabled for this private repository.

Neither is a finding, and neither can be resolved from inside the codebase.
Both need action on the GitHub account.
