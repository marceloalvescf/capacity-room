# Phase 9: Backend Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the eight backend items the owner gave phase 9 on 2026-10-13: a hostile answer no longer multiplies in memory (F-162), sample values written as JSON numbers are read (F-65), values too large to compute with fail with a code of their own (F-163), a DNS failure is retried (F-169), a second signal ends a stop (F-133), a cross-site request cannot rotate the session's CSRF token (F-159), a body is judged before an id (F-160), and the password writers' race is proven absent (F-168).

**Architecture:** Each item is a small change inside one package, with its test and its document. `internal/prom` decodes an answer one series at a time from `json.RawMessage`, so no whole-answer `interface{}` tree is ever held, and every per-series check stays the code it is today. `internal/snapshot` names a non-finite number by a sentinel, which the worker maps to the new code `SOURCE_VALUES_OUT_OF_RANGE`. The rest are local edits to the worker, the command's signal handling, and three API handlers.

**Tech Stack:** Go 1.27, PostgreSQL 18 (tagged tests), OpenAPI 3 with oapi-codegen and openapi-typescript, React locale files (JSON).

**Spec:** `docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md` (sections 11.4, 11.5, 11.10, 15.1), `AGENTS.md`, `docs/operations.md`, `docs/security.md`, and the owner decisions of 2026-10-13 in `ROADMAP.md` (triage, phase 9).

## Global Constraints

- Branch `feat/backend-hardening`; nothing is edited on `main`.
- Parity with the original binds numbers: every number of every scenario stays the same `float64`. The scenario tests of `internal/prom`, `internal/collect`, and `internal/analysis`, the comparisons with `testdata/python`, and `make update-testdata` (main thread) must show no change.
- The public surface changes only by addition (a minor version, 1.4.0): `ReportFailureCode` gains `SOURCE_VALUES_OUT_OF_RANGE`; `GET /auth/session` gains a 403 answer. No field, code, or flag is removed or changes meaning.
- Every report failure code exists in `api/openapi.yaml`, in the table of `docs/operations.md`, and as `reportFailure.<CODE>` in both `web/src/locales/en.json` and `web/src/locales/pt-BR.json`; the drift tests check all three.
- Diagnostics are English. Reader texts use the vocabulary rule: technical terms English in both languages.
- No test skips, no focused test; an expected number in a test is a literal; a test that needs PostgreSQL carries the `postgres` tag and fails when `KCR_TEST_DATABASE_DSN` is unset.
- A comment says what the code does; it names no task, step, finding, or phase.
- Subagents never run Docker. The main thread starts PostgreSQL with `make postgres-up` and hands the DSN to the subagent of Task 8; `make gate` runs on the main thread.
- Commits name their paths (`git add <paths>`), conventional format, subject under 50 characters.

## Review Focus

1. **Every malformed answer keeps the outcome it has today.** A body that is not JSON, not an object, without `status`, with `"data": null` or a non-object `data`, with `"result": null`, an absent `result`, a non-array `result`, a wrong `resultType`, `"metric": null`, a non-object item, and a repeated key must each give the same `*prom.Error` kind and message after the change as before. Pinned by the shape table of Task 2, written and passing before the refactor.
2. **A syntax error at the very end of a large answer** is still refused whole, as "metrics endpoint did not answer the Prometheus API", never parsed in part. Pinned in Task 2's shape table (a valid prefix with a truncated tail).
3. **A permanent NXDOMAIN** now spends `job_max_attempts` before it is stored; it must still end as `ENDPOINT_RESOLUTION_FAILED`, never as `INTERNAL_FAILURE` or `JOB_ATTEMPTS_EXHAUSTED`. Pinned by Task 4.
4. **A client that sends no `Sec-Fetch-Site`** (curl, an API client, an older browser) reads `GET /auth/session` exactly as today, rotation included. Pinned by Task 6.
5. **A second signal during `worker`'s drain** ends the process, as it does for `serve`; the first signal still drains. Pinned by Task 5's subprocess test, which covers the shared signal context of every command.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `internal/prom/result.go`, `internal/prom/range.go` | a sample value as text or number; the per-series walk from `json.RawMessage` | 1, 2 |
| `internal/prom/client.go` | `decodeAPIResponse` returns `json.RawMessage` | 2 |
| `internal/prom/decode_test.go` (new) | the shape table and the memory bound | 2 |
| `internal/prom/range_test.go`, `internal/prom/result_test.go` | number values accepted | 1 |
| `internal/snapshot/codec.go`, `internal/snapshot/codec_test.go` | `ErrNotFinite` | 3 |
| `internal/platform/worker/collect.go`, `internal/platform/worker/size_test.go` | the new code; the retried codes | 3, 4 |
| `api/openapi.yaml`, generated Go types, `web/src/api/schema.gen.ts` | the new code; the session route's 403 | 3, 6 |
| `web/src/locales/en.json`, `web/src/locales/pt-BR.json` | `reportFailure.SOURCE_VALUES_OUT_OF_RANGE` | 3 |
| `docs/operations.md` | failure table; retry column; stop | 3, 4, 5 |
| `cmd/k8s-capacity-report/main.go`, `internal/cli/signal.go` (new), `internal/cli/signal_test.go` (new) | the signal context | 5 |
| `internal/cli/serve.go`, `internal/cli/serve_stop_test.go` | the count of active connections | 5 |
| `internal/platform/api/auth.go`, `internal/platform/api/auth_test.go` (or the test file holding `getSession` tests) | the cross-site refusal | 6 |
| `internal/platform/api/users.go`, `internal/platform/api/profiles.go`, their tests | body before id | 7 |
| `internal/platform/auth/users.go`, `internal/platform/auth/users_lock_test.go` (new, tag `postgres`) | the lock comment and its proof | 8 |
| `docs/security.md`, `AGENTS.md` | the controls and rules each task changes | 2–6 |
| `CHANGELOG.md`, `ROADMAP.md` | records | 9 |

