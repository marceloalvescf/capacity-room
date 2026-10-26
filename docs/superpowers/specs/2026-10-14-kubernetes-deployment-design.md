# Phase 13: the Kubernetes deployment

Date: 2026-10-14. Status: approved in conversation by the owner, written for review.

## Purpose

CapacityRoom runs today from one Compose file on one host. This phase adds a
Helm chart that runs the same platform on Kubernetes, publishes the runtime
image to a private Docker Hub repository, and proves both by a real install on
the owner's Talos homelab. The phase ends as release 2.1.0. Phase 14, the
public release, follows it.

The phase is a deployment phase. The product changes in two places only: the
command `admin bootstrap-admin` gains `--password-file`, and the Compose file
mounts its settings folder at `/opt/capacity-room`. No settings key, no
migration, no route, no screen, and no number changes.

## Decisions of the owner (2026-10-14)

- The chart is validated on the Talos homelab alone. No work cluster is
  touched.
- The chart ships no database. It reads the DSN from an existing Secret. The
  homelab runs a minimal CloudNativePG `Cluster`: one instance, no backups, no
  pooler, no monitoring.
- The image is Docker Hub `alvesmarcelocf/capacity-room`, private until phase
  14, linux/amd64 only, tagged with `VERSION` and never `latest`, one image
  (the `runtime` stage) for `serve`, `worker`, and `admin`. The owner runs
  `docker login` and creates the pull secret, so no agent sees a token.
- The chart lives in the repository and is published only as the OCI chart
  `oci://ghcr.io/marceloalvescf/charts/capacity-room`, by `.github/workflows/release.yml`
  (amended 2026-10-26). Its values are generic. It has a Gateway API `HTTPRoute` template, proven live
  on the homelab, and an `Ingress` template, proven by schema and by a
  server-side dry run only, since the homelab has no Ingress controller.
- The chart creates no Secret. Values name existing Secrets.
- Nothing is mounted under `/etc`. The chart's files go under
  `/opt/capacity-room`, and Compose follows the chart.
- No migration Job. `serve` and `worker` apply the schema under the lock, as
  today, and both Deployments use `strategy: Recreate`.
- No `kubectl exec` and no `kubectl run` for the first administrator or for
  profiles. Both come from values, through Helm hook Jobs. A profile that the
  values name is not read-only on the web: an edit made there holds until the
  next `helm upgrade`. A read-only mark may come later as a feature of its own.
- `serve` and `worker` are two Deployments, not one pod.
- The chart is tested by `helm lint`, by Go tests over `helm template`, and by
  `kubeconform` against offline schemas, in the gate; and by
  `helm upgrade -i --dry-run=server` and a real install on the homelab, on the
  owner's word.
- The image `2.1.0` is pushed from the version commit after `make gate`, and
  pushed again under the same tag after a fix, until `v2.1.0` is tagged.
- No NetworkPolicy. The homelab trusts the pod range as proxies and accepts
  the trade of section 6.
- The homelab's own files stay untracked, under `.cache/phase-13/homelab/`.

## 1. The chart

The chart is `deploy/helm/capacity-room/`:

| File | Holds |
|---|---|
| `Chart.yaml` | `apiVersion: v2`, `name: capacity-room`, `version` and `appVersion` equal to `VERSION` |
| `values.yaml` | the defaults, each value documented by a comment above it |
| `values.schema.json` | the shape of the values, `additionalProperties: false` at every level |
| `templates/_helpers.tpl` | names, labels, the pod security context, the shared mounts |
| `templates/configmap.yaml` | `platform.toml`, and `profiles.toml` when `profiles` is not empty |
| `templates/serviceaccount.yaml` | one service account, `automountServiceAccountToken: false` |
| `templates/deployment-serve.yaml` | `serve` |
| `templates/deployment-worker.yaml` | `worker` |
| `templates/service.yaml` | `serve` on port 8080, `ClusterIP` |
| `templates/httproute.yaml` | when `route.enabled` |
| `templates/ingress.yaml` | when `ingress.enabled` |
| `templates/job-admin.yaml` | when `admin.username` is set |
| `templates/job-profiles.yaml` | when `profiles` is not empty |
| `templates/NOTES.txt` | the address of the web, and where the Jobs' logs are |
| `examples/` | example values for a Gateway API route and for an Ingress, with `example.com` |

