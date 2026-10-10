package main

import (
	"bytes"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

type execCall struct {
	path string
	argv []string
}

// runWith runs the switch with a stat that answers statErr and an exec that
// records its call and returns an error, as a real exec does on failure.
func runWith(t *testing.T, args []string, statErr error) (int, string, []execCall) {
	t.Helper()
	var stderr bytes.Buffer
	var calls []execCall
	stat := func(name string) (fs.FileInfo, error) {
		if name != "/switch/fail" {
			t.Fatalf("stat of %q", name)
		}
		return nil, statErr
	}
	exec := func(path string, argv []string, _ []string) error {
		calls = append(calls, execCall{path, argv})
		return errors.New("exec recorded")
	}
	code := run(args, &stderr, stat, exec)
	return code, stderr.String(), calls
}

func TestVersionPassesThrough(t *testing.T) {
	code, stderr, calls := runWith(t, []string{"--version"}, nil)
	want := []execCall{{"/usr/local/bin/typst", []string{"/usr/local/bin/typst", "--version"}}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	if code != 1 || !strings.HasPrefix(stderr, "typst-switch: ") {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
}

func TestCompilePassesWhenOff(t *testing.T) {
	args := []string{"compile", "--root", "/r", "a.typ", "b.pdf"}
	_, _, calls := runWith(t, args, fs.ErrNotExist)
	want := []execCall{{"/usr/local/bin/typst", []string{"/usr/local/bin/typst", "compile", "--root", "/r", "a.typ", "b.pdf"}}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestCompileRefusedWhenOn(t *testing.T) {
	code, stderr, calls := runWith(t, []string{"compile", "--root", "/r", "a.typ", "b.pdf"}, nil)
	if len(calls) != 0 {
		t.Fatalf("exec called: %v", calls)
	}
	if code != 1 || stderr != "typst-switch: compile refused by the switch\n" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
}

func TestUnreadableSwitchRefuses(t *testing.T) {
	code, stderr, calls := runWith(t, []string{"compile", "--root", "/r", "a.typ", "b.pdf"}, fs.ErrPermission)
	if len(calls) != 0 {
		t.Fatalf("exec called: %v", calls)
	}
	if code != 1 || !strings.HasPrefix(stderr, "typst-switch: ") {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
}

func TestNoArgumentsPassThrough(t *testing.T) {
	_, _, calls := runWith(t, nil, nil)
	want := []execCall{{"/usr/local/bin/typst", []string{"/usr/local/bin/typst"}}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}
