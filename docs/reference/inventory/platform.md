# Research notes: web platform of kubernetes-report 1.11.0 (from explore agent, 2026-09-27)

Source: older/kubernetes-report/kube_capacity_report/platform/, alembic/, docs/web-platform-*.md, deploy/

## Keyring and crypto (crypto.py)
- AES-256-GCM encrypts ONE thing: managed secrets = a credential pasted into a database profile. AAD binds ciphertext to its row id.
- Keyring.derive("login-throttle") gives the HMAC key for the login rate-limit table. Only other consumer of key material.
- Sessions and CSRF tokens are random tokens, stored as SHA-256 digests. Passwords are Argon2id. None of these use the keyring.
- rotate-secrets re-encrypts managed secrets under the active key, one savepoint per row, audited.
- Consequence for Go: dropping the keyring is safe ONLY IF pasted credentials are dropped too (credentials by reference to mounted files only), AND a server secret exists for the login-throttle HMAC.

## Authentication
- Argon2id, library defaults, rehash on successful login when parameters changed. Password policy: length 12..1024 only.
- Session token and CSRF token: 256-bit random each; only digests stored; constant-time comparison.
- Lifetime: absolute 12 h, idle 1 h sliding. Account re-read from the database on every request (no cached role).
- Login always rotates any presented cookie. Role change, deactivation, password change revoke all of the user's sessions. Revocation reasons recorded.
- Cookie: Secure, HttpOnly, SameSite=Lax, Path=/api, Max-Age = absolute lifetime.
- CSRF, two mechanisms: login CSRF (double-submit cookie vs X-CSRF-Token header, checked before credentials are parsed) and request CSRF (token digest bound to the specific session row).
- Login throttle: PostgreSQL table keyed by HMAC(server_key, normalised username + client IP); 5 failures / 300 s -> 300 s block; row locked FOR UPDATE.
  Throttled attempt costs no Argon2 and writes no audit event. Unknown username still costs one Argon2 verification against a dummy hash.
- Roles today: admin and user only. Reports belong to an owner: owner or admin can read. No operator or reader tier exists.

## Audit (audit.py)
- Append-only audit_events: actor, time, action constant, target type/id, outcome succeeded|failed, metadata JSONB.
- Covers login ok/failed, logout, bootstrap; user create/update/password/delete; profile create/revise/activate/deactivate/delete/test; secret rotation; report submit/delete/pdf-retry/cleanup; settings update.
- safe_metadata REFUSES (raises) when a key matches password|token|cookie|hash|digest|secret|dsn or a value is not a JSON scalar; records field names and from/to for role/active only.
- No API route reads it; operators read it with SQL. The security document treats it as required for incident response.

## SSRF (network.py) - treat as the spec
1. Static: http/https only; no userinfo, query, fragment; no control characters or backslash; path canonicalised, dot segments refused in raw and decoded form.
2. Address classes refused: 0.0.0.0/8, 127.0.0.0/8, 169.254.0.0/16, 224.0.0.0/4, 240.0.0.0/4, IPv6 loopback/link-local/multicast, NAT64 64:ff9b::/96, 6to4 2002::/16, Teredo 2001::/32, metadata 169.254.169.254 and fd00:ec2::254.
   A blocked class is lifted only by an allowlist CIDR naming exactly one address.
3. Every resolved address must be class-safe AND match an allowed hostname (exact, no wildcard) or an allowed CIDR; mixed answers fail closed.
4. Re-resolve right before each request, connect to the literal IP (hostname only in SNI and Host), compare the actual peer address; any 3xx is refused.
5. No ambient proxy; one optional approved proxy, itself checked.
Allowlists come from deployment configuration: allowed_endpoint_hostnames, allowed_endpoint_cidrs.

## Profiles
- Fields: id, display_name, active, endpoint_url, endpoint_type, timeout_seconds, credential_mode none|secret_reference|managed_secret, credential reference name, ca_bundle reference name, tls_verify,
  cluster_name, cluster_label, filter_mode, allowed_namespaces (>=1), excluded_containers, nodepool_label, node_autoscaling,
  selection policy: permitted + default for period, timezone, pdf profile, min severity; thresholds (7).
- Immutable revisions: each create/revise appends a row, never updated (database trigger refuses); optimistic concurrency with base_revision_id. A job pins the revision it was submitted with.
- File profiles are read-only through the API and reserve their id. (Go design drops file profiles.)
- References resolved only at job execution, outside a database transaction, never logged; path confined with strict resolve + is_relative_to; 1 MiB cap.
- Endpoint URL re-validated through the SSRF policy for every candidate root.
- The namespace list offered to the user comes from the profile's allowed_namespaces, not from a Prometheus query.

