# Operations guide

This guide is for the person who installs and runs the CapacityRoom
platform: the web server, the worker, and their database. The product is
called CapacityRoom; the command is named `capacity-room` and every
variable has the prefix `CAPACITYROOM_`. The guide covers the
install behind a TLS proxy, the settings, the first start, the day to day,
backups, upgrades, sizing, and the limits of the platform. The command-line
tool, `capacity-room report`, needs none of this; the README describes
it.

Commands are written for a Linux host with a shell, and are meant to be run
in order on a first install. A line that ends with `# example: replace` holds
a value you choose: change it before you run the line.

## 1. What runs

| Service | Runs | Listens | Role |
|---|---|---|---|
| `serve` | `capacity-room serve` | `127.0.0.1:8080` on the host, plain HTTP | The web interface and the API under `/api/v1`; applies the database schema before it listens |
| `worker` | `capacity-room worker` | `:8091` inside its container, health only | Collects reports from the metric sources and compiles their PDFs; runs the maintenance job every minute |
| `postgres` | PostgreSQL 18 | Inside the Compose network only | Accounts, profiles, reports, snapshots, PDFs, the job queue, the audit trail |

The three run from `deploy/compose.yaml` as the Compose project
`capacity-room`, on the network `172.31.213.0/24`. `serve` and
`worker` run as user `10001`, with a read-only root file system, no added
privileges, and every capability dropped; their only writable place is a
temporary file system at `/tmp`. They read one folder from the host, mounted
read-only at `/opt/capacity-room`.

`serve` speaks plain HTTP and sets its session cookie `Secure`, so a browser
reaches it only through a TLS proxy that you run on the host.

## 2. Install

You need Docker Engine with the Compose plugin, Git, GNU make, and a TLS
proxy on the host; the example below uses Caddy. The image is built from the
release tag on the host; nothing is downloaded from a registry but the base
images it names by digest.

```bash
git clone https://github.com/marceloalvescf/capacity-room.git  # example: replace
cd capacity-room
git checkout v1.0.0  # example: replace
make image
```

Every later command runs from the checkout, with the variables of an
environment file that you keep outside it. The file holds the database
password, so only you read it.

```bash
export CAPACITYROOM_HOME=/srv/capacity-room  # example: replace
mkdir -p "$CAPACITYROOM_HOME/config/credentials" "$CAPACITYROOM_HOME/config/ca" "$CAPACITYROOM_HOME/backups"
umask 077
cat > "$CAPACITYROOM_HOME/capacity-room.env" <<EOF
CAPACITYROOM_VERSION=$(cat VERSION)
CAPACITYROOM_CONFIG_DIR=$CAPACITYROOM_HOME/config
CAPACITYROOM_POSTGRES_PASSWORD=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
COMPOSE_FILE=deploy/compose.yaml:$CAPACITYROOM_HOME/compose.limits.yaml
EOF
set -a; . "$CAPACITYROOM_HOME/capacity-room.env"; set +a
```

The database password is written in hexadecimal on purpose: it is placed in
a connection URL, where `@ / : ? # %` would break it. PostgreSQL takes it
when its volume is first created; a password changed in `capacity-room.env` later
does not reach the database, and `serve` then fails to connect. Keep
`capacity-room.env` with your backups.

`COMPOSE_FILE` adds a second file of your own to the shipped one, holding
the limits of section 10. Create it now, and adjust the numbers to the host:

```bash
cat > "$CAPACITYROOM_HOME/compose.limits.yaml" <<'EOF'
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

`$CAPACITYROOM_HOME/config/platform.toml` is the one settings file of `serve` and
`worker`. A key it does not know is refused, and so is a value of the wrong
type or out of its bounds; the service then exits with code 2 and says what
it refused. The database
connection is not in this file: it comes from the environment
(`CAPACITYROOM_DATABASE_DSN`), which `deploy/compose.yaml` builds from
`CAPACITYROOM_POSTGRES_PASSWORD`.

```bash
cat > "$CAPACITYROOM_HOME/config/platform.toml" <<'EOF'
server_secret_path = "/opt/capacity-room/secret/server-secret"
trusted_proxies = ["172.31.213.1/32"]
allowed_endpoint_hostnames = ["prometheus.example.com"]  # example: replace
allowed_endpoint_cidrs = []
credential_dir = "/opt/capacity-room/credentials"
ca_bundle_dir = "/opt/capacity-room/ca"
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
mkdir -p "$CAPACITYROOM_HOME/config/secret"
head -c 32 /dev/urandom > "$CAPACITYROOM_HOME/config/secret/server-secret"
printf '%s\n' 'the-bearer-token-of-the-source' > "$CAPACITYROOM_HOME/config/credentials/prometheus-bearer"  # example: replace
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
docker run --rm -v "$CAPACITYROOM_HOME/config:/config" --entrypoint sh \
  postgres:18-alpine@sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873 \
  -c 'chown -R 10001:10001 /config && chmod -R u=rwX,go= /config'
