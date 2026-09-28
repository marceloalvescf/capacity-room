# Research notes: document, snapshot, and frontend of kubernetes-report 1.11.0 (from explore agent, 2026-09-27)

Source: older/kubernetes-report/kube_capacity_report/{renderers,snapshots}/, tests/golden/typst (10 goldens), frontend/

## Document order
preamble -> auxiliary page (outline depth 2 titled "Sumário", metadata table, status legend, quality block) -> pagebreak -> body
Detailed body: 1 "Capacidade por Namespace"; 2 "Recursos por carga de trabalho"; 3 "Histórico de HPA" (per namespace: "HPAs observados", "Cargas de trabalho sem HPA");
  4 "Uso de recursos por pool de nós" ("Sumário por pool de nós", "Perfil de uso agregado por pool de nós", "Detalhamento por nó"); 5 the whole executive body; 6 "Apêndice técnico de qualidade"
Executive body (min severity forced to at least attention): "Resumo executivo"; "Cargas de trabalho com prioridade crítica"; "Cargas de trabalho com prioridade alta";
  "Namespaces com carga de trabalho em risco"; "Cargas de trabalho com máximo de réplicas e sem HPA"; "Situação dos pools de nós"; "Plano de ação priorizado"; "Limitações materiais"; then the appendix (+ "Avisos de interpretação")
Every empty section prints its own fallback sentence instead of a table.

## Tables (header text; column fractions; font size)
- Metadados do relatório: Campo, Valor; (1fr,3fr); 8.5pt; inset-y 2pt. Rows Ambiente, Período analisado, Namespaces, Containers excluídos, [Organização], [Classificação]
- Legenda de status: Status, Interpretação; (0.7fr,3fr); 8.5pt; 5 rows healthy/attention/critical/unavailable/undefined
- Capacidade por Namespace: Recurso, Uso médio, Pico, Solicitação, Limite, % Pico/Solicitação, % Pico/Limite; (0.8,1,1,1,1,0.9,0.9); 7.1pt; rows CPU, Memória
- Recursos por carga de trabalho: Carga de trabalho, Pods, CPU média, CPU pico, % CPU/Solicitação, % CPU/Limite, Memória média, Memória pico, % Memória/Solicitação, % Memória/Limite; (2.2,0.45,0.65,0.65,0.72 x6); 6.2pt
- HPAs observados: HPA, Spec Min, Spec Max, Atual (Avg), Mín Atingido, Máx Atingido, Situação; (2.2,0.65,0.65,0.8,0.72,0.72,1.15); 7pt; sorted by name
- Cargas de trabalho sem HPA: Tipo, Carga de trabalho, Qtde. Pods, Situação; (0.9,2.2,0.65,2.1); 7.4pt; sorted (kind,name)
- Sumário por pool de nós: Pool de nós, Nós, CPU média, CPU pico, Memória média, Memória pico, Confiança; (1.6,0.45,0.75,0.75,0.85,0.85,0.9); 7.1pt
- Perfil de uso agregado por pool de nós: Pool de nós, CPU alocável/nó, CPU usada/nó, CPU solicitada/nó, Memória alocável/nó, Memória usada/nó, Memória solicitada/nó; (1.5,0.9,0.85,0.95,1,0.95,1.05); 6.8pt
- Detalhamento por nó: Nó, CPU méd/pico, Mem méd/pico, Motivo, Estado; (2.4,1.5,1.6,2.2,1.2); 7.1pt
- Prioridade crítica / alta: Namespace, Carga de trabalho, Qtde. Pods, Motivo; (1.25,2,0.55,4); 7.2pt; sorted (namespace,kind,name)
- Namespaces em risco: Namespace, CPU % / Lim, Mem % / Lim, Motivo, Status, Prioridade; (1.35,0.75,0.75,3,1.25,0.85); 7.2pt; sorted (-severity, -max pct, namespace)
- Máximo de réplicas e sem HPA: Namespace, Carga de trabalho, Tipo, Réplicas, Situação; (1.25,2,0.9,1.5,1.7); 7.2pt; critical rows first
- Situação dos pools de nós: Pool de nós, Situação, Observação; (1.8,0.9,3.5); 7.3pt; sorted (-severity, shown name)
- Plano de ação priorizado: Ordem, Prioridade, Ação, Cargas de trabalho / Escopo; (0.8,0.8,2.7,2.6); 7.2pt; sorted (-severity, code, scopes); excludes NODE_/NODEPOOL_ codes
- Limitações materiais: Capacidade, Cobertura, Observação; (1.8,0.8,3); 7.5pt
- Apêndice técnico de qualidade: Capacidade, Cobertura, Fonte, Observação, Evidência técnica; (1.6,0.8,1,2,2.4); 7pt
- Avisos de interpretação: Observação; (1fr); 7.5pt; de-duplicated warnings + notices

