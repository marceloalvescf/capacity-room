# Operations guide

This guide is for the person who installs and runs the k8s-capacity-report
platform: the web server, the worker, and their database. It covers the
install behind a TLS proxy, the settings, the first start, the day to day,
backups, upgrades, sizing, and the limits of the platform. The command-line
tool, `k8s-capacity-report report`, needs none of this; the README describes
it.

Commands are written for a Linux host with a shell, and are meant to be run
in order on a first install. A line that ends with `# example: replace` holds
a value you choose: change it before you run the line.

## 1. What runs

| Service | Runs | Listens | Role |
|---|---|---|---|
| `serve` | `k8s-capacity-report serve` | `127.0.0.1:8080` on the host, plain HTTP | The web interface and the API under `/api/v1`; applies the database schema before it listens |
| `worker` | `k8s-capacity-report worker` | `:8091` inside its container, health only | Collects reports from the metric sources and compiles their PDFs; runs the maintenance job every minute |
| `postgres` | PostgreSQL 18 | Inside the Compose network only | Accounts, profiles, reports, snapshots, PDFs, the job queue, the audit trail |

The three run from `deploy/compose.yaml` as the Compose project
`k8s-capacity-report`, on the network `172.31.213.0/24`. `serve` and
`worker` run as user `10001`, with a read-only root file system, no added
privileges, and every capability dropped; their only writable place is a
temporary file system at `/tmp`. They read one folder from the host, mounted
read-only at `/etc/kcr`.

`serve` speaks plain HTTP and sets its session cookie `Secure`, so a browser
reaches it only through a TLS proxy that you run on the host.

## 2. Install

You need Docker Engine with the Compose plugin, Git, GNU make, and a TLS
proxy on the host; the example below uses Caddy. The image is built from the
release tag on the host; nothing is downloaded from a registry but the base
images it names by digest.

```bash
git clone https://github.com/marceloalvescf/k8s-capacity-report.git  # example: replace
cd k8s-capacity-report
git checkout v1.0.0  # example: replace
make image
```

Every later command runs from the checkout, with the variables of an
environment file that you keep outside it. The file holds the database
password, so only you read it.

```bash
export KCR_HOME=/srv/kcr  # example: replace
mkdir -p "$KCR_HOME/config/credentials" "$KCR_HOME/config/ca" "$KCR_HOME/backups"
umask 077
cat > "$KCR_HOME/kcr.env" <<EOF
KCR_VERSION=$(cat VERSION)
KCR_CONFIG_DIR=$KCR_HOME/config
KCR_POSTGRES_PASSWORD=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
COMPOSE_FILE=deploy/compose.yaml:$KCR_HOME/compose.limits.yaml
EOF
set -a; . "$KCR_HOME/kcr.env"; set +a
```

The database password is written in hexadecimal on purpose: it is placed in
a connection URL, where `@ / : ? # %` would break it. PostgreSQL takes it
when its volume is first created; a password changed in `kcr.env` later
does not reach the database, and `serve` then fails to connect. Keep
`kcr.env` with your backups.

`COMPOSE_FILE` adds a second file of your own to the shipped one, holding
the limits of section 10. Create it now, and adjust the numbers to the host:

```bash
cat > "$KCR_HOME/compose.limits.yaml" <<'EOF'
services:
  serve:
    mem_limit: 1g
  worker:
    mem_limit: 4g
    environment:
      GOMEMLIMIT: 3GiB
  postgres:
    mem_limit: 2g
EOF
```

### The TLS proxy

The proxy ends TLS and passes every request to `127.0.0.1:8080`. This is a
complete `Caddyfile` for a host named `reports.example.com`; Caddy obtains
and renews its certificate by itself:

```text
reports.example.com {
	reverse_proxy 127.0.0.1:8080
}
```

