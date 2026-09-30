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

// Layers is the table of every package of this module ranked for the
// import-direction rule: a package may import a package of a lower rank,
// or of its own layer, never one of a higher rank. A package that matches
// no layer here is reported at the import that reaches it, whatever
// imports it.
var Layers = []Layer{
	{Path: "internal/guard", Rank: 0},
	{Path: "internal/arith", Rank: 0},
	{Path: "internal/typst", Rank: 0},
	{Path: "internal/redact", Rank: 0},
	{Path: "internal/promtest", Rank: 0},
	{Path: "template", Rank: 0},
	{Path: "template/golden", Rank: 10},
	{Path: "internal/config", Rank: 20},
	{Path: "internal/prom", Rank: 30},
	{Path: "internal/discovery", Rank: 40},
	{Path: "internal/resolve", Rank: 50},
	{Path: "internal/collect", Rank: 55},
	{Path: "internal/analysis", Rank: 60},
	{Path: "internal/snapshot", Rank: 70},
	{Path: "internal/catalog", Rank: 80},
	{Path: "internal/document", Rank: 90},
	{Path: "internal/goldentest", Rank: 95},
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
// Packages under cmd/ are free to import the root package, another package
// under cmd/, and any classified layer; they are not classified into a
// layer or reported as unclassified themselves, since they wire the program
// together and sit above every layer. The module's root package must not
// import any package of this module, so a lower layer cannot reach a higher
// one by routing through it; any other package may import the root package.
// A regular, layered package must match a known layer, may not import a
// package under cmd/ (that would be the same reverse route through a
// different door), and otherwise follows the rank rule: an import must go
// to a strictly lower rank, or within the same layer. A source that itself
// matches no layer is reported on its own line and skips the rank rule (it
// has no rank to compare with), but every other check below still applies
// to its imports.
//
// Every source, whatever kind it is, is also checked edge by edge: an
// import that is not the root package, not under cmd/, and matches no layer
// is reported at that edge, naming both the source and the target. This
// holds even when the target is never itself a key of imports (an import
// path with no package behind it in this scan), since a violation attributed
// only to a visited source package would never fire for such a target — and
// it holds even when the source itself is unclassified, since a violation
// attributed to the source's own line names only the source, never the
// target it imports.
func ImportViolations(module string, layers []Layer, imports map[string][]string) []string {
	violations := make(map[string]struct{})

	unclassifiedEdge := func(pkg, imp string) {
		violations[fmt.Sprintf("%s imports %s: %s is not classified in guard.Layers", pkg, imp, imp)] = struct{}{}
	}

	for pkg, pkgImports := range imports {
		switch {
		case isCmdPackage(pkg, module):
			for _, imp := range pkgImports {
				if !isModulePackage(imp, module) {
					continue
				}
				if isRootPackage(imp, module) || isCmdPackage(imp, module) {
					continue
				}
				if _, ok := matchLayer(imp, module, layers); !ok {
					unclassifiedEdge(pkg, imp)
				}
			}

		case isRootPackage(pkg, module):
			for _, imp := range pkgImports {
				if !isModulePackage(imp, module) {
					continue
				}
				violations[fmt.Sprintf("%s imports %s: the root package imports nothing of this module", pkg, imp)] = struct{}{}
			}

		default:
			srcLayer, srcOK := matchLayer(pkg, module, layers)
			if !srcOK {
				violations[fmt.Sprintf("%s: not classified in guard.Layers", pkg)] = struct{}{}
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
					unclassifiedEdge(pkg, imp)
					continue
				}

				if !srcOK {
					// The source itself has no rank to compare against; its
					// own line already reported it, and a classified target
					// is not itself a further violation.
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
