# Recipes use bash so that the gate can turn on `pipefail`. The flag is set
# inside each recipe line, because the `make` shipped with macOS is 3.81 and
# ignores `.SHELLFLAGS`.
SHELL := bash

VERSION := $(shell tr -d '[:space:]' < VERSION)
IMAGE := capacity-room:$(VERSION)
TEST_IMAGE := capacity-room-test:$(VERSION)
GOLANGCI := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
TYPST_VERSION := 0.15.1
TEST_DSN := postgres://kcr_test:kcr_test_only@127.0.0.1:55432/kcr_test?sslmode=disable

# Every build tag of the repository, as declared in guard.BuildTags. The linter
# runs once without tags and once with all of them, so that no file escapes it.
BUILD_TAGS := golden,postgres,template,toolchain
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

# CHECK_SNAPSHOT is the snapshot check-runtime renders inside the runtime
# image; it is a variable, rather than written inline below, so a run can
# point it at a missing file to confirm the check then fails.
# CHECK_RUNTIME_OUT is the folder of the host its output is written to.
CHECK_SNAPSHOT := testdata/snapshot/direct-complete.json
CHECK_RUNTIME_OUT := .cache/check-runtime

# The frontend's dependencies are installed from the lock file; the stamp
# is newer than the lock file once they are.
WEB_STAMP := web/node_modules/.package-lock.json

.PHONY: web build test lint image test-toolchain test-template test-golden \
	update-golden update-testdata check-runtime gate postgres-up postgres-down \
	test-postgres generate generate-check e2e audit

$(WEB_STAMP): web/package-lock.json
	cd web && npm ci

# Always run: Vite is fast, and a stale dist is worse than a rebuilt one.
web: $(WEB_STAMP)
	cd web && npm run build

build: web
	go build -o bin/capacity-room ./cmd/capacity-room

test: web
	go test $(TEST_FLAGS) ./...
	cd web && npm test

postgres-up:
	docker compose --file deploy/compose.test.yaml up --detach --wait

postgres-down:
	docker compose --file deploy/compose.test.yaml down --volumes

# Starts PostgreSQL, runs the tagged tests, and removes the server even when
# a test fails; the exit status is the tests'.
test-postgres: web
	@set -o pipefail; $(MAKE) postgres-up; \
	listed=$$(go list ./internal/platform/... ./internal/cli/...); status=$$?; \
	packages=$$(printf '%s\n' "$$listed" | grep -v -e /internal/platform/worker$$ -e /internal/cli$$); \
	if [ $$status -ne 0 ] || [ -z "$$packages" ]; then \
		echo "test-postgres: go list failed or found no package" >&2; status=1; \
	else \
		KCR_TEST_DATABASE_DSN="$(TEST_DSN)" go test $(TEST_FLAGS) -tags=postgres $$packages; \
		status=$$?; \
	fi; \
	if [ $$status -eq 0 ]; then \
		KCR_TEST_DATABASE_DSN="$(TEST_DSN)" go test $(TEST_FLAGS) -tags='postgres template' ./internal/platform/worker ./internal/cli; \
		status=$$?; \
	fi; \
	$(MAKE) postgres-down; exit $$status

OAPI_CODEGEN := github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0

generate: $(WEB_STAMP)
	go run $(OAPI_CODEGEN) -config api/oapi-codegen.yaml api/openapi.yaml
	gofmt -w internal/platform/api/openapi.gen.go
	cd web && npm run generate

# Fails when a fresh generation differs from the committed file.
generate-check: generate
	git diff --exit-code -- internal/platform/api/openapi.gen.go web/src/api/schema.gen.ts

# gofmt reads the Go files git knows about, tracked or new, so that scratch
# files under ignored folders cannot fail the check. pipefail is turned on
# for that one recipe line, so a failing `git ls-files` or a file gofmt
# cannot parse (it lists nothing and exits non-zero) fails the step instead
# of being read as "no files need formatting".
lint: web
	@set -o pipefail; unformatted="$$(git ls-files --cached --others --exclude-standard -z -- '*.go' | xargs -0 gofmt -l)"; \
	status=$$?; \
	if [ $$status -ne 0 ]; then \
		echo "gofmt: could not check formatting"; exit 1; \
	fi; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt: these files need formatting:"; echo "$$unformatted"; exit 1; \
	fi
	go vet ./...
	go vet -tags=postgres ./...
	go vet -tags=toolchain ./...
	go vet -tags=template ./...
	go vet -tags='template golden' ./...
	go run $(GOLANGCI) run
	go run $(GOLANGCI) run --build-tags=$(BUILD_TAGS)
	cd web && npm run typecheck && npm run lint

image:
	docker build $(BUILD_FLAGS) --file deploy/Containerfile --target runtime --tag $(IMAGE) .
	docker build $(BUILD_FLAGS) --file deploy/Containerfile --target test --tag $(TEST_IMAGE) .

