# Phase 13: the Kubernetes deployment — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Helm chart runs CapacityRoom on Kubernetes, the runtime image is on Docker Hub, and a real install on the Talos homelab proves both, released as 2.1.0.

**Architecture:** Six implementation tasks, in sequence, then the main thread's closing work. Task 1 adds the one change to the command (`--password-file`); Task 2 moves Compose to `/opt/capacity-room`; Tasks 3 to 5 grow the chart in layers (the core and its tooling, then routing with offline schema checks, then the hook Jobs), each with Go tests over `helm template` tagged `helm`; Task 6 writes the documents and `make push`. Task 7 is the main thread's: the gate, the reviews, the version, the push, and the homelab.

**Tech Stack:** Go 1.27, Helm 4.3.0, kubeconform 0.8.0, Kubernetes 1.37, Gateway API, Docker Compose, GNU make, all inside `nix develop`.

**Spec:** `docs/superpowers/specs/2026-10-14-kubernetes-deployment-design.md` (eabf5b1, approved 2026-10-14). Read it before any task; this plan argues from it.

## Global Constraints

- Chart folder `deploy/helm/capacity-room/`; chart name `capacity-room`; `version` and `appVersion` equal `VERSION`.
- Image `alvesmarcelocf/capacity-room`, tag `VERSION`, never `latest`, linux/amd64 only.
- Mount root `/opt/capacity-room`: `platform.toml`, `profiles.toml`, `secret/`, `credentials/`, `ca/`, `admin/`. Nothing under `/etc`.
- The chart creates no Secret, no PersistentVolumeClaim, no NetworkPolicy, and nothing that reads the Kubernetes API.
- One replica of `serve` and one of `worker`, fixed; `strategy: Recreate`.
- Every pod: user and group 10001, `runAsNonRoot`, `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false`, capabilities drop `ALL`, `seccompProfile: RuntimeDefault`, `automountServiceAccountToken: false`.
- Helm 4.3.0 (`helm-v4.3.0-linux-amd64.tar.gz`, SHA-256 `86584a54def73570558f66f5111cc53dfed56689637ae32c1201205d494f54fb`); kubeconform 0.8.0 (`kubeconform-linux-amd64.tar.gz`, SHA-256 `9bc2bffbf71f261128533edaf912153948b7ff238f9a531ae6d34466ec287883`); Kubernetes schemas 1.37.0; Gateway API v1.6.3.
- New build tag `helm`, declared in `guard.BuildTags` and in the Makefile's `BUILD_TAGS`.
- No new `platform.toml` key, no migration, no route, no screen, no catalog or locale entry.
- Diagnostics are English; a message may repeat a path the user gave, never the content of a password file.
- Every subagent: never wait on standard input, never write to `/tmp` (scratch goes under `.cache/`), leave no background process, run `go` and `make` only inside `nix develop -c ...`, never run Docker, never run `git stash`, `git restore`, `git checkout` of a path, or `git clean`; every commit names its paths (`git add <paths>`).
- Comments say what the code does; no comment names a task, a phase, a step, or a path under `.cache/`.
- The homelab's own host name, its addresses, and its kube context never enter the chart, its examples, the tests, or a document for operators; those use `example.com`. Only the spec and this plan name the homelab, as the record of the phase.

## Review Focus

- **A Secret volume under a read-only root.** Kubernetes writes a Secret as `name -> ..data/name`, `..data -> ..<timestamp>/`. A reader expects `serve` to read the server secret and the worker a credential through those links. Task 3 pins the server secret through that layout in `internal/platform/settings`, and Task 5 pins `ResolveCredential` through it in `internal/platform/profiles`.
- **Helm numbers.** Helm reads a value file's integers as floats, so a careless template writes `43200.0`, which `settings.Load` refuses. A reader expects every key accepted whether it comes from `-f` or `--set`. Task 3 renders all fifteen keys at their largest allowed value both ways and loads the result.
- **A profile text with quotes, a backslash, accents, `<`, or `&`.** A reader expects the imported profile to hold the text exactly. Task 5 round-trips such a `display_name` through the rendered `profiles.toml` and `profiles.ReadImportFile`.
- **A reinstall over a database that holds users.** A reader expects the admin Job to fail with one clear line and nothing written. Task 1 pins `--password-file` against a second user under PostgreSQL; the homelab proof of Task 7 sees it live.
- **A long release name.** A reader expects every object name to be a valid DNS label of at most 63 characters. Task 3 renders with a 60-character release name and checks every name.

---

### Task 1: `bootstrap-admin --password-file`

**Files:**
- Modify: `internal/cli/admin.go` (the command at 87-114, `bootstrapInput` at 167-170, `bootstrapAdmin` at 175-236, `readCredentials` at 310-331, and its call in `resetPassword`)
- Modify: `internal/cli/platform_test.go` (`TestBootstrapNeedsStdinFlag` and new tests beside it)
- Modify: `internal/cli/admin_test.go` (tag `postgres`)
- Modify: `docs/operations.md` (section 5, one sentence), `CHANGELOG.md` (`## [Unreleased]`)

**Interfaces:**
- Produces: the flag `--password-file <path>` of `admin bootstrap-admin`, used by Task 2 (e2e) and Task 5 (the admin Job).
- Produces: `readCredentials(source io.Reader, redactor *redact.Redactor, username string) (name, password string, err error)`; `readPasswordFile(path string) ([]byte, error)`; `const maxPasswordFileBytes = 4 << 10`.

- [ ] **Step 1: Write the failing tests (no database)** in `internal/cli/platform_test.go`. Replace `TestBootstrapNeedsStdinFlag` and add the rest beside it:

```go
func TestBootstrapNeedsOnePasswordSource(t *testing.T) {
	file := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(file, []byte("correct horse battery staple\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const want = "error: pass one of --password-stdin and --password-file\n"
	for name, extra := range map[string][]string{
		"neither": nil,
		"both":    {"--password-stdin", "--password-file", file},
	} {
		t.Run(name, func(t *testing.T) {
			env := platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1"}, "")
			env.Stdin = failingReader{t}
			args := append([]string{"admin", "bootstrap-admin", "--username", "root"}, extra...)
			code, stdout, stderr := runCommand(context.Background(), env, args...)
			if code != 2 || stderr != want || stdout != "" {
				t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
			}
		})
	}
}

func TestBootstrapPasswordFileRefusals(t *testing.T) {
	dir := t.TempDir()
	large := filepath.Join(dir, "large")
	if err := os.WriteFile(large, bytes.Repeat([]byte("a"), 4097), 0o600); err != nil {
		t.Fatal(err)
	}
	short := filepath.Join(dir, "short")
	if err := os.WriteFile(short, []byte("tooshort\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing")
	tests := []struct {
		name, path, want string
	}{
		{"missing", missing, "error: cannot read the password file at " + missing + ": no such file or directory\n"},
		{"folder", dir, "error: the password file at " + dir + " is not a regular file\n"},
		{"large", large, "error: the password file at " + large + " is larger than 4 KiB\n"},
		{"short", short, "error: the password is too short\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1", "CAPACITYROOM_DATABASE_DSN": closedPortDSN}, "")
			env.Stdin = failingReader{t}
			code, stdout, stderr := runCommand(context.Background(), env,
				"admin", "bootstrap-admin", "--username", "root", "--password-file", tt.path, "--config", writePlatformFiles(t))
			if code != 1 || stdout != "" || stderr != tt.want {
				t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
			}
			if strings.Contains(stderr, "tooshort") {
				t.Fatalf("stderr = %q holds the content of the file", stderr)
			}
		})
	}
}

func TestBootstrapPasswordFileArmsTheRedactor(t *testing.T) {
	const password = "correct horse battery staple"
	file := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(file, []byte(password+"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	redactor := redact.New()
	_, _, err := bootstrapAdmin(context.Background(),
		platformEnv(map[string]string{"CAPACITYROOM_BOOTSTRAP_MODE": "1", "CAPACITYROOM_DATABASE_DSN": closedPortDSN}, ""),
		redactor, bootstrapInput{username: "root", language: "en", config: writePlatformFiles(t), passwordFile: file})
	if err == nil {
		t.Fatal("bootstrapAdmin succeeded against a closed port")
	}
	if got := redactor.Text(password); strings.Contains(got, password) {
		t.Fatalf("redactor.Text(password) = %q, want it redacted", got)
	}
}
```

`failingReader` fails the test when standard input is read, so these tests also prove that a file source never reads standard input.

- [ ] **Step 2: Write the failing tests (PostgreSQL)** in `internal/cli/admin_test.go`, beside `TestBootstrapReadsOneLine`:

```go
// bootstrapFromFile runs bootstrap-admin against pool with the password in a
// file and returns the code and both streams.
func bootstrapFromFile(t *testing.T, pool *pgxpool.Pool, content string) (int, string, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	env := platformEnv(map[string]string{
		"CAPACITYROOM_BOOTSTRAP_MODE": "1",
		"CAPACITYROOM_DATABASE_DSN":   pool.Config().ConnString(),
	}, "")
	return runCommand(context.Background(), env,
		"admin", "bootstrap-admin", "--username", "Root", "--password-file", file, "--config", writePlatformFiles(t))
}

func TestBootstrapPasswordFileReadsOneLine(t *testing.T) {
	pool := storetest.New(t)
	code, stdout, stderr := bootstrapFromFile(t, pool, bootstrapPassword+"\r\nmore\n")
	if code != 0 || stdout != "created administrator root\n" || stderr != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	user, err := store.FindActiveUserByUsername(context.Background(), pool, "root")
	if err != nil {
		t.Fatal(err)
	}
	if match, _, _ := auth.VerifyPassword(context.Background(), bootstrapPassword, user.PasswordHash); !match {
		t.Fatal("the stored password is not the first line of the file without its line ending")
	}
}

func TestBootstrapPasswordFileRefusesSecondUser(t *testing.T) {
	pool := storetest.New(t)
	if code, _, stderr := bootstrapFromFile(t, pool, bootstrapPassword); code != 0 {
		t.Fatalf("first bootstrap: code = %d, stderr = %q", code, stderr)
	}
	code, stdout, stderr := bootstrapFromFile(t, pool, bootstrapPassword)
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "error: ") || strings.Count(stderr, "\n") != 1 {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want one error line and code 1", code, stdout, stderr)
	}
}
```

Read `TestBootstrapRefusesSecondUser` first and make the second test's expected line equal to the line it pins; add `strings` to the imports if it is missing.

- [ ] **Step 3: Run the tests to see them fail**

Run: `nix develop -c go test ./internal/cli/ -run 'TestBootstrap' -v`
Expected: FAIL, with `unknown field passwordFile` at compile time.

The PostgreSQL tests run on the main thread's server: `CAPACITYROOM_TEST_DATABASE_DSN=<the DSN the main thread gives> nix develop -c go test -tags=postgres ./internal/cli/ -run 'TestBootstrapPasswordFile' -v`. Expected: FAIL the same way.

- [ ] **Step 4: Implement** in `internal/cli/admin.go`.

In `newBootstrapAdminCmd`, add `var passwordFile string`, pass `passwordFile: passwordFile` in the `bootstrapInput`, and register the flag after `--password-stdin`:

```go
	flags.StringVar(&passwordFile, "password-file", "", "Read the password from the first line of this file.")
```