Tasks run in sequence: 1 and 2 share `internal/prom`; 3 and 4 share `collect.go` and `operations.md`; 3 and 6 share `openapi.yaml`; 2 to 6 share `docs/security.md` or `AGENTS.md`.

---

### Task 1: A sample value may be a JSON number (F-65)

The original reads a sample's value with `float()`, which takes a number or a text. This project refuses a number. Prometheus writes text, but a compatible API may write numbers.

**Files:**
- Modify: `internal/prom/result.go` (`parseSampleValue`, new helper `sampleValue`)
- Modify: `internal/prom/range.go` (`parsePoint`)
- Test: `internal/prom/range_test.go`, `internal/prom/result_test.go`

**Interfaces:**
- Produces: `func sampleValue(v interface{}) (float64, bool)` in `result.go`: a JSON string read by `readEncodedValue`, or a JSON number (already a `float64`) taken as it is; any other shape is false.

- [ ] **Step 1: Turn the refusal test into an acceptance test**

In `range_test.go`, remove the case `"value is a number"` from `TestQueryRangeRefusesBadPoint` (keep `"one element"`, and update the doc comment to name only the one-element point), and add:

```go
// TestQueryRangeReadsANumberValue checks that a point whose value is a JSON
// number is read as that number, as the original's float() reads it.
func TestQueryRangeReadsANumberValue(t *testing.T) {
	client := rangeClient(t, `{"status":"success","data":{"resultType":"matrix","result":[
		{"metric":{"pod":"a"},"values":[[1000,2],[1010,"3"],[1020,0.5]]}
	]}}`)
	series, err := client.QueryRange(context.Background(), "up", time.Unix(1, 0), time.Unix(2, 0), "1s")
	if err != nil {
		t.Fatal(err)
	}
	want := []Point{{Time: 1000, Value: 2, Valid: true}, {Time: 1010, Value: 3, Valid: true}, {Time: 1020, Value: 0.5, Valid: true}}
	if len(series) != 1 || !reflect.DeepEqual(series[0].Points, want) {
		t.Fatalf("series = %+v, want one series with %+v", series, want)
	}
}
```

