package remote

import (
	"reflect"
	"strings"
	"testing"
)

// 本测试约束远端连接对象的依赖图；新建本机核心、模型端口或影子索引字段会破坏边界。
func TestCustomConnectionDoesNotOwnLocalExecutionOrStorage(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var visit func(reflect.Type)
	visit = func(typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Map {
			typ = typ.Elem()
		}
		if seen[typ] {
			return
		}
		seen[typ] = true
		p := typ.PkgPath()
		if strings.HasPrefix(p, "github.com/JieWaZi/neux-knowledge/") && p != "github.com/JieWaZi/neux-knowledge/remote" {
			t.Fatalf("HTTP连接持有本地知识依赖: %s", typ)
		}
		if typ.Kind() == reflect.Struct && p == "github.com/JieWaZi/neux-knowledge/remote" {
			for i := 0; i < typ.NumField(); i++ {
				visit(typ.Field(i).Type)
			}
		}
	}
	visit(reflect.TypeFor[Custom]())
}