`serve` sees the proxy as the gateway of the Compose network,
`172.31.213.1`, and believes the client address that the proxy writes in
`X-Forwarded-For` only from that address. Section 3 sets it in
`trusted_proxies`. Without it, every client has the proxy's address, and
five wrong passwords from anyone block a user's sign-in for everyone.
`serve` reads the last address of the header, the one your proxy added, so a
value sent by the client is never believed. Another proxy works the same
way, as long as it adds the client's address to `X-Forwarded-For`.

If `172.31.213.0/24` is already used on the host, give the network another
range in your limits file, under `networks: default: ipam: config:`, and
change `trusted_proxies` to its first address.

## 3. Settings

`$KCR_HOME/config/platform.toml` is the one settings file of `serve` and
`worker`. A key it does not know is refused, and so is a value of the wrong
type or out of its bounds; the service then exits with code 2 and says what
it refused. The database
connection is not in this file: it comes from the environment
(`KCR_DATABASE_DSN`), which `deploy/compose.yaml` builds from
`KCR_POSTGRES_PASSWORD`.

```bash
cat > "$KCR_HOME/config/platform.toml" <<'EOF'
server_secret_path = "/etc/kcr/server-secret"
trusted_proxies = ["172.31.213.1/32"]
allowed_endpoint_hostnames = ["prometheus.example.com"]  # example: replace
allowed_endpoint_cidrs = []
credential_dir = "/etc/kcr/credentials"
ca_bundle_dir = "/etc/kcr/ca"
EOF
```

Every key, with its default and its bounds:

<!-- table: settings -->
| Key | Default | Bounds | Meaning |
|---|---|---|---|
| `session_ttl_seconds` | 43200 | 60 to 2592000 | The longest a session lives, from sign-in |
| `session_idle_seconds` | 3600 | 60 to 2592000, at most `session_ttl_seconds` | A session ends after this long without a request |
| `server_secret_path` | none, required | an absolute path to a regular file of 32 bytes to 64 KiB | The file of random bytes that keys the sign-in throttle |
| `trusted_proxies` | `[]` | CIDR ranges | The peers whose `X-Forwarded-For` is believed; name the proxy's address alone |
| `allowed_endpoint_hostnames` | `[]` | host names | Metric sources reachable by name; nothing is reachable until a name or a range is listed |
| `allowed_endpoint_cidrs` | `[]` | CIDR ranges | Metric sources reachable by address range |
| `credential_dir` | `/run/secrets/metrics` | an absolute path that holds neither the settings file nor the server secret | The folder of the credential files that profiles name |
| `ca_bundle_dir` | `/run/config/ca` | an absolute path that holds neither the settings file nor the server secret | The folder of the CA bundles that profiles name |
| `worker_slots` | 2 | 1 to 16 | Report jobs one worker runs at once |
| `worker_drain_seconds` | 30 | 1 to 3600 | At a stop, the wait for running jobs before they are cancelled |
| `collect_timeout_seconds` | 900 | 60 to 3600 | The longest collection |
| `render_timeout_seconds` | 300 | 10 to 3600 | The longest compilation of one PDF |
| `job_max_attempts` | 3 | 1 to 10 | Attempts of a job whose source was unreachable or that failed inside |
| `report_retention_days` | 7 | 1 to 365 | Days a report keeps its snapshot and PDFs |
| `expired_report_days` | 30 | 1 to 365 | Days an expired report is kept as a record before it is removed |
| `admission_limit` | 20 | 1 to 1000 | Reports queued or running at once |
| `admission_profile_limit` | 5, or `admission_limit` when smaller | 1 to `admission_limit` | Reports queued or running at once for one profile |
| `admission_retry_seconds` | 5 | 1 to 300 | The wait a refused request is told to keep |

The two allowlists are the outbound policy. Every address that a metric
source resolves to must be allowed by its name or by a range. Loopback,
link-local, multicast, reserved, and cloud metadata addresses are refused
unless an entry of one single address names them. An entry grants every
port of its host or range, and an administrator's profile test reports
whether a port answers, so list the sources, not whole networks.

## 4. Secrets and files

The settings folder holds the server secret, the settings, and two folders
of files that profiles name: credentials (a bearer token, one per file) and
CA bundles (PEM).

