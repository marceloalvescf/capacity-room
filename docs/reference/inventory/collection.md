# Research notes: collection side of kubernetes-report 1.11.0 (from explore agent, 2026-09-27)

Source: older/kubernetes-report/kube_capacity_report/{metrics,collectors,resolvers}/, collection.py, application.py, config.py, cli.py, redaction.py, tests/fakes, tests/fixtures/prometheus

## HTTP client (metrics/client.py, endpoint.py)
- Always POST, form-encoded body. /query {query,time}; /query_range {query,start,end,step}. Accept: application/json. Bearer token header when token set.
- Timeout 90 s per request. 3 attempts, backoff 2**attempt, on connection error/timeout and on 429/5xx. Retry-After honoured when all digits, capped at 60 s.
- Ambient proxy variables never honoured (trust_env False). verify = CA bundle path, else tls_verify bool. TLS failure -> TlsError.
- Errors: 401 AuthenticationError; 403 AuthorizationError; 404 and 405 EndpointNotFoundError (wrong candidate); 429/5xx retried then MetricsError; other non-OK QueryError;
  unparseable JSON or missing status: EndpointNotFoundError if the client never answered yet, else MetricsError "mid-collection" (flag `_answered` lives for the whole client);
  status != success -> QueryError with errorType.
- Candidates: URL must be absolute http/https, no userinfo, no query, no fragment; trailing slash stripped. grafana type requires "datasource" or "proxy" in path; auto infers grafana from path.
  Roots: [url] plus url + "/api/v1" unless path already ends in /api/v1 or contains /api/v1/. Deduplicated in order.
- Probing: smoke test `vector(1)`; only EndpointNotFoundError moves to next candidate; other errors propagate; all missing -> "no candidate endpoint answered the Prometheus API".

## Cluster scope (metrics/capabilities.py:48-112)
- `count by ({label}) (kube_node_info)`, fallback `count by ({label}) (up)`; also detects unlabeled series.
- required: name must be among values else ClusterScopeError; matcher on, EXACT.
- none: error if labeled and unlabeled both visible, or more than one value; matcher off.
- auto: name present -> as required; other values present without name -> error; no values -> matcher off, INFERRED.
- excluded_containers: added to built-in exclusions "" and "POD", applied only to container-level metrics.

## 9 capabilities (order of detect_capabilities)
namespace_usage, workload_usage, requests_limits, hpa_history, workloads_without_hpa, ip_node_mapping, nodepool_discovery, node_utilization, node_autoscaling
- namespace_usage: available when container_cpu_usage_seconds_total and container_memory_working_set_bytes both present; partial with one.
- workload_usage: usage + recording rule namespace_workload_pod:kube_pod_owner:relabel -> RECORDING_RULE; with raw kube_pod_owner -> RAW_METRICS (partial).
- requests_limits: all 4 unified kube_pod_container_resource_{requests,limits}{resource=cpu|memory}, or all 4 legacy _cpu_cores/_memory_bytes; partial otherwise.
- hpa_history: all 5 HPA metrics.
- workloads_without_hpa: deployment inventory + statefulset inventory + kube_horizontalpodautoscaler_info.
- ip_node_mapping: kube_node_info. nodepool_discovery: kube_node_labels (partial with only kube_node_info, name parsing).
- node_utilization: recording rules instance:node_cpu_utilisation:rate5m + instance:node_memory_utilisation:ratio, else raw node_cpu_seconds_total + node_memory_MemAvailable_bytes + node_memory_MemTotal_bytes.
- node_autoscaling: CONFIGURED when analysis.node_autoscaling enabled/disabled; else cluster_autoscaler_.+ metrics; never fully unavailable (partial).
Strategy: RECORDING_RULE, RAW_METRICS, CONFIGURED, NONE. Availability: AVAILABLE, PARTIAL, UNAVAILABLE.
Evidence = names actually confirmed present; collectors read evidence to choose legacy vs unified metric names (hidden coupling to port).

### Capability reason sentences (12)
- "As métricas necessárias não foram encontradas."
- "A métrica de uso de {CPU|memória} não foi encontrada."
- "As métricas de uso de CPU e memória não foram encontradas."
- "A associação histórica entre pods e cargas de trabalho não pôde ser reconstruída com exatidão." (+ optional missing-usage sentence appended)
- "A associação por carga de trabalho não está disponível; a cobertura está limitada a pods."
- "A cobertura de solicitações e limites está incompleta para CPU ou memória."
- "As métricas legadas oferecem cobertura parcial de recursos."
- "A cobertura histórica dos HPAs está incompleta."
- "O inventário de cargas de trabalho ou alvos de HPA está incompleto."
- "A associação entre endereços e nós pode estar incompleta."
- "Os rótulos de pools de nós não foram encontrados."
- "O estado do escalonamento automático de nós não pôde ser determinado."

## PromQL (17 structural templates)
- probe: `vector(1)`; `count({selector})`; `count({__name__=~"cluster_autoscaler_.+"[,cluster="X"]})`
- namespace: `sum by (namespace) (rate(<cpu>[{step}]))` range; `sum by (namespace) (max by (namespace, pod, container) (<mem>))` range;
  4x `sum by (namespace) (<requests|limits, resource>)` range; pods `sum by (namespace) (<phase="Running"> == 1)` instant
- workload, recording rule: `sum by (namespace, workload, workload_type) (<expr> * on ([cluster,]namespace, pod) group_left(workload, workload_type) <ownership>)` for cpu rate, memory, 4 requests/limits;
  `count by (namespace, workload, workload_type) (<ownership>)` instant
