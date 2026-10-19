# capacity-room chart

This chart runs the CapacityRoom web platform on Kubernetes: a `serve`
Deployment (the interface and the API), a `worker` Deployment (it collects the
reports and compiles their PDFs), a Service, a Gateway API HTTPRoute or an
Ingress, the ConfigMap that holds `platform.toml`, and two hook Jobs that
create the first administrator and import the profiles. It ships no database
and creates no Secret. It is installed from a checkout of the repository and
is not published. This page is the quick start; section 12 of
`docs/operations.md` is the full text.

## Prerequisites

- A Kubernetes cluster and `kubectl` pointed at it.
- Helm 3 or 4 (the chart's tests use Helm 4.3.0).
- A Gateway API gateway or an Ingress controller.
- A PostgreSQL 18 database the cluster reaches. Step 1 shows how to run one
  with CloudNativePG.

Run every command below from the root of the checkout, with `capacity-room` as
the release and the namespace.

## 1. A database

The chart reads the database DSN from an existing Secret named by
`database.existingSecret`, at the key `database.key` (default `uri`).

Create the namespace first, for either way:

```bash
kubectl create namespace capacity-room
```

With CloudNativePG, install the operator once per cluster. It is the chart
`cloudnative-pg`, version 0.29.1 (operator 1.30.1), and it watches every
namespace.

```bash
helm repo add cnpg https://cloudnative-pg.github.io/charts
helm upgrade -i cnpg cnpg/cloudnative-pg --version 0.29.1 -n cnpg-system --create-namespace --wait
```

Then make the database from `examples/cloudnative-pg-cluster.yaml`. It is a
manifest for `kubectl apply`, not a values file: one instance of PostgreSQL 18,
a `capacityroom` database owned by `capacityroom`, and 5Gi of storage. Set
`storage.size` to your need, name a `storage.storageClass` if the cluster has
no default one, and pin the image by digest,
`ghcr.io/cloudnative-pg/postgresql:18@sha256:<digest>`, which
`docker buildx imagetools inspect ghcr.io/cloudnative-pg/postgresql:18`
prints.

```bash
kubectl apply -n capacity-room -f deploy/helm/capacity-room/examples/cloudnative-pg-cluster.yaml
kubectl -n capacity-room wait cluster/capacity-room-db --for=condition=Ready --timeout=300s
```

The operator then creates the Secret `capacity-room-db-app`, whose key `uri`
holds the DSN. In the values, write `database.existingSecret: capacity-room-db-app`
and keep the default key `uri`. The example runs one instance, with no backups,
no pooler, and no monitoring; if the data matters, set up backups as
CloudNativePG's own documentation describes.

For any other PostgreSQL 18, make the Secret yourself and name it in
`database.existingSecret`.

```bash
kubectl -n capacity-room create secret generic capacity-room-db \
  --from-literal=uri='postgres://user:password@host:5432/capacityroom'  # example: replace
```

## 2. The Secrets

Create each in the namespace `capacity-room` before installing. The server
secret and the admin password are required; the rest are optional.

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

List the pull secret in the values as `imagePullSecrets: [{name: registry-pull}]`.
A key of the credentials Secret or of the CA ConfigMap is the file name that a
profile gives.

## 3. A values file

Copy `examples/gateway.yaml` for a Gateway API route, or `examples/ingress.yaml`
for an Ingress, to `values.yaml`. Replace the hosts (`example.com`), the Secret
names, the gateway's `parentRefs`, and the profile. Set `database.existingSecret`
to the Secret of step 1. The chart refuses a route and an Ingress enabled
together, and a key it does not know.

Three settings decide whether the first install works. Set `admin.username` and `admin.existingSecret` at the first install, or nobody can sign in. List each metric source in `platform.allowed_endpoint_hostnames` or `platform.allowed_endpoint_cidrs`, since nothing is reachable until one lists it. Behind a gateway, set `platform.trusted_proxies` to the gateway's range, or every client counts as the gateway for the sign-in throttle; section 12 of `docs/operations.md` shows how to find it.

The main values follow. Each has a comment in `values.yaml` that is the
reference.

| Value | Default | What it does |
|---|---|---|
| `image.repository`, `image.tag`, `image.pullPolicy` | `alvesmarcelocf/capacity-room`, the chart's `appVersion`, `IfNotPresent` | The runtime image. |
| `imagePullSecrets` | `[]` | Secrets that let the cluster pull a private image. |
| `database.existingSecret`, `database.key` | required, `uri` | The Secret that holds the database DSN, and its key. |
| `serverSecret.existingSecret`, `serverSecret.key` | required, `server-secret` | The Secret that holds the server secret, and its key. |
| `admin.username`, `admin.existingSecret`, `admin.passwordKey`, `admin.language` | empty, empty, `password`, `en` | The first administrator, created by a Job at the first install. Leave `username` empty over a database that already holds a user. |
| `credentials.existingSecret` | empty | The Secret whose keys are the credential files that profiles name. |
| `caBundles.existingConfigMap` | empty | The ConfigMap whose keys are the CA bundles that profiles name. |
| `platform` | `{}` | The keys of `platform.toml`, by their own names, such as `trusted_proxies` and `allowed_endpoint_hostnames`. |
| `profiles` | `[]` | The profiles a Job imports at every install and upgrade. |
| `route.enabled`, `route.parentRefs`, `route.hostnames` | `false`, `[]`, `[]` | A Gateway API HTTPRoute. |
| `ingress.enabled`, `ingress.className`, `ingress.annotations`, `ingress.hosts`, `ingress.tls` | `false`, empty, `{}`, `[]`, `[]` | An Ingress, for a cluster with an Ingress controller. |
| `serve.resources` | 100m CPU and 256Mi requested, 1Gi memory limit | Resources of the `serve` pod. |
| `worker.resources` | 250m CPU and 512Mi requested, 4Gi memory limit | Resources of the `worker` pod; write the limit in `Mi` or `Gi`, at least `512Mi`. |
| `jobs.resources` | 50m CPU and 128Mi requested, 256Mi memory limit | Resources of the two hook Jobs. |

## 4. Install

Check the render against the cluster first, then install.

```bash
helm upgrade -i capacity-room deploy/helm/capacity-room -n capacity-room --dry-run=server -f values.yaml
helm upgrade -i capacity-room deploy/helm/capacity-room -n capacity-room -f values.yaml --wait --timeout 10m
kubectl -n capacity-room get pods
```

## 5. Sign in

Open the route's host (`route.hostnames`, or `ingress.hosts`) in a browser and
sign in as `admin.username` with the password of the `capacity-room-admin`
Secret.

## Upgrade and uninstall

To upgrade, check out the new version and run the install command again.

```bash
helm uninstall capacity-room -n capacity-room
```

Uninstall removes what the chart made. The database and every Secret stay, so a
new install over the same database finds its users and reports; leave
`admin.username` empty then.

## More

Section 12 of `docs/operations.md` covers the Jobs, the probes and stop times,
and `trusted_proxies` behind a gateway.
