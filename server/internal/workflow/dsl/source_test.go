package dsl

import (
	"os"
	"strings"
	"testing"

	"github.com/dop251/goja/ast"
)

func TestParserAcceptsCompleteReferenceCorpus(t *testing.T) {
	for _, fixture := range []struct {
		// name 标识当前仓库中需要完整解析的工作流样例。
		name string
		// minStatements 防止解析器只返回样例开头的部分顶层语句。
		minStatements int
	}{
		{name: "weekly-report.workflow.js", minStatements: 10},
		{name: "environment-release.workflow.js", minStatements: 10},
		{name: "environment-review.workflow.js", minStatements: 8},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + fixture.name)
			if err != nil {
				t.Fatal(err)
			}
			parsed, diagnostics, err := parseSource(string(data))
			if err != nil {
				t.Fatalf("full corpus parse failed: %v %v", err, diagnostics)
			}
			if len(parsed.body) < fixture.minStatements || len(parsed.exports) == 0 {
				t.Fatal("full source was not parsed")
			}
			first := parsed.body[0].(*ast.LexicalDeclaration)
			position := parsed.sourceRange(first.List[0].Initializer).Start
			if position.Line != 1 || position.Column != 21 {
				t.Fatalf("meta source location: %+v", position)
			}
		})
	}
}

func TestSourcePositionUsesOriginalUTF8AndEditorUTF16(t *testing.T) {
	source := "const label = '中文😀';\nreturn await worker(`你好😀 ${args.name}`, {id:'draft'});"
	parsed, diagnostics, err := parseSource(source)
	if err != nil {
		t.Fatalf("%v %v", err, diagnostics)
	}
	statement := parsed.body[1].(*ast.ReturnStatement)
	await := statement.Argument.(*ast.AwaitExpression)
	call := await.Argument.(*ast.CallExpression)
	position := parsed.sourceRange(call.ArgumentList[0]).Start
	if position.Line != 2 || position.Column != 21 {
		t.Fatalf("template position: %+v", position)
	}
	offset := strings.Index(source, "${")
	position = positionAt(source, offset)
	if position.Column != 27 {
		t.Fatalf("emoji must occupy two UTF-16 units: %+v", position)
	}
}

func TestExportMaskPreservesStringTemplateAndComments(t *testing.T) {
	source := "// export const ignored = 1\nexport const meta = { name: 'export', description: `\nexport const sample = 2\n` };"
	masked, exports := maskExports(source)
	if len(exports) != 1 || len(masked) != len(source) || !strings.Contains(masked, "export const sample") {
		t.Fatalf("export adapter changed embedded source: %q %v", masked, exports)
	}
}

func TestParserReportsSyntaxLocationWithoutWrapperOffset(t *testing.T) {
	_, diagnostics, err := parseSource("export const meta = {};\nreturn await ;")
	if err == nil || len(diagnostics) != 1 || diagnostics[0].Range.Start.Line != 2 {
		t.Fatalf("%v %v", err, diagnostics)
	}
}

func TestExportMaskPreservesRegularExpressionsAndNestedTemplates(t *testing.T) {
	source := "export const meta={}; const pattern=/export const meta/; const template=`${`nested export`}\nexport const embedded=1`; return 10 / 2;"
	masked, exports := maskExports(source)
	if len(exports) != 1 || len(masked) != len(source) || masked[len("export"):] != source[len("export"):] {
		t.Fatalf("lexical source changed: %q %v", masked, exports)
	}
	if _, diagnostics, err := parseSource(source); err != nil {
		t.Fatalf("%v %+v", err, diagnostics)
	}
}
