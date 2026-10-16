package builtinrolepreset

import (
	"strings"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
)

func TestResolveRolePresetContentsUsesBuiltinDefinition(t *testing.T) {
	contents, err := ResolveRolePresetContents(
		PersonalAssistantRolePresetID,
		[]models.RolePresetDocument{{
			Kind:    constants.ProfileDocumentKindIdentity,
			Content: "stale database copy",
		}},
	)
	if err != nil {
		t.Fatalf("ResolveRolePresetContents() error = %v", err)
	}
	definition, exists := DefinitionByID(PersonalAssistantRolePresetID)
	if !exists {
		t.Fatal("missing personal assistant definition")
	}
	if contents[constants.ProfileDocumentKindIdentity] !=
		definition.Documents[constants.ProfileDocumentKindIdentity] {
		t.Fatal("builtin role preset must ignore stored documents")
	}
}

func TestResolveWakerContentsFallsBackToBuiltinDefinition(t *testing.T) {
	contents, err := ResolveWakerContents(PersonalAssistantWakerID, nil)
	if err != nil {
		t.Fatalf("ResolveWakerContents() error = %v", err)
	}
	definition, exists := DefinitionByWakerID(PersonalAssistantWakerID)
	if !exists {
		t.Fatal("missing personal assistant waker definition")
	}
	if contents[constants.ProfileDocumentKindPlaybook] !=
		definition.Documents[constants.ProfileDocumentKindPlaybook] {
		t.Fatal("builtin waker must fall back to catalog documents")
	}
}

func TestPersonalAssistantDocumentsAreAgentOnly(t *testing.T) {
	definition, exists := DefinitionByID(PersonalAssistantRolePresetID)
	if !exists {
		t.Fatal("missing personal assistant definition")
	}
	forbidden := []string{"## 擅长领域", "## 工作风格", "## 工作方式"}
	for kind, content := range definition.Documents {
		for _, heading := range forbidden {
			if strings.Contains(content, heading) {
				t.Fatalf("document %q must not contain UI section %q", kind, heading)
			}
		}
	}
	if len(definition.DisplayProjection().Expertise) != 5 {
		t.Fatalf("expertise count = %d, want 5", len(definition.DisplayProjection().Expertise))
	}
}

func TestResolveWakerContentsPrefersStoredOverride(t *testing.T) {
	contents, err := ResolveWakerContents(
		PersonalAssistantWakerID,
		[]models.WakerDocument{{
			Kind:    constants.ProfileDocumentKindPersona,
			Content: "## 工作风格\n\n- 用户定制",
		}},
	)
	if err != nil {
		t.Fatalf("ResolveWakerContents() error = %v", err)
	}
	if contents[constants.ProfileDocumentKindPersona] != "## 工作风格\n\n- 用户定制" {
		t.Fatalf("stored override was ignored: %q", contents[constants.ProfileDocumentKindPersona])
	}
}