`bootstrapInput` gains `passwordFile string`. In `bootstrapAdmin`, replace the `if !in.passwordStdin` block and the `readCredentials` call with:

```go
	if in.passwordStdin == (in.passwordFile != "") {
		return "", &exitError{2, errors.New("pass one of --password-stdin and --password-file")}
	}
	source := env.Stdin
	if in.passwordFile != "" {
		data, err := readPasswordFile(in.passwordFile)
		if err != nil {
			return "", &exitError{1, err}
		}
		source = bytes.NewReader(data)
	}

	name, password, err := readCredentials(source, redactor, in.username)
```

Change `readCredentials` to read from `source io.Reader` in place of `env.Stdin` (its first line becomes `line, err := bufio.NewReader(source).ReadString('\n')`; its message stays), and pass `env.Stdin` at its call in `resetPassword`. Add below it:

```go
// maxPasswordFileBytes bounds a password file; a password holds at most 1024
// characters, so a larger file is a mistake.
const maxPasswordFileBytes = 4 << 10

// readPasswordFile reads the regular file at path whole, at most 4 KiB. The
// open does not block on a FIFO. A message repeats the path the user gave and
// never the content.
func readPasswordFile(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, passwordFileError(path, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, passwordFileError(path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("the password file at %s is not a regular file", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxPasswordFileBytes+1))
	if err != nil {
		return nil, passwordFileError(path, err)
	}
	if len(data) > maxPasswordFileBytes {
		return nil, fmt.Errorf("the password file at %s is larger than 4 KiB", path)
	}
	return data, nil
}

// passwordFileError words a failure to read the password file with the
// user's own path and the system's cause.
func passwordFileError(path string, err error) error {
	cause := err.Error()
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		cause = pathErr.Err.Error()
	}
	return fmt.Errorf("cannot read the password file at %s: %s", path, cause)
}
```

A folder opens and stats as a directory, so it reaches the "not a regular file" line. Add the imports the file lacks (`bytes`, `io/fs`, `os`, `syscall`).

- [ ] **Step 5: Run the tests to see them pass**

Run: `nix develop -c go test ./internal/cli/ -v -run 'TestBootstrap|TestRoot|TestAdmin'`, then the PostgreSQL run of Step 3.
Expected: PASS. If the test that names every command and flag of the program fails on the new flag, add `--password-file` to its expected list.

- [ ] **Step 6: Documents.** In `docs/operations.md` section 5, after the paragraph that says the command reads the password from standard input, add: "`--password-file <path>` reads the password from the first line of a file instead, as the Kubernetes chart does." Under `## [Unreleased]` of `CHANGELOG.md` add:

```markdown
### Added

- The first administrator can be created from a password file, with `admin bootstrap-admin --password-file`.
```

- [ ] **Step 7: Full suite and commit**

Run: `nix develop -c make test` and `nix develop -c make lint`. Expected: both pass.

```bash
git add internal/cli/admin.go internal/cli/platform_test.go internal/cli/admin_test.go docs/operations.md CHANGELOG.md
git commit -m "feat(cli): read the first password from a file"
```

---

### Task 2: Compose mounts `/opt/capacity-room`

**Files:**
- Modify: `deploy/compose.yaml` (lines 18, 45, 59), `deploy/compose.integration.yaml` (lines 17, 67, 73-83, 87)
- Move: `deploy/platform/server-secret` to `deploy/platform/secret/server-secret` (`git mv`)
- Modify: `deploy/platform/platform.toml`, `deploy/platform/README.md`
- Modify: `Makefile` (the `bootstrap` line of `e2e`, line 196)
- Modify: `docs/operations.md` (lines 29, 125-130, section 4, 216, 238, 442, and a new subsection of section 8), `CHANGELOG.md`

**Interfaces:**
- Consumes: `--password-file` (Task 1).
- Produces: the settings folder layout `platform.toml`, `secret/server-secret`, `credentials/`, `ca/`, mounted at `/opt/capacity-room`, which the chart repeats.

- [ ] **Step 1: The mount.** In both Compose files, every `/etc/capacity-room` becomes `/opt/capacity-room`. `git grep -n '/etc/capacity-room' -- deploy Makefile` lists them all; after the step it lists none.

- [ ] **Step 2: The fixtures.** `git mv deploy/platform/server-secret deploy/platform/secret/server-secret`. In `deploy/platform/platform.toml`, `server_secret_path = "/opt/capacity-room/secret/server-secret"` and `credential_dir = "/opt/capacity-room/credentials"`. Name the new layout in `deploy/platform/README.md` in one sentence.

- [ ] **Step 3: The e2e bootstrap reads its file.** The `bootstrap` service of `deploy/compose.integration.yaml` passes `--password-file /opt/capacity-room/e2e-admin-password` in place of `--password-stdin`. In the `e2e` recipe of the `Makefile`, the bootstrap line drops `< deploy/platform/e2e-admin-password` and keeps `-T`:

```make
		&& $(E2E_COMPOSE) run --rm -T bootstrap 2>&1 | tee -a .cache/e2e/one-shot.log \
```

The secret scan still reads `deploy/platform/e2e-admin-password` (line 215), unchanged.

- [ ] **Step 4: Any test reading the fixtures.** `git grep -n 'deploy/platform' -- '*.go' '*.ts'` and `git grep -n 'server-secret' -- '*.go' '*.ts' web/e2e`; adjust a path that names the old place. If none, nothing changes.

- [ ] **Step 5: The guide.**
  - Section 1: "mounted read-only at `/opt/capacity-room`".
  - The settings example (125-130): the three paths under `/opt/capacity-room`, the secret at `/opt/capacity-room/secret/server-secret`.
  - Section 4: `mkdir -p "$CAPACITYROOM_HOME/config/secret"` before the `head -c 32` line, which writes `"$CAPACITYROOM_HOME/config/secret/server-secret"`.
  - Every `--config /etc/capacity-room/platform.toml` (216, 238, 442) becomes `/opt/capacity-room/platform.toml`.
  - Section 8 gains, after "Upgrading from 1.x":

```markdown
### Upgrading from 2.0.0

The settings folder is mounted at `/opt/capacity-room` from 2.1.0, and the
server secret lives in a folder of its own. Before you start 2.1.0, move the
secret and edit the paths of `platform.toml`:

```bash
mkdir "$CAPACITYROOM_HOME/config/secret"
mv "$CAPACITYROOM_HOME/config/server-secret" "$CAPACITYROOM_HOME/config/secret/server-secret"
```

```toml
server_secret_path = "/opt/capacity-room/secret/server-secret"
credential_dir = "/opt/capacity-room/credentials"
ca_bundle_dir = "/opt/capacity-room/ca"
```

Then give the folder to user `10001` again, as section 4 does. A
`platform.toml` that still names `/etc/capacity-room` stops `serve` at its
start with a line naming the path it could not read.
```

  (In the guide the inner fences are plain triple backticks; the outer fence above exists only in this plan.)

- [ ] **Step 6: Changelog.** Under `## [Unreleased]`:

```markdown
### Changed

- The Compose stack mounts its settings folder at `/opt/capacity-room`, with the server secret in a `secret` folder of its own; the operations guide says how to move an existing folder.
```

- [ ] **Step 7: Tests and commit**

Run: `nix develop -c make test` (the guide drift tests and the guards read these files). Expected: PASS. `make e2e` runs on the main thread after the task.

```bash
git add deploy/compose.yaml deploy/compose.integration.yaml deploy/platform/ Makefile docs/operations.md CHANGELOG.md
git commit -m "feat(deploy): mount the settings at /opt/capacity-room"
```

---

### Task 3: the chart's core and its tooling

**Files:**
- Modify: `flake.nix` (packages), `deploy/Containerfile` (the `test` stage)
- Modify: `internal/guard/buildtags.go:17`, `internal/guard/imports.go` (`Layers`), `Makefile` (`BUILD_TAGS`, `lint`, `test-chart`, `gate`, `.PHONY`)
- Create: `internal/chart/doc.go`, `internal/chart/chart_test.go`, `internal/chart/render_test.go`, `internal/chart/testdata/every-setting.yaml`
- Create: `internal/platform/settings/chart_test.go`, and a test in `internal/platform/settings/settings_test.go`
- Create: `deploy/helm/capacity-room/Chart.yaml`, `values.yaml`, `values.schema.json`, `.helmignore`, `templates/_helpers.tpl`, `templates/configmap.yaml`, `templates/serviceaccount.yaml`, `templates/deployment-serve.yaml`, `templates/deployment-worker.yaml`, `templates/service.yaml`, `templates/NOTES.txt`

