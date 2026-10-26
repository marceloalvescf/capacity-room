package cli

import (
	"crypto/rand"
	"encoding/hex"
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

// removeProbeFile is the seam TestWritableCheckReportsAProbeItCannotRemove
// replaces to force the clean-up path of checkWritableTarget to fail
// without needing a real read-only-after-create filesystem.
var removeProbeFile = os.Remove

// checkWritableTarget requires path's folder to exist and accept a new
// file, and path itself not to already name a folder. The probe file it
// creates to test writability is always removed, or its own name is
// reported: a probe left behind silently would be a folder that looks
// writable but is quietly filling with debris.
func checkWritableTarget(path string) error {
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("output directory must exist and be writable: %s", dir)
	}
	if pathInfo, err := os.Stat(path); err == nil && pathInfo.IsDir() {
		return fmt.Errorf("output path is a folder: %s", path)
	}

	probe, err := os.CreateTemp(dir, ".capacity-room-write-test-*")
	if err != nil {
		return fmt.Errorf("output directory must exist and be writable: %s", dir)
	}
	name := probe.Name()

	closeErr := probe.Close()
	removeErr := removeProbeFile(name)
	cause := closeErr
	if cause == nil {
		cause = removeErr
	}
	if cause != nil {
		return fmt.Errorf("output directory holds a probe file that could not be removed: %s: %s", name, cause)
	}
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

// defaultOutputName builds the default PDF file name:
// report-resources-<slug><suffix>-<language>-<YYYYMMDD-HHMMSS>.pdf. The suffix
// is -executive for the executive profile. emittedAt must already be in the zone the name is written for.
func defaultOutputName(cluster, profile, language string, emittedAt time.Time) string {
	suffix := ""
	if profile == "executive" {
		suffix = "-executive"
	}
	return fmt.Sprintf(
		"report-resources-%s%s-%s-%s.pdf",
		slug(cluster), suffix, strings.ToLower(language), emittedAt.Format("20060102-150405"),
	)
}

// randomHex returns n random bytes from crypto/rand, hex-encoded: the
// random characters of a temporary file's name.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// tempSiblingName returns a name of the shape ".<base>.<random>.tmp" beside
// path, using 16 random bytes from crypto/rand: enough that two calls never
// collide in practice.
func tempSiblingName(path string) (string, error) {
	suffix, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+"."+suffix+".tmp"), nil
}

// createTempSibling creates a new, empty, exclusively-owned file named
// ".<base>.<random>.tmp" beside path, with mode 0o644 less whatever the
// process umask clears: the kernel applies the umask to the mode given to
// O_CREATE, so this package never reads or sets it. It retries under a
// fresh random name on the vanishingly unlikely chance of a collision with
// an existing file.
func createTempSibling(path string) (*os.File, error) {
	for attempt := 0; attempt < 10; attempt++ {
		name, err := tempSiblingName(path)
		if err != nil {
			return nil, err
		}
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return f, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("could not create a temporary file beside %s", path)
}

// writeAtomicFile writes data to a new temporary file beside path, flushes
// it, and renames it into place; os.Rename preserves the temporary file's
// own mode, so the final file keeps the 0o644-less-umask mode
// createTempSibling gave it. The temporary file is removed on any failure.
func writeAtomicFile(path string, data []byte) (err error) {
	f, err := createTempSibling(path)
	if err != nil {
		return err
	}
	tmpPath := f.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
