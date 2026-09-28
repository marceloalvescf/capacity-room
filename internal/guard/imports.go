package guard

import (
	"fmt"
	"sort"
	"strings"
)

// Layer is one rung of the dependency ladder: packages below Path (or at
// it) may only be imported by packages ranked higher, never the reverse.
type Layer struct {
	Path string
	Rank int
}

// Layers is the table of known layers for this phase. Later phases insert
// the core packages between rank 0 and rank 100, in the order of spec
// section 4.1. A layer with no package yet (internal/typst, for now) is
// not a violation; only a package that matches no layer is.
var Layers = []Layer{
	{Path: "internal/guard", Rank: 0},
	{Path: "internal/typst", Rank: 0},
	{Path: "internal/cli", Rank: 100},
}

// isModulePackage reports whether pkg is module itself or a package below
// it, as opposed to a standard-library or third-party import.
func isModulePackage(pkg, module string) bool {
	return pkg == module || strings.HasPrefix(pkg, module+"/")
}

// isOutsideRule reports whether pkg is the module's root package or lives
// under cmd/: both are excluded from layering, as neither is a library
// package other packages should reason about in terms of rank.
func isOutsideRule(pkg, module string) bool {
	if pkg == module {
		return true
	}
	cmdRoot := module + "/cmd"
	return pkg == cmdRoot || strings.HasPrefix(pkg, cmdRoot+"/")
}

// matchLayer finds the layer whose path pkg falls under, relative to
// module. The longest matching layer path wins, so a more specific layer
// (e.g. internal/platform/api) takes precedence over a broader ancestor
// (internal/platform).
func matchLayer(pkg, module string, layers []Layer) (Layer, bool) {
	best := Layer{}
	bestLen := -1
	matched := false
	for _, layer := range layers {
		prefix := module + "/" + layer.Path
		if pkg != prefix && !strings.HasPrefix(pkg, prefix+"/") {
			continue
		}
		if len(prefix) > bestLen {
			best = layer
			bestLen = len(prefix)
			matched = true
		}
	}
	return best, matched
}

// ImportViolations checks every package of module in imports against
// layers and returns the sorted set of violations: packages that match no
// layer, and imports that go from a lower rank to a rank that is not
// strictly lower (nor the same layer). The module's root package and
// packages under cmd/ are outside the rule entirely.
func ImportViolations(module string, layers []Layer, imports map[string][]string) []string {
	violations := make(map[string]struct{})

	for pkg, pkgImports := range imports {
		if isOutsideRule(pkg, module) {
			continue
		}

		srcLayer, ok := matchLayer(pkg, module, layers)
		if !ok {
			violations[fmt.Sprintf("%s: not classified in guard.Layers", pkg)] = struct{}{}
			continue
		}

		for _, imp := range pkgImports {
			if !isModulePackage(imp, module) || isOutsideRule(imp, module) {
				continue
			}

			tgtLayer, ok := matchLayer(imp, module, layers)
			if !ok {
				// imp's own violation is reported when it is visited as
				// a source package; reporting it again here would be
				// redundant.
				continue
			}

			if tgtLayer.Path == srcLayer.Path || tgtLayer.Rank < srcLayer.Rank {
				continue
			}

			violations[fmt.Sprintf("%s (rank %d) imports %s (rank %d)", pkg, srcLayer.Rank, imp, tgtLayer.Rank)] = struct{}{}
		}
	}

	result := make([]string, 0, len(violations))
	for v := range violations {
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}