The chart holds no Secret, no PersistentVolumeClaim, no NetworkPolicy, and
nothing that reads the Kubernetes API.

### The Deployments

- One replica each, fixed and not a value: the platform is one `serve` and
  one `worker` (guide section 10).
- `strategy: Recreate`, so an old pod never runs against a newer schema.
- The pod template carries a checksum of the rendered settings, so a change of the
  settings recreates both pods, and a change of the profiles does not.
- `serve` runs `serve --config /opt/capacity-room/platform.toml --listen :8080`,
  and `worker` runs `worker --config /opt/capacity-room/platform.toml
  --health-listen :8091`.
- Probes are `httpGet`: a startup probe and readiness on `/health/ready`, and
  liveness on `/health/live`, on 8080 for `serve` and 8091 for `worker`, each with a timeout of 5 seconds. The
  startup probe gives a migration room before liveness counts.
- `terminationGracePeriodSeconds` is 45 for `serve`, which waits 30 seconds
  for the requests in flight. For `worker` it is twice `worker_drain_seconds`
  plus 15, read from `platform.worker_drain_seconds`, or from 30 when that
  value is left out.
- Default resources follow the guide: `serve` has a memory limit of 1Gi;
  `worker` has 4Gi, and `GOMEMLIMIT` is set at three quarters of the worker's
  memory limit. Values override both.

### Pod security

Every pod, the Jobs included, meets the Pod Security Standard `restricted`:
`runAsNonRoot`, user and group 10001, `readOnlyRootFilesystem`,
`allowPrivilegeEscalation: false`, every capability dropped, and
`seccompProfile: RuntimeDefault`. `/tmp` is an `emptyDir`, as the tmpfs of the
Compose file. No pod mounts a service account token.

### Mounts

Every mount is read-only.

| Path | From | Mounted on |
|---|---|---|
| `/opt/capacity-room/platform.toml`, `/opt/capacity-room/profiles.toml` | the chart's ConfigMap | `serve`, `worker`, both Jobs |
| `/opt/capacity-room/secret/` | `serverSecret.existingSecret` | `serve`, `worker`, both Jobs |
| `/opt/capacity-room/credentials/` | `credentials.existingSecret`, when set | `serve`, `worker`, both Jobs |
| `/opt/capacity-room/ca/` | `caBundles.existingConfigMap`, when set | `serve`, `worker`, both Jobs |
| `/opt/capacity-room/admin/` | `admin.existingSecret` | the admin Job alone |

`CAPACITYROOM_DATABASE_DSN` is read from `database.existingSecret` at the key
`database.key`, whose default `uri` is the key of a CloudNativePG `<cluster>-app`
Secret.

A Secret volume holds symbolic links into a hidden folder of its own. The
server secret is opened by path, and a reference file through `os.OpenRoot`,
which allows a link that stays inside its folder, so both read a Secret
volume as they are; no file mode is checked.

### Values

```yaml
image:
  repository: alvesmarcelocf/capacity-room
  tag: ""                  # empty means the chart's appVersion
  pullPolicy: IfNotPresent
imagePullSecrets: []
database:
  existingSecret: ""       # required
  key: uri
serverSecret:
  existingSecret: ""       # required; the file holds at least 32 random bytes
  key: server-secret
admin:
  username: ""             # empty: no admin Job
  # The first administrator's password, 12 to 1024 characters, read once by
  # the admin Job at the first install. Create the Secret before installing:
  #   kubectl -n <namespace> create secret generic capacity-room-admin \
  #     --from-file=password=./admin-password
  existingSecret: ""
  passwordKey: password
  language: en             # pt-BR or en
credentials:
  existingSecret: ""
caBundles:
  existingConfigMap: ""
platform: {}               # the keys of platform.toml, below
profiles: []               # the [[profiles]] tables of the import file
route:
  enabled: false
  parentRefs: []
  hostnames: []
ingress:
  enabled: false
  className: ""
  annotations: {}
  hosts: []
  tls: []
serve:
  resources: {}
worker:
  resources: {}
```