```

From then on you change a file of the folder as root, and run the same
command again after a change.

The other secrets live in `$CAPACITYROOM_HOME/capacity-room.env` (the database password) and in
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
`CAPACITYROOM_BOOTSTRAP_MODE=1`, reads the password from standard input, and refuses
once any user exists. A password has 12 to 1024 characters.

```bash
printf '%s\n' 'an administrator password' > "$CAPACITYROOM_HOME/admin-password"  # example: replace
docker compose run --rm -T -e CAPACITYROOM_BOOTSTRAP_MODE=1 serve \
  admin bootstrap-admin --username admin --password-stdin --config /opt/capacity-room/platform.toml \
  < "$CAPACITYROOM_HOME/admin-password"
rm "$CAPACITYROOM_HOME/admin-password"
```

`--password-file <path>` reads the password from the first line of a file instead, as the Kubernetes chart does.

`--language pt-BR` or `--language en` pins the administrator's language. When
the flag is omitted, the administrator follows the browser's language: the
web app uses the language of the browser, and the account stores `en` until
the administrator picks one.

Profiles can be created on the web by the administrator, or imported from a
file. An import creates or revises each profile the file names and leaves
the others alone; it can be run again. The file is checked whole before
anything is written.

```bash
cat > "$CAPACITYROOM_HOME/profiles.toml" <<'EOF'
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
docker compose run --rm -T serve admin import-profiles --config /opt/capacity-room/platform.toml \
  --file /dev/stdin < "$CAPACITYROOM_HOME/profiles.toml"
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
| `ENDPOINT_RESOLUTION_FAILED` | report | yes | The source's host name did not resolve |
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
| `SOURCE_VALUES_OUT_OF_RANGE` | report | no | The source sent values too large to compute with, such as samples near the largest number a report can hold |
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

### Node autoscaling

The report reads node autoscaling from kube-state-metrics, as
kube-prometheus-stack collects it, with nothing to configure. The cluster
autoscaler writes its status to the ConfigMap
`cluster-autoscaler-status`, found by its name in any namespace: by default
kube-system (AKS, the OKE add-on), or the namespace of a cluster autoscaler
installed by hand. kube-state-metrics exports its existence (`kube_configmap_info`) and each
change of it (`kube_configmap_metadata_resource_version`).

| Status | When |
|---|---|
| enabled | the ConfigMap was written during the period, or, when kube-state-metrics exports no resource version, it appeared in the period |
| disabled | ConfigMaps are collected, and that one never appeared or was never written during the period |
| unknown | kube-state-metrics exports no ConfigMap at all |

The report also states the current nodes, the nodes added during the
period, and the unschedulable pods, from `kube_node_info`,
`kube_node_created`, and `kube_pod_status_unschedulable`. It does not know
the minimum or the maximum of a node pool.

The status reads the ConfigMap over at least the last five minutes of the
period, so a scrape interval longer than two and a half minutes can read
disabled for a running autoscaler; state the node autoscaling in the
configuration there.

Two installations need the state written in the profile or in
`analysis.node_autoscaling`: a kube-state-metrics whose collector list was
trimmed and no longer holds `configmaps`, a kube-state-metrics started with
`--namespaces` that leaves out the autoscaler's namespace, and an OKE add-on
whose `statusConfigMapName` was changed. A cluster that scales its nodes with
Karpenter alone reads disabled.

### Stopping

