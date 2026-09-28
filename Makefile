# Recipes use bash so that the gate can turn on `pipefail`. The flag is set
# inside each recipe line, because the `make` shipped with macOS is 3.81 and
# ignores `.SHELLFLAGS`.
SHELL := /bin/bash

VERSION := $(shell tr -d '[:space:]' < VERSION)
IMAGE := k8s-capacity-report:$(VERSION)
TEST_IMAGE := k8s-capacity-report-test:$(VERSION)
GOLANGCI := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
TYPST_VERSION := 0.15.1
GATE_LOGS := .cache/gate

# Extra flags for `docker build`; the gate sets them to build from scratch.
BUILD_FLAGS ?=

# Extra flags for `go test`; the gate sets them so that no cached result counts.
TEST_FLAGS ?=

# The test image runs as the runtime user over a read-only source.
RUN_TEST_IMAGE := docker run --rm --user 10001:10001 \
	--volume "$(CURDIR):/src:ro" --workdir /src $(TEST_IMAGE)

.PHONY: build test lint image test-toolchain check-runtime gate

build:
	go build -o bin/k8s-capacity-report ./cmd/k8s-capacity-report

test:
	go test $(TEST_FLAGS) ./...

# gofmt reads the Go files git knows about, tracked or new, so that scratch
# files under ignored folders cannot fail the check.
lint:
	@unformatted="$$(git ls-files --cached --others --exclude-standard -z -- '*.go' | xargs -0 gofmt -l)"; \
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

check-runtime: image
	test "$$(docker run --rm $(IMAGE) --version)" = "k8s-capacity-report $(VERSION)"
	test "$$(docker run --rm --entrypoint /usr/local/bin/typst $(IMAGE) --version | cut -d' ' -f2)" = "$(TYPST_VERSION)"
	test "$$(docker inspect --format '{{.Config.User}}' $(IMAGE))" = "10001:10001"

# Runs every check from freshly built images and keeps one log per step.
# Each step is piped into `tee`, so each line turns on `pipefail`; without it
# the exit status would be that of `tee` and a failed step would pass. The
# first line proves the flag works in this shell before anything relies on it.
# The images are built without cache once; the later steps depend on `image`
# and rebuild from that fresh cache.
gate:
	@set -o pipefail; if false | tee /dev/null; then \
		echo "gate: pipefail has no effect in $(SHELL); refusing to run"; exit 1; \
	fi
	@mkdir -p $(GATE_LOGS)
	@set -o pipefail; $(MAKE) lint 2>&1 | tee $(GATE_LOGS)/lint.log
	@set -o pipefail; $(MAKE) test TEST_FLAGS="-count=1" 2>&1 | tee $(GATE_LOGS)/test.log
	@set -o pipefail; $(MAKE) image BUILD_FLAGS="--no-cache --pull" 2>&1 | tee $(GATE_LOGS)/image.log
	@set -o pipefail; $(MAKE) test-toolchain 2>&1 | tee $(GATE_LOGS)/test-toolchain.log
	@set -o pipefail; $(MAKE) check-runtime 2>&1 | tee $(GATE_LOGS)/check-runtime.log
	@echo "gate passed for version $(VERSION)"
