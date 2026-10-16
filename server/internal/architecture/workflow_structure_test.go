package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWorkflowCapabilitiesShareDomainDirectory 防止工作流能力重新散落到 internal 顶层。
func TestWorkflowCapabilitiesShareDomainDirectory(t *testing.T) {
	entries, err := os.ReadDir("..")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "workflow") && entry.Name() != "workflow" {
			t.Errorf("工作流能力 %s 必须归入 internal/workflow 的职责子包", entry.Name())
		}
	}
}

// TestWorkflowAndScheduleDoNotIsolatePrivateHelper 检查职责内聚，不以函数总数强制合并完整能力。
func TestWorkflowAndScheduleDoNotIsolatePrivateHelper(t *testing.T) {
	for _, root := range []string{"../workflow", "../../services/workflow", "../../services/schedule"} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			if ast.IsGenerated(file) {
				return nil
			}
			for _, group := range file.Comments {
				for _, comment := range group.List {
					if strings.HasPrefix(comment.Text, "//go:build ") {
						return nil
					}
				}
			}
			var onlyFunction *ast.FuncDecl
			functionCount, ownsDeclarations := 0, false
			for _, declaration := range file.Decls {
				switch declaration := declaration.(type) {
				case *ast.FuncDecl:
					functionCount++
					onlyFunction = declaration
				case *ast.GenDecl:
					ownsDeclarations = ownsDeclarations || declaration.Tok != token.IMPORT
				}
			}
			if functionCount == 1 && !ownsDeclarations && !onlyFunction.Name.IsExported() {
				t.Errorf("%s 只包含私有辅助函数 %s，应归入拥有该规则的职责文件", path, onlyFunction.Name.Name)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
