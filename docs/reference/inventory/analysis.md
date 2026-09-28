# Research notes: analysis layer of kubernetes-report 1.11.0 (from explore agent, 2026-09-27)

Source: older/kubernetes-report/kube_capacity_report/analysis/{capacity,risk,recommendations}.py, models.py, config.py

## Finding = (code, severity, scope_type, scope_name, summary, evidence[(name,value)], recommendation, confidence, scope_kind)
Sentences are literal pt-BR at the call site. Evidence already carries the numbers as name/value pairs.

## 66 finding codes
A. Resource findings, capacity.py:77-196. PREFIX in {NAMESPACE, WORKLOAD} x RESOURCE in {CPU, MEMORY}, 10 codes each = 40
  - {P}_{R}_REQUEST_{HEALTHY|ATTENTION|CRITICAL}: classify_percent(max(avg,peak) of request_percent); warning/critical_percent
      summary "Uso de {resource_label} em relação à solicitação." / rec "Revisar a solicitação de {resource_label} e o consumo observado."
  - {P}_{R}_LIMIT_{HEALTHY|ATTENTION|CRITICAL}: same on limit_percent
      summary "O uso de {resource_label} em relação ao limite indica risco aumentado de {risk}." risk = OOMKill (memory) | limitação de CPU (cpu)
      rec "Revisar o limite de {resource_label} e o perfil de consumo observado."
  - {P}_{R}_REQUEST_MISSING (ATTENTION): requests_limits_status AVAILABLE and all four request stats None
      "Há uma lacuna de configuração: solicitação de {resource_label} não observada." / "Definir a solicitação de {resource_label} com base no consumo observado."
  - {P}_{R}_LIMIT_MISSING (ATTENTION)
      "Há uma lacuna de configuração: limite de {resource_label} não observado." / "Definir ou justificar o limite de {resource_label} conforme a política da carga de trabalho."
  - {P}_{R}_OVERREQUESTED (ATTENTION): request_percent.average <= low_utilization_percent
      "O uso médio de {resource_label} ficou baixo em relação à solicitação." / "Revisar o dimensionamento da solicitação de {resource_label}."
  - {P}_{R}_BURSTY (ATTENTION): (peak-avg)/avg*100 >= bursty_gap_percent
      "O consumo de {resource_label} apresentou variação relevante entre média e pico." / "Inspecionar o perfil de rajadas de {resource_label} antes do dimensionamento."
  resource_label = "CPU" | "memória". classify_percent returns HEALTHY (not None) below warning: HEALTHY codes are really emitted.
B. Node findings, capacity.py:284-404 = 11
  - NODE_{CPU|MEMORY}_USAGE_{HEALTHY|ATTENTION|CRITICAL}: "Uso observado de {resource_label} no nó." / "Acompanhar pressão e capacidade do nó."
  - NODE_{CPU|MEMORY}_REQUEST_HIGH (ATTENTION or CRITICAL): "Solicitações de {resource_label} ocupam parcela elevada do nó." / "Revisar solicitações e margem de agendamento."
  - NODE_OVERREQUESTED (ATTENTION): REQUEST_HIGH and usage.average <= low_utilization_percent; evidence resources="cpu,memory"
      "Solicitações elevadas coexistem com baixa utilização real observada." / "Revisar o dimensionamento das cargas de trabalho agendadas no nó."
  - NODE_LOW_UTILIZATION (ATTENTION): max(cpu_avg, mem_avg) <= low_utilization_percent
      "O nó apresentou baixa utilização real média." / "Avaliar consolidação e redução segura de capacidade."
  - NODEPOOL_UNKNOWN (ATTENTION, scope nodepool "unknown", once, evidence affected_nodes)
      "O pool de nós não pôde ser determinado." / "Confirmar o pool de nós sem inferir identidade do provedor."
C. Node pool, capacity.py:407-452 = 2
  - NODEPOOL_{CPU|MEMORY}_UNEVEN (ATTENTION): >=2 nodes with peak; max(peak)-min(peak) >= nodepool_spread_points
      "A pressão de pico de {resource_label} variou entre nós do pool." / "Investigar distribuição de carga e restrições de agendamento."
