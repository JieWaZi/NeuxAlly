package skills

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestApplySkillResourceOperationsSupportsCompleteFileTreeChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "references"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "references", "old.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "remove.txt"), []byte("remove"), 0o600); err != nil {
		t.Fatal(err)
	}
	operations := []SkillResourceOperation{
		{Action: "mkdir", Path: "assets"},
		{Action: "write", Path: "assets/icon.bin", ContentBase64: base64.StdEncoding.EncodeToString([]byte{0, 1, 2})},
		{Action: "move", Path: "references/old.txt", TargetPath: "references/new.txt"},
		{Action: "delete", Path: "remove.txt"},
	}
	if err := applySkillResourceOperations(root, operations); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "assets", "icon.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if len(content) != 3 || content[0] != 0 || content[2] != 2 {
		t.Fatalf("binary content = %v", content)
	}
	if _, err := os.Stat(filepath.Join(root, "references", "new.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "references", "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old path still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "remove.txt")); !os.IsNotExist(err) {
		t.Fatalf("deleted path still exists: %v", err)
	}
}

func TestApplySkillResourceOperationsRejectsTraversalAndSkillManifestMutation(t *testing.T) {
	root := t.TempDir()
	for _, operation := range []SkillResourceOperation{
		{Action: "write", Path: "../outside", ContentBase64: ""},
		{Action: "delete", Path: "SKILL.md"},
		{Action: "move", Path: "file.txt", TargetPath: "../../outside"},
	} {
		if err := applySkillResourceOperations(root, []SkillResourceOperation{operation}); err == nil {
			t.Fatalf("operation %+v succeeded", operation)
		}
	}
}

func TestApplySkillResourceOperationsRejectsFileAsParent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "references"), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []SkillResourceOperation{
		{Action: "mkdir", Path: "references/nested"},
		{Action: "write", Path: "references/check.md", ContentBase64: ""},
		{Action: "move", Path: "references", TargetPath: "references/moved"},
	} {
		if err := applySkillResourceOperations(root, []SkillResourceOperation{operation}); err == nil {
			t.Fatalf("operation %+v succeeded", operation)
		}
	}
}

func TestApplySkillResourceOperationsPreservesExecutableScript(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "scripts", "run.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	operation := SkillResourceOperation{
		Action:        "write",
		Path:          "scripts/run.sh",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("new")),
	}
	if err := applySkillResourceOperations(root, []SkillResourceOperation{operation}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(script)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("script mode = %o, want executable bit", info.Mode().Perm())
	}
}
