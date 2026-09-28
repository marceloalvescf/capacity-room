# Recipes use bash so that the gate can turn on `pipefail`. The flag is set
# inside each recipe line, because the `make` shipped with macOS is 3.81 and
# ignores `.SHELLFLAGS`.
SHELL := /bin/bash

VERSION := $(shell tr -d '[:space:]' < VERSION)
IMAGE := k8s-capacity-report:$(VERSION)
TEST_IMAGE := k8s-capacity-report-test:$(VERSION)
GOLANGCI := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
TYPST_VERSION := 0.15.1

# Every build tag of the repository, as declared in guard.BuildTags. The linter
# runs once without tags and once with all of them, so that no file escapes it.
BUILD_TAGS := golden,template,toolchain
GATE_LOGS := .cache/gate

# Extra flags for `docker build`; the gate sets them to build from scratch.
BUILD_FLAGS ?=

# Extra flags for `go test`; the gate sets them so that no cached result counts.
TEST_FLAGS ?=

# The prerequisite that gives a target a built image. The gate builds the
# image once itself and then clears this for the steps that follow, so they
# run against that exact build instead of each triggering their own.
IMAGE_DEP ?= image

# The test image runs as the runtime user, over a read-only source and a
# read-only root file system, as the runtime image does in production.
RUN_TEST_IMAGE_FLAGS := --rm --user 10001:10001 --read-only --tmpfs /tmp:exec \
	--volume "$(CURDIR):/src:ro" --workdir /src
RUN_TEST_IMAGE := docker run $(RUN_TEST_IMAGE_FLAGS) $(TEST_IMAGE)

# Where a failed golden comparison leaves the actual pages and the difference
# images.
GOLDEN_OUT := .cache/template

# The snapshot check-runtime renders inside the runtime image, and the
# folder of the host its output is written to. A variable, so the
# controller can point CHECK_SNAPSHOT at a missing file to see the check
# fail.
CHECK_SNAPSHOT := testdata/snapshot/direct-complete.json
CHECK_RUNTIME_OUT := .cache/check-runtime

.PHONY: build test lint image test-toolchain test-template test-golden \
	update-golden update-testdata check-runtime gate

build:
	go build -o bin/k8s-capacity-report ./cmd/k8s-capacity-report

test:
	go test $(TEST_FLAGS) ./...

# gofmt reads the Go files git knows about, tracked or new, so that scratch
# files under ignored folders cannot fail the check. pipefail is turned on
# for that one recipe line, so a failing `git ls-files` or a file gofmt
# cannot parse (it lists nothing and exits non-zero) fails the step instead
# of being read as "no files need formatting".
lint:
	@set -o pipefail; unformatted="$$(git ls-files --cached --others --exclude-standard -z -- '*.go' | xargs -0 gofmt -l)"; \
	status=$$?; \
	if [ $$status -ne 0 ]; then \
		echo "gofmt: could not check formatting"; exit 1; \
	fi; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt: these files need formatting:"; echo "$$unformatted"; exit 1; \
	fi
	go vet ./...
	go vet -tags=toolchain ./...
	go vet -tags=template ./...
	go vet -tags='template golden' ./...
	go run $(GOLANGCI) run
	go run $(GOLANGCI) run --build-tags=$(BUILD_TAGS)

image:
	docker build $(BUILD_FLAGS) --file deploy/Containerfile --target runtime --tag $(IMAGE) .
	docker build $(BUILD_FLAGS) --file deploy/Containerfile --target test --tag $(TEST_IMAGE) .

test-toolchain: $(IMAGE_DEP)
	$(RUN_TEST_IMAGE) go test -v -tags=toolchain ./internal/typst/...

test-template: $(IMAGE_DEP)
	$(RUN_TEST_IMAGE) go test -v -tags=template ./internal/typst/... ./template/... ./internal/cli/...

