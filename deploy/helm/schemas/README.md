# Schemas for kubeconform

The chart's tests check every rendered manifest against the JSON schemas in
this folder, offline.

Pins, all in `update.sh`:

- yannh/kubernetes-json-schema at one commit (`K8S_SCHEMA_COMMIT`)
- Kubernetes `v1.37.0`, the `standalone-strict` schemas
- Gateway API `v1.6.3`, whose standard CRDs give `HTTPRoute`
- kubeconform `v0.8.0`, whose `openapi2jsonschema.py` converts the CRD

Kinds: ConfigMap, Deployment, Job, Ingress, Service, ServiceAccount, and
HTTPRoute.

Regenerate from the repository root. The script needs the network and nix;
it takes python from the nixpkgs that `flake.lock` pins:

```bash
deploy/helm/schemas/update.sh
```

A new kind the chart emits needs its schema here before kubeconform accepts
it.
