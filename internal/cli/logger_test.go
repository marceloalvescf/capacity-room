package cli

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

// stringer is a fmt.Stringer whose text holds a secret.
type stringer string

func (s stringer) String() string { return string(s) }

// A text handler escapes a quote, a backslash, and a format character
// before the stream redactor sees the line, so the logger redacts every
// value before it is formatted.
func TestLoggerRedactsBeforeFormatting(t *testing.T) {
	tokens := []string{
		`s3cr3t"t0ken-quote`,
		`s3cr3t\t0ken-backslash`,
		"s3cr3t\u200bt0ken-format",
	}
	for _, token := range tokens {
		t.Run(token, func(t *testing.T) {
			var out bytes.Buffer
			redactor := redact.New()
			logger := newLogger(redactor.Writer(&out), redactor)
			redactor.Add(token)

			logger.Warn("attempt failed near "+token,
				slog.String("value", "near "+token),
				slog.Any("cause", errors.New("bad query near "+token)),
				slog.Any("stringer", stringer("near "+token)),
				slog.Group("outer", slog.String("inner", token)),
			)
			logger.WithGroup("group").Info("again", slog.Any("cause", errors.New(token)))

			got := out.String()
			if strings.Contains(got, "t0ken") {
				t.Fatalf("the token reached the log:\n%s", got)
			}
			if n := strings.Count(got, redact.Mark); n != 6 {
				t.Fatalf("%d redaction marks, want 6:\n%s", n, got)
			}
		})
	}
}

// A command that builds its own handler writes values the run's redactor
// never reads before they are formatted, so only newLogger builds one.
func TestNewLoggerRedactsAnEscapedSecret(t *testing.T) {
	const secret = `s3cr3t"t0ken\value-123`
	const escaped = `s3cr3t\"t0ken\\value-123`
	var out bytes.Buffer
	redactor := redact.New()
	logger := newLogger(redactor.Writer(&out), redactor)
	bound := logger.With("cause", errors.New("bound "+secret))
	redactor.Add(secret)

	logger.Error("m", "cause", errors.New("x "+secret))
	bound.Error("m")

	got := out.String()
	if strings.Contains(got, secret) || strings.Contains(got, escaped) || strings.Contains(got, "t0ken") {
		t.Fatalf("the secret reached the log:\n%s", got)
	}
	if n := strings.Count(got, redact.Mark); n != 2 {
		t.Fatalf("%d redaction marks, want 2:\n%s", n, got)
	}
}

// loggerCalls lists the calls under dir, test files and testdata folders
// left out and the file allowedFile (a path as the walk gives it) excepted,
// that build a logger or a handler of their own: the handlers of log/slog,
// its default logger, and the functions of package log. A handler whose
// writer is io.Discard writes nothing and is let through.
func loggerCalls(t *testing.T, dir, allowedFile string) []string {
	t.Helper()
	var found []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || path == allowedFile {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			name := pkg.Name + "." + sel.Sel.Name
			switch {
			case name == "slog.SetDefault", name == "slog.Default":
			case name == "slog.NewTextHandler", name == "slog.NewJSONHandler":
				if discards(call) {
					return true
				}
			case pkg.Name == "log" && (strings.HasPrefix(sel.Sel.Name, "Print") || strings.HasPrefix(sel.Sel.Name, "Fatal") || strings.HasPrefix(sel.Sel.Name, "Panic")):
			default:
				return true
			}
			found = append(found, fset.Position(call.Pos()).String()+": "+name)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scanning %s: %v", dir, err)
	}
	return found
}

// discards reports whether the first argument of call is io.Discard.
func discards(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	sel, ok := call.Args[0].(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "io" && sel.Sel.Name == "Discard"
}

func TestNoLoggerOutsideTheConstructor(t *testing.T) {
	for _, dir := range []string{".", filepath.Join("..", "platform")} {
		for _, call := range loggerCalls(t, dir, "logger.go") {
			t.Errorf("a logger is built outside newLogger: %s", call)
		}
	}
}

func TestLoggerScanReportsAnOwnHandler(t *testing.T) {
	found := loggerCalls(t, filepath.Join("testdata", "loggerscan"), "logger.go")
	if len(found) != 1 || !strings.HasSuffix(found[0], "own_logger.go:9:18: slog.NewTextHandler") {
		t.Fatalf("scan found %q, want the one call of slog.NewTextHandler", found)
	}
}
