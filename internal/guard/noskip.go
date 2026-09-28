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

// testingHandleStarNames are the type names, selected off the testing
// package, that a pointer parameter can have to count as a testing handle
// (*testing.T, *testing.B, *testing.F).
var testingHandleStarNames = map[string]bool{
	"T": true,
	"B": true,
	"F": true,
}

// SkipCalls parses source as a Go file named filename and returns one entry
// per selector expression whose selected name is Skip, Skipf, or SkipNow,
// in source order, in exactly two cases: the selector is called ("t.Skip(...)",
// "s.T().SkipNow()", any receiver), or it is not called but its receiver is
// a plain identifier declared as a parameter, of an enclosing function
// declaration or function literal, typed *testing.T, *testing.B, *testing.F,
// or testing.TB ("skip := t.Skip" still skips when skip is invoked later).
// A field or method merely named Skip on an unrelated type is not reported
// unless it is actually called, since only a genuine testing handle can
// really skip a test. Shadowing is not modeled: an inner declaration that
// reuses a testing parameter's name for something else may still be
// reported, per the accepted simplification of a purely syntactic check
// (no go/types, standard library only). A call such as "t.Skip(...)"
// contains exactly one matching selector expression (its Fun), so it still
// yields exactly one entry.
func SkipCalls(filename string, source []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, 0)
	if err != nil {
		return nil, err
	}

	pkgAlias := testingImportName(file)

	// calledSelectors marks every *ast.SelectorExpr that is directly the
	// function of a call expression, regardless of the receiver: a called
	// Skip/Skipf/SkipNow always skips, whatever it is called on.
	calledSelectors := make(map[*ast.SelectorExpr]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			calledSelectors[sel] = true
		}
		return true
	})

	var found []string
	// testingParams is a stack of parameter-name sets, one per currently
	// open function declaration or function literal, holding the names of
	// that function's parameters typed as a testing handle. isTestingParam
	// checks the whole stack, so a closure sees its enclosing function's
	// testing parameter too.
	var testingParams []map[string]bool
	// openNodes mirrors ast.Inspect's traversal: every visited node is
	// pushed here and popped on the matching nil callback ast.Inspect
	// makes once that node's children are done, which is how a scope
	// opened by a FuncDecl/FuncLit is closed at the right time.
	var openNodes []ast.Node
	isTestingParam := func(name string) bool {
		for i := len(testingParams) - 1; i >= 0; i-- {
			if testingParams[i][name] {
				return true
			}
		}
		return false
	}

	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			top := openNodes[len(openNodes)-1]
			openNodes = openNodes[:len(openNodes)-1]
			if funcTypeOf(top) != nil {
				testingParams = testingParams[:len(testingParams)-1]
			}
			return true
		}
		openNodes = append(openNodes, n)
		if ft := funcTypeOf(n); ft != nil {
			testingParams = append(testingParams, testingParamNames(ft, pkgAlias))
		}

		sel, ok := n.(*ast.SelectorExpr)
		if !ok || !forbiddenSkipSelectors[sel.Sel.Name] {
			return true
		}

		reported := calledSelectors[sel]
		if !reported {
			if recv, ok := sel.X.(*ast.Ident); ok && isTestingParam(recv.Name) {
				reported = true
			}
		}
		if reported {
			line := fset.Position(sel.Pos()).Line
			found = append(found, fmt.Sprintf("%s:%d: %s is forbidden; a skipped gate is not a checked gate", filename, line, sel.Sel.Name))
		}
		return true
	})
	return found, nil
}

// funcTypeOf returns the *ast.FuncType of n when n is a function
// declaration or a function literal, the two node kinds that introduce a
// new set of parameters, or nil for any other node.
func funcTypeOf(n ast.Node) *ast.FuncType {
	switch fn := n.(type) {
	case *ast.FuncDecl:
		return fn.Type
	case *ast.FuncLit:
		return fn.Type
	}
	return nil
}

// testingParamNames returns the names of ft's parameters whose declared
// type is *testing.T, *testing.B, *testing.F, or testing.TB, using pkgAlias
// as the local name the file imports "testing" under.
func testingParamNames(ft *ast.FuncType, pkgAlias string) map[string]bool {
	names := make(map[string]bool)
	if ft.Params == nil {
		return names
	}
	for _, field := range ft.Params.List {
		if !isTestingHandleType(field.Type, pkgAlias) {
			continue
		}
		for _, name := range field.Names {
			names[name.Name] = true
		}
	}
	return names
}

// isTestingHandleType reports whether expr is one of the four shapes a
// testing handle parameter can have: a pointer to T, B, or F from the
// testing package, or the TB interface itself.
func isTestingHandleType(expr ast.Expr, pkgAlias string) bool {
	switch t := expr.(type) {
	case *ast.StarExpr:
		sel, ok := t.X.(*ast.SelectorExpr)
		return ok && isPackageIdent(sel.X, pkgAlias) && testingHandleStarNames[sel.Sel.Name]
	case *ast.SelectorExpr:
		return isPackageIdent(t.X, pkgAlias) && t.Sel.Name == "TB"
	}
	return false
}

// isPackageIdent reports whether expr is a plain identifier matching
// pkgAlias, the local name a selector's package qualifier must have to be
// the testing package as this file imports it.
func isPackageIdent(expr ast.Expr, pkgAlias string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == pkgAlias
}

// testingImportName returns the local name this file imports "testing"
// under ("testing" itself, unless the import is aliased, e.g. `tt
// "testing"`). Files that do not import "testing" at all cannot declare a
// testing-typed parameter either, so the exact name returned then does not
// matter; "testing" is returned as a harmless default.
func testingImportName(file *ast.File) string {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != "testing" {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "testing"
	}
	return "testing"
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