test-golden: $(IMAGE_DEP)
	@mkdir -p $(GOLDEN_OUT)
	docker run $(RUN_TEST_IMAGE_FLAGS) \
		--volume "$(CURDIR)/$(GOLDEN_OUT):/out" --env KCR_GOLDEN_OUT=/out \
		$(TEST_IMAGE) go test -v -tags='template golden' ./template/ -run 'Golden|Snapshots'

# Rewrites every golden page. Run it only for an intended visual change, and
# review each page before committing it.
update-golden: $(IMAGE_DEP)
	docker run $(RUN_TEST_IMAGE_FLAGS) \
		--volume "$(CURDIR)/template/tests/golden:/src/template/tests/golden" \
		$(TEST_IMAGE) go run ./template/golden/cmd/update-golden

# Rewrites the golden snapshot files under testdata/snapshot/ and the golden
# data files under template/tests/capacity/fixtures/command/. It needs no
# image. Read every changed file before committing it.
update-testdata:
	go run ./cmd/update-testdata

check-runtime: $(IMAGE_DEP)
	test "$$(docker run --rm $(IMAGE) --version)" = "k8s-capacity-report $(VERSION)"
	test "$$(docker run --rm --entrypoint /usr/local/bin/typst $(IMAGE) --version | cut -d' ' -f2)" = "$(TYPST_VERSION)"
	test "$$(docker inspect --format '{{.Config.User}}' $(IMAGE))" = "10001:10001"
	rm -rf $(CHECK_RUNTIME_OUT)
	mkdir -p $(CHECK_RUNTIME_OUT)
	chmod 0777 $(CHECK_RUNTIME_OUT)
	docker run --rm --user "$$(docker inspect --format '{{.Config.User}}' $(IMAGE))" \
		--read-only --tmpfs /tmp \
		--volume "$(CURDIR)/$(CHECK_SNAPSHOT):/data/snapshot.json:ro" \
		--volume "$(CURDIR)/$(CHECK_RUNTIME_OUT):/out" \
		$(IMAGE) report --from-snapshot /data/snapshot.json --lang en --output /out/report.pdf
	test "$$(head -c 5 $(CHECK_RUNTIME_OUT)/report.pdf)" = "%PDF-"

# Runs every check from freshly built images and keeps one log per step.
# Each step is piped into `tee`, so each line turns on `pipefail`; without it
# the exit status would be that of `tee` and a failed step would pass. The
# first line proves the flag works in this shell before anything relies on it.
# The images are built without cache once; the later steps pass IMAGE_DEP=
# empty, so they build nothing and run against that exact build instead of
# each rebuilding from Docker's cache.
gate:
	@set -o pipefail; if false | tee /dev/null; then \
		echo "gate: pipefail has no effect in $(SHELL); refusing to run"; exit 1; \
	fi
	@mkdir -p $(GATE_LOGS)
	@set -o pipefail; $(MAKE) lint 2>&1 | tee $(GATE_LOGS)/lint.log
	@set -o pipefail; $(MAKE) test TEST_FLAGS="-count=1" 2>&1 | tee $(GATE_LOGS)/test.log
	@set -o pipefail; $(MAKE) image BUILD_FLAGS="--no-cache --pull" 2>&1 | tee $(GATE_LOGS)/image.log
	@set -o pipefail; $(MAKE) test-toolchain IMAGE_DEP= 2>&1 | tee $(GATE_LOGS)/test-toolchain.log
	@set -o pipefail; $(MAKE) test-template IMAGE_DEP= 2>&1 | tee $(GATE_LOGS)/test-template.log
	@set -o pipefail; $(MAKE) test-golden IMAGE_DEP= 2>&1 | tee $(GATE_LOGS)/test-golden.log
	@set -o pipefail; $(MAKE) check-runtime IMAGE_DEP= 2>&1 | tee $(GATE_LOGS)/check-runtime.log
	@echo "gate passed for version $(VERSION)"