D. HPA, risk.py:48-159 = 6 (scope hpa "{namespace}/{name}", last two scope workload)
  - HPA_PINNED_REPLICAS (HEALTHY): spec_min == spec_max  "O HPA está fixado em um número constante de réplicas." / "Confirmar se o escalonamento automático é desejado para esta carga de trabalho."
  - HPA_MAX_REPLICAS_REACHED (CRITICAL): current.peak >= spec_max  "O HPA atingiu o máximo observado de réplicas." / "Revisar o limite máximo e a capacidade da carga de trabalho."
  - HPA_NEAR_MAX_REPLICAS (ATTENTION): current.average/spec_max*100 >= autoscaler_near_max_percent  "O HPA operou frequentemente próximo ao máximo observado." / "Validar a margem e o limite máximo do HPA."
    (the three above are mutually exclusive, if/elif)
  - HPA_REPLICA_DIVERGENCE (ATTENTION): abs(current.avg - desired.avg) >= hpa_replica_divergence  "Há diferença média persistente entre réplicas atuais e desejadas." / "Investigar eventos, restrições de agendamento e tempo de estabilização."
  - DEPLOYMENT_WITHOUT_HPA (ATTENTION)  "Deployment sem HPA observado." / "Validar se o escalonamento horizontal é necessário para este Deployment."
  - STATEFULSET_WITHOUT_HPA_INFO (HEALTHY)  "StatefulSet sem HPA observado; registro informativo." / "Confirmar a estratégia de escala adequada ao StatefulSet."
E. Cluster autoscaling, risk.py:162-262 = 7 (scope cluster, 0 or 1 finding)
  - NODE_AUTOSCALING_UNKNOWN (ATTENTION) "O estado do escalonamento automático de nós não pôde ser determinado." / "Confirmar operacionalmente se o escalonamento automático do cluster está habilitado."
  - NODE_AUTOSCALING_DISABLED (ATTENTION) / NODE_AUTOSCALING_DISABLED_PRESSURE (CRITICAL, node pressure) same text:
      "O escalonamento automático de nós está configurado como desabilitado." / "Planejar capacidade manual e revisar a decisão de escalonamento automático."
  - NODE_AUTOSCALING_NEAR_MAX (ATTENTION): current_nodes/max_nodes*100 >= autoscaler_near_max_percent "O número de nós está próximo ao máximo observado." / "Revisar o máximo configurado e a margem do cluster."
  - NODE_AUTOSCALING_BOUNDS_UNKNOWN (HEALTHY) "Escalonamento automático habilitado sem limites observáveis nas métricas." / "Registrar os limites operacionais sem inferir valores ausentes."
  - NODE_AUTOSCALING_AT_MIN (HEALTHY) "Escalonamento automático operando no mínimo observado sem pressão de nós." / "Manter acompanhamento da demanda."
  - NODE_AUTOSCALING_ENABLED (HEALTHY) "Escalonamento automático de nós habilitado com margem observada." / "Manter acompanhamento dos limites e eventos de escala."

## Recommendations
No own wording. build_recommendations drops HEALTHY, groups findings by (code, recommendation text), merges scopes (sorted set), severity = max, confidence = most exact. Order: severity desc, confidence exact first, scope, code. 24 distinct action sentences.

## Severity and confidence
Severity: HEALTHY=0 < ATTENTION=1 < CRITICAL=2. Finding order: (-severity, scope_name, code).
min_severity lives in the renderer: all|attention|critical; executive profile forces at least ATTENTION.
Confidence: EXACT, INFERRED, UNKNOWN.

## Value states
- unavailable: Availability.UNAVAILABLE / all four stats None
- no base: MetricStatistics.no_base = true (ratio base is zero over the whole period)
- approximate: approximate_ratios on namespace/workload -> Confidence.INFERRED + evidence ("approximate", True); no wording change
- unknown pool: pool="unknown" + pool_confidence UNKNOWN

## Thresholds (defaults)
warning_percent 70, critical_percent 85 (warning < critical), low_utilization_percent 30, autoscaler_near_max_percent 80, nodepool_spread_points 30 (all 0..100)
bursty_gap_percent 100 (>= 0, no cap), hpa_replica_divergence 1 (replica count, >= 0)

## Standalone sentences (2), models.py:9-14, no parameters
- HPA_AMBIGUOUS_SPEC_WARNING "Valores ambíguos de configuração ou alvo de HPA foram omitidos."
- HPA_AMBIGUOUS_TARGET_WARNING "Alvos ambíguos de HPA impedem uma cobertura confiável das cargas de trabalho."

## Consequences for the Go design
- Keep the 66 code strings as they are. A registry maps each code to a message key plus fixed parameters (resource, risk), so the catalog needs about 24 summary and 24 action messages, not 66.
- `resource` and `risk` become enum parameters; each language's catalog owns the agreement around them (separate message variants per resource are safer than interpolating a noun).
- Numbers are computed before wording; which raw number is chosen (average vs peak) is analysis logic and must be ported exactly.
