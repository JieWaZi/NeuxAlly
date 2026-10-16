package constants

import "testing"

func TestProfileDocumentFileName(t *testing.T) {
	wantByKind := map[ProfileDocumentKind]string{
		ProfileDocumentKindIdentity: ProfileDocumentIdentityFileName,
		ProfileDocumentKindPersona:  ProfileDocumentPersonaFileName,
		ProfileDocumentKindPlaybook: ProfileDocumentPlaybookFileName,
		ProfileDocumentKindMemory:   ProfileDocumentMemoryFileName,
	}
	for kind, want := range wantByKind {
		got, ok := ProfileDocumentFileName(kind)
		if !ok || got != want {
			t.Fatalf("ProfileDocumentFileName(%q) = (%q, %v), want (%q, true)", kind, got, ok, want)
		}
	}

	if got, ok := ProfileDocumentFileName(ProfileDocumentKind("unknown")); ok || got != "" {
		t.Fatalf("ProfileDocumentFileName(unknown) = (%q, %v), want (\"\", false)", got, ok)
	}
}