**Interfaces:**
- Produces: the helpers `capacity-room.fullname`, `capacity-room.labels`, `capacity-room.selectorLabels` (takes `dict "root" . "component" "<name>"`), `capacity-room.image`, `capacity-room.podSecurityContext`, `capacity-room.containerSecurityContext`, `capacity-room.databaseEnv`, `capacity-room.volumes`, `capacity-room.volumeMounts`; the ConfigMap named `{{ fullname }}` with the key `platform.toml`; the Service named `{{ fullname }}` on port 8080 named `http`.
- Produces (tests, `internal/chart`, tag `helm`): `chartDir` (the chart's path from the package), `required []string` (the `--set` flags of the two required Secrets), `render(t *testing.T, release string, args ...string) []object`, `renderError(t *testing.T, args ...string) string`, `type object map[string]any` with `kind() string`, `name() string`, `podSpec() map[string]any`, and `find(objects []object, kind, name string) object`.

- [ ] **Step 1: The tools.** In `flake.nix`, add `pkgs.kubernetes-helm` and `pkgs.kubeconform` to `packages` (the lock gives 4.3.0 and 0.8.0). In `deploy/Containerfile`, in the `test` stage after the typst lines:

```dockerfile
ADD --checksum=sha256:86584a54def73570558f66f5111cc53dfed56689637ae32c1201205d494f54fb \
    https://get.helm.sh/helm-v4.3.0-linux-amd64.tar.gz /opt/helm.tar.gz
ADD --checksum=sha256:9bc2bffbf71f261128533edaf912153948b7ff238f9a531ae6d34466ec287883 \
    https://github.com/yannh/kubeconform/releases/download/v0.8.0/kubeconform-linux-amd64.tar.gz /opt/kubeconform.tar.gz
RUN tar -xzf /opt/helm.tar.gz -C /usr/local/bin --strip-components=1 linux-amd64/helm \
    && tar -xzf /opt/kubeconform.tar.gz -C /usr/local/bin kubeconform \
    && rm /opt/helm.tar.gz /opt/kubeconform.tar.gz
RUN test "$(helm version --template '{{.Version}}')" = "v4.3.0"
RUN test "$(kubeconform -v)" = "v0.8.0"
```

Check the two version lines on the host first: `nix develop -c helm version --template '{{.Version}}'` and `nix develop -c kubeconform -v`; write the exact text they print. The main thread builds the image.

- [ ] **Step 2: The tag and the layer.** `BuildTags` becomes `[]string{"golden", "helm", "postgres", "template", "toolchain"}`, and the Makefile's `BUILD_TAGS := golden,helm,postgres,template,toolchain`. `Layers` gains `{Path: "internal/chart", Rank: 210}` after `internal/cli`. The `lint` recipe gains `go vet -tags=helm ./...` after the `toolchain` line. The Makefile gains, after `test-golden`:

```make
# The chart's tests: helm lint, the rendered manifests read back by the
# program's own parsers, and kubeconform against the schemas kept in
# deploy/helm/schemas. They run in the test image, which holds both tools.
test-chart: $(IMAGE_DEP)
	$(RUN_TEST_IMAGE) go test -v -count=1 -tags=helm ./internal/chart/
```

`test-chart` joins `.PHONY`, and `gate` runs it after `test-golden`:

```make
	@set -o pipefail; $(MAKE) test-chart IMAGE_DEP= 2>&1 | tee $(GATE_LOGS)/test-chart.log
```

`internal/chart/doc.go`:

```go
// Package chart holds the tests of the Helm chart under
// deploy/helm/capacity-room. They carry the build tag helm and run helm and
// kubeconform, which the test image holds.
package chart
```

- [ ] **Step 3: The chart's files.** `deploy/helm/capacity-room/Chart.yaml`:

```yaml
apiVersion: v2
name: capacity-room
description: CapacityRoom, capacity reports of a Kubernetes cluster from its metrics.
type: application
version: 2.0.0
appVersion: "2.0.0"
```

The two versions equal `VERSION` today; the version commit of Task 7 raises them with it.

`.helmignore`:

```text
examples/
```

`values.yaml` (the keys of this task; Tasks 4 and 5 add `route`, `ingress`, `admin`, and `profiles`):

```yaml
# The runtime image. An empty tag means the chart's appVersion.
image:
  repository: alvesmarcelocf/capacity-room
  tag: ""
  pullPolicy: IfNotPresent

# Secrets that let the cluster pull a private image, as
# [{name: <secret>}]. The chart creates none.
imagePullSecrets: []

# The Secret that holds the database DSN, at key. Required. A CloudNativePG
# cluster's <cluster>-app Secret holds it at uri.
database:
  existingSecret: ""
  key: uri

# The Secret that holds the server secret, at least 32 random bytes, at key.
# Required. For example:
#   head -c 32 /dev/urandom > server-secret
#   kubectl -n <namespace> create secret generic capacity-room-server-secret \
#     --from-file=server-secret=./server-secret
serverSecret:
  existingSecret: ""
  key: server-secret

# The Secret whose keys are the credential files that profiles name, each a
# bearer token. Empty: no credential folder.
credentials:
  existingSecret: ""

# The ConfigMap whose keys are the CA bundles that profiles name, each PEM.
# Empty: no CA folder.
caBundles:
  existingConfigMap: ""

# The keys of platform.toml, by their own names, as the operations guide
# lists them. A key left out keeps the program's default. The chart writes
# server_secret_path, credential_dir, and ca_bundle_dir itself.
platform: {}

serve:
  resources:
    requests:
      cpu: 100m
      memory: 256Mi
    limits:
      memory: 1Gi

worker:
  # The memory limit is written in Mi or Gi; GOMEMLIMIT is set at three
  # quarters of it.
  resources:
    requests:
      cpu: 250m
      memory: 512Mi
    limits:
      memory: 4Gi
```

`values.schema.json` (draft 7; `additionalProperties: false` at every object):

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "additionalProperties": false,
  "required": ["image", "imagePullSecrets", "database", "serverSecret", "credentials", "caBundles", "platform", "serve", "worker"],
  "properties": {
    "image": {
      "type": "object", "additionalProperties": false,
      "properties": {
        "repository": {"type": "string", "minLength": 1},
        "tag": {"type": "string"},
        "pullPolicy": {"enum": ["Always", "IfNotPresent", "Never"]}
      }
    },
    "imagePullSecrets": {
      "type": "array",
      "items": {"type": "object", "additionalProperties": false, "required": ["name"], "properties": {"name": {"type": "string", "minLength": 1}}}
    },
    "database": {
      "type": "object", "additionalProperties": false, "required": ["existingSecret", "key"],
      "properties": {"existingSecret": {"type": "string", "minLength": 1}, "key": {"type": "string", "minLength": 1}}
    },
    "serverSecret": {
      "type": "object", "additionalProperties": false, "required": ["existingSecret", "key"],
      "properties": {"existingSecret": {"type": "string", "minLength": 1}, "key": {"type": "string", "minLength": 1}}
    },
    "credentials": {
      "type": "object", "additionalProperties": false,
      "properties": {"existingSecret": {"type": "string"}}
    },
    "caBundles": {
      "type": "object", "additionalProperties": false,
      "properties": {"existingConfigMap": {"type": "string"}}
    },
    "platform": {
      "type": "object", "additionalProperties": false,
      "properties": {
        "session_ttl_seconds": {"type": "integer"},
        "session_idle_seconds": {"type": "integer"},
        "trusted_proxies": {"type": "array", "items": {"type": "string"}},
        "allowed_endpoint_hostnames": {"type": "array", "items": {"type": "string"}},
        "allowed_endpoint_cidrs": {"type": "array", "items": {"type": "string"}},
        "worker_slots": {"type": "integer"},
        "worker_drain_seconds": {"type": "integer"},
        "collect_timeout_seconds": {"type": "integer"},
        "render_timeout_seconds": {"type": "integer"},
        "job_max_attempts": {"type": "integer"},
        "report_retention_days": {"type": "integer"},
        "expired_report_days": {"type": "integer"},
        "admission_limit": {"type": "integer"},
        "admission_profile_limit": {"type": "integer"},
        "admission_retry_seconds": {"type": "integer"}
      }
    },
    "serve": {
      "type": "object", "additionalProperties": false,
      "properties": {"resources": {"type": "object"}}
    },
    "worker": {
      "type": "object", "additionalProperties": false, "required": ["resources"],
      "properties": {
        "resources": {
          "type": "object", "required": ["limits"],
          "properties": {
            "limits": {
              "type": "object", "required": ["memory"],
              "properties": {"memory": {"type": "string", "pattern": "^[1-9][0-9]*(Mi|Gi)$"}}
            }
          }
        }
      }
    }
  }
}
```

The ranges of the integer keys stay the program's: `settings.Load` judges them, and the schema checks only the type.

`templates/_helpers.tpl`:

```gotemplate
{{/* The base name of every object: the release name, cut so that a suffix
fits a DNS label. */}}
{{- define "capacity-room.fullname" -}}
{{- .Release.Name | trunc 50 | trimSuffix "-" -}}
{{- end -}}

{{- define "capacity-room.labels" -}}
app.kubernetes.io/name: capacity-room
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end -}}

{{- define "capacity-room.selectorLabels" -}}
app.kubernetes.io/name: capacity-room
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{- define "capacity-room.image" -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) -}}
{{- end -}}

{{- define "capacity-room.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: 10001
runAsGroup: 10001
fsGroup: 10001
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{- define "capacity-room.containerSecurityContext" -}}
runAsNonRoot: true
readOnlyRootFilesystem: true
allowPrivilegeEscalation: false
capabilities:
  drop: ["ALL"]
{{- end -}}

{{- define "capacity-room.databaseEnv" -}}
- name: CAPACITYROOM_DATABASE_DSN
  valueFrom:
    secretKeyRef:
      name: {{ .Values.database.existingSecret }}
      key: {{ .Values.database.key }}
{{- end -}}

{{/* platform.toml: the chart's three paths, then each key of
.Values.platform. The schema allows an integer or a list of strings, and an
integer is written as one, since Helm reads numbers as floats. */}}
{{- define "capacity-room.platformToml" -}}
server_secret_path = {{ printf "/opt/capacity-room/secret/%s" .Values.serverSecret.key | toJson }}
credential_dir = "/opt/capacity-room/credentials"
ca_bundle_dir = "/opt/capacity-room/ca"
{{- range $key, $value := .Values.platform }}
{{- if kindIs "slice" $value }}
{{ $key }} = {{ toJson $value }}
{{- else }}
{{ $key }} = {{ int64 $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "capacity-room.volumes" -}}
- name: tmp
  emptyDir: {}
- name: settings
  configMap:
    name: {{ include "capacity-room.fullname" . }}
- name: server-secret
  secret:
    secretName: {{ .Values.serverSecret.existingSecret }}
    defaultMode: 0440
    items:
      - key: {{ .Values.serverSecret.key }}
        path: {{ .Values.serverSecret.key }}
{{- with .Values.credentials.existingSecret }}
- name: credentials
  secret:
    secretName: {{ . }}
    defaultMode: 0440
{{- end }}
{{- with .Values.caBundles.existingConfigMap }}
- name: ca
  configMap:
    name: {{ . }}
{{- end }}
{{- end -}}

{{- define "capacity-room.volumeMounts" -}}
- name: tmp
  mountPath: /tmp
- name: settings
  mountPath: /opt/capacity-room/platform.toml
  subPath: platform.toml
  readOnly: true
- name: server-secret
  mountPath: /opt/capacity-room/secret
  readOnly: true
{{- if .Values.credentials.existingSecret }}
- name: credentials
  mountPath: /opt/capacity-room/credentials
  readOnly: true
{{- end }}
{{- if .Values.caBundles.existingConfigMap }}
- name: ca
  mountPath: /opt/capacity-room/ca
  readOnly: true
{{- end }}
{{- end -}}

{{/* The worker's memory limit in MiB, from a value the schema holds to Mi
or Gi. */}}
{{- define "capacity-room.workerMemoryMiB" -}}
{{- $memory := .Values.worker.resources.limits.memory -}}
{{- if hasSuffix "Gi" $memory -}}
{{- mul (trimSuffix "Gi" $memory | int) 1024 -}}
{{- else -}}
{{- trimSuffix "Mi" $memory | int -}}
{{- end -}}
{{- end -}}
```

`templates/configmap.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "capacity-room.fullname" . }}
  labels:
    {{- include "capacity-room.labels" . | nindent 4 }}
data:
  platform.toml: |
    {{- include "capacity-room.platformToml" . | nindent 4 }}
```

`templates/serviceaccount.yaml`:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "capacity-room.fullname" . }}
  labels:
    {{- include "capacity-room.labels" . | nindent 4 }}
automountServiceAccountToken: false
```

`templates/deployment-serve.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "capacity-room.fullname" . }}-serve
  labels:
    {{- include "capacity-room.labels" . | nindent 4 }}
    app.kubernetes.io/component: serve
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      {{- include "capacity-room.selectorLabels" (dict "root" . "component" "serve") | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "capacity-room.selectorLabels" (dict "root" . "component" "serve") | nindent 8 }}
      annotations:
        checksum/settings: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
    spec:
      serviceAccountName: {{ include "capacity-room.fullname" . }}
      automountServiceAccountToken: false
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      securityContext:
        {{- include "capacity-room.podSecurityContext" . | nindent 8 }}
      # The stop waits 30 seconds for the requests in flight, then closes the
      # rest; the grace period covers that wait with room to exit.
      terminationGracePeriodSeconds: 45
      containers:
        - name: serve
          image: {{ include "capacity-room.image" . }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          args: ["serve", "--config", "/opt/capacity-room/platform.toml", "--listen", ":8080"]
          env:
            {{- include "capacity-room.databaseEnv" . | nindent 12 }}
          ports:
            - name: http
              containerPort: 8080
          startupProbe:
            httpGet: {path: /health/ready, port: http}
            periodSeconds: 2
            failureThreshold: 150
          readinessProbe:
            httpGet: {path: /health/ready, port: http}
            periodSeconds: 5
          livenessProbe:
            httpGet: {path: /health/live, port: http}
            periodSeconds: 10
          securityContext:
            {{- include "capacity-room.containerSecurityContext" . | nindent 12 }}
          {{- with .Values.serve.resources }}
          resources:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          volumeMounts:
            {{- include "capacity-room.volumeMounts" . | nindent 12 }}
      volumes:
        {{- include "capacity-room.volumes" . | nindent 8 }}
```

