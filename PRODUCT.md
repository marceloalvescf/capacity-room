# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

SREs and platform engineers who run Kubernetes clusters. They request a
capacity report for a cluster, a set of namespaces, and a period; triage its
findings; and hand the PDF on. Within a deployment they hold one of three
roles: a reader reads every report, an operator also requests reports and
PDFs, and an administrator also manages users and metric-source profiles.
Managers and other decision makers receive the PDF; the web app is not
designed for them.

## Product Purpose

The product answers whether a cluster has room to grow and where its
capacity runs short: requests and limits against observed usage per
namespace, workload, HPA, node, and node pool, and the cluster autoscaler
against its maximum. It turns a period of metrics into findings with a
severity, an action, and their evidence, worded the same on the web and in
the PDF, in Portuguese (Brazil) or English.

Success: an SRE opens a report and knows within seconds what is critical,
where, and what to do first, and trusts every mark enough to act on it.

## Positioning

- Metrics only. It reads a Prometheus-compatible API and never uses a
  kubeconfig, the Kubernetes API, or cloud credentials, so it can be pointed
  at a cluster without any access to it.
- One snapshot is the single source of a report: the web view and the PDF
  are two renderings of it, with the same words and numbers.
- Its numbers equal those of the original program it replaced, exactly.

## Operating Context

- Open source, self-hosted by each team behind its own TLS proxy, with
  PostgreSQL, through the Compose stack described in `docs/operations.md`.
- Used on desktop browsers; 1024 px wide is the floor. Phones are not a
  target (owner, 2026-10-11).
- The PDF, rendered with Typst, is what travels to people outside the app.

## Capabilities and Constraints

- Reports: request, progress, a seven-tab report screen (overview, findings,
  namespaces, workloads, autoscaling, nodes, data quality), PDF download in
  either language, retention and expiry.
- Administration: users with roles; metric-source profiles with revisions,
  credential and CA files by name, and a connection test.
- Strict CSP: no inline style and no style injected at run time.
- Every text comes from the locale files; technical terms (request, limit,
  workload, node, node pool, HPA, replica, autoscaling, throttling) stay in
  English in both languages.
- Status is drawn as a shape, then a label, then a colour, with the five
  shapes of the PDF.

## Brand Commitments

- Product name in the web app and the documentation: **CapacityRoom**
  (owner, 2026-10-11). The command, the binary, the image, the repository,
  the `KCR_` environment variables, and the PDF keep the name
  `k8s-capacity-report`.
- No company branding: each deployment sits inside someone else's
  organisation.
- The PDF has its own monochrome look (the graphite template; no coloured
  pixel on a page). The web app need not copy it.

## Evidence on Hand

- Recorded scenarios of real and constructed clusters under `testdata/`,
  and the golden views under `web/tests/fixtures/view/`, for realistic
  screens.
- No customers, testimonials, benchmarks, or adoption figures exist; none
  may be invented.

## Product Principles

1. A mark must be true. A green mark beside a problem costs trust in every
   other mark; when the data does not say, the screen says so.
2. Answer the capacity question first, then show the evidence behind it.
3. Say it the way the report says it: one wording, one set of numbers.
4. Built for the operator's desk: dense where it helps scanning, never
   decorative.

## Accessibility & Inclusion

- WCAG AA contrast for every token pair in both themes (4.5:1 text, 3:1 large
  text, borders, and focus rings), checked by tests.
- Full keyboard use; every status carries its word for screen readers; axe
  runs in every end-to-end spec.