In `result_test.go`, add the same for an instant query, using the helper that file already uses to build a client from a body (read the file for its name), with `"value":[1000,7]`, expecting one sample `{Value: 7, Valid: true}` with labels `{"pod":"a"}`. Also add a case to the existing refusal table of that file (or a new test) that a value of `true`, of `null`, and of `[1]` is still refused with `KindQuery` and the message `Prometheus response contains an invalid sample`.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/prom/ -run 'NumberValue|InvalidSample' -v`
Expected: the two number tests FAIL with "Prometheus response contains an invalid sample"; the refusal cases PASS.

- [ ] **Step 3: Read a number or a text**

In `result.go`, add after `readEncodedValue`:

```go
// sampleValue reads a sample's value member: a JSON string read by
// readEncodedValue, or a JSON number, already a float64, taken as it is, as
// the original's float() takes either.
func sampleValue(v interface{}) (float64, bool) {
	switch value := v.(type) {
	case string:
		return readEncodedValue(value)
	case float64:
		return value, true
	default:
		return 0, false
	}
}
```

In `parseSampleValue` replace

```go
	encoded, ok := pair[1].(string)
	if !ok {
		return 0, false, invalid
	}
	value, ok = readEncodedValue(encoded)
	if !ok {
```

with

```go
	value, ok = sampleValue(pair[1])
	if !ok {
```

and do the same in `parsePoint` of `range.go` (`value, ok := sampleValue(pair[1])`). Update both functions' doc comments: "encoded is a JSON string … or a JSON number".

- [ ] **Step 4: Run the package**

Run: `go test ./internal/prom/ ./internal/collect/ ./internal/analysis/ ./internal/cli/`
Expected: PASS; the scenario and parity tests are unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/prom/result.go internal/prom/range.go internal/prom/range_test.go internal/prom/result_test.go
git commit -m "fix(prom): read a sample value written as a number"
```

---

### Task 2: An answer is decoded one series at a time (F-162)

Today `decodeAPIResponse` unmarshals the whole body into `interface{}`, and `parseVector` and `parseMatrix` walk that tree. A 32 MiB body grew the heap by 983 MiB (`.cache/phase-5/review/area-6.md`, A6-1). The owner chose option A: decode into typed pieces, keep the 256 MiB cap, keep every refusal and its order.

The design: `decodeAPIResponse` unmarshals the body into `map[string]json.RawMessage`, checks `status` and `errorType` as today, and returns the `data` member as a `json.RawMessage` that is a JSON object. `parseVector` and `parseMatrix` take that `json.RawMessage`, unmarshal it into `map[string]json.RawMessage`, read `resultType` into an `interface{}` (a small value), unmarshal `result` into `[]json.RawMessage`, and then, for each element in order, unmarshal that one element into `interface{}` and run today's per-item code on it unchanged (`parseLabels`, `parseSampleValue`, `parsePoint`). Only one series' tree exists at a time. `encoding/json` validates the whole input before it decodes, so a syntax error anywhere is still found first.

**Files:**
- Create: `internal/prom/decode_test.go`
- Modify: `internal/prom/client.go` (`decodeAPIResponse`, its callers in `runQuery`, `Connect`, `Query`)
- Modify: `internal/prom/result.go` (`parseVector`), `internal/prom/range.go` (`parseMatrix`, `QueryRange`)

**Interfaces:**
- Consumes: `sampleValue` from Task 1 (through `parseSampleValue` and `parsePoint`).
- Produces: `func decodeAPIResponse(body []byte, answered bool) (json.RawMessage, error)`; `func parseVector(data json.RawMessage) ([]Sample, error)`; `func parseMatrix(data json.RawMessage) ([]Series, error)`; `runQuery` returns `(json.RawMessage, error)`.

- [ ] **Step 1: Write the shape table against today's code**

Create `internal/prom/decode_test.go` with a table test `TestAnswerShapesKeepTheirOutcome`. Each case is a body, a parser (`"vector"` or `"matrix"`), and the expected outcome: either a `Kind` plus an exact message, or success with an exact result. The test calls `decodeAPIResponse([]byte(body), true)` then the parser, the way `Query` and `QueryRange` do, and compares. Cases, each with the outcome today's code gives (run them to read the outcome before you write the expected value; a case whose outcome surprises you is reported, not adjusted):

| Name | Body | Parser |
|---|---|---|
| not JSON | `{"status":` | vector |
| truncated tail of a long answer | `{"status":"success","data":{"resultType":"vector","result":[` + 10 000 copies of `{"metric":{"a":"b"},"value":[1,"1"]},` + `{"metric":` | vector |
| array, not object | `[]` | vector |
| no status | `{"data":{}}` | vector |
| status error with errorType | `{"status":"error","errorType":"timeout"}` | vector |
| status error without errorType | `{"status":"error"}` | vector |
| status a number | `{"status":1,"data":{}}` | vector |
| data null | `{"status":"success","data":null}` | vector |
| data an array | `{"status":"success","data":[]}` | vector |
| data absent | `{"status":"success"}` | vector |
| result null | `{"status":"success","data":{"resultType":"vector","result":null}}` | vector |
| result absent | `{"status":"success","data":{"resultType":"vector"}}` | vector |
| result an object | `{"status":"success","data":{"resultType":"vector","result":{}}}` | vector |
| resultType a number | `{"status":"success","data":{"resultType":3,"result":[]}}` | vector |
| matrix asked of a vector | `{"status":"success","data":{"resultType":"matrix","result":[]}}` | vector |
| item not an object | `{"status":"success","data":{"resultType":"vector","result":[1]}}` | vector |
| metric null | `{"status":"success","data":{"resultType":"vector","result":[{"metric":null,"value":[1,"2"]}]}}` | vector |
| metric with a number label | `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"a":1},"value":[1,"2"]}]}}` | vector |
| repeated key, last wins | `{"status":"error","status":"success","data":{"resultType":"vector","result":[]}}` | vector |
| repeated result, last wins | `{"status":"success","data":{"resultType":"vector","result":[1],"result":[]}}` | vector |
| matrix without values | `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{}}]}}` | matrix |
| matrix values null | `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":null}]}}` | matrix |
| matrix good | `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"pod":"a"},"values":[[1,"1"],[2,"NaN"]]}]}}` | matrix |
| vector good | `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"pod":"a"},"value":[1,"+Inf"]}]}}` | vector |

Compare a `*Error` by `Kind` and `Message` (use `errors.As`); compare a success by `reflect.DeepEqual` on the slice. A `NaN` value cannot be compared with `DeepEqual`; for the two good cases compare `Valid` and, for valid points, `Value`.

- [ ] **Step 2: Write the memory bound**

In the same file:

```go
// TestAnswerMemoryStaysBounded decodes a 32 MiB instant answer and samples
// the live heap while it does: the peak growth stays a small multiple of the
// body, where a whole-answer tree grew to about thirty times it.
func TestAnswerMemoryStaysBounded(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"status":"success","data":{"resultType":"vector","result":[`)
	for i := 0; b.Len() < 32<<20; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"metric":{"a":"b"},"value":[1,"1"]}`)
	}
	b.WriteString(`]}}`)
	body := []byte(b.String())

	defer debug.SetGCPercent(debug.SetGCPercent(10))
	runtime.GC()
	sample := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(sample)
	base := sample[0].Value.Uint64()

	var peak atomic.Uint64
	done := make(chan struct{})
	go func() {
		s := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
		for {
			select {
			case <-done:
				return
			default:
			}
			metrics.Read(s)
			if v := s[0].Value.Uint64(); v > peak.Load() {
				peak.Store(v)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	data, err := decodeAPIResponse(body, true)
	var samples []Sample
	if err == nil {
		samples, err = parseVector(data)
	}
	close(done)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	metrics.Read(sample)
	kept := sample[0].Value.Uint64()
	runtime.KeepAlive(samples)
	if kept >= peak.Load() {
		t.Fatalf("the sampler saw no point above what the result keeps (%d MiB); nothing was measured", kept>>20)
	}
	overhead := peak.Load() - kept
	t.Logf("body %d MiB, peak heap %d MiB above the start, %d MiB above what the result keeps, samples %d",
		len(body)>>20, (peak.Load()-base)>>20, overhead>>20, len(samples))
	if overhead > 6*uint64(len(body)) {
		t.Fatalf("decoding held %d MiB beyond its result for a body of %d MiB", overhead>>20, len(body)>>20)
	}
}
```

The bound is on the overhead: the peak minus what the result itself keeps once decoding is over. For this answer the samples alone are several times the body (a labels map each), and that part is the same before and after the change; the whole-answer tree is what the overhead measures. The sampler can only miss a peak, never invent one, so the test cannot fail by chance on code that stays under the bound.

- [ ] **Step 3: Run both against today's code**

