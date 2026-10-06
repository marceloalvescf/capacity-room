package redact_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"testing/slogtest"

	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

const handlerSecret = `s3cr3t"t0ken\value-123`
const handlerSecretEscaped = `s3cr3t\"t0ken\\value-123`

type noteStruct struct {
	note string
}

type textStringer string

func (s textStringer) String() string { return string(s) }

type probeErr struct{ msg string }

func (e *probeErr) Error() string { return e.msg }

type panicStringer struct{}

func (panicStringer) String() string { panic("boom") }

func newHandlerLogger(out *bytes.Buffer, r *redact.Redactor) *slog.Logger {
	return slog.New(redact.NewHandler(r, slog.NewTextHandler(out, nil)))
}

func assertRedacted(t *testing.T, got string) {
	t.Helper()
	if strings.Contains(got, handlerSecret) || strings.Contains(got, handlerSecretEscaped) || strings.Contains(got, "t0ken") {
		t.Fatalf("the secret reached the line:\n%s", got)
	}
	if !strings.Contains(got, redact.Mark) {
		t.Fatalf("no redaction mark in the line:\n%s", got)
	}
}

func TestHandlerRedactsEveryKind(t *testing.T) {
	kinds := map[string]func() (string, []any){
		"message": func() (string, []any) { return "near " + handlerSecret, nil },
		"string":  func() (string, []any) { return "m", []any{slog.String("k", handlerSecret)} },
		"error":   func() (string, []any) { return "m", []any{"k", errors.New("x " + handlerSecret)} },
		"wrapped": func() (string, []any) { return "m", []any{"k", fmt.Errorf("ctx: %w", errors.New(handlerSecret))} },
		"join": func() (string, []any) {
			return "m", []any{"k", errors.Join(errors.New("a"), errors.New(handlerSecret))}
		},
		"stringer": func() (string, []any) { return "m", []any{"k", textStringer("x " + handlerSecret)} },
		"bytes":    func() (string, []any) { return "m", []any{"k", []byte("x " + handlerSecret)} },
		"map":      func() (string, []any) { return "m", []any{"k", map[string]string{"a": handlerSecret}} },
		"slice":    func() (string, []any) { return "m", []any{"k", []string{"a", handlerSecret}} },
		"struct":   func() (string, []any) { return "m", []any{"k", noteStruct{note: handlerSecret}} },
		"group":    func() (string, []any) { return "m", []any{slog.Group("g", slog.String("k", handlerSecret))} },
	}
	for name, build := range kinds {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			r := redact.New()
			logger := newHandlerLogger(&out, r)
			r.Add(handlerSecret)
			msg, args := build()
			logger.Info(msg, args...)
			assertRedacted(t, out.String())
		})
	}
}

func TestHandlerRedactsBoundBeforeTheSecret(t *testing.T) {
	var out bytes.Buffer
	r := redact.New()
	logger := newHandlerLogger(&out, r).With(slog.String("k", handlerSecret))
	r.Add(handlerSecret)
	logger.Info("m")
	assertRedacted(t, out.String())
}

func TestHandlerRedactsBoundInsideAGroup(t *testing.T) {
	var out bytes.Buffer
	r := redact.New()
	logger := newHandlerLogger(&out, r).WithGroup("g").With("k", handlerSecret)
	r.Add(handlerSecret)
	logger.Info("m")
	assertRedacted(t, out.String())
}

func TestHandlerPassesSlogtest(t *testing.T) {
	var out bytes.Buffer
	h := redact.NewHandler(redact.New(), slog.NewJSONHandler(&out, nil))
	results := func(t *testing.T) []map[string]any {
		var ms []map[string]any
		for _, line := range bytes.Split(out.Bytes(), []byte{'\n'}) {
			if len(line) == 0 {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal(line, &m); err != nil {
				t.Fatal(err)
			}
			ms = append(ms, m)
		}
		return ms
	}
	if err := slogtest.TestHandler(h, func() []map[string]any { return results(t) }); err != nil {
		t.Fatal(err)
	}
}

func TestHandlerReplaysGroupsInOrder(t *testing.T) {
	var got, want bytes.Buffer
	opts := &slog.HandlerOptions{ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		if len(groups) == 0 && a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return a
	}}
	logger := slog.New(redact.NewHandler(redact.New(), slog.NewTextHandler(&got, opts)))
	plain := slog.New(slog.NewTextHandler(&want, opts))
	for _, l := range []*slog.Logger{logger, plain} {
		l.WithGroup("a").With("b", 1).WithGroup("c").Info("m", "d", 2)
	}
	if got.String() != want.String() {
		t.Fatalf("line = %q, want %q", got.String(), want.String())
	}
	if !strings.Contains(got.String(), "a.b=1 a.c.d=2") {
		t.Fatalf("line = %q, want a.b=1 a.c.d=2", got.String())
	}
}

func TestHandlerTypedNilError(t *testing.T) {
	var out bytes.Buffer
	logger := newHandlerLogger(&out, redact.New())
	logger.Info("m", slog.Any("cause", (*probeErr)(nil)))
	if !strings.Contains(out.String(), "cause=<nil>") {
		t.Fatalf("line = %q, want cause=<nil>", out.String())
	}
}

func TestHandlerPanickingStringer(t *testing.T) {
	var out bytes.Buffer
	logger := newHandlerLogger(&out, redact.New())
	logger.Info("m", slog.Any("k", panicStringer{}))
	if !strings.Contains(out.String(), "%!v(PANIC=") {
		t.Fatalf("line = %q, want the fmt panic form", out.String())
	}
}
