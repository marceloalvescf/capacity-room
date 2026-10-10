# The worker of the end-to-end stack: the runtime image named by
# RUNTIME_IMAGE, plus typst-switch, which the worker runs as typst. `make e2e`
# builds it; `make image` never does. The Go base equals the one of
# deploy/Containerfile, which a guard test checks.
ARG RUNTIME_IMAGE

FROM golang:1.27.2@sha256:5bc7f572bbaa98885a3a1fd9c0aa76b59e3e14e8628bfc316bbfd0c701e4818c AS switch
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/typst-switch/ ./cmd/typst-switch/
RUN CGO_ENABLED=0 go build -trimpath -o /out/typst-switch ./cmd/typst-switch

FROM ${RUNTIME_IMAGE}
COPY --from=switch /out/typst-switch /usr/local/bin/typst-switch
