# Security

This document says whom the k8s-capacity-report platform defends against,
how, and what an operator must do for it, and records the security review of
version 1.0.0. `docs/operations.md` is the guide to running the platform.

## Scope and attackers

The platform turns the metrics of a Prometheus-compatible API into capacity
reports. It runs as `serve` (the web interface and the API, behind the
operator's TLS proxy), `worker` (collections and PDF compiles), and
PostgreSQL, on one host.

What it protects: passwords, sessions, the credentials of the metric sources,
the database password and the server secret, the internal network the worker
can reach, the reports' data (names and sizing of a cluster), the audit
trail, and the platform's availability.

| Attacker | Holds | Wants |
|---|---|---|
| A visitor with no account | Network access to the proxy | Any read or write, a session, users locked out, the names of accounts |
| A signed-in reader | A reader's session | An operator's or an administrator's rights, another user's data or session |
| An operator | An operator's session | An administrator's rights, other users' reports deleted, the platform exhausted |
| An administrator abusing profiles | An administrator's session | An address the outbound policy refuses, a file outside the credential and CA folders, a map of the internal network |
| A hostile metric source | The answers of an allowed source | Markup or script run in the PDF or the browser, memory or time exhausted, a redirect to an internal address |

Out of scope: whoever holds the host, the Docker socket, the containers'
files, or the database. Such an attacker can already do anything,
including running the two commands that need `KCR_BOOTSTRAP_MODE=1`.

## Controls

| Control | What it stops | Where it is enforced |
|---|---|---|
| Argon2id password hashes (64 MiB, 3 passes), rehashed at sign-in when the parameters change; 12 to 1024 characters | Offline guessing of a stolen hash | `internal/platform/auth/password.go` |
| At most four hashes at once, 64 MiB each; a waiting sign-in leaves the queue when its client leaves | Memory exhaustion by sign-in attempts | `internal/platform/auth/password.go` |
| An unknown user costs one hash against a dummy hash made at start | Learning which accounts exist by timing | `internal/platform/auth/` |
| Five failures in five minutes block a user name and client address pair for five minutes; IPv6 clients count by /64 | Password guessing | `internal/platform/auth/throttle.go`, `internal/platform/api/client.go` |
| `X-Forwarded-For` believed only from `trusted_proxies`, last address only | A client choosing its own address | `internal/platform/api/client.go`, `deploy/compose.yaml` (fixed subnet) |
| Session tokens of 256 bits, stored as SHA-256; cookie `Secure`, `HttpOnly`, `SameSite=Lax`, `Path=/api`; 12 hours absolute, 1 hour idle | Session theft and fixation | `internal/platform/auth/session.go`, `internal/platform/api/auth.go` |
| A new session at every sign-in; every session revoked on a change of role, active flag, or password | A stale session keeping rights | `internal/platform/auth/` |
| CSRF token bound to the session, checked on every change; a double-submit pair for sign-in | Cross-site requests | `internal/platform/api/` |
| Checks in one order on every route (session, role, CSRF, body, then the service); role read from the database on every request | A route that forgets a check | `internal/platform/api/` |
| Reports readable by every signed-in user, deletable by an administrator or their owner; idempotency keys scoped to their owner | One user acting on another's report | `internal/platform/reports/` |
| Outbound policy: `http` and `https` only, refused address classes, allowlists by name or range, resolution before each request, connection to the literal address, peer checked after connecting, no redirect, no proxy | SSRF to the metadata service, loopback, or the internal network | `internal/platform/netpolicy/` |
| Credential and CA files named by a strict pattern, opened once inside their folder, regular files of at most 1 MiB; the folders may not hold the settings file or the server secret | Reading other files of the host, or sending them as a token | `internal/platform/profiles/refs.go`, `internal/platform/settings/settings.go` |
| One redactor per run, fed every secret as soon as it is known (database password, credentials, server secret, passwords typed on the host), over every log line and every message | Secrets in logs | `internal/redact/`, `internal/cli/` |
| No flag error repeats what the user typed | A secret typed at the wrong place reaching a log | `internal/cli/run.go` |
| Stored failures are codes; API errors are fixed sentences; audit metadata refuses secret-like keys | Secrets in answers or stored rows | `internal/platform/api/errors.go`, `internal/platform/audit/` |
| The template evaluates no value as markup; the web view renders no HTML from data | Markup or script from metric data | `template/capacity/`, `web/src/` |
| Typst runs with an empty environment, no system fonts, no network, a timeout; snapshots at most 32 MiB, PDFs at most 64 MiB | A compile used to read files or never end | `internal/typst/`, `internal/platform/worker/` |
| A render or collection attempt that River replaced writes nothing | A stale attempt overwriting a newer outcome | `internal/platform/store/reports.go`, `internal/platform/worker/` |
| Content-Security-Policy `default-src 'self'`, no inline script or style, `frame-ancestors 'none'`, `object-src 'none'`, `base-uri 'none'`; `X-Frame-Options: DENY`; `nosniff`; `Referrer-Policy: same-origin`; private, revalidated caching of reports | Script injection, framing, leaks through caches | `internal/platform/api/headers.go` |
| Containers as user 10001, read-only root, no added privileges, every capability dropped; distroless runtime image; bases pinned by digest; PostgreSQL not published | A compromise spreading inside the host | `deploy/Containerfile`, `deploy/compose.yaml` |
| An append-only audit trail; deletion is deactivation | Erasing what was done | `internal/platform/store/migrations/` |
| `make audit` runs govulncheck, `npm audit`, and trivy at every release | Known vulnerabilities shipped | `Makefile` |

## What an operator must do

- Run a TLS proxy in front of `serve`, and set `trusted_proxies` to the
  address `serve` sees it from (`172.31.213.1/32` with the shipped Compose
  file). Without it, one client can block a user's sign-in for everyone.
  Strict-Transport-Security, if you want it, is the proxy's to send.
- Give the settings folder to user 10001 alone, mode 0600 for files and
  0700 for folders. Keep the credential and CA folders apart from the
  settings file and the server secret; the platform refuses otherwise.
- List the metric sources in the allowlists, not whole networks: an entry
  grants every port of its host or range.
- Treat `kcr.env`, the output of `docker inspect` and
  `docker compose config`, and every database dump as secrets: they hold the
  database password or the password hashes.
- Set memory limits on the services, as the guide does: a metric source the
  allowlists admit can send answers large enough to need several gigabytes.
- Consider a rate limit on `/api/v1/auth/login` at the proxy. The sign-in
  throttle counts failures per user name and address, not per address alone.
- Upgrade when a release fixes a vulnerability. Each release passes the
  three scanners on the day it is tagged.

## The review of 1.0.0

On 2026-10-10 the program was reviewed whole at commit `4166418`, as an
attacker would read it, in eight areas, each by its own reviewer:

1. sign-in and sessions;
2. CSRF and roles;
3. the outbound policy;
4. secrets and the redactor;
5. the reads of credential and CA files;
6. hostile metric data up to the compile and the web view;
7. the container and the Compose deployment;
8. the frontend and the response headers.

The scanners were govulncheck 1.8.0, npm 11.20.0 (`npm audit --omit=dev`),
and trivy 0.74.0. govulncheck judges the Go code: it reports only a
vulnerability whose code the program calls. trivy reads the image and leaves
out an advisory with no fixed version, which no change could act on, and
those named in `deploy/trivy-ignore.yaml`, each with its reason and a date
after which it returns to the audit. A claim was ruled only once it was proven: by a request
against a running stack, by a test that failed on the reviewed commit, or by
a run of the built program.

A finding blocked the release when it let someone with no account act, let
a role exceed its rights, leaked a secret to a log, an answer, or a stored
row, reached an address the policy refuses, or was a known vulnerability in
code the program calls. Every other finding was fixed or deferred with its
reason.

| Claim | Area | Class | Ruling | Commit or reason |
|---|---|---|---|---|
| The Go standard library 1.27.1 holds ten vulnerabilities the program calls (net/http, http2, net/textproto, crypto/tls) | scanners, 7 | blocking: known vulnerability, called | fixed: Go 1.27.2 | `74ccbf0` |
| A flag error repeats what was typed, so a secret typed at the wrong place reaches standard error and a container's log | 4 | blocking: secret to a log | fixed | `d17a8b6`, `01b7c1d` |
| A password read on the host, and the server secret, never enter the redactor; nothing printed them | 4 | hardening | fixed | `d17a8b6`, `01b7c1d` |
| Behind a host proxy, every client shares one address unless `trusted_proxies` names the proxy, so anyone can block a user's sign-in | 1, 7 | hardening | fixed: a fixed Compose subnet, and the guide sets `trusted_proxies` | `ffd80c0`, `docs/operations.md` |
| A credential folder pointed at the settings folder lets an administrator send the server secret to a source as a token | 7 | hardening | fixed: such a folder is refused | `f181191` |
| The server secret is read uncapped, from any kind of file | 5 | hardening | fixed: a regular file of at most 64 KiB; its mode is the operator's | `f181191`, `docs/operations.md` |
| A reference file is checked, then opened by path; a FIFO swapped in between blocks the read | 5 | hardening | fixed: one non-blocking open inside the folder, checked on the handle | `c77760e` |
| A profile test against a port that does not speak HTTP answers 500 and writes no audit row | 3 | hardening | fixed: `ENDPOINT_UNREACHABLE`, audited | `813fc1c` |
| Sign-ins with varying user names queue Argon2 work, and a waiting sign-in ignores its client leaving | 1 | hardening | fixed for the queue; a per-address limit deferred, since it changes the throttle's rule | `9037ad2`; the guide advises a proxy rate limit |
| The throttle keys on raw `X-Forwarded-For` text and on the full IPv6 address | 1 | hardening | fixed: a canonical address, IPv6 by /64 | `9037ad2` |
| The dummy hash is made on first use, so the first unknown-user sign-in is slower | 1 | hardening | fixed: made at start | `9037ad2` |
| PostgreSQL runs without `no-new-privileges`; a password with URL characters breaks the DSN | 7 | hardening | fixed; the guide makes a hexadecimal password | `8bfcef5`, `docs/operations.md` |
| No memory limits; `/tmp` unbounded; Typst without a memory bound | 6, 7 | hardening | documented: the guide sets memory limits, which bound `/tmp` too | `docs/operations.md` |
| The DSN shows in `docker inspect` | 7 | out of scope (needs the Docker socket) | documented | `docs/operations.md` |
| The README made the server secret world-readable | 7 | hardening | documented: the guide gives the folder to user 10001 alone | `docs/operations.md` |
| The profile tests tell ports apart within an allowed host or range | 3 | inherent | documented: an entry grants every port | `docs/operations.md` |
| An answer of up to 256 MiB grows about 30 times in memory when decoded | 6 | hardening | deferred | availability only, from a source the operator allowed; bounded by the memory limits |
| Samples near the largest float make an average infinite, and the collection fails | 6 | hardening | deferred | hostile data only; the report fails, it never shows a wrong number |
| A cross-site navigation to the session route rotates the CSRF token | 2 | hardening | deferred | the page heals with one retry; a fix changes the route's contract |
| Two routes judge an id before the body | 2 | no security effect | deferred | every check of identity and role runs first |
| Two password writes of one user at one instant can answer 500 | 1 | not a security defect | deferred | needs two writes at the same instant |
| Bidirectional characters in a name reach the PDF and the view | 6 | hardening | deferred | shown as text, never run |
| A login racing `admin reset-password` keeps a session | 1 | not a finding | — | the sign-in compares the hash it verified under a row lock |

No claim of areas 2, 3, 5, 6, or 8 reached the blocking classes. The reviews
found the following sound, among others: the order of checks on every route,
the CSRF binding, the owner scoping of deletions and idempotency keys,
authorization before every conditional answer, every refused class of
address in every spelling tried, DNS rebinding, redirects, the confinement of
reference files to their folders, the template's and the web view's
handling of hostile text, the CSP, the static file handler, and the `next`
parameter of sign-in.

## Reporting a vulnerability

Report it privately through a security advisory on the repository's GitHub
page (Security, then Report a vulnerability), not in a public issue. Say
which version you ran and how to reproduce the problem.