`templates/deployment-worker.yaml` is the same shape with these differences: name suffix and component `worker`; `args: ["worker", "--config", "/opt/capacity-room/platform.toml", "--health-listen", ":8091"]`; the port is `name: health`, `containerPort: 8091`, and the three probes use `port: health`; no `env` other than the DSN plus

```yaml
            - name: GOMEMLIMIT
              value: {{ printf "%dMiB" (div (mul (include "capacity-room.workerMemoryMiB" . | int) 3) 4) | quote }}
```

`resources` from `.Values.worker.resources`; and the grace period

```yaml
      # The stop waits worker_drain_seconds for the jobs, cancels the rest,
      # and waits as long again; the grace period covers both waits.
      terminationGracePeriodSeconds: {{ add (mul (.Values.platform.worker_drain_seconds | default 30 | int) 2) 15 }}
```

`templates/service.yaml`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ include "capacity-room.fullname" . }}
  labels:
    {{- include "capacity-room.labels" . | nindent 4 }}
spec:
  type: ClusterIP
  selector:
    {{- include "capacity-room.selectorLabels" (dict "root" . "component" "serve") | nindent 4 }}
  ports:
    - name: http
      port: 8080
      targetPort: http
```

`templates/NOTES.txt`:

```text
CapacityRoom {{ .Chart.AppVersion }} is installed as {{ .Release.Name }} in the namespace {{ .Release.Namespace }}.

The web answers on the Service {{ include "capacity-room.fullname" . }}, port 8080.
```

- [ ] **Step 4: The tests' helpers**, `internal/chart/render_test.go`:

```go
//go:build helm

package chart

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// chartDir is the chart, from this package's folder.
const chartDir = "../../deploy/helm/capacity-room"

// required names the two Secrets the schema requires.
var required = []string{"--set", "database.existingSecret=db", "--set", "serverSecret.existingSecret=server"}

// object is one rendered manifest.
type object map[string]any

func (o object) kind() string { return o["kind"].(string) }

func (o object) name() string { return o["metadata"].(map[string]any)["name"].(string) }

// podSpec returns the pod template's spec of a Deployment or a Job.
func (o object) podSpec() map[string]any {
	template := o["spec"].(map[string]any)["template"].(map[string]any)
	return template["spec"].(map[string]any)
}

