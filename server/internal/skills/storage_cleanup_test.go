package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupTemporaryArtifactsRemovesOnlyManagedStaging(t *testing.T) {
	dataDirectory := t.TempDir()
	temporaryPaths := []string{
		filepath.Join(dataDirectory, "skills", "tmp", "import-dead", "SKILL.md"),
		filepath.Join(dataDirectory, "skills", "version-staging-dead", "SKILL.md"),
	}
	keptPath := filepath.Join(dataDirectory, "skills", "versions", "skill-1", "1.0.0", "SKILL.md")
	for _, path := range append(temporaryPaths, keptPath) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := CleanupTemporaryArtifacts(dataDirectory); err != nil {
		t.Fatal(err)
	}
	for _, path := range temporaryPaths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("temporary artifact %q still exists: %v", path, err)
		}
	}
	if _, err := os.Stat(keptPath); err != nil {
		t.Fatalf("authority version was removed: %v", err)
	}
}

func TestCleanupFunctionsRejectEmptyDataDirectory(t *testing.T) {
	if err := CleanupTemporaryArtifacts(""); err == nil {
		t.Fatal("CleanupTemporaryArtifacts() accepted empty data directory")
	}
	if err := CleanupUnreferencedVersions("", nil); err == nil {
		t.Fatal("CleanupUnreferencedVersions() accepted empty data directory")
	}
}

func TestCleanupUnreferencedVersionsKeepsDatabaseReferences(t *testing.T) {
	dataDirectory := t.TempDir()
	keptRelative := filepath.Join("skills", "versions", "skill-1", "1.0.0")
	orphanRelative := filepath.Join("skills", "versions", "skill-1", "2.0.0")
	stagingRelative := filepath.Join("skills", "versions", "skill-2", "version-staging-3.0.0-dead")
	for _, relative := range []string{keptRelative, orphanRelative, stagingRelative} {
		path := filepath.Join(dataDirectory, relative, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := CleanupUnreferencedVersions(dataDirectory, []string{keptRelative}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDirectory, keptRelative)); err != nil {
		t.Fatalf("referenced version was removed: %v", err)
	}
	for _, relative := range []string{orphanRelative, stagingRelative} {
		if _, err := os.Stat(filepath.Join(dataDirectory, relative)); !os.IsNotExist(err) {
			t.Fatalf("unreferenced path %q still exists: %v", relative, err)
		}
	}
}