`platform` takes the fifteen keys of `platform.toml` that an operator sets,
by their own names: `session_ttl_seconds`, `session_idle_seconds`,
`trusted_proxies`, `allowed_endpoint_hostnames`, `allowed_endpoint_cidrs`,
`worker_slots`, `worker_drain_seconds`, `collect_timeout_seconds`,
`render_timeout_seconds`, `job_max_attempts`, `report_retention_days`,
`expired_report_days`, `admission_limit`, `admission_profile_limit`, and
`admission_retry_seconds`. A key left out is not rendered, so the binary's
default holds. The chart writes the three path keys itself and refuses them
under `platform`: `server_secret_path` is
`/opt/capacity-room/secret/<serverSecret.key>`, `credential_dir` is
`/opt/capacity-room/credentials`, and `ca_bundle_dir` is
`/opt/capacity-room/ca`.

`profiles` takes the keys of a `[[profiles]]` table of `admin import-profiles`
and no other. A profile names its credential by file name, so the values hold
no secret.

The schema refuses an unknown value at every level, `route.enabled` and
`ingress.enabled` together, and a missing `database.existingSecret` or
`serverSecret.existingSecret`. `admin.existingSecret` is required when
`admin.username` is set. The schema holds the bounds of each integer key, equal to the program's by a test; the rules between keys (`session_idle_seconds` at most `session_ttl_seconds`, `admission_profile_limit` at most `admission_limit`) stay the program's.

## 2. The Jobs

Both Jobs are `batch/v1` Jobs of the chart, made hooks by annotation. Helm
creates a hook after the release's other objects, waits for it to finish, and
deletes it when it succeeds. Each runs the same image, with the same security
context, ConfigMap, mounts, DSN, and pull secrets as `serve`;
`restartPolicy: Never`, `backoffLimit: 3`, and
`helm.sh/hook-delete-policy: before-hook-creation,hook-succeeded`. Each
command applies the schema under the lock itself, so neither waits for
`serve`.

- **The admin Job**, `helm.sh/hook: post-install`, rendered when
  `admin.username` is set. It runs `admin bootstrap-admin --username
  <admin.username> --language <admin.language> --password-file
  /opt/capacity-room/admin/<admin.passwordKey> --config
  /opt/capacity-room/platform.toml`, with `CAPACITYROOM_BOOTSTRAP_MODE=1` set
  on its pod alone. It runs at the first install only. When the database
  already holds a user, as in a reinstall over a kept database, the command
  refuses, the Job fails, and the release is marked failed; the comment of
  `admin.username` says to leave it empty in that case.
- **The profiles Job**, `helm.sh/hook: post-install,post-upgrade`, rendered
  when `profiles` is not empty. It runs `admin import-profiles --file
  /opt/capacity-room/profiles.toml --config /opt/capacity-room/platform.toml`.
  The import keeps its semantics: the whole file is checked before any write,
  each profile is created, revised, recreated, or left unchanged, and the
  profiles the file does not name are left alone. An edit made on the web to
  a profile the values name holds until the next upgrade, which revises it
  back. A refused file fails the hook, with the cause in the Job's log.

A failed hook leaves the release failed. A failed profiles Job is recovered by
correcting the values and upgrading; `before-hook-creation` removes the failed
Job first. The admin Job runs at the first install only, so a failed one is
recovered by correcting the cause, uninstalling, and installing again; a failed
bootstrap writes no user.

## 3. The one change to the command

`admin bootstrap-admin` takes `--password-file <path>`, as an alternative to
`--password-stdin`. Exactly one of the two is given; both or neither exits 2.

The file is opened once, without blocking, and must be a regular file of at
most 4 KiB, as the server secret is read. Its content follows the rules of
standard input: one line, with a trailing `\n` or `\r\n` removed, then the
password policy (12 to 1024 characters), then the redactor. A path the user
gave may be repeated in a message; the content never is.

