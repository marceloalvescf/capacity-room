VERSION := $(shell tr -d '[:space:]' < VERSION)
IMAGE := k8s-capacity-report:$(VERSION)
TEST_IMAGE := k8s-capacity-report-test:$(VERSION)
GOLANGCI := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
TYPST_VERSION := 0.15.1
GATE_LOGS := .cache/gate

# Extra flags for `docker build`; the gate sets them to build from scratch.
BUILD_FLAGS ?=

# The test image runs as the runtime user over a read-only source.
RUN_TEST_IMAGE := docker run --rm --user 10001:10001 \
	--volume "$(CURDIR):/src:ro" --workdir /src $(TEST_IMAGE)

.PHONY: build test lint image test-toolchain check-runtime gate

build:
	go build -o bin/k8s-capacity-report ./cmd/k8s-capacity-report

test:
	go test ./...

lint:
	@unformatted="$$(gofmt -l . )"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt: these files need formatting:"; echo "$$unformatted"; exit 1; \
	fi
	go vet ./...
	go vet -tags=toolchain ./...
	go run $(GOLANGCI) run

image:
	docker build $(BUILD_FLAGS) --file deploy/Containerfile --target runtime --tag $(IMAGE) .
	docker build $(BUILD_FLAGS) --file deploy/Containerfile --target test --tag $(TEST_IMAGE) .

test-toolchain: image
	$(RUN_TEST_IMAGE) go test -v -tags=toolchain ./internal/typst/...

check-runtime:
	test "$$(docker run --rm $(IMAGE) --version)" = "k8s-capacity-report $(VERSION)"
	test "$$(docker run --rm --entrypoint /usr/local/bin/typst $(IMAGE) --version | cut -d' ' -f2)" = "$(TYPST_VERSION)"
	test "$$(docker inspect --format '{{.Config.User}}' $(IMAGE))" = "10001:10001"

# Runs every check from freshly built images and keeps one log per step.
# `pipefail` makes a failing step stop the gate even though its output is
# piped into `tee`.
gate: SHELL := /bin/bash
gate: .SHELLFLAGS := -o pipefail -c
gate:
	@mkdir -p $(GATE_LOGS)
	$(MAKE) lint 2>&1 | tee $(GATE_LOGS)/lint.log
	$(MAKE) test 2>&1 | tee $(GATE_LOGS)/test.log
	$(MAKE) image BUILD_FLAGS="--no-cache --pull" 2>&1 | tee $(GATE_LOGS)/image.log
	$(MAKE) test-toolchain 2>&1 | tee $(GATE_LOGS)/test-toolchain.log
	$(MAKE) check-runtime 2>&1 | tee $(GATE_LOGS)/check-runtime.log
	@echo "gate passed for version $(VERSION)"