`docker compose stop` asks each service to stop. `serve` waits up to 30
seconds for the requests in flight, then closes the rest and exits. `worker`
takes no new job, waits `worker_drain_seconds` for the running ones, cancels
them, and waits as long again; a cancelled job runs again from the start
later. Compose gives `serve` 45 seconds and `worker` 75 seconds before it
kills them. A second interrupt or `SIGTERM` during the stop ends the process
at once, like a kill. A second interrupt also ends a one-shot command, such as the report command, at once; the file it was writing is then left beside its target as a hidden `.<name>.<random>.tmp` file, which you can delete. If you raise `worker_drain_seconds`, raise the worker's
`stop_grace_period` in your limits file to at least twice that plus 15
seconds. A worker killed before it stops leaves its jobs to be picked up
again once the longer of the two job timeouts and one more minute have
passed.

## 7. Backup and restore

Everything the platform stores is in PostgreSQL. The settings folder holds
the rest: back it up with your host's own backups, as root.

```bash
docker compose exec -T postgres pg_dump -U capacityroom -Fc capacityroom > "$CAPACITYROOM_HOME/backups/capacity-room.dump"
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
docker compose exec -T postgres pg_restore -U capacityroom -d capacityroom --clean --if-exists < "$CAPACITYROOM_HOME/backups/capacity-room.dump"
docker compose up -d --wait
```

A dump restores into the version of the platform that wrote it, or into a
later one, which migrates it when it starts.

## 8. Upgrade

```bash
docker compose exec -T postgres pg_dump -U capacityroom -Fc capacityroom > "$CAPACITYROOM_HOME/backups/capacity-room-before-upgrade.dump"
git fetch --tags  # example: replace
git checkout v1.0.1  # example: replace
make image
sed -i "s/^CAPACITYROOM_VERSION=.*/CAPACITYROOM_VERSION=$(cat VERSION)/" "$CAPACITYROOM_HOME/capacity-room.env"
set -a; . "$CAPACITYROOM_HOME/capacity-room.env"; set +a
docker compose up -d --wait
```

`serve` and `worker` apply the new schema under a database lock when they
start, whichever comes first. A database written by a version before 1.0.0
is not carried forward: start 1.0.0 with an empty database. Then sign in, open a report stored before the
upgrade, and check the version at the bottom of the sidebar. From 1.0.0 on,
an upgrade keeps the stored reports, and a change that would break the API,
the command line, the settings files, or the snapshot file waits for a major
version; the changelog names every change.

### Upgrading from 1.x

Version 2.0.0 renames the platform:

- The Compose project, the volume, and the database are new, so the reports
  of a 1.x stack are not carried forward.
- Every environment variable now starts with `CAPACITYROOM_`, and the command
  is now `capacity-room`.
- The environment file is now `capacity-room.env`. Write it afresh as section 2
  shows: the variables of a 1.x file are not read.
- Sessions end at the upgrade, and everyone signs in again.
- A profile that is revised or imported again writes one new revision, even
  when its document is unchanged.

### Upgrading from 2.0.0

The settings folder is mounted at `/opt/capacity-room` from 2.1.0, and the
server secret lives in a folder of its own. Before you start 2.1.0, move the
secret and edit the paths of `platform.toml`:

```bash
mkdir "$CAPACITYROOM_HOME/config/secret"
mv "$CAPACITYROOM_HOME/config/server-secret" "$CAPACITYROOM_HOME/config/secret/server-secret"
```

```toml
server_secret_path = "/opt/capacity-room/secret/server-secret"
credential_dir = "/opt/capacity-room/credentials"
ca_bundle_dir = "/opt/capacity-room/ca"
```

Then give the folder to user `10001` again, as section 4 does. A
`platform.toml` that still names `/etc/capacity-room` stops `serve` at its
start with a line naming the path it could not read.

### Upgrading from 2.3.0

A report stored by this version whose node autoscaling reads disabled from
kube-state-metrics cannot be opened by 2.3.0 or earlier. A rollback to 2.3.0
loses the view and the PDF renders of those reports; reports stored before the
upgrade are read as before.

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

- The platform is one `serve` and one `worker`, as the Compose
  file or the chart runs them; no other layout is tested.
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
- The chart runs one `serve` and one `worker`, one replica each, and ships no
  NetworkPolicy. Its Ingress is checked against the schema of the Ingress
  kind alone; no cluster has served through it (section 12).