`admin reset-password` keeps standard input alone. The flag is an addition to
the public surface of spec 15.1, so a minor version.

## 4. Compose follows the chart

`deploy/compose.yaml` mounts `CAPACITYROOM_CONFIG_DIR` at `/opt/capacity-room`
in place of `/etc/capacity-room`. The settings folder takes the chart's
layout: `platform.toml`, `secret/server-secret`, `credentials/`, and `ca/`.
`deploy/platform/` and the e2e files follow, and `make e2e` proves the move.

An install whose `platform.toml` names `/etc/capacity-room` fails to start,
naming the path it could not read, until its paths are edited.
The Upgrade section of the guide names the edit and the move of the server
secret into `secret/`. No stored data is touched.

## 5. The image

`.github/workflows/release.yml` pushes `alvesmarcelocf/capacity-room:$(VERSION)`,
the `runtime` image for `linux/amd64` that `make gate` built and tested, when
the owner pushes the annotated tag of a version, and refuses a version already
published (amended 2026-10-26; `make push` is gone).

The image is pushed from the version commit of 2.1.0 after `make gate` passes
on it. A defect found on the homelab is fixed on the branch, the gate passes
again, and the same tag is pushed again. The tag is final once `v2.1.0` is
tagged.

## 6. The homelab

The target is the context `<homelab-context>` of
`~/.kube/homelab.yaml`, namespace `capacity-room`. Every step that
changes the cluster names its target and waits for the owner's word. The
values, the CloudNativePG manifest, the generated secrets, and the evidence
live under `.cache/phase-13/homelab/`, untracked.

1. **CloudNativePG.** The operator's official chart, at a pinned version,
   into `cnpg-system`. A `Cluster` `capacity-room-db`: one instance, the
   PostgreSQL 18 image pinned by digest, 5Gi on Longhorn, database and owner
   `capacityroom`, no backups, no pooler, no monitoring. Its Secret
   `capacity-room-db-app` holds `uri`.
2. **Secrets.** The owner creates the pull secret `dockerhub-pull` with
   `kubectl create secret docker-registry`. The main thread creates
   `capacity-room-server-secret` from 32 random bytes and
   `capacity-room-admin` from a random password, from files of mode 0600 under
   `.cache/phase-13/homelab/`, since it signs in to verify.
3. **Values.** `route.parentRefs` names `default-gateway` in `istio-ingress`,
   section `https`; the host is `capacity-room.example.com`, whose
   certificate is the gateway's wildcard and whose HTTP redirect already
   exists. `platform.trusted_proxies` is `["10.244.0.0/16"]`, the pod range,
   since the gateway pod's address changes when it restarts.
   `platform.allowed_endpoint_hostnames` is
   `["monitoring-kube-prometheus-prometheus.monitoring.svc.cluster.local"]`,
   and one profile `homelab` points at it on port 9090 with no credential.
4. **The proof**, recorded under `.cache/phase-13/homelab/`:
   - `helm upgrade -i --dry-run=server` with the route values, and with the
     Ingress example values;
   - the install: the image pulled through the pull secret, both Jobs
     succeeded, both pods ready;
   - sign-in at `https://capacity-room.example.com` as the administrator,
     with the audit row naming the owner's LAN address rather than the
     gateway's, which proves `trusted_proxies`;
   - the profile tested, a report run to `collected`, its view opened, its
     PDF downloaded in both languages;
   - the logs of every pod and Job searched for the administrator's password
     and the DSN, and neither found;
   - `helm upgrade` with one changed value: both pods recreated, the profiles
     Job reporting `unchanged`;
   - `helm uninstall`, then an install over the same database with
     `admin.username` empty.

**The trade of the pod range.** Trusting `10.244.0.0/16` lets any pod of the
cluster send `serve` a forged `X-Forwarded-For`. It can change the client
address that the sign-in throttle counts and write a false address into the
audit; it gains no access, since it still needs a valid password. Flannel
cannot enforce a NetworkPolicy, and the chart ships none. The guide and the
security document state the trade, and an operator whose proxy has a fixed
address names that address alone.

## 7. Tests and tooling