```bash
head -c 32 /dev/urandom > "$KCR_HOME/config/server-secret"
printf '%s\n' 'the-bearer-token-of-the-source' > "$KCR_HOME/config/credentials/prometheus-bearer"  # example: replace
```

A credential is UTF-8, at least 8 characters, on one line. A file name
matches `^[a-z0-9](?:[a-z0-9._-]{0,61}[a-z0-9])?$`, and a file is at most
1 MiB. A credential or a CA bundle is read when a report runs or a profile
is tested, never when a profile is saved, so a file can be replaced without
touching the profile.

`serve` and `worker` run as user `10001`. Give the folder to that user, and
to no one else. Docker access is root on the host, so a short container does
it:

```bash
docker run --rm -v "$KCR_HOME/config:/config" --entrypoint sh \
  postgres:18-alpine@sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873 \
  -c 'chown -R 10001:10001 /config && chmod -R u=rwX,go= /config'
```

From then on you change a file of the folder as root, and run the same
command again after a change.

The other secrets live in `$KCR_HOME/kcr.env` (the database password) and in
the database (password hashes, session digests). The output of
`docker inspect` and of `docker compose config` shows the database password;
treat both as secrets.

## 5. First start

```bash
docker compose up -d --wait
```

`--wait` returns when the three services are healthy. `serve` has applied
the schema by then.

Create the first administrator. The command runs only with
`KCR_BOOTSTRAP_MODE=1`, reads the password from standard input, and refuses
once any user exists. A password has 12 to 1024 characters.

```bash
printf '%s\n' 'an administrator password' > "$KCR_HOME/admin-password"  # example: replace
docker compose run --rm -T -e KCR_BOOTSTRAP_MODE=1 serve \
  admin bootstrap-admin --username admin --password-stdin --config /etc/kcr/platform.toml \
  < "$KCR_HOME/admin-password"
rm "$KCR_HOME/admin-password"
```

Profiles can be created on the web by the administrator, or imported from a
file. An import creates or revises each profile the file names and leaves
the others alone; it can be run again. The file is checked whole before
anything is written.

```bash
cat > "$KCR_HOME/profiles.toml" <<'EOF'
[[profiles]]
id = "prod"
display_name = "Production"
endpoint_url = "https://prometheus.example.com/api/v1"  # example: replace
endpoint_type = "prometheus"
cluster_name = "prod"  # example: replace
allowed_namespaces = ["payments", "checkout"]  # example: replace
credential_file = "prometheus-bearer"
active = true
EOF
docker compose run --rm -T serve admin import-profiles --config /etc/kcr/platform.toml \
  --file /dev/stdin < "$KCR_HOME/profiles.toml"
```

Then sign in at `https://reports.example.com` as `admin`, open Profiles, and
test the profile. A test sends the first requests of a report through the
outbound policy and answers with one of the codes of section 6.

## 6. Day to day

### Health

```bash
docker compose ps
curl -fsS https://reports.example.com/health/ready  # example: replace
```

`docker compose ps` shows each service healthy or not; Compose checks
`serve` at `/health/ready` and `worker` at its own `/health/ready` every two
seconds. `/health/live` answers while the process runs, and `/health/ready`
answers 200 once the database is reached and the schema is current, and 503
otherwise, with the result of each check.

### Logs

```bash
docker compose logs --since 1h serve worker
```

Each line is `time=... level=... msg=...` with its fields. A secret that the
program knows (the database password, a credential, a password typed at the
command line) is written `[redacted]`, wherever it would have appeared.
`serve` logs failed requests and its start and stop; `worker` logs each job's
failure with its code and its cause.

### What a failure code means

A report or a PDF that fails stores one code, which the web shows in words.
"Tried again" means the worker runs the job again, up to `job_max_attempts`,
before the code is stored.

