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

// isRootPackage reports whether pkg is the module's own root package.
func isRootPackage(pkg, module string) bool {
	return pkg == module
}

// isCmdPackage reports whether pkg lives under the module's cmd/ tree.
func isCmdPackage(pkg, module string) bool {
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
// layers and returns the sorted set of violations.
//
// Packages under cmd/ are free to import anything of this module, including
// each other, and are not classified into a layer or reported as
// unclassified: they wire the program together and sit above every layer.
// The module's root package must not import any package of this module, so
// a lower layer cannot reach a higher one by routing through it; any other
// package may import the root package. A regular, layered package must
// match a known layer, may not import a package under cmd/ (that would be
// the same reverse route through a different door), and otherwise follows
// the rank rule: an import must go to a strictly lower rank, or within the
// same layer.
func ImportViolations(module string, layers []Layer, imports map[string][]string) []string {
	violations := make(map[string]struct{})

	for pkg, pkgImports := range imports {
		switch {
		case isCmdPackage(pkg, module):
			continue

		case isRootPackage(pkg, module):
			for _, imp := range pkgImports {
				if !isModulePackage(imp, module) {
					continue
				}
				violations[fmt.Sprintf("%s imports %s: the root package imports nothing of this module", pkg, imp)] = struct{}{}
			}

		default:
			srcLayer, ok := matchLayer(pkg, module, layers)
			if !ok {
				violations[fmt.Sprintf("%s: not classified in guard.Layers", pkg)] = struct{}{}
				continue
			}

			for _, imp := range pkgImports {
				if !isModulePackage(imp, module) {
					continue
				}
				if isRootPackage(imp, module) {
					// Any package may import the root package.
					continue
				}
				if isCmdPackage(imp, module) {
					violations[fmt.Sprintf("%s imports %s: packages under cmd/ are not importable", pkg, imp)] = struct{}{}
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
	}

	result := make([]string, 0, len(violations))
	for v := range violations {
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}
