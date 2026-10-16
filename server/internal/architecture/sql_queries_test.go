package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestPersistenceQueriesAvoidJoins 固定默认无联表规则和已批准的 memory 检索特例；新增例外必须提供必要性证据。
func TestPersistenceQueriesAvoidJoins(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve SQL architecture path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	join := regexp.MustCompile(`(?i)\bjoin(?:\s|$)`)
	for _, directory := range []string{"repositories", "internal/memory"} {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			positions := token.NewFileSet()
			parsed, err := parser.ParseFile(positions, path, nil, 0)
			if err != nil {
				return err
			}
			// 用户明确批准的检索特例仅覆盖这两个函数，不豁免整个 memory 包。
			exempt := map[*ast.BasicLit]bool{}
			rel, _ := filepath.Rel(root, path)
			allowedFunction := ""
			if filepath.ToSlash(rel) == "internal/memory/keyword.go" {
				allowedFunction = "searchFTS"
			}
			if filepath.ToSlash(rel) == "internal/memory/vecstore.go" {
				allowedFunction = "search"
			}
			for _, decl := range parsed.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name.Name != allowedFunction {
					continue
				}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					if literal, ok := node.(*ast.BasicLit); ok {
						exempt[literal] = true
					}
					return true
				})
			}
			ast.Inspect(parsed, func(node ast.Node) bool {
				if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "Joins" {
					t.Errorf("%s: JOIN requires documented necessity and query-plan evidence", positions.Position(node.Pos()))
				}
				if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
					value, err := strconv.Unquote(literal.Value)
					if err == nil && join.MatchString(value) && !exempt[literal] {
						t.Errorf("%s: SQL JOIN is forbidden by the current persistence baseline", positions.Position(node.Pos()))
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
