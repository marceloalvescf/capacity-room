# k8s-capacity-report

`k8s-capacity-report` generates Kubernetes capacity reports from a
Prometheus-compatible API. It reads metrics only: it never uses a kubeconfig,
the Kubernetes API, or cloud credentials. Reports are produced as PDF through
Typst, in Portuguese (Brazil) or English.

The project is under construction. This file documents what exists today; the
full design is in
[docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md](docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md).

## What exists today

- One binary that prints its version.
- A container image that carries the binary and Typst 0.15.1.
- Guards that protect the structure of the repository.
- The report template, in two layers under [template/](template/): a generic
  page shell and the layout of the capacity report. It turns a JSON data file
  into a PDF, in Portuguese (Brazil) or English.
- Golden pages that pin how the template renders.

## Requirements

- Go 1.27
- Docker with Buildx

## Commands

```bash
make build            # bin/k8s-capacity-report
make test             # Go tests that need no service and no tool
make lint             # gofmt, go vet, golangci-lint
make image            # runtime image and test image
make test-toolchain   # checks typst and pdftotext inside the test image
make test-template    # compiles the template and its fixtures in the test image
make test-golden      # compares rendered pages with the golden pages
make gate             # everything above, from freshly built images
make update-golden    # rewrites the golden pages after an intended change
```

```bash
bin/k8s-capacity-report --version
```

## Compiling a report by hand

Until the command that collects metrics exists, the template can be compiled
from one of its fixtures. It needs `typst` 0.15.1.

```bash
mkdir -p .cache/try
cp -R template/graphite template/capacity .cache/try/
cp template/tests/capacity/fixtures/detailed.en.json .cache/try/report.json
typst compile --root .cache/try --font-path .cache/try/graphite/assets/fonts \
  --ignore-system-fonts --ignore-embedded-fonts --input data=/report.json \
  .cache/try/capacity/main.typ .cache/try/report.pdf
```

## Documents

| File | Content |
|---|---|
| [AGENTS.md](AGENTS.md) | Working agreement: environment, gates, phases, versioning |
| [CHANGELOG.md](CHANGELOG.md) | What changed in each version |
| [docs/superpowers/specs/](docs/superpowers/specs/) | Design |
| [docs/superpowers/plans/](docs/superpowers/plans/) | Implementation plans, one per phase |
| [docs/decisions/](docs/decisions/) | Decisions taken from measurements |
| [docs/reference/](docs/reference/) | Documents and inventories of the two original projects |
| [template/graphite/README.md](template/graphite/README.md) | The generic template package |

## Licence

The code is under the MIT licence, in [LICENSE](LICENSE). The Liberation Sans
fonts under `template/graphite/assets/fonts` are under the SIL Open Font
License, kept beside them.