// render runs helm template for release with args and returns its manifests.
// A failed render fails the test.
func render(t *testing.T, release string, args ...string) []object {
	t.Helper()
	cmd := exec.Command("helm", append([]string{"template", release, chartDir, "--namespace", "capacity-room"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helm template: %v: %s", err, stderr.String())
	}
	var objects []object
	decoder := yaml.NewDecoder(&stdout)
	for {
		var o object
		err := decoder.Decode(&o)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if o != nil {
			objects = append(objects, o)
		}
	}
	return objects
}

// renderError runs helm template with args, expects it to fail, and returns
// its standard error.
func renderError(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("helm", append([]string{"template", "capacity-room", chartDir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("helm template %s succeeded, want a refusal", strings.Join(args, " "))
	}
	return stderr.String()
}

// find returns the object of kind and name, failing the test when there is
// none.
func find(t *testing.T, objects []object, kind, name string) object {
	t.Helper()
	for _, o := range objects {
		if o.kind() == kind && o.name() == name {
			return o
		}
	}
	t.Fatalf("no %s %s among the rendered objects", kind, name)
	return nil
}
```

- [ ] **Step 5: Write the failing tests**, `internal/chart/chart_test.go` (tag `helm`). Each is written whole; run it red before the template it pins exists (Step 3 may be split: write the tests first, then the templates).

```go
//go:build helm

package chart

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/marceloalvescf/capacity-room/internal/platform/settings"
)

func TestChartVersionEqualsVERSION(t *testing.T) {
	raw, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(raw))
	data, err := os.ReadFile(filepath.Join(chartDir, "Chart.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var chart struct {
		Version    string `yaml:"version"`
		AppVersion string `yaml:"appVersion"`
	}
	if err := yaml.Unmarshal(data, &chart); err != nil {
		t.Fatal(err)
	}
	if chart.Version != want || chart.AppVersion != want {
		t.Fatalf("Chart.yaml version %q, appVersion %q, want both %q", chart.Version, chart.AppVersion, want)
	}
}

func TestLint(t *testing.T) {
	out, err := exec.Command("helm", append([]string{"lint", "--strict", chartDir}, required...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("helm lint: %v\n%s", err, out)
	}
}

// loadSettings writes the platform.toml of the rendered ConfigMap into a
// folder and loads it as serve would.
func loadSettings(t *testing.T, objects []object) settings.Settings {
	t.Helper()
	data := find(t, objects, "ConfigMap", "capacity-room")["data"].(map[string]any)
	path := filepath.Join(t.TempDir(), "platform.toml")
	if err := os.WriteFile(path, []byte(data["platform.toml"].(string)), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := settings.Load(path, func(key string) string {
		if key == "CAPACITYROOM_DATABASE_DSN" {
			return "postgres://user:password@db:5432/capacityroom"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("settings.Load: %v\n%s", err, data["platform.toml"])
	}
	return loaded
}

func TestDefaultSettingsLoad(t *testing.T) {
	loaded := loadSettings(t, render(t, "capacity-room", required...))
	if loaded.ServerSecretPath != "/opt/capacity-room/secret/server-secret" ||
		loaded.CredentialDir != "/opt/capacity-room/credentials" || loaded.CABundleDir != "/opt/capacity-room/ca" {
		t.Fatalf("paths: %q %q %q", loaded.ServerSecretPath, loaded.CredentialDir, loaded.CABundleDir)
	}
	if loaded.WorkerSlots != 2 || loaded.WorkerDrainSeconds != 30 {
		t.Fatalf("defaults: slots %d, drain %d", loaded.WorkerSlots, loaded.WorkerDrainSeconds)
	}
}

// TestEverySettingLoads sets the fifteen keys at their largest allowed value,
// from a values file and again from --set, so that a number Helm reads as a
// float still reaches the program as an integer.
func TestEverySettingLoads(t *testing.T) {
	fromFile := loadSettings(t, render(t, "capacity-room", append(required, "-f", "testdata/every-setting.yaml")...))
	fromSet := loadSettings(t, render(t, "capacity-room", append(required,
		"--set", "platform.session_ttl_seconds=2592000",
		"--set", "platform.session_idle_seconds=2592000",
		"--set", "platform.worker_slots=16",
		"--set", "platform.worker_drain_seconds=3600",
		"--set", "platform.collect_timeout_seconds=3600",
		"--set", "platform.render_timeout_seconds=3600",
		"--set", "platform.job_max_attempts=10",
		"--set", "platform.report_retention_days=365",
		"--set", "platform.expired_report_days=365",
		"--set", "platform.admission_limit=1000",
		"--set", "platform.admission_profile_limit=1000",
		"--set", "platform.admission_retry_seconds=300",
		"--set", "platform.trusted_proxies={10.244.0.0/16}",
		"--set", "platform.allowed_endpoint_hostnames={prometheus.monitoring.svc.cluster.local}",
		"--set", "platform.allowed_endpoint_cidrs={10.96.0.0/12}",
	)...))
	if !reflect.DeepEqual(fromFile, fromSet) {
		t.Fatalf("the file and --set give different settings:\n%+v\n%+v", fromFile, fromSet)
	}
	if fromFile.SessionTTLSeconds != 2592000 || fromFile.WorkerSlots != 16 || fromFile.AdmissionLimit != 1000 {
		t.Fatalf("settings = %+v", fromFile)
	}
}

func TestChartOwnedKeysAreRefused(t *testing.T) {
	for _, key := range []string{"server_secret_path", "credential_dir", "ca_bundle_dir", "unknown_key"} {
		renderError(t, append(required, "--set", "platform."+key+"=/x")...)
	}
}

func TestRequiredValues(t *testing.T) {
	renderError(t, "--set", "serverSecret.existingSecret=server")
	renderError(t, "--set", "database.existingSecret=db")
	renderError(t, append(required, "--set", "unknown=1")...)
	renderError(t, append(required, "--set", "worker.resources.limits.memory=4G")...)
}

// pods returns the pod spec of every Deployment and Job rendered.
func pods(objects []object) map[string]map[string]any {
	specs := map[string]map[string]any{}
	for _, o := range objects {
		if o.kind() == "Deployment" || o.kind() == "Job" {
			specs[o.kind()+" "+o.name()] = o.podSpec()
		}
	}
	return specs
}

func TestPodSecurity(t *testing.T) {
	for name, spec := range pods(render(t, "capacity-room", required...)) {
		if spec["automountServiceAccountToken"] != false {
			t.Errorf("%s mounts a service account token", name)
		}
		pod := spec["securityContext"].(map[string]any)
		if pod["runAsNonRoot"] != true || pod["runAsUser"] != 10001 || pod["runAsGroup"] != 10001 ||
			pod["seccompProfile"].(map[string]any)["type"] != "RuntimeDefault" {
			t.Errorf("%s: pod security context %v", name, pod)
		}
		for _, c := range spec["containers"].([]any) {
			sc := c.(map[string]any)["securityContext"].(map[string]any)
			drop := sc["capabilities"].(map[string]any)["drop"].([]any)
			if sc["readOnlyRootFilesystem"] != true || sc["allowPrivilegeEscalation"] != false ||
				len(drop) != 1 || drop[0] != "ALL" {
				t.Errorf("%s: container security context %v", name, sc)
			}
		}
	}
}

func TestDeployments(t *testing.T) {
	objects := render(t, "capacity-room", required...)
	loaded := loadSettings(t, objects)
	for _, name := range []string{"capacity-room-serve", "capacity-room-worker"} {
		spec := find(t, objects, "Deployment", name)["spec"].(map[string]any)
		if spec["replicas"] != 1 || spec["strategy"].(map[string]any)["type"] != "Recreate" {
			t.Errorf("%s: replicas %v, strategy %v", name, spec["replicas"], spec["strategy"])
		}
	}
	serve := find(t, objects, "Deployment", "capacity-room-serve").podSpec()
	if serve["terminationGracePeriodSeconds"] != 45 {
		t.Errorf("serve grace %v, want 45", serve["terminationGracePeriodSeconds"])
	}
	worker := find(t, objects, "Deployment", "capacity-room-worker").podSpec()
	if got, want := worker["terminationGracePeriodSeconds"], int(2*loaded.WorkerDrainSeconds+15); got != want {
		t.Errorf("worker grace %v, want %d from the program's default drain", got, want)
	}
	drained := render(t, "capacity-room", append(required, "--set", "platform.worker_drain_seconds=60")...)
	if got := find(t, drained, "Deployment", "capacity-room-worker").podSpec()["terminationGracePeriodSeconds"]; got != 135 {
		t.Errorf("worker grace %v with a drain of 60, want 135", got)
	}
	env := worker["containers"].([]any)[0].(map[string]any)["env"].([]any)
	if !containsEnv(env, "GOMEMLIMIT", "3072MiB") {
		t.Errorf("worker env %v, want GOMEMLIMIT 3072MiB for a limit of 4Gi", env)
	}
}

func containsEnv(env []any, name, value string) bool {
	for _, e := range env {
		m := e.(map[string]any)
		if m["name"] == name && m["value"] == value {
			return true
		}
	}
	return false
}

func TestSettingsChangeRecreatesThePods(t *testing.T) {
	checksum := func(objects []object) string {
		template := find(t, objects, "Deployment", "capacity-room-serve")["spec"].(map[string]any)["template"].(map[string]any)
		return template["metadata"].(map[string]any)["annotations"].(map[string]any)["checksum/settings"].(string)
	}
	before := checksum(render(t, "capacity-room", required...))
	after := checksum(render(t, "capacity-room", append(required, "--set", "platform.worker_slots=4")...))
	if before == after {
		t.Fatal("the checksum did not change with the settings")
	}
}

func TestMounts(t *testing.T) {
	objects := render(t, "capacity-room", append(required,
		"--set", "credentials.existingSecret=creds", "--set", "caBundles.existingConfigMap=cas")...)
	want := map[string]bool{
		"/tmp": false, "/opt/capacity-room/platform.toml": true, "/opt/capacity-room/secret": true,
		"/opt/capacity-room/credentials": true, "/opt/capacity-room/ca": true,
	}
	for name, spec := range pods(objects) {
		c := spec["containers"].([]any)[0].(map[string]any)
		got := map[string]bool{}
		for _, m := range c["volumeMounts"].([]any) {
			mount := m.(map[string]any)
			got[mount["mountPath"].(string)] = mount["readOnly"] == true
		}
		for path, readOnly := range want {
			if r, ok := got[path]; !ok || r != readOnly {
				t.Errorf("%s: mount %s missing or readOnly %v", name, path, r)
			}
		}
		for path := range got {
			if strings.HasPrefix(path, "/etc") {
				t.Errorf("%s mounts %s", name, path)
			}
		}
		dsn := c["env"].([]any)[0].(map[string]any)
		ref := dsn["valueFrom"].(map[string]any)["secretKeyRef"].(map[string]any)
		if dsn["name"] != "CAPACITYROOM_DATABASE_DSN" || ref["name"] != "db" || ref["key"] != "uri" {
			t.Errorf("%s: DSN env %v", name, dsn)
		}
	}
}

var dnsLabel = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func TestLongReleaseName(t *testing.T) {
	release := strings.Repeat("a", 59) + "b"
	for _, o := range render(t, release, required...) {
		if name := o.name(); len(name) > 63 || !dnsLabel.MatchString(name) {
			t.Errorf("%s name %q (%s characters) is not a DNS label", o.kind(), name, strconv.Itoa(len(name)))
		}
	}
}
```

`internal/chart/testdata/every-setting.yaml` holds the fifteen keys under `platform:` with the values of `TestEverySettingLoads`, lists written as YAML lists.

- [ ] **Step 6: The schema's keys equal the program's**, `internal/platform/settings/chart_test.go` (no tag: it reads a file and needs no tool):

```go
package settings

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

// chartOwnedKeys are the keys the chart writes itself and refuses under
// platform.
var chartOwnedKeys = []string{"server_secret_path", "credential_dir", "ca_bundle_dir"}

func TestChartSchemaNamesEverySetting(t *testing.T) {
	data, err := os.ReadFile("../../../deploy/helm/capacity-room/values.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			Platform struct {
				Properties map[string]struct {
					Type string `json:"type"`
				} `json:"properties"`
			} `json:"platform"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	got := append([]string(nil), chartOwnedKeys...)
	for key := range schema.Properties.Platform.Properties {
		got = append(got, key)
	}
	want := codeKeys()
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schema and chart-owned keys %v, want the settings keys %v", got, want)
	}
	rt := reflect.TypeOf(tomlSettings{})
	for i := 0; i < rt.NumField(); i++ {
		key := rt.Field(i).Tag.Get("toml")
		entry, ok := schema.Properties.Platform.Properties[key]
		if !ok {
			continue
		}
		want := map[reflect.Kind]string{reflect.Int64: "integer", reflect.Slice: "array"}[rt.Field(i).Type.Kind()]
		if entry.Type != want {
			t.Errorf("schema type of %s is %q, want %q", key, entry.Type, want)
		}
	}
}
```

- [ ] **Step 7: The server secret through a Secret volume**, in `internal/platform/settings/settings_test.go`:

```go
// TestReadServerSecretThroughSecretVolumeLinks lays the file out as Kubernetes
// writes a Secret volume: name -> ..data/name, ..data -> ..<timestamp>.
func TestReadServerSecretThroughSecretVolumeLinks(t *testing.T) {
	dir := t.TempDir()
	stamped := filepath.Join(dir, "..2026_10_14_12_00_00.000000001")
	if err := os.Mkdir(stamped, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := bytes.Repeat([]byte("k"), 32)
	if err := os.WriteFile(filepath.Join(stamped, "server-secret"), secret, 0o440); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(stamped), filepath.Join(dir, "..data")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..data/server-secret", filepath.Join(dir, "server-secret")); err != nil {
		t.Fatal(err)
	}
	got, err := Settings{ServerSecretPath: filepath.Join(dir, "server-secret")}.ReadServerSecret()
	if err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("ReadServerSecret = %q, %v", got, err)
	}
}
```

Add `bytes`, `os`, and `path/filepath` to the file's imports where they are missing.

- [ ] **Step 8: Run red, implement, run green.**

Run: `nix develop -c go test -tags=helm ./internal/chart/ -v` and `nix develop -c go test ./internal/platform/settings/ ./internal/guard/ -v -run 'Chart|SecretVolume|BuildTag|Layer'`.
Expected before the templates: FAIL (`helm template` errors, missing schema file). After the files of Step 3: PASS.

- [ ] **Step 9: Full suite and commit**

Run: `nix develop -c make test` and `nix develop -c make lint`. Expected: PASS.

```bash
git add flake.nix deploy/Containerfile internal/guard/buildtags.go internal/guard/imports.go Makefile \
  internal/chart/ internal/platform/settings/chart_test.go internal/platform/settings/settings_test.go \
  deploy/helm/capacity-room/
git commit -m "feat(deploy): add the Helm chart's core"
```

---

### Task 4: routing and offline schema checks

**Files:**
- Create: `deploy/helm/capacity-room/templates/httproute.yaml`, `templates/ingress.yaml`, `examples/gateway.yaml`, `examples/ingress.yaml`
- Modify: `deploy/helm/capacity-room/values.yaml`, `values.schema.json`, `templates/NOTES.txt`
- Create: `deploy/helm/schemas/update.sh`, `deploy/helm/schemas/README.md`, `deploy/helm/schemas/kubernetes/{configmap-v1,deployment-apps-v1,job-batch-v1,ingress-networking-v1,service-v1,serviceaccount-v1}.json`, `deploy/helm/schemas/gateway.networking.k8s.io/httproute_v1.json`
- Create: `internal/chart/route_test.go`, `internal/chart/kubeconform_test.go`

**Interfaces:**
- Consumes: the helpers and test helpers of Task 3.
- Produces: `kubeconform(t *testing.T, objects ...[]byte)` in `internal/chart`, used by Task 5 for the Jobs; `renderRaw(t *testing.T, args ...string) []byte` (the raw multi-document output of `helm template`).

- [ ] **Step 1: The schemas.** `deploy/helm/schemas/update.sh` writes them from pinned sources:

```bash
#!/usr/bin/env bash
# Writes the JSON schemas that kubeconform reads in the chart's tests: the
# Kubernetes kinds the chart emits, from yannh/kubernetes-json-schema at a
# fixed commit, and HTTPRoute, converted from the Gateway API release's CRD
# by kubeconform's own script. Run it from the repository root, inside
# nix develop; it needs the network.
set -euo pipefail
cd "$(dirname "$0")"
K8S_SCHEMA_COMMIT=<the commit resolved in this step>
K8S_VERSION=v1.37.0
GATEWAY_API=v1.6.3
KUBECONFORM=v0.8.0
mkdir -p kubernetes gateway.networking.k8s.io
for kind in configmap-v1 deployment-apps-v1 job-batch-v1 ingress-networking-v1 service-v1 serviceaccount-v1; do
  curl -fsSL "https://raw.githubusercontent.com/yannh/kubernetes-json-schema/${K8S_SCHEMA_COMMIT}/${K8S_VERSION}-standalone-strict/${kind}.json" \
    -o "kubernetes/${kind}.json"
done
mkdir -p "$(git rev-parse --show-toplevel)/.cache"
work="$(mktemp -d "$(git rev-parse --show-toplevel)/.cache/schemas.XXXXXX")"
trap 'rm -rf "$work"' EXIT
curl -fsSL "https://github.com/kubernetes-sigs/gateway-api/releases/download/${GATEWAY_API}/standard-install.yaml" -o "$work/standard-install.yaml"
curl -fsSL "https://raw.githubusercontent.com/yannh/kubeconform/${KUBECONFORM}/scripts/openapi2jsonschema.py" -o "$work/openapi2jsonschema.py"
( cd "$work" && DENY_ROOT_ADDITIONAL_PROPERTIES=true FILENAME_FORMAT='{kind}_{version}' \
  nix shell --impure --expr '(builtins.getFlake "nixpkgs").legacyPackages.${builtins.currentSystem}.python3.withPackages (p: [ p.pyyaml ])' \
  -c python3 openapi2jsonschema.py standard-install.yaml )
cp "$work/httproute_v1.json" gateway.networking.k8s.io/httproute_v1.json
```

Resolve the commit first: `git ls-remote https://github.com/yannh/kubernetes-json-schema master`, and write it into `K8S_SCHEMA_COMMIT`. Check that `v1.37.0-standalone-strict/deployment-apps-v1.json` exists at it (`curl -fsI`). Then run the script. If any of its commands fails, stop and report the command and its error; do not hand-write a schema. `deploy/helm/schemas/README.md` names the four pins, the kinds, and the command, and says a new kind the chart emits needs its schema here before kubeconform accepts it.

- [ ] **Step 2: The values and schema.** `values.yaml` gains:

```yaml
# A Gateway API HTTPRoute to serve. parentRefs names the Gateway, for
# example [{name: gateway, namespace: gateway-system, sectionName: https}];
# TLS is the Gateway's. route and ingress exclude each other.
route:
  enabled: false
  parentRefs: []
  hostnames: []

# An Ingress to serve, for a cluster with an Ingress controller. hosts is a
# list of host names; tls is the Ingress tls list as Kubernetes writes it.
ingress:
  enabled: false
  className: ""
  annotations: {}
  hosts: []
  tls: []
```

`values.schema.json` adds `route` and `ingress` to `required` and to `properties`:

```json
    "route": {
      "type": "object", "additionalProperties": false,
      "properties": {
        "enabled": {"type": "boolean"},
        "parentRefs": {"type": "array", "items": {"type": "object"}},
        "hostnames": {"type": "array", "items": {"type": "string"}}
      },
      "if": {"properties": {"enabled": {"const": true}}},
      "then": {"properties": {"parentRefs": {"minItems": 1}, "hostnames": {"minItems": 1}}}
    },
    "ingress": {
      "type": "object", "additionalProperties": false,
      "properties": {
        "enabled": {"type": "boolean"},
        "className": {"type": "string"},
        "annotations": {"type": "object", "additionalProperties": {"type": "string"}},
        "hosts": {"type": "array", "items": {"type": "string"}},
        "tls": {"type": "array", "items": {"type": "object"}}
      },
      "if": {"properties": {"enabled": {"const": true}}},
      "then": {"properties": {"hosts": {"minItems": 1}}}
    }
```

and at the root, beside `required`:

```json
  "not": {
    "properties": {
      "route": {"properties": {"enabled": {"const": true}}},
      "ingress": {"properties": {"enabled": {"const": true}}}
    }
  },
```

- [ ] **Step 3: The templates.** `templates/httproute.yaml`:

```yaml
{{- if .Values.route.enabled }}
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: {{ include "capacity-room.fullname" . }}
  labels:
    {{- include "capacity-room.labels" . | nindent 4 }}
spec:
  parentRefs:
    {{- toYaml .Values.route.parentRefs | nindent 4 }}
  hostnames:
    {{- toYaml .Values.route.hostnames | nindent 4 }}
  rules:
    - backendRefs:
        - name: {{ include "capacity-room.fullname" . }}
          port: 8080
{{- end }}
```

`templates/ingress.yaml`:

```yaml
{{- if .Values.ingress.enabled }}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "capacity-room.fullname" . }}
  labels:
    {{- include "capacity-room.labels" . | nindent 4 }}
  {{- with .Values.ingress.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- with .Values.ingress.className }}
  ingressClassName: {{ . }}
  {{- end }}
  {{- with .Values.ingress.tls }}
  tls:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  rules:
    {{- range .Values.ingress.hosts }}
    - host: {{ . | quote }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ include "capacity-room.fullname" $ }}
                port:
                  number: 8080
    {{- end }}
{{- end }}
```

`NOTES.txt` gains, after its first paragraph:

```text
{{- if .Values.route.enabled }}
The web answers at https://{{ first .Values.route.hostnames }}.
{{- else if .Values.ingress.enabled }}
The web answers at https://{{ first .Values.ingress.hosts }}.
{{- end }}
```

`examples/gateway.yaml`:

```yaml
database:
  existingSecret: capacity-room-db-app
serverSecret:
  existingSecret: capacity-room-server-secret
imagePullSecrets:
  - name: registry-pull
platform:
  trusted_proxies: ["10.244.0.0/16"]
  allowed_endpoint_hostnames: ["prometheus.monitoring.svc.cluster.local"]
route:
  enabled: true
  parentRefs:
    - name: gateway
      namespace: gateway-system
      sectionName: https
  hostnames: ["capacityroom.example.com"]
```

`examples/ingress.yaml`: the same `database`, `serverSecret`, and `platform`, and

```yaml
ingress:
  enabled: true
  className: nginx
  hosts: ["capacityroom.example.com"]
  tls:
    - hosts: ["capacityroom.example.com"]
      secretName: capacityroom-example-com-tls
```

- [ ] **Step 4: Write the failing tests.** `internal/chart/kubeconform_test.go`:

```go
//go:build helm

package chart

import (
	"bytes"
	"os/exec"
	"testing"
)

// renderRaw runs helm template with args and returns its output whole.
func renderRaw(t *testing.T, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("helm", append([]string{"template", "capacity-room", chartDir, "--namespace", "capacity-room"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helm template: %v: %s", err, stderr.String())
	}
	return stdout.Bytes()
}

// kubeconform checks manifests against the schemas kept in the repository
// alone; a kind with no schema there fails.
func kubeconform(t *testing.T, manifests []byte) {
	t.Helper()
	cmd := exec.Command("kubeconform", "-strict", "-summary",
		"-schema-location", "../../deploy/helm/schemas/kubernetes/{{.ResourceKind}}{{.KindSuffix}}.json",
		"-schema-location", "../../deploy/helm/schemas/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json",
		"-")
	cmd.Stdin = bytes.NewReader(manifests)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kubeconform: %v\n%s", err, out)
	}
}

func TestKubeconform(t *testing.T) {
	kubeconform(t, renderRaw(t, required...))
	kubeconform(t, renderRaw(t, "-f", "../../deploy/helm/capacity-room/examples/gateway.yaml"))
	kubeconform(t, renderRaw(t, "-f", "../../deploy/helm/capacity-room/examples/ingress.yaml"))
}

func TestKubeconformRefusesAWrongManifest(t *testing.T) {
	wrong := []byte("apiVersion: v1\nkind: Service\nmetadata:\n  name: x\nspec:\n  portz: []\n")
	cmd := exec.Command("kubeconform", "-strict",
		"-schema-location", "../../deploy/helm/schemas/kubernetes/{{.ResourceKind}}{{.KindSuffix}}.json", "-")
	cmd.Stdin = bytes.NewReader(wrong)
	if err := cmd.Run(); err == nil {
		t.Fatal("kubeconform accepted a Service with an unknown field")
	}
}
```

`internal/chart/route_test.go`:

```go
//go:build helm

package chart

import "testing"

func TestRoute(t *testing.T) {
	objects := render(t, "capacity-room", "-f", "../../deploy/helm/capacity-room/examples/gateway.yaml")
	route := find(t, objects, "HTTPRoute", "capacity-room")["spec"].(map[string]any)
	backend := route["rules"].([]any)[0].(map[string]any)["backendRefs"].([]any)[0].(map[string]any)
	if backend["name"] != "capacity-room" || backend["port"] != 8080 {
		t.Fatalf("backend %v", backend)
	}
	for _, o := range objects {
		if o.kind() == "Ingress" {
			t.Fatal("an Ingress rendered with the route")
		}
	}
}

func TestIngress(t *testing.T) {
	objects := render(t, "capacity-room", "-f", "../../deploy/helm/capacity-room/examples/ingress.yaml")
	spec := find(t, objects, "Ingress", "capacity-room")["spec"].(map[string]any)
	if spec["ingressClassName"] != "nginx" {
		t.Fatalf("ingressClassName %v", spec["ingressClassName"])
	}
	rule := spec["rules"].([]any)[0].(map[string]any)
	if rule["host"] != "capacityroom.example.com" {
		t.Fatalf("rule %v", rule)
	}
}

func TestRouteAndIngressExcludeEachOther(t *testing.T) {
	renderError(t, append(required,
		"--set", "route.enabled=true", "--set", "route.parentRefs[0].name=gateway", "--set", "route.hostnames={a.example.com}",
		"--set", "ingress.enabled=true", "--set", "ingress.hosts={a.example.com}")...)
	renderError(t, append(required, "--set", "route.enabled=true")...)
	renderError(t, append(required, "--set", "ingress.enabled=true")...)
}

func TestLintExamples(t *testing.T) {
	for _, file := range []string{"gateway.yaml", "ingress.yaml"} {
		lint(t, "-f", "../../deploy/helm/capacity-room/examples/"+file)
	}
}
```

Move the body of `TestLint` (Task 3) into a helper `lint(t *testing.T, args ...string)` in `render_test.go` that runs `helm lint --strict chartDir args...` and fails on an error; `TestLint` calls `lint(t, required...)`.

- [ ] **Step 5: Run red, implement, run green**

Run: `nix develop -c go test -tags=helm ./internal/chart/ -v`.
Expected before Step 3: FAIL (no HTTPRoute rendered, schema refusals missing). After: PASS, with kubeconform's summary showing every resource valid.

- [ ] **Step 6: Full suite and commit**

Run: `nix develop -c make test` and `nix develop -c make lint`. Expected: PASS.

```bash
git add deploy/helm/ internal/chart/
git commit -m "feat(deploy): route the chart by Gateway or Ingress"
```

---

### Task 5: the hook Jobs

**Files:**
- Create: `deploy/helm/capacity-room/templates/job-admin.yaml`, `templates/job-profiles.yaml`
- Modify: `deploy/helm/capacity-room/values.yaml`, `values.schema.json`, `templates/_helpers.tpl`, `templates/configmap.yaml`, `templates/NOTES.txt`, `examples/gateway.yaml`
- Create: `internal/chart/jobs_test.go`, `internal/chart/testdata/profiles.yaml`, `internal/platform/profiles/chart_test.go`
- Modify: `internal/platform/profiles/refs_test.go`

**Interfaces:**
- Consumes: `--password-file` (Task 1); the helpers of Task 3; `kubeconform` and `renderRaw` (Task 4).
- Produces: the ConfigMap key `profiles.toml`; the Jobs `{{ fullname }}-admin` and `{{ fullname }}-profiles`.

- [ ] **Step 1: The values and schema.** `values.yaml` gains:

```yaml
# The first administrator, created by a Job at the first install. Leave
# username empty when the database already holds a user, as in a reinstall
# over a kept database: the command refuses then, and the install fails.
admin:
  username: ""
  # The first administrator's password, 12 to 1024 characters, read once by
  # the admin Job at the first install. Create the Secret before installing:
  #   kubectl -n <namespace> create secret generic capacity-room-admin \
  #     --from-file=password=./admin-password
  existingSecret: ""
  passwordKey: password
  # pt-BR or en.
  language: en

# The profiles, as the [[profiles]] tables of admin import-profiles, written
# as YAML. A Job imports them at every install and upgrade; an edit made on
# the web to a profile named here holds until the next upgrade. A profile
# names its credential and CA bundle by file name, so no secret goes here.
profiles: []
```

`values.schema.json` adds both to `required` and to `properties`:

```json
    "admin": {
      "type": "object", "additionalProperties": false,
      "properties": {
        "username": {"type": "string"},
        "existingSecret": {"type": "string"},
        "passwordKey": {"type": "string", "minLength": 1},
        "language": {"enum": ["pt-BR", "en"]}
      },
      "if": {"properties": {"username": {"minLength": 1}}},
      "then": {"properties": {"existingSecret": {"minLength": 1}}}
    },
    "profiles": {
      "type": "array",
      "items": {
        "type": "object", "additionalProperties": false, "required": ["id"],
        "properties": {
          "id": {"type": "string"},
          "display_name": {"type": "string"},
          "active": {"type": "boolean"},
          "endpoint_url": {"type": "string"},
          "endpoint_type": {"type": "string"},
          "timeout_seconds": {"type": "number"},
          "tls_verify": {"type": "boolean"},
          "credential_file": {"type": "string"},
          "ca_bundle_file": {"type": "string"},
          "cluster_name": {"type": "string"},
          "cluster_label": {"type": "string"},
          "filter_mode": {"type": "string"},
          "nodepool_label": {"type": "string"},
          "node_autoscaling": {"type": "string"},
          "allowed_namespaces": {"type": "array", "items": {"type": "string"}},
          "excluded_containers": {"type": "array", "items": {"type": "string"}},
          "permitted_periods": {"type": "array", "items": {"type": "string"}},
          "default_period": {"type": "string"},
          "permitted_timezones": {"type": "array", "items": {"type": "string"}},
          "default_timezone": {"type": "string"},
          "permitted_report_profiles": {"type": "array", "items": {"type": "string"}},
          "default_report_profile": {"type": "string"},
          "permitted_min_severities": {"type": "array", "items": {"type": "string"}},
          "default_min_severity": {"type": "string"},
          "thresholds": {
            "type": "object", "additionalProperties": false,
            "properties": {
              "warning_percent": {"type": "number"},
              "critical_percent": {"type": "number"},
              "low_utilization_percent": {"type": "number"},
              "autoscaler_near_max_percent": {"type": "number"},
              "nodepool_spread_points": {"type": "number"},
              "bursty_gap_percent": {"type": "number"},
              "hpa_replica_divergence": {"type": "number"}
            }
          }
        }
      }
    }
```

The import judges the values; the schema checks only the keys and their types.

- [ ] **Step 2: The schema's profile keys equal the import's**, `internal/platform/profiles/chart_test.go`:

```go
package profiles

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

type schemaEntry struct {
	Type       string                 `json:"type"`
	Properties map[string]schemaEntry `json:"properties"`
	Items      *schemaEntry           `json:"items"`
}

// tomlKeys returns the toml keys of t's fields and the schema type each
// field's Go type stands for.
func tomlKeys(t reflect.Type) map[string]string {
	types := map[string]string{}
	for i := 0; i < t.NumField(); i++ {
		key := t.Field(i).Tag.Get("toml")
		if key == "" || key == "-" {
			continue
		}
		ft := t.Field(i).Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		types[key] = map[reflect.Kind]string{
			reflect.String: "string", reflect.Bool: "boolean", reflect.Float64: "number",
			reflect.Slice: "array", reflect.Struct: "object",
		}[ft.Kind()]
	}
	return types
}

func compareKeys(t *testing.T, where string, schema map[string]schemaEntry, code map[string]string) {
	t.Helper()
	var got, want []string
	for k := range schema {
		got = append(got, k)
	}
	for k := range code {
		want = append(want, k)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: schema keys %v, want the import's %v", where, got, want)
	}
	for k, typ := range code {
		if schema[k].Type != typ {
			t.Errorf("%s: schema type of %s is %q, want %q", where, k, schema[k].Type, typ)
		}
	}
}

func TestChartSchemaNamesEveryImportKey(t *testing.T) {
	data, err := os.ReadFile("../../../deploy/helm/capacity-room/values.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			Profiles schemaEntry `json:"profiles"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	entry := schema.Properties.Profiles.Items
	if entry == nil {
		t.Fatal("the schema has no profiles items")
	}
	compareKeys(t, "profiles", entry.Properties, tomlKeys(reflect.TypeOf(ImportEntry{})))
	compareKeys(t, "thresholds", entry.Properties["thresholds"].Properties, tomlKeys(reflect.TypeOf(ImportThresholds{})))
}
```

- [ ] **Step 3: A credential through a Secret volume**, in `internal/platform/profiles/refs_test.go`:

```go
// TestResolveCredentialThroughSecretVolumeLinks lays the folder out as
// Kubernetes writes a Secret volume: name -> ..data/name, ..data ->
// ..<timestamp>, all inside the folder.
func TestResolveCredentialThroughSecretVolumeLinks(t *testing.T) {
	dir := t.TempDir()
	stamped := "..2026_10_14_12_00_00.000000001"
	if err := os.Mkdir(filepath.Join(dir, stamped), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, stamped), "prometheus-bearer", []byte("a-bearer-token-value\n"))
	if err := os.Symlink(stamped, filepath.Join(dir, "..data")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..data/prometheus-bearer", filepath.Join(dir, "prometheus-bearer")); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveCredential(dir, "prometheus-bearer")
	if err != nil || got != "a-bearer-token-value" {
		t.Fatalf("ResolveCredential = %q, %v", got, err)
	}
}
```

Read `write` at `refs_test.go:22` first and adapt the call to its signature.

- [ ] **Step 4: The templates.** `_helpers.tpl` gains the profiles file:

```gotemplate
{{/* profiles.toml: one [[profiles]] table per profile, each value written
as JSON, which TOML reads as the same string, number, boolean, or list. */}}
{{- define "capacity-room.profilesToml" -}}
{{- range .Values.profiles }}
[[profiles]]
{{- range $key, $value := . }}
{{- if ne $key "thresholds" }}
{{ $key }} = {{ toJson $value }}
{{- end }}
{{- end }}
{{- with .thresholds }}
[profiles.thresholds]
{{- range $key, $value := . }}
{{ $key }} = {{ toJson $value }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}
```

`templates/configmap.yaml` gains, under `data`:

```yaml
  {{- with .Values.profiles }}
  profiles.toml: |
    {{- include "capacity-room.profilesToml" $ | nindent 4 }}
  {{- end }}
```

`templates/job-admin.yaml`:

```yaml
{{- if .Values.admin.username }}
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ include "capacity-room.fullname" . }}-admin
  labels:
    {{- include "capacity-room.labels" . | nindent 4 }}
    app.kubernetes.io/component: admin
  annotations:
    helm.sh/hook: post-install
    helm.sh/hook-delete-policy: before-hook-creation,hook-succeeded
spec:
  backoffLimit: 3
  template:
    metadata:
      labels:
        {{- include "capacity-room.selectorLabels" (dict "root" . "component" "admin") | nindent 8 }}
    spec:
      restartPolicy: Never
      serviceAccountName: {{ include "capacity-room.fullname" . }}
      automountServiceAccountToken: false
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      securityContext:
        {{- include "capacity-room.podSecurityContext" . | nindent 8 }}
      containers:
        - name: admin
          image: {{ include "capacity-room.image" . }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          args:
            - admin
            - bootstrap-admin
            - --username
            - {{ .Values.admin.username | quote }}
            - --language
            - {{ .Values.admin.language | quote }}
            - --password-file
            - {{ printf "/opt/capacity-room/admin/%s" .Values.admin.passwordKey | quote }}
            - --config
            - /opt/capacity-room/platform.toml
          env:
            {{- include "capacity-room.databaseEnv" . | nindent 12 }}
            - name: CAPACITYROOM_BOOTSTRAP_MODE
              value: "1"
          securityContext:
            {{- include "capacity-room.containerSecurityContext" . | nindent 12 }}
          volumeMounts:
            {{- include "capacity-room.volumeMounts" . | nindent 12 }}
            - name: admin
              mountPath: /opt/capacity-room/admin
              readOnly: true
      volumes:
        {{- include "capacity-room.volumes" . | nindent 8 }}
        - name: admin
          secret:
            secretName: {{ .Values.admin.existingSecret }}
            defaultMode: 0440
            items:
              - key: {{ .Values.admin.passwordKey }}
                path: {{ .Values.admin.passwordKey }}
{{- end }}
```

`templates/job-profiles.yaml` is the same Job with: name suffix and component `profiles`; `helm.sh/hook: post-install,post-upgrade`; `{{- if .Values.profiles }}`; `args: ["admin", "import-profiles", "--file", "/opt/capacity-room/profiles.toml", "--config", "/opt/capacity-room/platform.toml"]`; the DSN env alone; the shared mounts plus

```yaml
            - name: settings
              mountPath: /opt/capacity-room/profiles.toml
              subPath: profiles.toml
              readOnly: true
```

and no admin volume. `NOTES.txt` gains:

```text
{{- if .Values.admin.username }}
The administrator {{ .Values.admin.username }} is created by the Job {{ include "capacity-room.fullname" . }}-admin; Helm removes it once it succeeds, and a failure keeps it for its log.
{{- end }}
{{- if .Values.profiles }}
The profiles are imported by the Job {{ include "capacity-room.fullname" . }}-profiles at every install and upgrade.
{{- end }}
```

`examples/gateway.yaml` gains `admin: {username: admin, existingSecret: capacity-room-admin}` and one profile:

```yaml
profiles:
  - id: prod
    display_name: Production
    endpoint_url: http://prometheus.monitoring.svc.cluster.local:9090
    endpoint_type: prometheus
    cluster_name: prod
    allowed_namespaces: ["payments", "checkout"]
    active: true
```

- [ ] **Step 5: Write the failing tests**, `internal/chart/jobs_test.go`:

```go
//go:build helm

package chart

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
	"github.com/marceloalvescf/capacity-room/internal/platform/profiles"
)

func TestNoJobsByDefault(t *testing.T) {
	for _, o := range render(t, "capacity-room", required...) {
		if o.kind() == "Job" {
			t.Fatalf("a Job %s rendered with no admin and no profiles", o.name())
		}
	}
}

func TestAdminJob(t *testing.T) {
	objects := render(t, "capacity-room", append(required,
		"--set", "admin.username=admin", "--set", "admin.existingSecret=admin-secret")...)
	job := find(t, objects, "Job", "capacity-room-admin")
	annotations := job["metadata"].(map[string]any)["annotations"].(map[string]any)
	if annotations["helm.sh/hook"] != "post-install" ||
		annotations["helm.sh/hook-delete-policy"] != "before-hook-creation,hook-succeeded" {
		t.Fatalf("annotations %v", annotations)
	}
	c := job.podSpec()["containers"].([]any)[0].(map[string]any)
	want := []any{"admin", "bootstrap-admin", "--username", "admin", "--language", "en",
		"--password-file", "/opt/capacity-room/admin/password", "--config", "/opt/capacity-room/platform.toml"}
	if !equalLists(c["args"].([]any), want) {
		t.Fatalf("args %v", c["args"])
	}
	if !containsEnv(c["env"].([]any), "CAPACITYROOM_BOOTSTRAP_MODE", "1") {
		t.Fatalf("env %v", c["env"])
	}
	for name, spec := range pods(objects) {
		hasAdmin := false
		for _, v := range spec["volumes"].([]any) {
			if v.(map[string]any)["name"] == "admin" {
				hasAdmin = true
			}
		}
		if hasAdmin != (name == "Job capacity-room-admin") {
			t.Errorf("%s: admin volume %v", name, hasAdmin)
		}
		if name != "Job capacity-room-admin" {
			for _, e := range spec["containers"].([]any)[0].(map[string]any)["env"].([]any) {
				if e.(map[string]any)["name"] == "CAPACITYROOM_BOOTSTRAP_MODE" {
					t.Errorf("%s sets the bootstrap mode", name)
				}
			}
		}
	}
	renderError(t, append(required, "--set", "admin.username=admin")...)
	renderError(t, append(required, "--set", "admin.username=admin", "--set", "admin.existingSecret=s", "--set", "admin.language=fr")...)
}

func equalLists(got, want []any) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestProfilesRoundTrip renders testdata/profiles.yaml, whose display name
// holds quotes, a backslash, accents, <, and &, and reads the rendered file
// with the import's own parser.
func TestProfilesRoundTrip(t *testing.T) {
	objects := render(t, "capacity-room", append(required, "-f", "testdata/profiles.yaml")...)
	job := find(t, objects, "Job", "capacity-room-profiles")
	annotations := job["metadata"].(map[string]any)["annotations"].(map[string]any)
	if annotations["helm.sh/hook"] != "post-install,post-upgrade" {
		t.Fatalf("annotations %v", annotations)
	}
	data := find(t, objects, "ConfigMap", "capacity-room")["data"].(map[string]any)
	path := filepath.Join(t.TempDir(), "profiles.toml")
	if err := os.WriteFile(path, []byte(data["profiles.toml"].(string)), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded := loadSettings(t, objects)
	entries, err := profiles.ReadImportFile(path, netpolicy.Policy{
		Hostnames: loaded.AllowedEndpointHostnames, Networks: loaded.AllowedEndpointCIDRs,
	})
	if err != nil {
		t.Fatalf("ReadImportFile: %v\n%s", err, data["profiles.toml"])
	}
	if len(entries) != 2 || entries[0].ID != "prod" || entries[1].ID != "staging" {
		t.Fatalf("entries %+v", entries)
	}
	const name = `Prod "east" \ ação <b> & co`
	if entries[0].Document.DisplayName != name {
		t.Fatalf("display name %q, want %q", entries[0].Document.DisplayName, name)
	}
	if entries[1].Document.Thresholds.WarningPercent != 70 {
		t.Fatalf("thresholds %+v", entries[1].Document.Thresholds)
	}
}

func TestKubeconformJobs(t *testing.T) {
	kubeconform(t, renderRaw(t, append(required, "-f", "testdata/profiles.yaml",
		"--set", "admin.username=admin", "--set", "admin.existingSecret=admin-secret")...))
	kubeconform(t, renderRaw(t, "-f", "../../deploy/helm/capacity-room/examples/gateway.yaml"))
}
```

Read `Document` in `internal/platform/profiles` before writing the two field checks: name the display name and the warning threshold by their real field names and types (a pointer field is compared through its value).

`internal/chart/testdata/profiles.yaml`:

```yaml
platform:
  allowed_endpoint_hostnames: ["prometheus.monitoring.svc.cluster.local"]
profiles:
  - id: prod
    display_name: 'Prod "east" \ ação <b> & co'
    endpoint_url: http://prometheus.monitoring.svc.cluster.local:9090
    endpoint_type: prometheus
    cluster_name: prod
    allowed_namespaces: ["payments"]
    timeout_seconds: 30
    active: true
  - id: staging
    display_name: Staging
    endpoint_url: http://prometheus.monitoring.svc.cluster.local:9090
    endpoint_type: prometheus
    cluster_name: staging
    allowed_namespaces: ["default"]
    active: false
    thresholds:
      warning_percent: 70
      critical_percent: 90
```

If `ReadImportFile` refuses one of these documents for a rule of the import (a field the validation requires), add that field with a valid value and say so in the report; never loosen the import.

- [ ] **Step 6: Run red, implement, run green**

Run: `nix develop -c go test -tags=helm ./internal/chart/ -v` and `nix develop -c go test ./internal/platform/profiles/ -v -run 'Chart|SecretVolume'`.
Expected before Step 4: FAIL. After: PASS.

- [ ] **Step 7: Full suite and commit**

Run: `nix develop -c make test` and `nix develop -c make lint`. Expected: PASS.

```bash
git add deploy/helm/capacity-room/ internal/chart/ internal/platform/profiles/chart_test.go internal/platform/profiles/refs_test.go
git commit -m "feat(deploy): create the admin and profiles by Jobs"
```

---

### Task 6: `make push` and the documents

**Files:**
- Modify: `Makefile` (`push`, `.PHONY`)
- Modify: `docs/operations.md` (a new section 12, and section 10), `docs/security.md`, `AGENTS.md`, `README.md`, `CHANGELOG.md`

**Interfaces:**
- Consumes: everything above.

- [ ] **Step 1: `make push`**, after `audit`:

```make
# Builds the runtime image for linux/amd64 and pushes it to Docker Hub under
# the version. It refuses a working tree with changes, so the image is the
# commit's. It is outward: run `docker login` first, and run it only on the
# word of the owner. make gate never runs it.
PUSH_IMAGE := alvesmarcelocf/capacity-room:$(VERSION)

push:
	@test -z "$$(git status --porcelain)" || { echo "push: the working tree has changes"; exit 1; }
	docker build --platform linux/amd64 --file deploy/Containerfile --target runtime --tag $(PUSH_IMAGE) .
	docker push $(PUSH_IMAGE)
```

`push` joins `.PHONY`. Check the refusal without Docker: create an untracked file under the repository root, run `nix develop -c make push`, see `push: the working tree has changes` and exit 2 from make, then delete the file.

- [ ] **Step 2: The guide.** `docs/operations.md` gains `## 12. Kubernetes` after `## 11. A lost administrator password` and before `## Reporting a vulnerability`, so no section is renumbered. The section holds, in normal prose with commands:
  1. What runs: the two Deployments, the Service, the route or the Ingress, the two hook Jobs, the service account without a token; one replica of each, `Recreate`; no Secret, no volume claim, no NetworkPolicy from the chart.
  2. The Secrets to create first: the database DSN (or a CloudNativePG cluster's `<cluster>-app`, key `uri`), the server secret (the two commands of the `serverSecret` comment), the admin password (the `admin.existingSecret` comment's command), the credentials (one key per credential file), the CA bundles ConfigMap, and the pull secret of a private registry (`kubectl create secret docker-registry`).
  3. A values file: `examples/gateway.yaml` named as the starting point, `platform` taking the keys of section 3 (Settings) and refusing the three paths.
  4. Install: `helm upgrade -i capacity-room deploy/helm/capacity-room -n capacity-room --create-namespace -f values.yaml`, and `--dry-run=server` first.
  5. The Jobs: when each runs, where its log is (`kubectl -n capacity-room logs job/capacity-room-admin`), a failure that leaves the release failed, a reinstall over a kept database with `admin.username` empty, and a web edit to a profile of the values that the next upgrade revises back.
  6. Probes, resources, `GOMEMLIMIT`, the grace periods.
  7. Upgrade (`helm upgrade` with the new chart; both pods are recreated, the schema is applied under the lock) and uninstall (the database and the Secrets stay).
  8. `trusted_proxies` behind an in-cluster gateway: name the gateway's fixed address when it has one; trusting the pod range lets any pod of the cluster forge `X-Forwarded-For`, which changes the address the sign-in throttle counts and the audit records, and gives no access.

  Section 10 (Limits) names the chart's limits: one replica each, no NetworkPolicy, the Ingress proven by schema only.

- [ ] **Step 3: The security document.** `docs/security.md` gains, among the controls: no secret in the values or in Helm's release state; no service account token in any pod; the admin password Secret mounted on the admin Job alone, and the bootstrap mode set on that pod alone; the pull secret made by the operator; the `restricted` pod security of every pod. Among the trades: the forged `X-Forwarded-For` of a trusted pod range, as the guide states it.

- [ ] **Step 4: AGENTS.md.**
  - Commands: `make test-chart` ("helm lint, the chart's Go tests, and kubeconform in the test image") and `make push` ("the runtime image to Docker Hub; outward, main thread, on the owner's word").
  - The tags table: `helm` | `helm`, `kubeconform` | `make test-chart`.
  - The layers table: `210` | `internal/chart`.
  - A new section `## The chart` with these rules: the chart lives in `deploy/helm/capacity-room/` and is not published; it creates no Secret, no PersistentVolumeClaim, no NetworkPolicy; nothing is mounted under `/etc`; `Chart.yaml`'s `version` and `appVersion` equal `VERSION`, held by a test; the schema's `platform` keys with the three chart-owned keys equal the settings keys, and its `profiles` keys equal the import file's, held by tests; every pod meets `restricted` and mounts no service account token; the admin Job is `post-install` alone and the profiles Job `post-install,post-upgrade`; kubeconform reads only the schemas of `deploy/helm/schemas/`, so a new kind needs its schema there; the homelab's own files never enter the repository.
  - The subagent list of commands that stay on the main thread gains `make test-chart` and `make push`.

- [ ] **Step 5: README and changelog.** `README.md` gains one sentence beside the Compose install: "A Helm chart in `deploy/helm/capacity-room` runs the platform on Kubernetes; section 12 of the operations guide says how." Under `### Added` of `## [Unreleased]`, before the Task 1 item:

```markdown
- A Helm chart runs the platform on Kubernetes, with a Gateway API route or an Ingress, and creates the first administrator and the profiles from its values.
```

- [ ] **Step 6: Tests and commit**

Run: `nix develop -c make test` (the guide drift tests read the renumbered guide). Expected: PASS.

```bash
git add Makefile docs/operations.md docs/security.md AGENTS.md README.md CHANGELOG.md
git commit -m "docs: run CapacityRoom on Kubernetes"
```

---

### Task 7: gate, reviews, version, image, homelab (main thread)

No subagent runs any step of this task. Every outward step names its target and waits for the owner's word.

- [ ] **Step 1: After each of Tasks 1 to 6**, the task reviewer's verdict, then on the main thread: Task 1, `make postgres-up` and the PostgreSQL run with the DSN; Task 2, `make e2e` (the look stack down and port 8080 free first); Tasks 3 to 5, `make image` then `make test-chart`.
- [ ] **Step 2: Seen failing.** Break the chart on purpose (change `readOnlyRootFilesystem: true` to `false` in `_helpers.tpl`) and run `make test-chart`: it must fail on `TestPodSecurity`. Restore, run again: it passes. Record both runs under `.cache/phase-13/`.
- [ ] **Step 3: `make gate`** on the branch, read to its last line `gate passed for version 2.0.0`.
- [ ] **Step 4: The adversarial review** of the whole branch (`git diff main...HEAD > .cache/phase-13/branch.diff`), by the `adversarial-reviewer` agent; one fix wave; every finding into the ROADMAP's findings log first.
- [ ] **Step 5: The version commit 2.1.0**: `VERSION`, `Chart.yaml` (`version: 2.1.0`, `appVersion: "2.1.0"`), and `## [2.1.0] - <date>` in `CHANGELOG.md`. `make gate` on it, read to `gate passed for version 2.1.0`; `make audit` clean. Logs under `.cache/release-2.1.0/`.
- [ ] **Step 6: Push the image**, on the owner's word for the target `docker.io/alvesmarcelocf/capacity-room:2.1.0` (the owner has run `docker login`): `make push`.
- [ ] **Step 7: The homelab**, context `<homelab-context>`, namespace `capacity-room`, each step on the owner's word, files under `.cache/phase-13/homelab/` (spec section 6):
  1. CloudNativePG: `helm repo add cnpg https://cloudnative-pg.github.io/charts`, the latest chart version pinned and named to the owner, installed into `cnpg-system`; the `Cluster` `capacity-room-db` (one instance, PostgreSQL 18 image by digest, 5Gi on `longhorn`, database and owner `capacityroom`), and its Secret `capacity-room-db-app` checked for `uri`.
  2. The owner creates `dockerhub-pull`; the main thread creates `capacity-room-server-secret` and `capacity-room-admin` from files of mode 0600.
  3. The values: `image.pullPolicy: Always` (the tag can be pushed again before the release tag); `imagePullSecrets: [{name: dockerhub-pull}]`; `database.existingSecret: capacity-room-db-app`; the route to `default-gateway`/`istio-ingress`/`https` for `capacity-room.example.com`; `trusted_proxies: ["10.244.0.0/16"]`; the allowlist of `monitoring-kube-prometheus-prometheus.monitoring.svc.cluster.local`; `admin.username: admin`; the profile `homelab` (cluster name `homelab`, the namespaces the owner names).
  4. `helm upgrade -i --dry-run=server` with these values, and with `examples/ingress.yaml`.
  5. The install; then each check of spec section 6, step 4, with its evidence.
  6. A defect: fix on the branch, gate, push the same tag, upgrade on the homelab, check again.
- [ ] **Step 8: Close.** ROADMAP (the phase's items `DONE` with their commits, the decisions, the learned rules) and `CHANGELOG.md` read back; merge `main` by fast-forward, tag `v2.1.0`, push `main` and the tag over SSH, each on the owner's word.
