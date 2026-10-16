package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadMetadataParsesStandardSkillFrontmatter(t *testing.T) {
	directory := t.TempDir()
	content := "---\nname: code-review\ndescription: Review code changes\n---\n\n# Review\n"
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	metadata, err := ReadMetadata(directory)
	if err != nil {
		t.Fatalf("ReadMetadata() error = %v", err)
	}
	if metadata.Name != "code-review" || metadata.Description != "Review code changes" {
		t.Fatalf("metadata = %+v", metadata)
	}
}

func TestReadMetadataRejectsMissingDescription(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte("---\nname: review\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadMetadata(directory); err == nil {
		t.Fatal("ReadMetadata() error = nil, want invalid frontmatter")
	}
}

func TestUpdateMetadataPreservesOtherFrontmatterAndBody(t *testing.T) {
	directory := t.TempDir()
	content := "---\nname: code-review\ndescription: Review code changes\nallowed-tools:\n  - Read\n---\n\n# Review\n"
	path := filepath.Join(directory, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := UpdateMetadata(directory, Metadata{
		Name:        "review-workflow",
		Description: "Review important changes",
	}); err != nil {
		t.Fatalf("UpdateMetadata() error = %v", err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	for _, expected := range []string{
		"name: review-workflow",
		"description: Review important changes",
		"allowed-tools:",
		"# Review",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("updated SKILL.md missing %q:\n%s", expected, text)
		}
	}
}
