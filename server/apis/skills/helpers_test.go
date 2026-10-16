package skills

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/dbhelper"
)

func TestWakerSkillResponsesExposeSharedChangeImpact(t *testing.T) {
	database, err := dbhelper.NewDB(conf.DB{
		Type:    conf.Sqlite,
		DBName:  filepath.Join(t.TempDir(), "skill-impact.db"),
		Migrate: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	previousWakers := global.WakerControllerManager
	t.Cleanup(func() { global.WakerControllerManager = previousWakers })
	global.WakerControllerManager = controllers.NewWakerController(database.DB)
	ctx := context.Background()
	for _, waker := range []models.Waker{
		{ID: "waker-1", Name: "伙伴一"},
		{ID: "waker-2", Name: "伙伴二"},
	} {
		if err := global.WakerControllerManager.Create(ctx, waker); err != nil {
			t.Fatal(err)
		}
	}
	responses, err := wakerSkillResponses(ctx, []models.SkillWithState{
		{SkillPackage: models.SkillPackage{ID: "public", Scope: constants.SkillScopePublic}},
		{SkillPackage: models.SkillPackage{ID: "private", Scope: constants.SkillScopePrivate}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if responses[0].AffectedWakerCount != 2 {
		t.Fatalf("public affected wakers = %d, want 2", responses[0].AffectedWakerCount)
	}
	if responses[1].AffectedWakerCount != 1 {
		t.Fatalf("private affected wakers = %d, want 1", responses[1].AffectedWakerCount)
	}
}