- workload, raw: kube_pod_owner{owner_is_controller="true"} and kube_replicaset_owner{owner_is_controller="true"} instant; `sum by (namespace, pod) (...)` range; join done client-side
- node: bare selectors on kube_node_info, node_uname_info, kube_node_labels, kube_node_status_capacity, kube_node_status_allocatable, kube_node_status_condition{condition="Ready"}, kube_node_spec_unschedulable;
  `sum by (node, instance) (<requests>)`; rule `<selector> * 100` range; raw cpu `100 - avg by(instance) (rate(<node_cpu_seconds_total,mode="idle">[{step}])) * 100`; raw mem `(1 - <MemAvailable> / <MemTotal>) * 100`
- hpa: kube_horizontalpodautoscaler_info, _spec_min_replicas, _spec_max_replicas instant; _status_current_replicas, _status_desired_replicas range; inventory kube_deployment_labels|kube_deployment_spec_replicas, kube_statefulset_labels|kube_statefulset_replicas
- autoscaler (only when node_autoscaling=auto), range: cluster_autoscaler_node_groups_count, _nodes_count, _node_group_min_nodes_count, _node_group_max_nodes_count, _unschedulable_pods_count; `increase(<cluster_autoscaler_scaled_up_nodes_total>[{window}s])`

## Collector sentences
- namespaces: "As métricas de uso dos namespaces estão incompletas."; "As métricas de solicitações e limites dos namespaces estão incompletas."; "... estão indisponíveis."
- workloads: "As métricas de uso das cargas de trabalho estão incompletas."; requests/limits pair incompletas/indisponíveis; raw only "O histórico usa a associação atual entre pods ativos e cargas de trabalho.";
  notices (plural pairs): "{count} cargas de trabalho deixadas de fora por não terem consumo observado no período."; "{jobs} Jobs foram deixados de fora do inventário de cargas de trabalho; o consumo continua contabilizado nos totais dos namespaces."
- hpa: notice "{omitted} cargas deixadas de fora do inventário de cobertura por estarem com escala zero." plus the two HPA ambiguity warnings from models.py
- nodes: "As métricas de identidade dos nós estão indisponíveis."; "As evidências de capacidade dos nós estão incompletas."
Warning vs notice: a notice survives perfect Prometheus answers; a warning vanishes with them.
Total reader sentences on the collection side: 26 families.

## Resolvers
- workload: pod owner; ReplicaSet replaced by its own owner (Deployment) when known; no owner -> StandalonePod.
- node: unique indexes by name, internal IP, hostname; ambiguous values dropped; resolve by node label, else host from instance label (port stripped) by IP, hostname, name.
- node pool, first match wins: configured label (EXACT); known labels label_kubernetes_azure_com_agentpool, label_agentpool, label_cloud_google_com_gke_nodepool, label_eks_amazonaws_com_nodegroup (EXACT);
  AKS name `^aks-([a-z0-9]+?)-[0-9]+-vmss$` (INFERRED); OKE name starting "oke" with >= 4 dash parts -> pool-{parts[2]} (INFERRED); label_name shared by more than one node (INFERRED); else unknown (UNKNOWN).
  Pool confidence = minimum over member nodes.

## Configuration
Unknown keys or sections are errors. Precedence: file, then environment, then flags. Token: --token wins over the variable named by metrics.token_env; metrics.token in file is rejected.
Keys: metrics.{url*, endpoint_type auto|prometheus|grafana, token_env, tls_verify true, ca_bundle, timeout_seconds 90}; cluster.{name*, namespaces*, label "cluster", filter_mode required|auto|none, nodepool_label};
analysis.{period "24h" ^[1-9][0-9]*[mhd]$, timezone "America/Sao_Paulo", node_autoscaling auto|enabled|disabled, excluded_containers}; thresholds (7); report.{output_directory, profile detailed|executive, min_severity all|attention|critical, organization, classification, graphite_package}
Env: KCR_METRICS_URL, KCR_ENDPOINT_TYPE, KCR_CLUSTER, KCR_CLUSTER_LABEL, KCR_CLUSTER_FILTER, KCR_NAMESPACES, KCR_PERIOD, KCR_TIMEZONE, KCR_NODE_AUTOSCALING, KCR_MIN_SEVERITY, KCR_REPORT_PROFILE, KCR_OUTPUT_DIRECTORY
Flags: --config, --metrics-url, --endpoint-type, --token, --insecure, --cluster, --cluster-label, --cluster-filter, --namespaces, --period, --timezone, --node-autoscaling, --min-severity, --report-profile, --output, --check, --verbose
Exit codes: 0 ok; 2 configuration; 3 metrics; 4 cluster scope; 5 insufficient capabilities; 6 report output.
Step table auto_step: <=30m "30s"; <=6h "2m"; <=24h "5m"; <=7d "15m"; else "1h".

## Redaction order
literal secrets; quoted authorization value; bare authorization line; token=/access_token=/api_key=; URL query string; URL userinfo. Applied to every printed error and message.

## Fake Prometheus
- scripted queue (low-level client tests) and fixture matcher: JSON file with "requests" list; match on method, exact path, optional query_contains substring, optional authorization; first match popped; order independent.
- 6 scenarios, 63-73 requests each: direct-complete, grafana-authenticated, thanos-multicluster, raw-fallback, partial-data, value-edge-cases.

## Hazards for the Go port
- NaN and Inf samples become "no data", never zero.
- Determinism: sorted matchers, sorted outputs, order-preserving de-duplication of warnings and notices.
- Period start = local emission time minus period seconds, with no daylight-saving correction; keep as is. Embed time zone data (time/tzdata).
- PromQL escaping: regex-escape each value, then escape \, newline, and " for the string; double escaping must match.
- Do not add step alignment.
