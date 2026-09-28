package guard

import (
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

// forbiddenModulePrefixes are module paths that would pull a Kubernetes or
// cloud client into a project that reads Prometheus metrics only.
var forbiddenModulePrefixes = []string{
	"k8s.io",
	"sigs.k8s.io",
	"github.com/aws",
	"cloud.google.com/go",
	"github.com/Azure",
	"github.com/oracle/oci-go-sdk",
	"github.com/kubernetes",
}

// hasForbiddenPrefix reports whether module matches prefix on path
// segments: module equals prefix, or module starts with prefix + "/".
// This keeps "k8s.io" from matching "k8s.iox". The comparison is
// case-insensitive, since a module path spelled with different letter case
// (or replaced to a locally vendored copy under that spelling) is the same
// forbidden dependency to the Go toolchain.
func hasForbiddenPrefix(module, prefix string) bool {
	module = strings.ToLower(module)
	prefix = strings.ToLower(prefix)
	return module == prefix || strings.HasPrefix(module, prefix+"/")
}

// ForbiddenModules parses gomod and returns the sorted, de-duplicated set
// of required or replaced module paths that match a forbidden prefix.
// Direct and indirect requires are both checked, along with replace
// targets, since either can smuggle a Kubernetes or cloud client into the
// dependency graph.
func ForbiddenModules(gomod []byte) ([]string, error) {
	file, err := modfile.Parse("go.mod", gomod, nil)
	if err != nil {
		return nil, err
	}

	found := make(map[string]struct{})
	for _, req := range file.Require {
		checkForbidden(req.Mod.Path, found)
	}
	for _, rep := range file.Replace {
		checkForbidden(rep.New.Path, found)
	}

	violations := make([]string, 0, len(found))
	for module := range found {
		violations = append(violations, module)
	}
	sort.Strings(violations)
	return violations, nil
}

func checkForbidden(module string, found map[string]struct{}) {
	for _, prefix := range forbiddenModulePrefixes {
		if hasForbiddenPrefix(module, prefix) {
			found[module] = struct{}{}
			return
		}
	}
}
