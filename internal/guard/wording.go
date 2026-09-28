package guard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// wordingExemptPrefix is the text a trailing line comment must start with to
// exempt the string literal ending that same line from being reported as
// reader text. The remainder of the comment is the reason, and it must not
// be empty.
const wordingExemptPrefix = "// wording: not reader text,"

// verbPattern matches one printf verb: '%', then optional flags and width,
// then one letter. It never matches "%%", because neither '%' of that pair
// can start a match: from the first '%', the next character ('%') is not a
// flag, a digit, or a letter; from the second '%', nothing follows to supply
// the required letter. So "%%" is removed by neither branch and is kept
// as-is, which is how this guard treats it as a literal percent sign rather
// than a verb.
var verbPattern = regexp.MustCompile(`%[-+ 0#]*[0-9]*[A-Za-z]`)

// ReaderText parses source as a Go file and returns one entry for each
// string literal that looks like text for a reader, in source order, as
// "<file>:<line>: <literal>". With strict false it applies the rule that
// holds for every package; with strict true it adds the rules that hold
// for internal/document.
//
// Never reported: an import path; a struct tag; an argument of
// errors.New, of fmt.Errorf, or of a call whose function name ends in
// Errorf; and the key of a key-value element of a composite literal whose
// type is written catalog.Params, or, inside the package catalog itself,
// Params. Such a key is the name of a catalog parameter, an identifier
// rather than text for a reader; the corresponding value is still read,
// since a value is what the reader sees. The type is read exactly as
// written in the source, so a map[string]string later converted to
// catalog.Params is not exempt.
//
// A line ending with the comment "// wording: not reader text, <reason>",
// its reason not empty, exempts every string literal that ends that same
// line, not only the one nearest the comment.
func ReaderText(filename string, source []byte, strict bool) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	exempt := exemptLiterals(file)
	exemptedLines := commentExemptedLines(fset, file)

	var literals []*ast.BasicLit
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			literals = append(literals, lit)
		}
		return true
	})
	sort.Slice(literals, func(i, j int) bool { return literals[i].Pos() < literals[j].Pos() })

	var found []string
	for _, lit := range literals {
		if exempt[lit] {
			continue
		}
		endLine := fset.Position(lit.End()).Line
		if exemptedLines[endLine] {
			continue
		}

		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			// A literal go/parser accepted always unquotes; this is kept as
			// a defensive skip rather than a panic, never exercised by a
			// well-formed source.
			continue
		}

		if !looksLikeReaderText(value, strict) {
			continue
		}

		line := fset.Position(lit.Pos()).Line
		found = append(found, fmt.Sprintf("%s:%d: %s", filename, line, lit.Value))
	}
	return found, nil
}

// looksLikeReaderText applies the rule that holds for every package (value
// holds a letter outside ASCII), and, when strict is true, the two further
// rules that hold for internal/document, read after every printf verb is
// removed from value.
func looksLikeReaderText(value string, strict bool) bool {
	if hasNonASCIILetter(value) {
		return true
	}
	if !strict {
		return false
	}

	stripped := verbPattern.ReplaceAllString(value, "")
	return hasTwoAdjacentWords(stripped) || hasCapitalisedWord(stripped)
}