<!-- table: failure-codes -->
| Code | Stored on | Tried again | Meaning |
|---|---|---|---|
| `ENDPOINT_URL_REJECTED` | report | no | The profile's address is not a plain `http` or `https` URL the policy accepts |
| `ENDPOINT_ADDRESS_REJECTED` | report | no | The source resolves to an address the allowlists do not allow, or to a refused class |
| `ENDPOINT_RESOLUTION_FAILED` | report | no | The source's host name did not resolve |
| `ENDPOINT_CONNECTION_REJECTED` | report | no | The connection reached another address than the one the policy allowed |
| `ENDPOINT_REDIRECT_REJECTED` | report | no | The source answered with a redirect, which is never followed |
| `PROFILE_CREDENTIAL_UNAVAILABLE` | report | no | The credential file is missing, unreadable, too large, or not a valid credential |
| `PROFILE_CA_BUNDLE_UNAVAILABLE` | report | no | The CA bundle is missing, unreadable, too large, or holds no certificate |
| `ENDPOINT_TLS_FAILED` | report | no | The TLS handshake failed: the certificate is not trusted or does not name the host |
| `ENDPOINT_AUTH_REJECTED` | report | no | The source refused the credential |
| `ENDPOINT_API_NOT_FOUND` | report | no | The address answers, but not as a Prometheus API |
| `ENDPOINT_UNREACHABLE` | report | yes | The source did not answer, or answered something that is not HTTP |
| `CLUSTER_SCOPE_NOT_FOUND` | report | no | The source holds no series for the profile's cluster name |
| `CAPABILITIES_INSUFFICIENT` | report | no | The source lacks the metrics a report needs |
| `NOTHING_TO_REPORT` | report | no | The source answered, but nothing was found to report for the namespaces |
| `SNAPSHOT_TOO_LARGE` | report | no | The collected data is larger than 32 MiB |
| `COLLECTION_TIMEOUT` | report | no | The collection took longer than `collect_timeout_seconds` |
| `JOB_ATTEMPTS_EXHAUSTED` | report | no | The job ended without storing an outcome, for example a worker killed during its last attempt |
| `PDF_COMPILE_FAILED` | PDF | no | The PDF could not be compiled within `render_timeout_seconds` |
| `PDF_TOO_LARGE` | PDF | no | The PDF is larger than 64 MiB |
| `PDF_ATTEMPTS_EXHAUSTED` | PDF | no | The PDF job ended without storing an outcome |
| `INTERNAL_FAILURE` | report and PDF | yes | Anything else; the worker's log holds the cause |

A failed PDF is compiled again from the stored snapshot when an operator
asks for it on the web; the collection is not repeated.

### Exit codes and the API

`serve`, `worker`, and every `admin` command exit with 0 on success, 1 for a
failure at run time (the database, the listener, Typst for `worker`, jobs
that did not stop after the drain, a refusal of `bootstrap-admin` or
`reset-password`), and 2 for the arguments or the settings file. Every
failure prints one line, `error: <message>`, on standard error.
`admin migrate` applies the schema by itself, under the same lock that
`serve` and `worker` take.

The web interface uses the API under `/api/v1`, which
`api/openapi.yaml` describes route by route, with every answer and error
code. A script uses it as the browser does: a session cookie from
`POST /api/v1/auth/login`, and the session's CSRF token in `X-CSRF-Token` on
every request that changes something.

### Stopping

`docker compose stop` asks each service to stop. `serve` waits up to 30
seconds for the requests in flight, then closes the rest and exits. `worker`
takes no new job, waits `worker_drain_seconds` for the running ones, cancels
them, and waits as long again; a cancelled job runs again from the start
later. Compose gives `serve` 45 seconds and `worker` 75 seconds before it
kills them. If you raise `worker_drain_seconds`, raise the worker's
`stop_grace_period` in your limits file to at least twice that plus 15
seconds. A worker killed before it stops leaves its jobs to be picked up
again once the longer of the two job timeouts and one more minute have
passed.

## 7. Backup and restore

Everything the platform stores is in PostgreSQL. The settings folder holds
the rest: back it up with your host's own backups, as root.

