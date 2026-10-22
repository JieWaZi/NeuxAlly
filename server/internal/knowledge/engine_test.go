package knowledge

import (
	"errors"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/neux-knowledge/core"
)

func TestSourceUnavailableMapsToConflictWithoutPermissionEscalation(t *testing.T) {
	err := MapError(&core.Error{Code: core.ErrorCode("source_unavailable"), Operation: "job.sources", Detail: "accepted source was replaced"})
	if !errors.Is(err, status_error.KnowledgeConflict) {
		t.Fatalf("source lifecycle change became an unrelated permission/storage failure: %v", err)
	}
	if err = MapError(&core.Error{Code: core.ErrScopeViolation, Operation: "job", Detail: "permission revoked"}); !errors.Is(err, status_error.KnowledgeForbidden) {
		t.Fatalf("actual revoked authorization lost its boundary: %v", err)
	}
}