## Typst helpers (sections.py:44-89)
- report-ident(value) = text(size: 0.92em, hyphenate: false, lang: "en", value)
- report-placeholder(value) = text(size: 0.92em, value)
- report-status(kind, value): healthy "●", attention "▲", critical "♦", undefined "○", else "—"; grey luma(45%) for healthy/unavailable/undefined, black otherwise; bold for critical
- data-table(headers, rows, columns, size 8pt, align left+top, inset-y 3.5pt): text size, par(justify false, leading 0.45em, spacing 0.4em), table inset (x 3pt), stroke 0.5pt, bold header, table.header(repeat: true)
- Callout markers of the package: note "●", info "■", warning "▲". A status glyph may share a marker only when the meaning agrees; "■" is not used for status.

## How graphite-report is called
Only three arguments: title = custom cover block, outline false, footer = custom footer. No header. Organization and classification appear in the metadata table.
Cover: "LEVANTAMENTO DE CAPACIDADE" 22pt bold; "Cluster: <name>" 12pt; "ELABORADO POR: Time SRE" (author hard-coded); "EMITIDO EM: dd/mm/yyyy hh:mm" 9pt.
Footer: from page 2, right aligned, "<Classificação> · Página <n>" (prefix only when classification set).

## Labels (renderers/text.py)
severity Saudável/Atenção/Crítico; availability Disponível/Parcial/Indisponível; strategy Regra de gravação/Métricas brutas/Configuração/Sem fonte; confidence Exata/Inferida/Desconhecida; priority Baixa/Alta/Crítica
capabilities: Uso por namespace; Uso por carga de trabalho; Solicitações e limites; Histórico de HPA; Cobertura de cargas de trabalho sem HPA; Associação entre endereço e nó; Identificação de pools de nós; Utilização dos nós; Escalonamento automático de nós
Web differs: strategy Configurada/Nenhuma. Web-only labels: HPA coverage Coberto/Sem HPA/Desconhecido; autoscaling status Ativado/Desativado/Desconhecido; scope types.
The renderer also BUILDS prose: _finding_reason_fragment per code suffix (e.g. "{CPU}: {pct}% solicitada do alocável", "{resource}: variação de {pct}%"), capability evidence sentences (subject + status suffix), risk status "1 crítica · 2 altas".

## Formatters
decimal: fixed digits, trailing zeros trimmed, comma. count: rounded integer. replica average: 1 decimal.
cpu: |v| < 1 -> millicores 1 decimal "mCPU"; else 2 decimals "cores". memory: B/KiB/MiB/GiB/TiB by 1024, 2 decimals. percent: 1 decimal + "%".
datetime dd/mm/yyyy hh:mm in report timezone. period: largest exact unit, singular/plural ("24 horas").

## Placeholders
unavailable -> "Indisponível" (status kind unavailable, glyph "—"); no base -> "Não definido" (kind undefined, "○"); unknown pool -> "Indeterminado";
HPA without target -> "(alvo não identificado)"; unknown workload kind -> "(Tipo não informado)"; more than 3 targets in a recommendation -> "<N> <scope summary>".

