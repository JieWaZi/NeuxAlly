package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListSkillResourcesReturnsVersionRelativePaths(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "references"), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"SKILL.md":                   "# Test",
		"references/review-check.md": "check",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(root, "references", "review-check.md"), 0o700); err != nil {
		t.Fatal(err)
	}
	resources, err := listSkillResources(root)
	if err != nil {
		t.Fatalf("listSkillResources() error = %v", err)
	}
	if len(resources) != 3 ||
		resources[0].Path != "SKILL.md" || resources[0].Directory ||
		resources[0].Size != int64(len("# Test")) || resources[0].Digest == "" ||
		resources[1].Path != "references" || !resources[1].Directory ||
		resources[2].Path != "references/review-check.md" || resources[2].Directory ||
		resources[2].Size != int64(len("check")) || resources[2].Digest == "" || !resources[2].Executable {
		t.Fatalf("resources = %+v", resources)
	}
}

func TestReadSkillVersionFileReturnsTextAndRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "template.sh"), []byte("echo ok\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := readSkillVersionFile(root, "template.sh")
	if err != nil {
		t.Fatalf("readSkillVersionFile() error = %v", err)
	}
	if file.Path != "template.sh" || file.Binary || file.Content != "echo ok\n" || !file.Executable {
		t.Fatalf("file = %+v", file)
	}
	if _, err := readSkillVersionFile(root, "../outside"); err == nil {
		t.Fatal("readSkillVersionFile() traversal error = nil")
	}
}
