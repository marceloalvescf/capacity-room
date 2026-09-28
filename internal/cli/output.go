package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// checkOutputPath refuses a PDF output path that does not end in ".pdf", or
// whose folder is missing, unwritable, or itself named by the path.
func checkOutputPath(path string) error {
	if !strings.HasSuffix(path, ".pdf") {
		return errors.New("output path must end in .pdf")
	}
	return checkWritableTarget(path)
}

// checkSnapshotOutputPath refuses a snapshot output path that does not end
// in ".json", or whose folder is missing, unwritable, or itself named by the
// path.
func checkSnapshotOutputPath(path string) error {
	if !strings.HasSuffix(path, ".json") {
		return errors.New("snapshot path must end in .json")
	}
	return checkWritableTarget(path)
}

// checkWritableTarget requires path's folder to exist and accept a new
// file, and path itself not to already name a folder.
func checkWritableTarget(path string) error {
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("output directory must exist and be writable: %s", dir)
	}
	if pathInfo, err := os.Stat(path); err == nil && pathInfo.IsDir() {
		return fmt.Errorf("output path is a folder: %s", path)
	}

	probe, err := os.CreateTemp(dir, ".kcr-write-test-*")
	if err != nil {
		return fmt.Errorf("output directory must exist and be writable: %s", dir)
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

// slug lowers name and turns every run of characters outside a-z0-9 into one
// "-", trimming a leading or trailing one. An empty result becomes
// "cluster".
func slug(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "cluster"
	}
	return s
}

// defaultOutputName builds the default PDF file name of R4:
// report-resources-<slug><suffix>-<language>-<YYYYMMDD-HHMMSS>.pdf. emittedAt
// must already be in the zone the name is written for.
func defaultOutputName(cluster, minSeverity, language string, emittedAt time.Time) string {
	suffix := ""
	if minSeverity != "all" {
		suffix = "-" + minSeverity
	}
	return fmt.Sprintf(
		"report-resources-%s%s-%s-%s.pdf",
		slug(cluster), suffix, strings.ToLower(language), emittedAt.Format("20060102-150405"),
	)
}
