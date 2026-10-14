#!/usr/bin/env bash
# Writes the JSON schemas that kubeconform reads in the chart's tests: the
# Kubernetes kinds the chart emits, from yannh/kubernetes-json-schema at a
# fixed commit, and HTTPRoute, converted from the Gateway API release's CRD
# by kubeconform's own script. Run it from the repository root; it needs
# the network and nix, whose locked nixpkgs gives it python.
set -euo pipefail
cd "$(dirname "$0")"
K8S_SCHEMA_COMMIT=817113f21a2643311cbc3e992bb44ffe1c416c54
K8S_VERSION=v1.37.0
GATEWAY_API=v1.6.3
KUBECONFORM=v0.8.0
mkdir -p kubernetes gateway.networking.k8s.io
for kind in configmap-v1 deployment-apps-v1 job-batch-v1 ingress-networking-v1 service-v1 serviceaccount-v1; do
  curl -fsSL "https://raw.githubusercontent.com/yannh/kubernetes-json-schema/${K8S_SCHEMA_COMMIT}/${K8S_VERSION}-standalone-strict/${kind}.json" \
    -o "kubernetes/${kind}.json"
done
mkdir -p "$(git rev-parse --show-toplevel)/.cache"
work="$(mktemp -d "$(git rev-parse --show-toplevel)/.cache/schemas.XXXXXX")"
trap 'rm -rf "$work"' EXIT
curl -fsSL "https://github.com/kubernetes-sigs/gateway-api/releases/download/${GATEWAY_API}/standard-install.yaml" -o "$work/standard-install.yaml"
curl -fsSL "https://raw.githubusercontent.com/yannh/kubeconform/${KUBECONFORM}/scripts/openapi2jsonschema.py" -o "$work/openapi2jsonschema.py"
( cd "$work" && DENY_ROOT_ADDITIONAL_PROPERTIES=true FILENAME_FORMAT='{kind}_{version}' \
  nix shell --impure --expr '(builtins.getFlake "git+file://'"$(git rev-parse --show-toplevel)"'").inputs.nixpkgs.legacyPackages.${builtins.currentSystem}.python3.withPackages (p: [ p.pyyaml ])' \
  -c python3 openapi2jsonschema.py standard-install.yaml )
cp "$work/httproute_v1.json" gateway.networking.k8s.io/httproute_v1.json
