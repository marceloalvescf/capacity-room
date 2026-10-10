package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// goBases returns the golang base of every FROM line of a Containerfile,
// cut before the stage name.
func goBases(text string) []string {
	var bases []string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "FROM golang:") {
			continue
		}
		base, _, _ := strings.Cut(line, " AS ")
		bases = append(bases, strings.TrimSpace(base))
	}
	return bases
}

// sameGoBase reports an error unless every file, keyed by name, holds at
// least one FROM golang: line and all of those lines are equal.
func sameGoBase(files map[string]string) error {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var first, firstName string
	for _, name := range names {
		bases := goBases(files[name])
		if len(bases) == 0 {
			return fmt.Errorf("%s holds no FROM golang: line", name)
		}
		for _, base := range bases {
			if first == "" {
				first, firstName = base, name
				continue
			}
			if base != first {
				return fmt.Errorf("%s has %q, but %s has %q", name, base, firstName, first)
			}
		}
	}
	return nil
}

func TestContainerfilesShareTheGoBase(t *testing.T) {
	root := moduleRoot(t)
	files := map[string]string{}
	for _, name := range []string{"deploy/Containerfile", "deploy/worker-e2e.Containerfile"} {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		files[name] = string(raw)
	}
	if err := sameGoBase(files); err != nil {
		t.Fatal(err)
	}
}

func TestSameGoBase(t *testing.T) {
	const a = "FROM golang:1.27@sha256:aaaa AS build\nFROM scratch\n"
	const b = "FROM golang:1.27@sha256:bbbb AS switch\n"
	dir := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return text
	}

	if err := sameGoBase(map[string]string{"a": write("a", a), "b": write("b", a)}); err != nil {
		t.Fatalf("equal bases: %v", err)
	}
	if err := sameGoBase(map[string]string{"a": write("a", a), "b": write("b", b)}); err == nil {
		t.Fatal("different digests passed")
	}
	if err := sameGoBase(map[string]string{"a": a, "b": "FROM scratch\n"}); err == nil {
		t.Fatal("a file with no golang line passed")
	}
}
