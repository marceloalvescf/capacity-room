package guard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// forbiddenSkipSelectors are the testing.TB methods that bypass a test
// instead of running it. A skipped gate is not a checked gate, so this
// project forbids all three regardless of the receiver they are called on.
var forbiddenSkipSelectors = map[string]bool{
	"Skip":    true,
	"Skipf":   true,
	"SkipNow": true,
}

// SkipCalls parses source as a Go file named filename and returns one entry
// per selector expression whose selected name is Skip, Skipf, or SkipNow,
// in source order. The receiver is irrelevant: "t.Skip", "b.Skip",
// "s.T().Skip" all match, since only the selected method name distinguishes
// a skip. Matching the selector expression directly, rather than only a
// call expression, also catches the method taken as a value and invoked
// later ("skip := t.Skip; skip(...)"), since that still genuinely skips the
// test. A call such as "t.Skip(...)" contains exactly one such selector
// expression (its Fun), so it still yields exactly one entry.
func SkipCalls(filename string, source []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, 0)
	if err != nil {
		return nil, err
	}

	var found []string
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || !forbiddenSkipSelectors[sel.Sel.Name] {
			return true
		}
		line := fset.Position(sel.Pos()).Line
		found = append(found, fmt.Sprintf("%s:%d: %s is forbidden; a skipped gate is not a checked gate", filename, line, sel.Sel.Name))
		return true
	})
	return found, nil
}

// ScannedForSkips reports whether filename should be scanned for skip
// calls: either its name ends in "_test.go", or it imports the package
// "testing" (a plain helper file cannot itself be a test, but it can still
// hold a *testing.T handed to it by a real test and call Skip on it). Only
// the import block is parsed, so a file with an unparsable body past its
// imports does not block the check.
func ScannedForSkips(filename string, source []byte) (bool, error) {
	if strings.HasSuffix(filename, "_test.go") {
		return true, nil
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, parser.ImportsOnly)
	if err != nil {
		return false, err
	}

	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return false, err
		}
		if path == "testing" {
			return true, nil
		}
	}
	return false, nil
}