## 11. A lost administrator password

An administrator resets another user's password on the web. When the only
administrator has lost theirs, reset it on the host. The command needs
`CAPACITYROOM_BOOTSTRAP_MODE=1` and the password on standard input, ends every
session of that user, and writes the audit record `user.password_reset`. It
does not change the role or reactivate an account.

```bash
printf '%s\n' 'a new administrator password' > "$CAPACITYROOM_HOME/admin-password"  # example: replace
docker compose run --rm -T -e CAPACITYROOM_BOOTSTRAP_MODE=1 serve \
  admin reset-password --username admin --password-stdin --config /opt/capacity-room/platform.toml \
  < "$CAPACITYROOM_HOME/admin-password"
rm "$CAPACITYROOM_HOME/admin-password"
```

A user blocked by the sign-in throttle is free again after five minutes.

## 12. Kubernetes

The Helm chart in `deploy/helm/capacity-room` runs the platform on a
Kubernetes cluster. It is installed from a checkout and is not published.
It needs Helm 3 or 4 (the chart's tests use Helm 4.3.0), a PostgreSQL database that the cluster reaches, and either a
Gateway API gateway or an Ingress controller. Its values are documented by
the comments of `deploy/helm/capacity-room/values.yaml`, which this section
points at rather than repeats.

### What runs

The chart creates two Deployments (`serve` and `worker`, one replica each,
strategy `Recreate`), a Service on port 8080, an HTTPRoute or an Ingress, a
ConfigMap that holds `platform.toml`, a service account whose token no pod
mounts, and two hook Jobs. It creates no Secret, no volume claim, and no
NetworkPolicy: the Secrets are yours, the database is outside the chart, and
the cluster's network policy is yours to write. Every pod meets the
`restricted` pod security level.

### A database with CloudNativePG

The chart ships no database. It reads the DSN from an existing Secret named
by `database.existingSecret`, at the key `database.key` (default `uri`). Any
PostgreSQL 18 the cluster reaches will do; CloudNativePG is one way to run it
inside the cluster. Create the namespace first, for either way:

```bash
kubectl create namespace capacity-room
```

Install the operator once per cluster. It is the chart `cloudnative-pg`,
version 0.29.1 (operator 1.30.1), and it watches every namespace.

```bash
helm repo add cnpg https://cloudnative-pg.github.io/charts
helm upgrade -i cnpg cnpg/cloudnative-pg --version 0.29.1 -n cnpg-system --create-namespace --wait
```

Then make the database from `deploy/helm/capacity-room/examples/cloudnative-pg-cluster.yaml`.
It is a manifest for `kubectl apply`, not a values file: one instance of
PostgreSQL 18, a `capacityroom` database owned by `capacityroom`, and 5Gi of
storage. Set `storage.size` to your need, name a `storage.storageClass` if the
cluster has no default one, and pin the image by digest,
`ghcr.io/cloudnative-pg/postgresql:18@sha256:<digest>`, which
`docker buildx imagetools inspect ghcr.io/cloudnative-pg/postgresql:18`
prints.

```bash
kubectl apply -n capacity-room -f deploy/helm/capacity-room/examples/cloudnative-pg-cluster.yaml
kubectl -n capacity-room wait cluster/capacity-room-db --for=condition=Ready --timeout=300s
```

The operator then creates the Secret `capacity-room-db-app`, whose key `uri`
holds the DSN. In the values, write `database.existingSecret: capacity-room-db-app`
and keep the default key `uri`; there is no database Secret to create by
hand. The example runs one instance, with no backups, no pooler, and no
monitoring; if the data matters, set up backups as CloudNativePG's own
documentation describes.

For any other PostgreSQL 18, make the Secret yourself and name it in
`database.existingSecret`.

```bash
kubectl -n capacity-room create secret generic capacity-room-db \
  --from-literal=uri='postgres://user:password@host:5432/capacityroom'  # example: replace
```

### Secrets to create first

Create each in the namespace of the release before installing. The namespace
exists from the previous subsection.

```bash
# The server secret: at least 32 random bytes.
head -c 32 /dev/urandom > server-secret
kubectl -n capacity-room create secret generic capacity-room-server-secret \
  --from-file=server-secret=./server-secret
rm server-secret

# The first administrator's password, 12 to 1024 characters.
printf '%s' 'a long administrator password' > admin-password  # example: replace
kubectl -n capacity-room create secret generic capacity-room-admin \
  --from-file=password=./admin-password
rm admin-password

# One key per credential file that a profile names, each a bearer token of 8 characters or more.
kubectl -n capacity-room create secret generic capacity-room-credentials \
  --from-file=prod=./prod-token  # example: replace

# One key per CA bundle that a profile names, each PEM.
kubectl -n capacity-room create configmap capacity-room-ca \
  --from-file=internal-ca=./internal-ca.pem  # example: replace

# The pull secret of a private registry.
kubectl -n capacity-room create secret docker-registry registry-pull \
  --docker-username=example --docker-password=CHANGE-ME  # example: replace
```

The image on Docker Hub is public, so a pull secret is needed only for a
private registry or mirror; list it in the values as
`imagePullSecrets: [{name: registry-pull}]`.

The credentials Secret and the CA ConfigMap are optional; a key of either
is the file name that a profile gives. A key of the admin Secret other than
`password` is named by `admin.passwordKey`, which takes the characters
`-._a-zA-Z0-9`.

### A values file

Start from `deploy/helm/capacity-room/examples/gateway.yaml` for a Gateway API
route, or `examples/ingress.yaml` for an Ingress. Replace the hosts, the
Secret names, the gateway's `parentRefs`, and the profile.

Three settings decide whether the first install works:

- Set `admin.username` and `admin.existingSecret` at the first install. The
  admin Job runs at that install only, and with `admin.username` empty over a
  database that holds no user, nobody can sign in.
- List each metric source in `allowed_endpoint_hostnames` or
  `allowed_endpoint_cidrs`, since nothing is reachable until one lists it.
- Behind a gateway, set `trusted_proxies` to the gateway's range, or every
  client counts as the gateway for the sign-in throttle. The subsection
  Behind a gateway shows how to find the range.

`platform` takes the keys of section 3, by their own names, and the schema
refuses a key that section does not list. The chart writes
`server_secret_path`, `credential_dir`, and `ca_bundle_dir` itself, so
`platform` refuses those three. As in section 3, nothing is reachable until
`allowed_endpoint_hostnames` or `allowed_endpoint_cidrs` lists a metric
source.

`profiles` takes the `[[profiles]]` tables of `admin import-profiles` as YAML,
and a profile names its credential and its CA bundle by file name. No secret
goes in the values, and none enters Helm's release state.

### Install

Check the render against the cluster first, then install.

```bash
helm upgrade -i capacity-room deploy/helm/capacity-room -n capacity-room --dry-run=server -f values.yaml
helm upgrade -i capacity-room deploy/helm/capacity-room -n capacity-room -f values.yaml --wait --timeout 10m
```

With `--wait`, Helm runs the Jobs once the new pods are ready, after the old ones are gone, so a Job never migrates while an old pod is still stopping.

The values schema refuses an unknown key, a missing required Secret name,
and a route and an Ingress enabled together, before anything reaches the
cluster. Check `kubectl -n capacity-room get pods` until both pods are ready.

### The Jobs

Two hook Jobs run the operator commands of the platform. Helm waits for each
and removes it once it succeeds; a failed Job stays, so its log can be read.

- The admin Job, `capacity-room-admin` for a release of that name, runs
  `admin bootstrap-admin` after the first install only, when `admin.username`
  is set. It passes `--language` only when `admin.language` is `pt-BR` or
  `en`; the default, empty, makes the administrator follow the browser's
  language. The bootstrap mode is set on its pod alone. With `admin.username`
  empty over a database that holds no user, no Job creates one and nobody can
  sign in, as the notes Helm prints say. Set `admin.username` and
  `admin.existingSecret`, run `helm uninstall capacity-room -n capacity-room`,
  and install again; the Job runs at that install.
- The profiles Job, `capacity-room-profiles`, runs `admin import-profiles`
  after every install and upgrade, when `profiles` is not empty. It prints
  `created`, `revised`, `recreated`, or `unchanged` for each profile. A
  profile edited on the web holds only until the next upgrade, which revises
  it back to the values; profiles the values do not name are left alone.

```bash
kubectl -n capacity-room logs job/capacity-room-admin
kubectl -n capacity-room logs job/capacity-room-profiles
```

A Job that fails after its retries leaves the release failed.

- A failed admin Job runs at the first install only, so an upgrade neither
  removes it nor runs it again. Read its log, correct the cause (the Secret,
  the password policy, the database), then run
  `helm uninstall capacity-room -n capacity-room` and install again; the Job
  runs again at that install. A failed bootstrap writes no user, so the new
  install finds the database empty, and uninstalling keeps the database and
  the Secrets. Over a database that already holds a user, as in a reinstall
  over a kept database, the command refuses; leave `admin.username` empty
  then, and the Job is not rendered.
- A failed profiles Job: correct the values and run `helm upgrade` again.
  `before-hook-creation` removes the failed Job before the new one runs.

### Probes, resources, and stop times

Both pods have a startup, a readiness, and a liveness probe on
`/health/ready` and `/health/live`. `serve` has the memory limit and request
of `serve.resources`, and `worker` those of `worker.resources`; the worker's
`GOMEMLIMIT` is set at three quarters of its memory limit, which must be
written in `Mi` or `Gi`. `serve` has 45 seconds to stop. The worker has twice
`worker_drain_seconds` (30 by default) plus 15 seconds, since it waits for running jobs, cancels
the rest, and waits again. Size the memory as section 9 does.

The two Jobs have the requests and limit of `jobs.resources`. No container has
a CPU limit by default; a cluster whose admission policy requires one on every
container, such as an Azure Policy initiative or Gatekeeper, takes
`limits.cpu` in `serve.resources`, `worker.resources`, and `jobs.resources`.

### Upgrade and uninstall

To upgrade, check out the new version and run the same `helm upgrade -i`
command. Both pods are recreated, and the schema is applied under the lock
by the first command that starts. The image tag is the chart's `appVersion`
unless `image.tag` is set.

```bash
helm uninstall capacity-room -n capacity-room
```

Uninstall removes what the chart made. The database and the Secrets stay, so
a new install over the same database finds its users and reports; leave
`admin.username` empty then.

### Behind a gateway

`trusted_proxies` must name the peers whose `X-Forwarded-For` is believed. When
the gateway has a fixed address, name that address alone. When its pod
address changes at every restart, the pod range of the cluster is the only
stable name (`examples/gateway.yaml` uses `10.244.0.0/16`), and it is a
trade: any pod of the cluster can then send `serve` a forged
`X-Forwarded-For`. That changes the client address the sign-in throttle counts
and the address the audit records, and gives no access, since the sender
still needs a valid password. The chart ships no NetworkPolicy, so a cluster
whose network can enforce one may narrow this.

With `trusted_proxies` empty behind a gateway, every client has the address of
the gateway pod, so one person's failed attempts block everyone's sign-in. To
find the range, read the address of a gateway pod, then the range it belongs
to.

```bash
kubectl -n <gateway namespace> get pods -o wide
kubectl get nodes -o jsonpath='{.items[*].spec.podCIDRs}'
```

The second command lists the pod range of each node where the network plugin
assigns one. On AKS, `az aks show` prints the pod range of kubenet and of
Azure CNI Overlay.

```bash
az aks show -g <group> -n <cluster> --query networkProfile.podCidr
```

Azure CNI without overlay gives pods addresses of a virtual network subnet:
the node pool's `podSubnetId`, or its `vnetSubnetId` when it has none. Read
the ids of the pool, then the prefix of the subnet.

```bash
az aks nodepool show -g <group> --cluster-name <cluster> -n <pool> --query '[podSubnetId, vnetSubnetId]'
az network vnet subnet show --ids <id> --query addressPrefix
```

The range is enough only when the gateway sees each client's own address.
With `externalTrafficPolicy: Cluster`, the default of a `LoadBalancer`
Service, kube-proxy replaces the client's address with a node's before the
gateway writes `X-Forwarded-For`, so every client that arrives through one
node still counts as one. Set `externalTrafficPolicy: Local` on the gateway's
Service to keep the client's address.

## Reporting a vulnerability

`docs/security.md` describes what the platform defends against and how its
controls work. Report a vulnerability privately, through a security advisory
on the repository's GitHub page, not in a public issue.
