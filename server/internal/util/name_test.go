package util

import "testing"

func TestNormalizeNameTrimsOuterWhitespaceAndPreservesCase(t *testing.T) {
	t.Parallel()

	actual := NormalizeName(" \n Alice \t")
	if actual != "Alice" {
		t.Fatalf("NormalizeName() = %q, want %q", actual, "Alice")
	}
}
