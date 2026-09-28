package guard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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
// per call whose selector is Skip, Skipf, or SkipNow, in source order. The
// receiver of the call is irrelevant: "t.Skip", "b.Skip", "s.T().Skip" all
// match, since only the selected method name distinguishes a skip.
func SkipCalls(filename string, source []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, 0)
	if err != nil {
		return nil, err
	}

	var found []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !forbiddenSkipSelectors[sel.Sel.Name] {
			return true
		}
		line := fset.Position(call.Pos()).Line
		found = append(found, fmt.Sprintf("%s:%d: %s is forbidden; a skipped gate is not a checked gate", filename, line, sel.Sel.Name))
		return true
	})
	return found, nil
}
