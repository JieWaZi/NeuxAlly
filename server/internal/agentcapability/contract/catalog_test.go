package contract

import (
	"reflect"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
)

func TestMemoryScopeContractsDistinguishSearchAndOwnership(t *testing.T) {
	search, _ := Find("memory.entries.search")
	create, _ := Find("memory.entries.create")
	for _, scope := range []constants.MemoryScope{constants.MemoryScopePartner, constants.MemoryScopeProject, constants.MemoryScopeAll} {
		if err := search.Validate(map[string]any{"query": "preferences", "scope": scope.String()}); err != nil {
			t.Fatalf("visible search scope rejected: %s: %v", scope, err)
		}
	}
	if search.Validate(map[string]any{"query": "preferences"}) != nil ||
		search.Discovery["defaultScope"] != constants.MemoryScopeAll.String() {
		t.Fatal("omitted search scope lost its all-visible default")
	}
	for _, scope := range []constants.MemoryScope{constants.MemoryScopePartner, constants.MemoryScopeProject} {
		if create.Validate(map[string]any{"scope": scope.String(), "content": "Stable fact"}) != nil {
			t.Fatalf("memory ownership scope rejected: %s", scope)
		}
	}
	for _, scope := range []string{"all", "", "global"} {
		if create.Validate(map[string]any{"scope": scope, "content": "Stable fact"}) == nil {
			t.Fatalf("invalid memory ownership accepted: %q", scope)
		}
	}
	if create.Validate(map[string]any{"content": "Stable fact"}) == nil ||
		search.Validate(map[string]any{"query": "preferences", "scope": "global"}) == nil {
		t.Fatal("missing ownership or unknown search scope accepted")
	}
	for _, op := range []Operation{search, create} {
		properties := op.InputSchema["properties"].(map[string]any)
		scope := properties["scope"].(map[string]any)
		if !reflect.DeepEqual(op.Discovery["scopes"], scope["enum"]) {
			t.Fatalf("capabilities scope choices differ from validation: %s", op.ID)
		}
	}
}

func TestWorkflowSDKTopicsAreDiscoveredAndCheckedByKind(t *testing.T) {
	op, _ := Find("workflow.sdk.get")
	for _, params := range []map[string]any{
		{"topic": "function"}, {"topic": "overview", "kind": "sdk"},
		{"topic": "f0", "kind": "example"},
	} {
		if err := op.Validate(params); err != nil {
			t.Fatalf("advertised SDK input rejected: %v: %v", params, err)
		}
	}
	for _, params := range []map[string]any{
		{"topic": "functionTask"}, {"topic": "index"}, {"topic": "phase"},
		{"topic": "f0"}, {"topic": "function", "kind": "example"},
	} {
		if op.Validate(params) == nil {
			t.Fatalf("invalid SDK query accepted: %v", params)
		}
	}
	mapping := op.Discovery["primitiveTopics"].(map[string]string)
	if mapping["functionTask"] != "function" || mapping["phase"] != "overview" {
		t.Fatalf("missing primitive-to-topic guidance: %v", mapping)
	}
	resources, _ := Find("workflow.resources.list")
	if resources.Validate(map[string]any{"kind": "wakers"}) != nil ||
		resources.Validate(map[string]any{"kind": "builtinActions"}) != nil ||
		resources.Validate(map[string]any{"kind": "partners"}) == nil {
		t.Fatal("resource discovery differs from accepted kinds")
	}
	prepare, _ := Find("workflow.authoring.prepare")
	if prepare.Validate(map[string]any{"primitives": []string{}}) == nil ||
		prepare.Validate(map[string]any{"primitives": []string{"functionTask"}, "features": []string{"unknown"}}) == nil {
		t.Fatal("preparation accepted missing primitives or an unknown feature")
	}
}