Run: `go test ./internal/prom/ -run 'AnswerShapes|AnswerMemory' -v`
Expected: `TestAnswerShapesKeepTheirOutcome` PASSES (it records today's behaviour); `TestAnswerMemoryStaysBounded` FAILS with an overhead far above 6 times the body (the review measured 983 MiB of growth for 32 MiB). Write both logged numbers into the task report. If the shape test does not pass on today's code, fix the expected values from what today's code answers, not the reverse.

- [ ] **Step 4: Decode the envelope into raw members**

In `client.go`, replace `decodeAPIResponse` with:

```go
// decodeAPIResponse reads the JSON body of a 2xx answer through the
// "status" and "data" checks shared by every query, and returns the "data"
// object undecoded: the parsers decode it one series at a time, so no
// whole-answer tree is ever held in memory.
func decodeAPIResponse(body []byte, answered bool) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil || obj == nil {
		return nil, endpointError("metrics endpoint did not answer the Prometheus API", answered)
	}
	statusRaw, hasStatus := obj["status"]
	if !hasStatus {
		return nil, endpointError("metrics endpoint did not answer the Prometheus API", answered)
	}
	var status interface{}
	_ = json.Unmarshal(statusRaw, &status)
	if s, _ := status.(string); s != "success" {
		errType := "unknown"
		var et interface{}
		_ = json.Unmarshal(obj["errorType"], &et)
		if s, ok := et.(string); ok && s != "" {
			errType = s
		}
		return nil, &Error{Kind: KindQuery, Message: fmt.Sprintf("Prometheus query failed with type %s", errType)}
	}
	data := obj["data"]
	if !isJSONObject(data) {
		return nil, &Error{Kind: KindQuery, Message: "Prometheus response has no data object"}
	}
	return data, nil
}

// isJSONObject reports whether raw, already validated JSON, is an object.
func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	return len(trimmed) > 0 && trimmed[0] == '{'
}
```

`json.Unmarshal` of `[]` into a map fails and of `null` leaves the map nil; both are refused as today. `json.Unmarshal` of a nil `RawMessage` (an absent `errorType`) fails and leaves `et` nil, which reads as `unknown`, as today. Change `runQuery`'s result type to `json.RawMessage` and leave its body otherwise as it is.

- [ ] **Step 5: Walk the result one element at a time**

In `result.go`, add the shared reader:

```go
// resultOf reads the "resultType" and "result" members of a data object.
// result is nil when the member is absent or null; ok is false when it is
// present and not an array.
func resultOf(data json.RawMessage) (resultType interface{}, result []json.RawMessage, ok bool) {
	var members map[string]json.RawMessage
	_ = json.Unmarshal(data, &members)
	_ = json.Unmarshal(members["resultType"], &resultType)
	raw, present := members["result"]
	if !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return resultType, nil, true
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return resultType, nil, false
	}
	return resultType, result, true
}
```

Rewrite the head of `parseVector(data json.RawMessage)`:

```go
	resultTypeRaw, resultList, ok := resultOf(data)
	if !ok {
		return nil, unsupportedResultType(resultTypeRaw)
	}
	resultType, _ := resultTypeRaw.(string)
	if resultType != "vector" {
		return nil, unsupportedResultType(resultTypeRaw)
	}

	samples := make([]Sample, 0, len(resultList))
	for i, element := range resultList {
		var itemRaw interface{}
		_ = json.Unmarshal(element, &itemRaw)
		resultList[i] = nil
		item, ok := itemRaw.(map[string]interface{})
```

and keep the rest of the loop body as it is. `resultList[i] = nil` lets each element's bytes go once it is read. Do the same in `parseMatrix(data json.RawMessage)` with `"matrix"`. Update `Connect`, `Query`, and `QueryRange` for the new types (they pass `data` through unchanged). Update the doc comments of both parsers: "the data object, undecoded, of a successful answer".

- [ ] **Step 6: Run the package and the scenarios**

Run: `go test ./internal/prom/ -v -run 'AnswerShapes|AnswerMemory'` then `go test ./internal/prom/ ./internal/collect/ ./internal/analysis/ ./internal/cli/ ./internal/pipeline/`
Expected: both new tests PASS (write both logged numbers into the task report; an overhead above 4 times the body means the bound has too little margin: stop and report before going on); every other test PASSES unchanged.

- [ ] **Step 7: Record the control**

In `docs/security.md`, in the review table, change the row "An answer of up to 256 MiB grows about 30 times in memory when decoded" to `fixed` in the outcome column, its last cell reading "decoded one series at a time; peak growth measured at N times the body" with the measured N. In the controls table, add the row:

`| An answer of at most 256 MiB, decoded one series at a time | Memory exhaustion by a large or hostile answer | internal/prom/client.go |`

- [ ] **Step 8: Commit**

```bash
git add internal/prom/client.go internal/prom/result.go internal/prom/range.go internal/prom/decode_test.go docs/security.md
git commit -m "fix(prom): decode an answer one series at a time"
```

---

### Task 3: Values too large to compute with fail with their own code (F-163)

Two finite samples near the `float64` maximum make an average `+Inf`; `snapshot.Encode` then fails inside `encoding/json`, and the report is stored as `INTERNAL_FAILURE` after its retries. `arith.Mean` stays untouched.

**Files:**
- Modify: `internal/snapshot/codec.go` (`Encode`, new `ErrNotFinite`), `internal/snapshot/codec_test.go`
- Modify: `internal/platform/worker/collect.go` (`codeSourceValuesOutOfRange`, `classify`), its test of `classify` (find it with `grep -n 'classify(' internal/platform/worker/*_test.go`; if none exists, add `classify_test.go`)
- Modify: `api/openapi.yaml` (`ReportFailureCode`), then `make generate`
- Modify: `web/src/locales/en.json`, `web/src/locales/pt-BR.json`
- Modify: `docs/operations.md` (failure table)

**Interfaces:**
- Produces: `var ErrNotFinite = errors.New("snapshot holds a number that is not finite")` in `internal/snapshot`; `Encode` wraps `*json.UnsupportedValueError` with it (`errors.Is(err, snapshot.ErrNotFinite)` is true). Worker code `"SOURCE_VALUES_OUT_OF_RANGE"`, not retried.

- [ ] **Step 1: Write the failing tests**

In `codec_test.go`:

```go
func TestEncodeNamesANumberThatIsNotFinite(t *testing.T) {
	s := validSnapshotWithRows(t)
	inf := math.Inf(1)
	s.Namespaces[0].Resources.CPU.Usage.Average = &inf
	_, err := Encode(s)
	if !errors.Is(err, ErrNotFinite) {
		t.Fatalf("err = %v, want ErrNotFinite", err)
	}
	if _, err := Encode(validSnapshotWithRows(t)); err != nil {
		t.Fatalf("a finite snapshot: %v", err)
	}
}
```

`validSnapshotWithRows` is the builder at `codec_test.go:1216`. The field path above is a guess at the shape of `snapshot.go` (an `Average *float64` sits at line 121): use the real path to one `*float64` average of the first namespace row. The point is one `+Inf` anywhere in the encoded tree.

In the worker test of `classify`:

```go
func TestClassifyValuesOutOfRange(t *testing.T) {
	err := fmt.Errorf("encode: %w", fmt.Errorf("%w: %w", snapshot.ErrNotFinite, errors.New("json: unsupported value: +Inf")))
	code, stop := classify(context.Background(), err)
	if code != "SOURCE_VALUES_OUT_OF_RANGE" || stop != nil {
		t.Fatalf("classify = %q, %v", code, stop)
	}
	if retried(code) {
		t.Fatal("SOURCE_VALUES_OUT_OF_RANGE is retried")
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/snapshot/ ./internal/platform/worker/ -run 'NotFinite|OutOfRange' -v`
Expected: FAIL (undefined `ErrNotFinite`).

- [ ] **Step 3: Name the error and map it**

In `codec.go`:

```go
// ErrNotFinite marks a snapshot that cannot be written because one of its
// numbers is NaN or infinite, as a source's values near the float64 maximum
// make an average or a sum.
var ErrNotFinite = errors.New("snapshot holds a number that is not finite")
```

and in `Encode`:

```go
	if err := enc.Encode(s); err != nil {
		var unsupported *json.UnsupportedValueError
		if errors.As(err, &unsupported) {
			return nil, fmt.Errorf("%w: %w", ErrNotFinite, err)
		}
		return nil, err
	}
```

In `collect.go`, add `codeSourceValuesOutOfRange = "SOURCE_VALUES_OUT_OF_RANGE"` to the code block, and in `classify`, after the `errSnapshotTooLarge` check:

```go
	if errors.Is(err, snapshot.ErrNotFinite) {
		return codeSourceValuesOutOfRange, nil
	}
```

- [ ] **Step 4: Add the code to the contract, the guide, and the locales**

- `api/openapi.yaml`, `ReportFailureCode.enum`: add `SOURCE_VALUES_OUT_OF_RANGE` after `CAPABILITIES_INSUFFICIENT`.
- `docs/operations.md`, failure table, after `CAPABILITIES_INSUFFICIENT`:
  `| `SOURCE_VALUES_OUT_OF_RANGE` | report | no | The source sent values too large to compute with, such as samples near the largest number a report can hold |`
- `web/src/locales/en.json`, `reportFailure`: `"SOURCE_VALUES_OUT_OF_RANGE": "The metric source sent values too large to compute with."`
- `web/src/locales/pt-BR.json`, `reportFailure`: `"SOURCE_VALUES_OUT_OF_RANGE": "A fonte de métricas enviou valores grandes demais para o cálculo."`
- Run `make generate`.

- [ ] **Step 5: Run the suites**

Run: `make generate-check && go test ./internal/snapshot/ ./internal/platform/... ./internal/cli/ && cd web && npm test`
Expected: PASS, including the contract and guide drift tests and the locale tests. A drift test that fails names the table still missing the code; add it there.

- [ ] **Step 6: Record the control**

In `docs/security.md`, review table: the row on values near the `float64` maximum (its words are "+Inf" or "float64"; find it with `grep -n 'float64\|Inf' docs/security.md`) reads `fixed`, last cell "a code of its own, `SOURCE_VALUES_OUT_OF_RANGE`, not retried".

- [ ] **Step 7: Commit**

```bash
git add internal/snapshot/codec.go internal/snapshot/codec_test.go internal/platform/worker/ api/ web/src/api/schema.gen.ts web/src/locales/en.json web/src/locales/pt-BR.json docs/operations.md docs/security.md
git commit -m "feat(worker): name values too large to compute"
```

Name the generated Go file among the paths too (`git status` shows it).

---

### Task 4: A DNS failure is tried again (F-169)

**Files:**
- Modify: `internal/platform/worker/collect.go` (`retried`), `internal/platform/worker/size_test.go` (`TestRetriedCodes`)
- Modify: `docs/operations.md` (the row of `ENDPOINT_RESOLUTION_FAILED`, and any sentence listing the retried codes: `grep -n 'tried again\|retried' docs/operations.md`)
- Modify: `AGENTS.md` ("Only an unreachable source and an internal failure are retried")

**Interfaces:**
- Consumes: `netpolicy.CodeResolutionFailed` (`"ENDPOINT_RESOLUTION_FAILED"`).

- [ ] **Step 1: Write the failing test**

In `TestRetriedCodes`, add `"ENDPOINT_RESOLUTION_FAILED": true` and `"SOURCE_VALUES_OUT_OF_RANGE": false` to the map.

Then find the worker test that runs a collect job to a stored failure with retries (`grep -n 'job_max_attempts\|MaxAttempts\|ENDPOINT_UNREACHABLE' internal/platform/worker/*_test.go`) and add, beside the unreachable case, a case whose profile names a host that the test's resolver refuses (the tests inject `Resolve`; use a resolver that returns an error), expecting the report to end `failed` with `ENDPOINT_RESOLUTION_FAILED` after exactly `job_max_attempts` attempts. That test carries the `postgres` tag; the main thread gives you `KCR_TEST_DATABASE_DSN`.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/platform/worker/ -run TestRetriedCodes -v` and `go test -tags postgres -count=1 ./internal/platform/worker/ -run <the new test> -v`
Expected: FAIL (`retried(ENDPOINT_RESOLUTION_FAILED) = false`; the report fails after one attempt).

- [ ] **Step 3: Retry it**

```go
// retried reports whether River tries a failure with this code again while
// attempts remain: a source that did not answer, a name that did not
// resolve, and an internal failure.
func retried(code string) bool {
	return code == profiles.CodeUnreachable || code == string(netpolicy.CodeResolutionFailed) || code == codeInternalFailure
}
```

- [ ] **Step 4: Update the guide and AGENTS.md**

`docs/operations.md`: the `ENDPOINT_RESOLUTION_FAILED` row's retried column reads `yes`. `AGENTS.md`, "The worker and the reports": "Only an unreachable source, a name that did not resolve, and an internal failure are retried, up to `job_max_attempts`; a deadline and every other code are stored at once."

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/platform/... && go test -tags postgres -count=1 ./internal/platform/worker/`
Expected: PASS.

```bash
git add internal/platform/worker/collect.go internal/platform/worker/ docs/operations.md AGENTS.md
git commit -m "fix(worker): retry a name that did not resolve"
```

---

### Task 5: A second signal ends a stop (F-133)

`signal.NotifyContext`'s stop runs only after `cli.Run` returns, so every signal after the first is swallowed while `serve` waits 30 seconds or `worker` drains. The count in `serve`'s warning may include idle connections that `Shutdown` closed without a `StateClosed` call. `Close`'s error path cannot be reached once `Shutdown` has closed the listeners.

**Files:**
- Create: `internal/cli/signal.go`, `internal/cli/signal_test.go`
- Modify: `cmd/k8s-capacity-report/main.go`
- Modify: `internal/cli/serve.go` (`connCount`, `stopServer`), `internal/cli/serve_stop_test.go`
- Modify: `docs/operations.md` (the section on stopping `serve` and `worker`: `grep -n 'SIGTERM\|stop' docs/operations.md`), `AGENTS.md` ("`serve`'s stop is bounded", "The `worker` stop is bounded")

**Interfaces:**
- Produces: `func SignalContext(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc)` in `internal/cli`: the context ends at the first signal, and from then on the signals have their default action again, so a second one ends the process.

- [ ] **Step 1: Write the failing subprocess test**

```go
package cli

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestSecondSignalEndsTheProcess runs this test binary again as a child that
// waits in SignalContext and then ignores its context for ten seconds, as a
// long stop does; the first SIGTERM ends the context, the second the child.
func TestSecondSignalEndsTheProcess(t *testing.T) {
	if os.Getenv("KCR_SIGNAL_CHILD") == "1" {
		ctx, stop := SignalContext(context.Background(), syscall.SIGTERM)
		defer stop()
		os.Stdout.WriteString("ready\n")
		<-ctx.Done()
		os.Stdout.WriteString("stopping\n")
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestSecondSignalEndsTheProcess$")
	cmd.Env = append(os.Environ(), "KCR_SIGNAL_CHILD=1")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewScanner(out)
	expect := func(want string) {
		t.Helper()
		if !lines.Scan() || lines.Text() != want {
			t.Fatalf("child said %q, want %q", lines.Text(), want)
		}
	}
	expect("ready")
	_ = cmd.Process.Signal(syscall.SIGTERM)
	expect("stopping")
	begun := time.Now()
	_ = cmd.Process.Signal(syscall.SIGTERM)
	err = cmd.Wait()
	if took := time.Since(begun); took > 5*time.Second {
		t.Fatalf("the child took %v after the second signal", took)
	}
	status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGTERM {
		t.Fatalf("child ended with %v, want death by SIGTERM", err)
	}
}
```

Add `"bufio"` to the imports. `os.Stdout.WriteString` in a test child is not a logger call; if the logging guard of `internal/cli` refuses it, write through `fmt.Fprintln(os.Stdout, …)` and read the guard's message for what it allows in a `_test.go` file.

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/cli/ -run TestSecondSignalEndsTheProcess -v`
Expected: FAIL (undefined `SignalContext`).

- [ ] **Step 3: Write the signal context**

`internal/cli/signal.go`:

```go
package cli

import (
	"context"
	"os"
	"os/signal"
)

// SignalContext returns a context that ends at the first of signals. From
// then on the signals have their default action again, so a second one ends
// the process while a command is still stopping.
func SignalContext(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(parent, signals...)
	go func() {
		<-ctx.Done()
		stop()
	}()
	return ctx, stop
}
```

`main.go`:

```go
func main() {
	ctx, stop := cli.SignalContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Run(ctx, cli.RealEnv(), os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
```

Drop the `os/signal` import of `main.go`.

- [ ] **Step 4: Count only connections in use**

Write the failing test first, in `serve_stop_test.go`: a server whose handler stalls (reuse `stalledServer`) plus a second client that made one finished request on a keep-alive connection and stays idle; after `stopServer` with 200 ms, the log reads `connections closed before they finished: 1`, not 2. In `TestStopClosesAStalledDownload`, also require `took >= 200*time.Millisecond` (the timeout was waited for) beside the existing ceiling.

Then change `connCount` to keep each connection's last state (`map[net.Conn]http.ConnState`), delete on `StateClosed` and `StateHijacked`, and make `count` return the connections whose state is `StateActive`. Rename `count` to `active` and update its doc comment ("the connections serving a request"). In `stopServer`, replace

```go
	if err := httpServer.Close(); err != nil {
		return fmt.Errorf("cannot stop the server: %w", err)
	}
```

with

```go
	// Shutdown has closed the listeners already, so Close only ends the
	// connections still serving a request.
	_ = httpServer.Close()
```

and read `open := conns.active()` as before. The doc comment of `stopServer` keeps "It fails only when the listener cannot be closed", which `Shutdown`'s non-deadline error covers.

- [ ] **Step 5: Run the package**

Run: `go test -count=1 ./internal/cli/`
Expected: PASS.

- [ ] **Step 6: Update the guide and AGENTS.md**

`docs/operations.md`, where it says how `serve` and `worker` stop: add "A second interrupt or `SIGTERM` during the stop ends the process at once; jobs it held return to the queue with their attempt restored when the worker starts again." Check the second half against River's behaviour as `AGENTS.md` states it ("cancels the rest (River returns them with the attempt restored)"); a killed process does not cancel, so say instead what happens to a running job of a killed worker: it is rescued by River after its timeout, and if that was its last attempt the maintenance job stores `JOB_ATTEMPTS_EXHAUSTED`. Write the sentence that is true, and name in the task report which one you wrote.

`AGENTS.md`: in "`serve`'s stop is bounded" add "A second signal ends the process at once." and in "The `worker` stop is bounded" the same.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/signal.go internal/cli/signal_test.go cmd/k8s-capacity-report/main.go internal/cli/serve.go internal/cli/serve_stop_test.go docs/operations.md AGENTS.md
git commit -m "fix(cli): end the stop at a second signal"
```

---

### Task 6: A cross-site request cannot rotate the session token (F-159)

`GET /api/v1/auth/session` rotates the CSRF token of a signed-in caller, and a cross-site top-level navigation carries the `SameSite=Lax` cookie. The route refuses a request whose `Sec-Fetch-Site` header is present and is neither `same-origin` nor `none` (`none` is the reader typing the address). A request without the header is answered as today.

**Files:**
- Modify: `internal/platform/api/auth.go` (`getSession`)
- Test: the test file of `getSession` (`grep -ln 'getSession\|/auth/session' internal/platform/api/*_test.go`; its tests carry the `postgres` tag if they reach the store)
- Modify: `api/openapi.yaml` (`/auth/session` `get`: a `403` answer), then `make generate`
- Modify: `docs/security.md`, `AGENTS.md`

**Interfaces:**
- Consumes: `CodeCSRFRejected` and `WriteProblem` of `internal/platform/api` (read `errors.go` for the exact name of the CSRF code constant).

- [ ] **Step 1: Write the failing tests**

Three cases against the server the existing session tests build, each signed in:
1. `Sec-Fetch-Site: cross-site` → 403, code `CSRF_REJECTED`, no `Set-Cookie` header, and the session's CSRF token is unchanged (a following request with the old token to a state-changing route the tests already use passes the CSRF check).
2. `Sec-Fetch-Site: same-site` → 403, as 1.
3. No `Sec-Fetch-Site` → 200 with a new `csrf_token`, as today; and `same-origin` and `none` → 200.

And one anonymous case: `Sec-Fetch-Site: cross-site` with no cookie → 403 and no `kcr_login_csrf` cookie set.

- [ ] **Step 2: Run them to see them fail**

Run: `go test -tags postgres -count=1 ./internal/platform/api/ -run Session -v`
Expected: the cross-site and same-site cases FAIL (200).

- [ ] **Step 3: Refuse a cross-site request**

At the top of `getSession`:

```go
	// A request from another site, such as a top-level navigation that
	// carries the Lax cookie, must not rotate the token of an open page.
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		WriteProblem(w, &Problem{Code: CodeCSRFRejected})
		return
	}
```

Use the constant's real name.

- [ ] **Step 4: Update the contract**

In `api/openapi.yaml`, under `/auth/session` `get` `responses`, add a `"403"` answer that references the error envelope the other 403 answers reference (copy the form used by a CSRF-checked route), with the description "A request from another site (`Sec-Fetch-Site` neither `same-origin` nor `none`); nothing is rotated." Run `make generate`, then `make generate-check`, and the route-table test (`go test ./internal/platform/api/ -run Contract`).

- [ ] **Step 5: Record the control and the rule**

`docs/security.md`: the review row "A cross-site navigation to the session route rotates the CSRF token" reads `fixed`, last cell "the route refuses a request from another site"; the CSRF control row adds "; the session route refuses a request from another site". `AGENTS.md`, under "The checks of a route run in one order", add: "`GET /auth/session` first refuses a request whose `Sec-Fetch-Site` is present and neither `same-origin` nor `none`, with 403 `CSRF_REJECTED`."

- [ ] **Step 6: Run and commit**

Run: `make generate-check && go test ./internal/platform/api/ && go test -tags postgres -count=1 ./internal/platform/api/ && cd web && npm test`
Expected: PASS.

```bash
git add internal/platform/api/auth.go internal/platform/api/ api/openapi.yaml web/src/api/schema.gen.ts docs/security.md AGENTS.md
git commit -m "fix(api): refuse a cross-site session read"
```

Name the generated Go file too.

---

### Task 7: A body is judged before an id (F-160)

`AGENTS.md` orders the checks: session, role, CSRF, body or query (400), then the service, which judges the id. `patchUser` and `patchProfile` judge the id first. `POST /admin/profiles/{id}/test` has no body and stays as it is; the report routes already judge the body first.

**Files:**
- Modify: `internal/platform/api/users.go` (`patchUser`), `internal/platform/api/profiles.go` (`patchProfile`)
- Test: their test files (`grep -ln 'patchUser\|PATCH.*admin/users\|PATCH.*admin/profiles' internal/platform/api/*_test.go`)

- [ ] **Step 1: Write the failing tests**

For each route, a PATCH with a malformed id (`not-a-uuid` for a user, `Bad_Id` for a profile) and a body with an unknown key (`{"nope":1}`), as an administrator with a valid CSRF token, answers 400 `REQUEST_INVALID` with the field the body check names, not 404. A PATCH with a malformed id and a valid body still answers 404 `USER_NOT_FOUND` or `PROFILE_NOT_FOUND`.

- [ ] **Step 2: Run them to see them fail**

Run: `go test -tags postgres -count=1 ./internal/platform/api/ -run 'Patch' -v` (or without the tag if the tests do not reach the store)
Expected: the malformed-id-and-body cases FAIL with 404.

- [ ] **Step 3: Move the id check after the body**

In `patchUser`, move

```go
	id, ok := userID(w, r)
	if !ok {
		return
	}
```

below the `DecodeBody` block. In `patchProfile`, move the `profileID` block below the `json.Unmarshal(data, &fields)` block, just before `delete(fields, "base_revision_id")`.

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/platform/api/ && go test -tags postgres -count=1 ./internal/platform/api/`
Expected: PASS.

```bash
git add internal/platform/api/users.go internal/platform/api/profiles.go internal/platform/api/
git commit -m "fix(api): judge a body before its id"
```

---

### Task 8: The password writers wait for the table lock (F-168)

F-168 says `Users.Update` assumes its table lock excludes the other password writers (the sign-in rehash, `Account.Change`, `ResetPassword`), which do not take it. They do not need to: `LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE` conflicts with the `ROW EXCLUSIVE` lock every `UPDATE` takes, so a writer that started first is waited for and one that starts later waits, and under `READ COMMITTED` the row `Update` reads after the lock is the latest. The compare-and-set therefore cannot miss. This task proves it with a test and makes the comment say why; it adds no lock.

**Files:**
- Create: `internal/platform/auth/users_lock_test.go` (tag `postgres`)
- Modify: `internal/platform/auth/users.go` (the comment above `UpdatePasswordHash`, lines 220-227)

**Interfaces:**
- Consumes: `store.Transact`, `store.LockUsers`, `store.UpdatePasswordHash`, `storetest.New(t)`, and `newUser(t, pool, name, password, role, language) store.User` of `internal/platform/auth/session_test.go:38` (same package; its `session_test.go` carries the `postgres` tag, so this file does too).

- [ ] **Step 1: Write the proof**

```go
//go:build postgres

package auth

// TestLockHoldsOffAPasswordWriter holds the users table lock in one
// transaction and starts a password write outside it, as the sign-in rehash
// does: the write waits until the lock is released, so a transaction that
// read a hash under the lock writes over that same hash.
func TestLockHoldsOffAPasswordWriter(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	user := newUser(t, pool, "lock-user", "lock-user-password", string(RoleOperator), "en")
	locked := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- store.Transact(ctx, pool, func(tx pgx.Tx) error {
			if err := store.LockUsers(ctx, tx); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked
	wrote := make(chan bool, 1)
	go func() {
		ok, err := store.UpdatePasswordHash(ctx, pool, user.ID, user.PasswordHash, "other", time.Now())
		if err != nil {
			t.Error(err)
		}
		wrote <- ok
	}()
	select {
	case <-wrote:
		t.Fatal("the password write did not wait for the table lock")
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if ok := <-wrote; !ok {
		t.Fatal("the password write missed after the lock was released")
	}
}
```

The 300 ms window can only make the test pass wrongly if the write is slow for another reason, never fail wrongly; the assertion that matters is that the write did not finish while the lock was held.

- [ ] **Step 2: Run it**

Run (the main thread gives the DSN): `KCR_TEST_DATABASE_DSN=<dsn> go test -tags postgres -count=1 ./internal/platform/auth/ -run TestLockHoldsOffAPasswordWriter -v`
Expected: PASS. If it FAILS, stop and report: the finding is then real, and the main thread rules the fix.

- [ ] **Step 3: Prove the test can fail**

Temporarily replace `store.LockUsers(ctx, tx)` in the test with a no-op (`tx.Exec(ctx, "SELECT 1")`), run it, and see it FAIL with "the password write did not wait for the table lock". Restore the lock. Write both outcomes in the task report.

- [ ] **Step 4: Make the comment say why**

Replace the comment above `UpdatePasswordHash` in `Users.Update` with:

```go
			// Written over the hash that was read. The table lock conflicts
			// with the row lock of every UPDATE, so a password writer that
			// started first has committed and one that starts later waits;
			// a miss is a broken invariant.
```

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/platform/auth/ && KCR_TEST_DATABASE_DSN=<dsn> go test -tags postgres -count=1 ./internal/platform/auth/`
Expected: PASS.

```bash
git add internal/platform/auth/users.go internal/platform/auth/users_lock_test.go
git commit -m "test(auth): prove the lock holds off writers"
```

---

### Task 9: Records and gate (main thread)

- [ ] **Step 1:** Run `make update-testdata` and confirm `git status` shows no change under `testdata/`, `template/tests/capacity/fixtures/command/`, or `web/tests/fixtures/view/`.
- [ ] **Step 2:** `CHANGELOG.md`, under `## [Unreleased]`:

```markdown
### Changed

- A report whose metric source's host name does not resolve is tried again, as a source that does not answer is.
- A second interrupt ends `serve`, `worker`, or a command at once while it is stopping.

### Fixed

- A metric source that writes sample values as numbers is read, as the original program reads it.
- A report whose source sends values too large to compute with fails with a reason of its own, no longer as an internal failure.
- A change request with both a wrong id and a wrong body names the body's fault, as every other route does.

### Security

- The worker reads a large answer of a metric source one series at a time, with far less memory.
- Opening the session address from another site no longer changes the token of an open page.
```

State the omission in the closing report: F-168 adds a test and a comment, with no effect for users.

- [ ] **Step 3:** `ROADMAP.md`: each item DONE with its commit; F-168 DONE "not a defect, proven by <commit>"; the phase 9 summary row; any finding met on the way in the findings log.
- [ ] **Step 4:** `make gate` (log under `.cache/phase-9/gate.log`); read the tool's own summary line.
- [ ] **Step 5:** Adversarial review of the whole branch (`git diff main...feat/backend-hardening > .cache/phase-9/branch.diff`), its findings ruled, fixed, and gated again.
- [ ] **Step 6:** Read `ROADMAP.md` and `CHANGELOG.md` back; merge into `main` by fast-forward; version 1.4.0 on the owner's word.
