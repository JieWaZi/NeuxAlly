package builtin

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability/contract"
)

func TestSystemGuidesAndRelativeLinksResolveBundledResources(t *testing.T) {
	item, err := systemDefinition()
	if err != nil {
		t.Fatal(err)
	}
	resources := make(map[string]bool, len(item.Files))
	for _, resource := range item.Files {
		resources[resource.Path] = true
	}
	links := regexp.MustCompile(`\]\(([^)]+)\)`)
	for _, resource := range item.Files {
		for _, match := range links.FindAllSubmatch(resource.Data, -1) {
			target := strings.SplitN(string(match[1]), "#", 2)[0]
			if target == "" || strings.Contains(target, "://") {
				continue
			}
			if !resources[path.Clean(path.Join(path.Dir(resource.Path), target))] {
				t.Errorf("broken bundled link: %s -> %s", resource.Path, target)
			}
		}
	}
	for _, operation := range contract.Operations() {
		for _, id := range operation.Guides {
			if content, err := Guide(id); err != nil || content == "" {
				t.Errorf("operation guide unavailable: %s -> %s: %v", operation.ID, id, err)
			}
		}
	}
}

func TestSystemSkillRejectsModifiedResourcesAndRedirectedDirectory(t *testing.T) {
	data := t.TempDir()
	directory, err := SystemDirectory(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte("modified"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SystemDirectory(data); !errors.Is(err, status_error.SkillContentInvalid) {
		t.Fatalf("modified system skill accepted: %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	data = t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(data, "system-skills")); err != nil {
		t.Fatal(err)
	}
	if _, err := SystemDirectory(data); !errors.Is(err, status_error.SkillContentInvalid) {
		t.Fatalf("redirected system skill directory accepted: %v", err)
	}
}