// hasNonASCIILetter reports whether value holds a rune that is a letter
// (unicode.IsLetter) and lies outside ASCII. The report's status marks (●,
// ▲, ♦, ○, —) are not letters at all, so they never trigger this rule.
func hasNonASCIILetter(value string) bool {
	for _, r := range value {
		if r > unicode.MaxASCII && unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// wordRun is one maximal run of unicode letters inside a string, given as
// its runes and its half-open range of rune indices into that string.
type wordRun struct {
	runes []rune
	start int
	end   int
}

// wordRuns splits value into its maximal runs of unicode letters, in order.
func wordRuns(value string) []wordRun {
	runes := []rune(value)
	var runs []wordRun
	i := 0
	for i < len(runes) {
		if !unicode.IsLetter(runes[i]) {
			i++
			continue
		}
		start := i
		for i < len(runes) && unicode.IsLetter(runes[i]) {
			i++
		}
		runs = append(runs, wordRun{runes: runes[start:i], start: start, end: i})
	}
	return runs
}

// hasTwoAdjacentWords reports whether value holds two words of two or more
// letters each, with exactly one space between them.
func hasTwoAdjacentWords(value string) bool {
	runes := []rune(value)
	runs := wordRuns(value)
	for i := 0; i+1 < len(runs); i++ {
		a, b := runs[i], runs[i+1]
		if len(a.runes) < 2 || len(b.runes) < 2 {
			continue
		}
		if b.start-a.end == 1 && runes[a.end] == ' ' {
			return true
		}
	}
	return false
}

// hasCapitalisedWord reports whether value holds a word that starts with
// one upper-case letter followed by a lower-case letter.
func hasCapitalisedWord(value string) bool {
	for _, run := range wordRuns(value) {
		if len(run.runes) >= 2 && unicode.IsUpper(run.runes[0]) && unicode.IsLower(run.runes[1]) {
			return true
		}
	}
	return false
}

// exemptLiterals returns the set of string literals of file that are never
// reported regardless of their text: an import path, a struct tag, a
// literal that is directly one of the arguments of a call to errors.New, to
// fmt.Errorf, or to a call whose function name ends in Errorf, and the key
// of a key-value element of a catalog.Params composite literal (see
// isCatalogParamsType).
func exemptLiterals(file *ast.File) map[*ast.BasicLit]bool {
	exempt := make(map[*ast.BasicLit]bool)
	insideCatalog := file.Name != nil && file.Name.Name == "catalog"

	for _, imp := range file.Imports {
		exempt[imp.Path] = true
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.Field:
			if node.Tag != nil {
				exempt[node.Tag] = true
			}
		case *ast.CallExpr:
			if isExemptDiagnosticCall(node.Fun) {
				for _, arg := range node.Args {
					if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						exempt[lit] = true
					}
				}
			}
		case *ast.CompositeLit:
			if isCatalogParamsType(node.Type, insideCatalog) {
				for _, elt := range node.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if lit, ok := kv.Key.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						exempt[lit] = true
					}
				}
			}
		}
		return true
	})
	return exempt
}

// isCatalogParamsType reports whether typ, a composite literal's type as
// written in the source, is catalog.Params, or, when insideCatalog is true
// (the file being scanned is itself package catalog), the unqualified
// Params. Only the written form is read; this guard does not resolve types,
// so a map[string]string later converted to catalog.Params does not match.
func isCatalogParamsType(typ ast.Expr, insideCatalog bool) bool {
	switch t := typ.(type) {
	case *ast.SelectorExpr:
		pkg, ok := t.X.(*ast.Ident)
		return ok && pkg.Name == "catalog" && t.Sel.Name == "Params"
	case *ast.Ident:
		return insideCatalog && t.Name == "Params"
	}
	return false
}

// isExemptDiagnosticCall reports whether fun, the function expression of a
// call, is errors.New, or a call whose selected or plain name ends in
// Errorf (which already covers fmt.Errorf).
func isExemptDiagnosticCall(fun ast.Expr) bool {
	switch f := fun.(type) {
	case *ast.Ident:
		return strings.HasSuffix(f.Name, "Errorf")
	case *ast.SelectorExpr:
		if strings.HasSuffix(f.Sel.Name, "Errorf") {
			return true
		}
		pkg, ok := f.X.(*ast.Ident)
		return ok && pkg.Name == "errors" && f.Sel.Name == "New"
	}
	return false
}

// commentExemptedLines returns the set of source lines ending with a
// comment of the form "// wording: not reader text, <reason>" whose reason
// is not empty. A literal whose last line is in this set is exempt,
// including a multi-line raw string exempted by a comment on its last line.
func commentExemptedLines(fset *token.FileSet, file *ast.File) map[int]bool {
	lines := make(map[int]bool)
	for _, group := range file.Comments {
		for _, c := range group.List {
			reason := strings.TrimSpace(strings.TrimPrefix(c.Text, wordingExemptPrefix))
			if !strings.HasPrefix(c.Text, wordingExemptPrefix) || reason == "" {
				continue
			}
			lines[fset.Position(c.Pos()).Line] = true
		}
	}
	return lines
}
