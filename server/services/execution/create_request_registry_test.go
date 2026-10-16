package execution

import "testing"

func TestCreateRequestRegistryAllowsOneConcurrentOwner(t *testing.T) {
	registry := newCreateRequestRegistry()
	if !registry.reserve("chat-1") {
		t.Fatal("first reserve rejected")
	}
	if registry.reserve("chat-1") {
		t.Fatal("duplicate reserve accepted")
	}
	registry.release("chat-1")
	if !registry.reserve("chat-1") {
		t.Fatal("reserve after release rejected")
	}
}
