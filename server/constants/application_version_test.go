package constants

import "testing"

func TestApplicationVersionDefault(t *testing.T) {
	if ApplicationVersion != "0.1.0" {
		t.Fatalf("ApplicationVersion = %q, want 0.1.0", ApplicationVersion)
	}
}
