package knowledge

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/JieWaZi/NeuxAlly/server/internal/chatstorage"
	"github.com/JieWaZi/NeuxAlly/server/internal/workspace"
	chatservice "github.com/JieWaZi/NeuxAlly/server/services/chat"
	"path/filepath"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/JieWaZi/neux-knowledge/core"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/dbhelper"
)

func TestLibraryDeleteRejectsStaleVersionAndCleansTaskConfiguration(t *testing.T) {
	ctx := context.Background()
	engine, err := core.Open(ctx, core.Options{DBPath: filepath.Join(t.TempDir(), "knowledge.db"), RawDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	database, err := dbhelper.NewDB(conf.DB{Type: conf.Sqlite, DBName: filepath.Join(t.TempDir(), "ally.db"), Migrate: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB.DB()
	defer sqlDB.Close()
	oldEngine, oldRefs := global.KnowledgeEngine, global.KnowledgeControllerManager
	global.KnowledgeEngine = engine
	global.KnowledgeControllerManager = controllers.NewKnowledgeController(database.DB)
	defer func() { global.KnowledgeEngine = oldEngine; global.KnowledgeControllerManager = oldRefs }()
	g := &gin.Context{}
	value, err := (&CreateLibrary{Body: CreateLibraryInput{RequestID: "delete-library-create", Name: "永久删除", WikiLanguage: "zh-CN"}}).Output(g)
	if err != nil {
		t.Fatal(err)
	}
	id := value.(Receipt).ResourceID
	_, err = (&ImportSource{LibraryID: id, Body: ImportSourceInput{RequestID: "delete-library-import", Filename: "private.md", Bytes: base64.StdEncoding.EncodeToString([]byte("private original"))}}).Output(g)
	if err != nil {
		t.Fatal(err)
	}
	library, err := engine.GetLibrary(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.DB.Create(&models.KnowledgeTaskConfiguration{ID: "configuration", RuntimeID: "runtime", Backend: "local", ConnectionID: "local", KnowledgeBaseID: id}).Error; err != nil {
		t.Fatal(err)
	}
	_, err = (&DeleteLibrary{LibraryID: id, Body: DeleteLibraryInput{RequestID: "stale-delete", ExpectedVersion: library.Version + 1}}).Output(g)
	if !errors.Is(err, status_error.KnowledgeConflict) {
		t.Fatal(err)
	}
	var refs []models.KnowledgeTaskConfiguration
	err = database.DB.Where("knowledge_base_id = ?", id).Find(&refs).Error
	if err != nil || len(refs) != 1 {
		t.Fatal(refs, err)
	}
	operation := &DeleteLibrary{LibraryID: id, Body: DeleteLibraryInput{RequestID: "confirmed-delete", ExpectedVersion: library.Version}}
	value, err = operation.Output(g)
	if err != nil {
		t.Fatal(err)
	}
	if value.(Receipt).State != "accepted" {
		t.Fatal(value)
	}
	if _, err = engine.GetLibrary(ctx, id); err == nil {
		t.Fatal("deleted library remains readable")
	}
	refs = nil
	err = database.DB.Where("knowledge_base_id = ?", id).Find(&refs).Error
	if err != nil || len(refs) != 0 {
		t.Fatal(refs, err)
	}
	statusValue, err := (&GetLibraryDeletion{LibraryID: id}).Output(g)
	if err != nil {
		t.Fatal(err)
	}
	status := statusValue.(*core.LibraryDeletionStatus)
	if status.State != "completed" || status.DeletedBlobCount != 1 || status.PendingBlobCount != 0 {
		t.Fatal(status)
	}
	if _, err = operation.Output(g); err != nil {
		t.Fatal("same request cannot replay after logical removal", err)
	}
}

func TestLibraryDeleteLeavesAllChatsToTheirOriginalLifecycle(t *testing.T) {
	ctx := context.Background()
	engine, err := core.Open(ctx, core.Options{DBPath: filepath.Join(t.TempDir(), "knowledge.db"), RawDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	database, err := dbhelper.NewDB(conf.DB{Type: conf.Sqlite, DBName: filepath.Join(t.TempDir(), "ally.db"), Migrate: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB.DB()
	defer sqlDB.Close()
	root := t.TempDir()
	layout, err := chatstorage.NewLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := chatservice.New(chatservice.Config{
		StorageDirectory: root, Projects: controllers.NewProjectController(database.DB),
		QueryIndexes: chatservice.QueryIndexDependencies{
			Chats: controllers.NewChatController(database.DB), RuntimeSessions: controllers.NewRuntimeSessionController(database.DB),
			DeletionIntents: controllers.NewChatDeletionIntentController(database.DB),
			Transactions:    controllers.NewTransactionController(dbhelper.NewGormTransactionManager(database.DB)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	oldEngine, oldRefs, oldChats, oldService := global.KnowledgeEngine, global.KnowledgeControllerManager, global.ChatService, global.KnowledgeService
	global.KnowledgeEngine, global.KnowledgeControllerManager, global.ChatService, global.KnowledgeService = engine, controllers.NewKnowledgeController(database.DB), manager, nil
	defer func() {
		global.KnowledgeEngine, global.KnowledgeControllerManager, global.ChatService, global.KnowledgeService = oldEngine, oldRefs, oldChats, oldService
	}()
	receipt, err := engine.CreateLibrary(ctx, core.CreateLibraryInput{RequestID: "create", Name: "删除测试", Kind: "general", WikiLanguage: "zh-CN"})
	if err != nil {
		t.Fatal(err)
	}
	library, err := engine.GetLibrary(ctx, receipt.LibraryID)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"11111111111111111111111111111111", "22222222222222222222222222222222", "33333333333333333333333333333333", "44444444444444444444444444444444"} {
		dir, err := layout.ChatWorkspace(id)
		if err != nil {
			t.Fatal(err)
		}
		if err = workspace.EnsureExecutionWorkspace(dir, id, "waker", nil); err != nil {
			t.Fatal(err)
		}
		if _, err = manager.CreateDormantChats(ctx, []chatservice.DormantChat{{Chat: models.Chat{ID: id, WakerID: "waker", WorkspaceDirectory: dir, AdditionalWorkingDirectories: []string{dir}}}}); err != nil {
			t.Fatal(err)
		}
		kb := library.ID
		if id == "44444444444444444444444444444444" {
			kb = "another-library"
		}
		if err = global.KnowledgeControllerManager.BeginOperation(ctx, models.KnowledgeOperationLink{RequestID: id, Operation: "chat.organize", Digest: id, Backend: "local", ConnectionID: "local", KnowledgeBaseID: kb, ChatID: id, ReceiptConfirmed: true}); err != nil {
			t.Fatal(err)
		}
	}

	operation := &DeleteLibrary{LibraryID: library.ID, Body: DeleteLibraryInput{RequestID: "delete", ExpectedVersion: library.Version}}
	if _, err = operation.Output(&gin.Context{}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"11111111111111111111111111111111", "22222222222222222222222222222222", "33333333333333333333333333333333", "44444444444444444444444444444444"} {
		if _, err = manager.GetChatSummary(ctx, id); err != nil {
			t.Fatalf("unrelated chat removed: %s %v", id, err)
		}
	}
	if _, err = operation.Output(&gin.Context{}); err != nil {
		t.Fatalf("delete replay failed: %v", err)
	}
}
