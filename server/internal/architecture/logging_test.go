// Package architecture 验证跨模块静态边界不会随业务改动退化。
package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestProductionLoggingUsesLogxBoundary(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve architecture test path")
	}
	serverRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	logxBoundary := filepath.Join(serverRoot, "internal", "logx", "logx.go")
	err := filepath.WalkDir(serverRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if filepath.Base(path) == "acp_logx_adapter.go" {
			return nil
		}
		parsed, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, imported := range parsed.Imports {
			importPath, unquoteErr := strconv.Unquote(imported.Path.Value)
			if unquoteErr != nil {
				return unquoteErr
			}
			if importPath == "log" || importPath == "log/slog" {
				t.Errorf("production logging boundary violation: file=%s import=%s", path, importPath)
			}
			if importPath == "github.com/shrewx/ginx/pkg/logx" && path != logxBoundary {
				t.Errorf("production logging boundary violation: file=%s imports upstream logx directly", path)
			}
		}
		parsed, parseErr = parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			owner, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			forbidden := owner.Name == "fmt" && strings.HasPrefix(selector.Sel.Name, "Print")
			forbidden = forbidden || owner.Name == "os" &&
				(selector.Sel.Name == "Stdout" || selector.Sel.Name == "Stderr")
			if forbidden {
				t.Errorf("production logging boundary violation: file=%s selector=%s.%s", path, owner.Name, selector.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan production Go files: %v", err)
	}
}