test-toolchain: $(IMAGE_DEP)
	$(RUN_TEST_IMAGE) go test -v -tags=toolchain ./internal/typst/...

test-template: web $(IMAGE_DEP)
	$(RUN_TEST_IMAGE) go test -v -tags=template ./internal/typst/... ./template/... ./internal/cli/...

test-golden: web $(IMAGE_DEP)
	@mkdir -p $(GOLDEN_OUT)
	docker run $(RUN_TEST_IMAGE_FLAGS) \
		--user "$$(id -u):$$(id -g)" \
		--volume "$(CURDIR)/$(GOLDEN_OUT):/out" --env KCR_GOLDEN_OUT=/out \
		$(TEST_IMAGE) go test -v -tags='template golden' ./template/ -run 'Golden|Snapshots'

# Rewrites every golden page. Run it only for an intended visual change, and
# review each page before committing it. Both golden targets run as the
# caller, since they write into folders of the checkout; Docker keeps the
# last --user it is given.
update-golden: $(IMAGE_DEP)
	docker run $(RUN_TEST_IMAGE_FLAGS) \
		--user "$$(id -u):$$(id -g)" \
		--volume "$(CURDIR)/template/tests/golden:/src/template/tests/golden" \
		$(TEST_IMAGE) go run ./template/golden/cmd/update-golden

# Rewrites the golden snapshot files under testdata/snapshot/ and the golden
# data files under template/tests/capacity/fixtures/command/. It needs no
# image. Read every changed file before committing it.
update-testdata:
	go run ./cmd/update-testdata

check-runtime: $(IMAGE_DEP)
	test "$$(docker run --rm $(IMAGE) --version)" = "capacity-room $(VERSION)"
	set -o pipefail; test "$$(docker run --rm --entrypoint /usr/local/bin/typst $(IMAGE) --version | cut -d' ' -f2)" = "$(TYPST_VERSION)"
	test "$$(docker inspect --format '{{.Config.User}}' $(IMAGE))" = "10001:10001"
	# The snapshot must exist: Docker makes a folder at a mount source that
	# is missing. The output folder's mode is 1777, not 0777: the sticky
	# bit keeps another account from replacing report.pdf.
	test -f $(CHECK_SNAPSHOT)
	mkdir -p $(CHECK_RUNTIME_OUT)
	chmod 1777 $(CHECK_RUNTIME_OUT)
	rm -f $(CHECK_RUNTIME_OUT)/report.pdf
	docker run --rm --user "$$(docker inspect --format '{{.Config.User}}' $(IMAGE))" \
		--read-only --tmpfs /tmp \
		--volume "$(CURDIR)/$(CHECK_SNAPSHOT):/data/snapshot.json:ro" \
		--volume "$(CURDIR)/$(CHECK_RUNTIME_OUT):/out" \
		$(IMAGE) report --from-snapshot /data/snapshot.json --lang en --output /out/report.pdf
	test "$$(head -c 5 $(CHECK_RUNTIME_OUT)/report.pdf)" = "%PDF-"

# The end-to-end stack: PostgreSQL, the schema, the first administrator, the
# profiles, then serve, worker, two fake Prometheus servers, and the Playwright
# container, whose exit status is the target's. The worker runs the test image
# whose typst the specs can make fail. Each fake stops with one answer unasked,
# as a whole run of direct-complete does in internal/goldentest, and no log of
# the stack holds a secret the run used. The stack is removed with its volumes
# first and last.
E2E_COMPOSE := docker compose -f deploy/compose.yaml -f deploy/compose.integration.yaml
E2E_FAKES := fake-prometheus fake-recovery