## Snapshot V1 of the original (pydantic, extra forbid, schema_version "1.0")
report{report_id, job_id, profile_id, profile_revision, generator_version, emitted_at, period_start, period_end, timezone, pdf_profile, min_severity, graphite_package, organization, classification, thresholds{warning,critical}, settings{...nonsecret}}
scope{cluster_name, cluster_label, filter_mode, matcher_enabled, confidence, discovered_values, selected/requested/allowed/excluded_namespaces, excluded_containers}
quality{capabilities[{name,status,strategy,evidence,reason}], warnings[str], notices[str], unavailable_sections, omitted_sections, collection_started_at, collection_completed_at}
summary{counts..., autoscaling{status, source, confidence, managed_node_groups, current_nodes, min_nodes, max_nodes, scale_events, unschedulable_pods}}
namespaces[{namespace, active_pods, approximate_ratios, requests_limits_status, 10 x MetricStatistics}]
workloads[{namespace, kind, name, observed_usage, hpa_coverage covered|uncovered|unknown, active_pods, approximate_ratios, requests_limits_status, 10 x MetricStatistics}]
hpas[{namespace, name, target_kind, target_name, spec_min, spec_max, current, desired}]
nodes[{name, pool, pool_confidence, capacities, allocatables, 4 x MetricStatistics, ready, schedulable}]  (internal IP, hostname, provider id never published)
nodepools[{name, confidence, nodes[names]}]
findings[{code, severity, scope_type, scope_kind, scope_name, summary, evidence[{name,value}], recommendation, confidence}]
recommendations[{code, severity, scope_type, action, scopes, confidence}]
MetricStatistics{average, minimum, peak, current, no_base}
Mapper sorts everything deterministically and checks integrity (no repeated identity; a workload cannot be both HPA target and "without HPA"). Serving narrows findings by effective minimum severity; storage is never narrowed.

## Frontend stack of the original
react 19.2, react-router 8.3, @tanstack/react-query 5, @tanstack/react-table 9, react-hook-form 7 + zod 4, radix-ui 1.6, class-variance-authority, tailwind-merge, @phosphor-icons/react, tailwindcss 4.3 (CSS-first tokens), vite 8, typescript 7
fonts @fontsource-variable/inter and geist-mono; tests vitest 5, testing-library, vitest-axe, msw 2, playwright 1.63
Routes: /login; /reports; /reports/:id/{overview,findings,namespaces,workloads,autoscaling,nodes,data-quality}; /generate; /jobs/:jobId; admin: /profiles, /profiles/new, /profiles/:id/edit, /users, /settings
Findings tab: master/detail, filters in the URL, free-text search, fixed critical-first order, selection in URL. Shared ResourceTable with sticky header and first column, URL-held sort and filter, detail sheet per row.
Generate form: profile, namespaces (from profile allowed_namespaces), period (profile permitted list or presets 1h/6h/24h/7d/15d/30d), timezone, PDF profile, minimum severity; long lookback note above 15 days; idempotency key per submission.
Progress: polling 1.5 s x20, 3 s x20, then 6 s; stops when inactive; phase stepper; short completion beat before opening the report.
Design: OKLCH tokens, light and dark, state colours success/attention/critical, status always glyph + label (same glyphs as the PDF), WCAG AA contrast test, reduced motion honoured, skip link, aria-live regions.
CSP: no inline style; two pinned style hashes for Radix; react-remove-scroll-bar replaced by a local module that injects no style element.
Tests: 34 Vitest files, vocabulary guard (8 cases), 4 Playwright specs (happy path, security, concurrency, PDF recovery).

## Consequences for the Go design
- The catalog is larger than the findings: headings, table headers, legend, fallback sentences, reason fragments, capability sentences, labels. Expect a few hundred entries per language.
- Author is hard-coded "Time SRE" in the original; make it a configuration value.
- graphite-report is used only as page shell (cover, footer, theme); language must be passed through theme lang/region for hyphenation.
- The real graphite-report source is not inside kubernetes-report.
