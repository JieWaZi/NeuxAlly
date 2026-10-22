package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/JieWaZi/neux-knowledge/core"
	"github.com/JieWaZi/neux-knowledge/httpapi"
	"testing"
)

func regressionJob(t *testing.T, s *Service, lib *core.Library) *core.Job {
	t.Helper()
	ctx := context.Background()
	_, e := s.config.Engine.ImportSource(ctx, core.ImportSourceInput{RequestID: "source", LibraryID: lib.ID, Filename: "a.md", Bytes: []byte("原始依据")})
	if e != nil {
		t.Fatal(e)
	}
	r, _, e := s.Organize(ctx, OrganizeCommand{RequestID: "job", LibraryID: lib.ID, Configuration: models.KnowledgeTaskConfiguration{RuntimeID: constants.RuntimeIDCodex}})
	if e != nil {
		t.Fatal(e)
	}
	j, e := s.config.Engine.GetJob(ctx, r.ResourceID)
	if e != nil {
		t.Fatal(e)
	}
	return j
}
func TestDefaultOrganizeSelectsOnlyCurrentSourceHeads(t *testing.T) {
	s, lib := newServiceFixture(t)
	ctx := context.Background()
	r, e := s.config.Engine.ImportSource(ctx, core.ImportSourceInput{RequestID: "v1", LibraryID: lib.ID, Filename: "a.md", Bytes: []byte("v1")})
	if e != nil {
		t.Fatal(e)
	}
	v, _ := s.config.Engine.GetSource(ctx, lib.ID, r.ResourceID)
	_, e = s.config.Engine.ImportSource(ctx, core.ImportSourceInput{RequestID: "v2", LibraryID: lib.ID, SourceID: v.SourceID, ExpectedVersion: v.Version, Filename: "a.md", Bytes: []byte("v2")})
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = s.Organize(ctx, OrganizeCommand{RequestID: "job", LibraryID: lib.ID, Configuration: models.KnowledgeTaskConfiguration{RuntimeID: constants.RuntimeIDCodex}})
	if e != nil {
		t.Fatalf("default organize rejected replaced pending source: %v", e)
	}
}
func TestStartIgnoresHistoryOfDeletedLibrary(t *testing.T) {
	s, lib := newServiceFixture(t)
	ctx := context.Background()
	job := regressionJob(t, s, lib)
	if _, err := s.config.Engine.CancelOrganize(ctx, core.JobActionInput{RequestID: "cancel-before-delete", JobID: job.ID, ExpectedVersion: job.Version}); err != nil {
		t.Fatal(err)
	}
	current, err := s.config.Engine.GetLibrary(ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.config.Engine.DeleteLibrary(ctx, core.DeleteLibraryInput{RequestID: "delete", LibraryID: lib.ID, ExpectedVersion: current.Version}); err != nil {
		t.Fatal(err)
	}
	if err = s.config.References.DeleteLibraryReferences(ctx, "local", "local", lib.ID); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Start(ctx); err != nil {
		t.Fatalf("deleted historical job blocked startup: %v", err)
	}
	security, err := httpapi.NewPolicyStore(httpapi.SecurityConfig{Tokens: map[string]httpapi.Principal{"test-token": {Subject: "test-user"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewRemoteAdapter(s, ProfileMap{"test": {RuntimeID: "codex"}}, security); err != nil {
		t.Fatalf("deleted history blocked remote adapter startup: %v", err)
	}

	if _, err = s.config.References.GetOperation(ctx, "job"); err != nil {
		t.Fatalf("historical audit anchor was removed: %v", err)
	}
}

// startupConfigurationReferences 用稳定错误模拟当前任务配置缺失与数据库故障。
type startupConfigurationReferences struct {
	// KnowledgeController 保留真实操作枚举与查询。
	controllers.KnowledgeController
	// configurationError 是任务配置读取结果。
	configurationError error
}

func (r startupConfigurationReferences) GetTaskConfiguration(context.Context, string) (models.KnowledgeTaskConfiguration, error) {
	return models.KnowledgeTaskConfiguration{}, r.configurationError
}
func TestStartRejectsMissingConfigurationAndPreservesDatabaseFailure(t *testing.T) {
	for _, expected := range []error{status_error.KnowledgeNotFound, status_error.DatabaseBusy} {
		t.Run(expected.Error(), func(t *testing.T) {
			s, lib := newServiceFixture(t)
			regressionJob(t, s, lib)
			s.config.References = startupConfigurationReferences{KnowledgeController: s.config.References, configurationError: expected}
			defer s.Close()
			err := s.Start(context.Background())
			if !errors.Is(err, expected) {
				t.Fatalf("task configuration failure swallowed: %v", err)
			}
			guardErr := s.SetRemoteGuard(func(context.Context, json.RawMessage) (func(), error) { return func() {}, nil })
			if !errors.Is(guardErr, expected) {
				t.Fatalf("remote guard swallowed task configuration failure: %v", guardErr)
			}
		})
	}
}

func TestRemoteGuardIgnoresDeletedJobWithRetainedConfiguration(t *testing.T) {
	s, lib := newServiceFixture(t)
	ctx := context.Background()
	if err := s.config.References.BeginOperation(ctx, models.KnowledgeOperationLink{RequestID: "stale-remote", Operation: "job.accept", Digest: "stale", Backend: "local", ConnectionID: "local", KnowledgeBaseID: lib.ID, ResourceID: "missing-job", ConfigurationID: "configuration", ReceiptConfirmed: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.config.References.FreezeTaskConfiguration(ctx, models.KnowledgeTaskConfiguration{ID: "configuration", KnowledgeBaseID: lib.ID, RuntimeID: "codex", RemoteSubject: "remote-user"}); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRemoteGuard(func(context.Context, json.RawMessage) (func(), error) { return func() {}, nil }); err != nil {
		t.Fatalf("missing remote job blocked startup: %v", err)
	}
}