e2e: $(IMAGE_DEP)
	docker build $(BUILD_FLAGS) --file deploy/Containerfile --target fake-prometheus --tag capacity-room-fake-prometheus:$(VERSION) .
	docker build $(filter-out --pull,$(BUILD_FLAGS)) --file deploy/worker-e2e.Containerfile --build-arg RUNTIME_IMAGE=$(IMAGE) --tag capacity-room-worker-e2e:$(VERSION) .
	@set -o pipefail; \
	export KCR_VERSION=$(VERSION) KCR_CONFIG_DIR=$(CURDIR)/deploy/platform KCR_POSTGRES_PASSWORD=kcr_e2e_only; \
	mkdir -p .cache/e2e; : > .cache/e2e/one-shot.log; \
	$(E2E_COMPOSE) down --volumes; \
	$(E2E_COMPOSE) up -d --wait postgres \
		&& $(E2E_COMPOSE) run --rm -T migrate 2>&1 | tee -a .cache/e2e/one-shot.log \
		&& $(E2E_COMPOSE) run --rm -T bootstrap < deploy/platform/e2e-admin-password 2>&1 | tee -a .cache/e2e/one-shot.log \
		&& $(E2E_COMPOSE) run --rm -T profiles 2>&1 | tee -a .cache/e2e/one-shot.log \
		&& $(E2E_COMPOSE) up --abort-on-container-exit --exit-code-from playwright serve worker playwright; \
	status=$$?; \
	$(E2E_COMPOSE) stop $(E2E_FAKES); \
	for fake in $(E2E_FAKES); do \
		$(E2E_COMPOSE) logs --no-log-prefix $$fake > .cache/e2e/$$fake.log; \
		grep -qx 'remaining: 1' .cache/e2e/$$fake.log \
			|| { echo "e2e: $$fake did not leave exactly the one unasked answer; its last lines:"; \
			$(E2E_COMPOSE) logs --no-log-prefix --tail 5 $$fake; [ $$status -ne 0 ] || status=1; }; \
	done; \
	$(E2E_COMPOSE) logs --no-log-prefix serve worker > .cache/e2e/platform.log \
		&& grep -qF 'msg="serve: stopping"' .cache/e2e/platform.log \
		&& grep -qF 'msg="worker: stopped"' .cache/e2e/platform.log \
		|| { echo 'e2e: the logs of the stack could not be read'; [ $$status -ne 0 ] || status=1; }; \
	cat .cache/e2e/one-shot.log >> .cache/e2e/platform.log; \
	if [ ! -s .cache/e2e/secrets.txt ]; then \
		echo 'e2e: the specs recorded no secret'; [ $$status -ne 0 ] || status=1; \
	fi; \
	cat .cache/e2e/secrets.txt deploy/platform/e2e-admin-password deploy/platform/credentials/e2e-recovery \
		| { cat; echo kcr_e2e_only; } | grep -v '^$$' > .cache/e2e/secret-patterns \
		|| { echo 'e2e: the list of secrets could not be built'; [ $$status -ne 0 ] || status=1; }; \
	grep -qF -f .cache/e2e/secret-patterns .cache/e2e/platform.log; found=$$?; \
	if [ $$found -eq 0 ]; then \
		echo 'e2e: a secret reached the logs of the stack'; [ $$status -ne 0 ] || status=1; \
	elif [ $$found -ne 1 ]; then \
		echo 'e2e: the secret check could not run'; [ $$status -ne 0 ] || status=1; \
	fi; \
	$(E2E_COMPOSE) down --volumes; exit $$status

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
	@set -o pipefail; $(MAKE) generate-check 2>&1 | tee $(GATE_LOGS)/generate-check.log
	@set -o pipefail; $(MAKE) lint 2>&1 | tee $(GATE_LOGS)/lint.log
	@set -o pipefail; $(MAKE) test TEST_FLAGS="-count=1" 2>&1 | tee $(GATE_LOGS)/test.log
	@set -o pipefail; $(MAKE) test-postgres TEST_FLAGS="-count=1" 2>&1 | tee $(GATE_LOGS)/test-postgres.log
	@set -o pipefail; $(MAKE) image BUILD_FLAGS="--no-cache --pull" 2>&1 | tee $(GATE_LOGS)/image.log
	@set -o pipefail; $(MAKE) test-toolchain IMAGE_DEP= 2>&1 | tee $(GATE_LOGS)/test-toolchain.log
	@set -o pipefail; $(MAKE) test-template IMAGE_DEP= 2>&1 | tee $(GATE_LOGS)/test-template.log
	@set -o pipefail; $(MAKE) test-golden IMAGE_DEP= 2>&1 | tee $(GATE_LOGS)/test-golden.log
	@set -o pipefail; $(MAKE) check-runtime IMAGE_DEP= 2>&1 | tee $(GATE_LOGS)/check-runtime.log
	@set -o pipefail; $(MAKE) e2e IMAGE_DEP= BUILD_FLAGS="--no-cache --pull" 2>&1 | tee $(GATE_LOGS)/e2e.log
	@echo "gate passed for version $(VERSION)"

# audit runs the three scanners of spec 16.1 over the Go modules, the
# production dependencies of web/, and the runtime image. trivy leaves out
# advisories with no fixed version, and those named in deploy/trivy-ignore.yaml,
# each with a reason and an expiry. It needs the network and Docker, so make gate never runs it.
audit:
	@mkdir -p .cache/audit; status=0; \
	( govulncheck -version && govulncheck ./... ) > .cache/audit/govulncheck.log 2>&1 || status=1; \
	( npm --version && cd web && npm audit --omit=dev ) > .cache/audit/npm-audit.log 2>&1 || status=1; \
	( trivy --version && trivy image --scanners vuln --no-progress --ignore-unfixed --ignorefile deploy/trivy-ignore.yaml --exit-code 1 $(IMAGE) ) > .cache/audit/trivy.log 2>&1 || status=1; \
	for log in govulncheck npm-audit trivy; do echo "audit: .cache/audit/$$log.log"; done; \
	exit $$status
