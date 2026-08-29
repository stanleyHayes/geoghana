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

## Addendum — the sandbox gRPC bridge (post-commit review)

The sign-off above was written before commit `89879c4`. That commit landed
`apps/sandbox/src/app/api/grpc/route.ts`, a bridge that lets an anonymous
visitor drive the real gRPC service from a browser. It was verified to build
and typecheck, **not** reviewed for logic, and a background review of the
pushed commit then reported three findings against it. All three were real,
and all three were reproduced against the running server before being fixed.

### 1. The rate limit could be reset at will — confirmed

The per-caller window was keyed on `X-Forwarded-For`, a header the caller
supplies. Exhausting the budget and then incrementing the header produced a
fresh one:

```
31st request as 7.7.7.7   → 429
same attacker, new header → 200
```

Fixed by adding a **global** window (300/min) that no header can influence,
checked before the per-caller one. The per-caller limit is retained as a
courtesy to honest callers behind a shared IP, but it is no longer what
bounds the endpoint. Re-tested with 320 requests, each from a different
spoofed IP: the ceiling engaged at ~300.

### 2. The 16 KiB body cap could be skipped entirely — confirmed

The cap tested the `Content-Length` header. That header is optional, and a
chunked request omits it, so `Number(null ?? 0)` evaluated to 0 and passed.
A 200 KB body went straight through a 16 KiB limit:

```
declared content-length (200KB) → 413
chunked, no content-length      → 200   ← bypass
```

Fixed by reading the body through its stream and counting the bytes that
actually arrive, aborting the moment the real total exceeds the cap — so an
oversized body is never fully buffered either. Both forms now return 413.

### 3. Unbounded resource growth — confirmed, and worse than reported

Three distinct problems, not one:

- The rate-limit map was only ever written to, never swept. Every spoofed
  identity added a permanent entry. Now swept per request and hard-capped;
  past the cap, per-identity tracking is abandoned and the global ceiling
  carries the load.
- Streams had no time limit. A client that opened one and simply stopped
  reading held a gRPC call open indefinitely. Now capped at 30s and at 8
  concurrent streams; verified closing at 30.1s, and the 11th concurrent
  stream is refused.
- **Not in the original report:** the proto was parsed and a new gRPC channel
  opened *per request*. `loadSync` is blocking and measured **43.9ms**, so
  every request stalled Node's single event loop for that long — a
  self-inflicted denial of service that needed no attacker. The parsed
  contract and the channel are now created once and reused, as a gRPC channel
  is designed to be. Median request time fell from tens of ms to **3ms**.

### A misleading status code, found while verifying

Load-testing the fix produced 171 `502`s out of 295. The upstream was not
broken: it was the Go API's own rate limiter answering `RESOURCE_EXHAUSTED`,
which the bridge collapsed into `502 Bad Gateway` along with every other
non-`INVALID_ARGUMENT` code. That would send whoever is debugging after a
failing service that is in fact working exactly as designed. gRPC statuses are
now mapped to their HTTP equivalents; `RESOURCE_EXHAUSTED` surfaces as `429`.
The same 200-request storm now returns 121 × 200 and 79 × 429, and no 502.

### Status

Fixed and verified against the running server, not by reading the diff.
Normal operation is unchanged: valid calls return data, unknown methods 400,
malformed JSON 400.

**This qualifies the sign-off above.** "No unresolved critical or high
findings" was true of the code reviewed at the time and false of the code in
the commit, because that file was committed on a green build rather than a
read. Building is not reviewing; a bridge that takes anonymous input needed
the second thing and did not get it.