```bash
docker compose exec -T postgres pg_dump -U kcr -Fc kcr > "$KCR_HOME/backups/kcr.dump"
```

The dump holds password hashes and the reports' data; keep it as you keep
the settings folder. Without a backup, a lost database volume loses every
account, profile, and report: the platform starts empty, and the first
administrator is created again.

To restore, stop the two services, restore into the database, and start
them:

```bash
docker compose stop serve worker
docker compose up -d --wait postgres
docker compose exec -T postgres pg_restore -U kcr -d kcr --clean --if-exists < "$KCR_HOME/backups/kcr.dump"
docker compose up -d --wait
```

A dump restores into the version of the platform that wrote it, or into a
later one, which migrates it when it starts.

## 8. Upgrade

```bash
docker compose exec -T postgres pg_dump -U kcr -Fc kcr > "$KCR_HOME/backups/kcr-before-upgrade.dump"
git fetch --tags  # example: replace
git checkout v1.0.1  # example: replace
make image
sed -i "s/^KCR_VERSION=.*/KCR_VERSION=$(cat VERSION)/" "$KCR_HOME/kcr.env"
set -a; . "$KCR_HOME/kcr.env"; set +a
docker compose up -d --wait
```

`serve` and `worker` apply the new schema under a database lock when they
start, whichever comes first. A database written by a version before 1.0.0
is not carried forward: start 1.0.0 with an empty database. Then sign in, open a report stored before the
upgrade, and check the version at the bottom of the sidebar. From 1.0.0 on,
an upgrade keeps the stored reports, and a change that would break the API,
the command line, the settings files, or the snapshot file waits for a major
version; the changelog names every change.

## 9. Sizing

A report stores its snapshot and one PDF per language asked for. On a real
cluster of 2026, the snapshot was 0.6 MB and a detailed PDF 0.85 MB; an
executive PDF is about 0.25 MB. A snapshot is refused above 32 MiB and a PDF
above 64 MiB.

A report keeps its data for `report_retention_days` (7), then only its row
for `expired_report_days` (30). At 20 reports a day with both PDFs, the
database holds about 140 reports with data, roughly 300 MB, plus the job
queue and the audit trail, which grow slowly. Plan the database volume at
ten times the expected size.

The worker uses most of the memory: each slot holds a collection's answers
and a Typst compile. The limits of section 2 suit the default two slots;
raise the worker's limit with `worker_slots`.

## 10. Limits

- The platform is one `serve` and one `worker` on one host, as the Compose
  file runs them; no other layout is tested.
- `serve` speaks plain HTTP: TLS is the proxy's.
- No outbound proxy: the worker connects to the metric sources directly.
- The sign-in throttle counts failures per user and client address. It does
  not limit the number of user names one address tries; a rate limit on
  `/api/v1/auth/login` in your proxy adds that bound.
- A metric source that the allowlists admit is trusted to answer with
  reasonable sizes: an answer of up to 256 MiB is read, and decoding it takes
  many times that in memory. The memory limits of section 2 keep such an
  answer from reaching the rest of the host.
- An allowlist entry grants every port of its host or range (section 3).

## 11. A lost administrator password

An administrator resets another user's password on the web. When the only
administrator has lost theirs, reset it on the host. The command needs
`KCR_BOOTSTRAP_MODE=1` and the password on standard input, ends every
session of that user, and writes the audit record `user.password_reset`. It
does not change the role or reactivate an account.

```bash
printf '%s\n' 'a new administrator password' > "$KCR_HOME/admin-password"  # example: replace
docker compose run --rm -T -e KCR_BOOTSTRAP_MODE=1 serve \
  admin reset-password --username admin --password-stdin --config /etc/kcr/platform.toml \
  < "$KCR_HOME/admin-password"
rm "$KCR_HOME/admin-password"
```

A user blocked by the sign-in throttle is free again after five minutes.

## Reporting a vulnerability

`docs/security.md` describes what the platform defends against and how its
controls work. Report a vulnerability privately, through a security advisory
on the repository's GitHub page, not in a public issue.