## Queue and jobs
- status queued -> running -> succeeded|failed; succeeded exactly when the snapshot row commits (same transaction creates the pending PDF row).
- phase waiting -> preflight -> collecting -> analyzing -> snapshotting -> rendering_pdf -> uploading_pdf -> complete; monotonic; progress_seq integer.
- pdf_status pending -> ready|failed; manual retry is the only path back to pending and always renders from the stored snapshot.
- Leases use the database clock; ownership (id, worker_id, attempt); heartbeat 15 s, lease 60 s. Reaper: snapshot exists -> success; else requeue with backoff 2^(attempt-1) capped 300 s; then JOB_ATTEMPTS_EXHAUSTED.
- Worker slots 1..16 default 2. Stop: no new claims, grace 30 s, unfinished jobs left to the reaper.
- Admission limits: global and per profile, queued and running; over limit -> 429 with Retry-After.
- Idempotency-Key header required on submit; same key + same body -> same job; same key + different body -> 409.
- Browser polls GET /report-jobs/{id}; no push.

## Artifacts and retention
- snapshot: JSONB document + sha256, immutable. PDF: content-addressed in S3, two-phase upload, digest checked three times. Snapshot digest re-checked at PDF render (SNAPSHOT_DIGEST_MISMATCH).
- Retention: three-stage resumable state machine with tombstones because database and S3 fail independently; bidirectional reconciliation hourly; expiry every 60 s.
- With PostgreSQL-only storage the stages collapse into one transaction. Size the blobs: retention 7 days, tombstones 30 days.

## Tables
users, sessions, managed_secrets, profiles, profile_revisions, jobs, snapshots, pdf_artifacts, audit_events, expiry_tombstones, platform_settings (single row), idempotency_keys, login_rate_limits

## API (all under /api/v1)
auth: GET /auth/session, POST /auth/login, POST /auth/logout
profiles: GET /profiles (active summaries); admin: GET/POST /admin/profiles, GET/PATCH/DELETE /admin/profiles/{id}, POST /admin/profiles/{id}/test
users (admin): GET/POST /admin/users, GET/PATCH/DELETE /admin/users/{id} (delete = deactivate)
settings (admin): GET/PATCH /admin/settings
reports: POST /report-jobs, GET /report-jobs (keyset pagination, status/profile_id/owner_scope, limit 1..200), GET /report-jobs/{id}, GET .../snapshot (ETag), GET .../pdf (streamed, ETag, attachment, never a pre-signed URL),
  POST .../pdf-retries, DELETE /report-jobs/{id}, POST /report-jobs/delete (bulk <= 200)
health: /health/live, /health/ready; worker has its own /live and /ready on port 8091
Errors: one envelope {"error": {"code", "message", "fields"}}; message is a fixed sentence looked up by code, never exception text.

## Operator commands
bootstrap-admin (password from stdin only, gated by KCR_BOOTSTRAP_MODE=1, refuses if any user exists), generate-key, rotate-secrets, worker, probe-worker, migrate (advisory lock), probe-web

## Image and Compose
- Stages: node 24 (frontend build), ghcr.io/typst/typst:0.15.0 (binary copied to /usr/local/bin/typst), python slim runtime.
- typst invoked by absolute path with empty PATH; version checked at build and at process start. Build-time smoke test compiles a real document as the runtime user.
- UID 10001; read_only root; tmpfs /tmp (256 MB web, 1 GB worker); no-new-privileges; cap_drop ALL.
- Health checks through probe subcommands (no curl in the image).
- Integration overlay: fake-prometheus service, fixture init, fixture platform.toml, typst fault wrapper to simulate compile failures.

## Security headers
CSP: default-src 'self'; script-src 'self'; style-src 'self' + two pinned hashes (Radix Select viewport style, and the empty string); font-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'
Also: x-content-type-options nosniff, referrer-policy same-origin, x-frame-options DENY, Cache-Control no-store by default.

## Easy to forget in a rewrite
1 deep SSRF rules; 2 audit metadata gate refuses rather than redacts; 3 throttle keyed by HMAC; 4 two CSRF mechanisms; 5 dummy Argon2 for unknown users; 6 immutable profile revisions pinned by jobs; 7 no redirects, no pre-signed URLs; 8 CSP hashes, never unsafe-inline.

## Effect on approved Go decisions
- No keyring: holds if pasted credentials are dropped; needs one server secret for the throttle HMAC.
- PostgreSQL storage: holds; add size limits and sizing note.
- Database profiles only: loses the read-only guarantee of deployment-owned profiles.
- Three roles and "everyone sees every report": new authorisation model; original scoped reports to owner or admin.
- Audit trail: was missing from the Go design; original treats it as required. Recommend restoring.