- **The build tag `helm`** joins `guard.BuildTags`. The chart's Go tests carry
  it and fail when `helm` or `kubeconform` is missing.
- **`make test-chart`** runs in the test image, since a native result never
  closes a gate, and joins `make gate`. The `test` stage gets `helm` 4.3.0
  and `kubeconform` 0.8.0, each downloaded by exact version and checked by
  SHA-256. The devShell gets the same two tools, so the tests also run
  natively while a task iterates.
- **`helm lint`** over the default values with the required Secret names set,
  and over each file of `examples/`.
- **Go tests over `helm template`**, in a new package that `guard.Layers`
  lists, named by the plan:
  - the rendered `platform.toml` loads through `internal/platform/settings`,
    and the rendered `profiles.toml` through the parser of
    `admin import-profiles`;
  - the schema's `platform` keys together with the three chart-owned keys
    equal the keys of the settings, both ways, and the schema's `profiles`
    keys equal the keys of the import file, both ways;
  - `Chart.yaml`'s `version` and `appVersion` equal `VERSION`;
  - every pod and Job meets the security context of section 1, and no pod
    mounts a service account token;
  - a missing required Secret name, an unknown value, and `route` with
    `ingress` each fail the render;
  - the worker's grace period follows `worker_drain_seconds`, and the
    chart's fallback of 30 equals the binary's default;
  - the admin Job appears only with `admin.username`, the profiles Job only
    with `profiles`, and the admin Secret is mounted on the admin Job alone.
- **`kubeconform -strict`**, offline, over the rendered manifests, against
  schemas kept in the repository for the kinds the chart emits alone
  (ConfigMap, Deployment, Service, ServiceAccount, Job, Ingress, HTTPRoute),
  from Kubernetes 1.37.0 and a pinned Gateway API release. A script beside
  them regenerates them, and the README of their folder says how.
- **The command**: the tests of `internal/cli` cover `--password-file`: both
  flags, neither, a missing file, a file that is not regular, a file over
  4 KiB, a password outside the policy, a trailing `\r\n`, and the password
  reaching the redactor. The test that names every command and flag follows.
- **Compose**: `make e2e` runs the stack with the mount at
  `/opt/capacity-room`.

## 8. Documents

Each changes in the commit that changes what it describes.

- `docs/operations.md`: a Kubernetes section (the Secrets to create, the
  values, install, upgrade, uninstall, the Jobs and their failures, the
  probes, the trade of the pod range); the Compose paths at
  `/opt/capacity-room`; the Upgrade edit for an install of 2.0.0.
- `docs/security.md`: the chart's controls (no Secret in values or in Helm's
  release state, no service account token, the admin Secret on the admin Job
  alone, the pull secret made by the operator) and the forged
  `X-Forwarded-For` of the pod range.
- `AGENTS.md`: the chart and its rules, the tag `helm`, `make test-chart` and
  `make push`, the new row of the layers.
- `README.md`: one line naming the chart and the guide's section.
- `CHANGELOG.md`, 2.1.0. Added: the Helm chart; `--password-file` for the
  first administrator. Changed: the Compose settings folder moves to
  `/opt/capacity-room`.

## 9. Out of scope

- A chart repository. The OCI chart is published since 2026-10-26.
- A NetworkPolicy, a PodDisruptionBudget, autoscaling, and more than one
  replica.
- A read-only mark for profiles that the values name.
- A first-run setup screen on the web.
- An image for linux/arm64, and the tag `latest`.
- Making the Docker Hub repository public (phase 14).

## Definition of done

- `make gate` passes on the version commit 2.1.0, with `make test-chart` in
  it, and `make test-chart` has been seen failing on a broken chart.
- `make audit` is clean on that commit.
- `alvesmarcelocf/capacity-room:2.1.0` is on Docker Hub, pushed from that
  commit.
- Every step of the proof of section 6 passed on the homelab, with its
  evidence under `.cache/phase-13/homelab/`.
- An adversarial review of the whole branch ran, and its fix wave is in.
- `ROADMAP.md` and `CHANGELOG.md` are read back after their update.
