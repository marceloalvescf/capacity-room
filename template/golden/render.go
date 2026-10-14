package golden

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/marceloalvescf/capacity-room/internal/typst"
)

// job builds the typst.Job that renders s from root (a compile root, i.e. a
// copy of the template folder) into dir. It is extracted from Render so the
// rule about Data and Inputs["data"] — see Snapshot.Data's doc comment —
// can be tested without a real toolchain.
func job(root string, s Snapshot, dir string) (typst.Job, error) {
	if s.Data != "" {
		if _, exists := s.Inputs["data"]; exists {
			return typst.Job{}, fmt.Errorf(
				"rendering %s: Data (%q) and an input named \"data\" (%q) are both set",
				s.Name, s.Data, s.Inputs["data"],
			)
		}
	}

	inputs := make(map[string]string, len(s.Inputs)+1)
	for k, v := range s.Inputs {
		inputs[k] = v
	}
	if s.Data != "" {
		inputs["data"] = "/" + s.Data
	}

	pages := make([]string, len(s.Pages))
	for i, page := range s.Pages {
		pages[i] = strconv.Itoa(page)
	}

	return typst.Job{
		Root:     root,
		FontPath: filepath.Join(root, "graphite", "assets", "fonts"),
		Source:   filepath.Join(root, s.Source),
		Output:   filepath.Join(dir, s.Name+"-{p}.png"),
		Format:   typst.PNG,
		Pages:    strings.Join(pages, ","),
		PPI:      PPI,
		Inputs:   inputs,
	}, nil
}

// Render writes the pages of s into dir, one PNG per page, compiling from
// root (a compile root, i.e. a copy of the template folder). It returns an
// error when the compile fails, when standard error is not empty, or when
// a file of s.Files() is missing afterwards, so a caller never mistakes a
// silently incomplete render for success.
func Render(ctx context.Context, tc typst.Toolchain, root string, s Snapshot, dir string) error {
	j, err := job(root, s, dir)
	if err != nil {
		return err
	}

	stderr, err := tc.Compile(ctx, j)
	if err != nil {
		return fmt.Errorf("rendering %s: %w", s.Name, err)
	}
	if stderr != "" {
		return fmt.Errorf("rendering %s: typst reported: %s", s.Name, stderr)
	}

	for _, name := range s.Files() {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("rendering %s: expected page %s: %w", s.Name, name, err)
		}
		if info.IsDir() {
			return fmt.Errorf("rendering %s: expected page %s, found a directory", s.Name, name)
		}
	}
	return nil
}

// Digest returns the SHA-256 of the file at path, in hexadecimal.
func Digest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("digesting %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ReadSums parses a SHA256SUMS file: one "<digest><two spaces><name>" line
// per file, as sha256sum itself writes. It rejects a line that does not
// contain the two-space separator and a name repeated on two lines, naming
// the line number in both cases, since either signals the file was hand-
// edited rather than produced by WriteSums.
func ReadSums(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	sums := make(map[string]string)
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if text == "" {
			continue
		}
		idx := strings.Index(text, "  ")
		if idx < 0 {
			return nil, fmt.Errorf("%s:%d: malformed line, want \"<digest>  <name>\": %q", path, line, text)
		}
		digest, name := text[:idx], text[idx+2:]
		if _, exists := sums[name]; exists {
			return nil, fmt.Errorf("%s:%d: %s is listed more than once", path, line, name)
		}
		sums[name] = digest
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return sums, nil
}

// WriteSums writes a SHA256SUMS file, sorted by name so the file's diff is
// stable across runs regardless of map iteration order.
func WriteSums(path string, sums map[string]string) error {
	names := make([]string, 0, len(sums))
	for name := range sums {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "%s  %s\n", sums[name], name)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// decodePNG opens and decodes the PNG at path.
func decodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	return img, nil
}

// HasColour reports whether any pixel of the PNG at path has red, green,
// and blue that are not all equal. Alpha is ignored: Typst writes RGBA
// even for a page with no colour at all, so the check must read pixels
// rather than rely on the file's declared colour model.
func HasColour(path string) (bool, error) {
	img, err := decodePNG(path)
	if err != nil {
		return false, err
	}

	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r != g || g != b {
				return true, nil
			}
		}
	}
	return false, nil
}

// WriteDifference writes to out an image the size of golden and actual: a
// pixel equal in both is white, a pixel that differs is black. It returns
// an error naming both sizes when they differ, since the images cannot be
// compared pixel by pixel in that case.
func WriteDifference(golden, actual, out string) error {
	g, err := decodePNG(golden)
	if err != nil {
		return err
	}
	a, err := decodePNG(actual)
	if err != nil {
		return err
	}

	gb, ab := g.Bounds(), a.Bounds()
	if gb.Dx() != ab.Dx() || gb.Dy() != ab.Dy() {
		return fmt.Errorf("WriteDifference: %s is %dx%d, %s is %dx%d",
			golden, gb.Dx(), gb.Dy(), actual, ab.Dx(), ab.Dy())
	}

	diff := image.NewGray(image.Rect(0, 0, gb.Dx(), gb.Dy()))
	for y := 0; y < gb.Dy(); y++ {
		for x := 0; x < gb.Dx(); x++ {
			gr, gg, gbl, ga := g.At(gb.Min.X+x, gb.Min.Y+y).RGBA()
			ar, ag, abl, aa := a.At(ab.Min.X+x, ab.Min.Y+y).RGBA()
			if gr == ar && gg == ag && gbl == abl && ga == aa {
				diff.SetGray(x, y, color.Gray{Y: 255})
			} else {
				diff.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := png.Encode(f, diff); err != nil {
		_ = f.Close()
		return fmt.Errorf("encoding %s: %w", out, err)
	}
	return f.Close()
}
