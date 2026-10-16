package controllers

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/dbhelper"
)

func TestAttentionNoticeControllerDismissAndList(t *testing.T) {
	database, err := dbhelper.NewDB(conf.DB{
		Type:    conf.Sqlite,
		DBName:  filepath.Join(t.TempDir(), "attention-notice.db"),
		Migrate: true,
	})
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	sqlDB, err := database.DB.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	controller := NewAttentionNoticeController(database.DB)
	ctx := context.Background()

	if dismissed, err := controller.IsDismissed(ctx, "approval:test"); err != nil || dismissed {
		t.Fatalf("IsDismissed() = %v, %v, want false", dismissed, err)
	}
	if err := controller.Dismiss(ctx, "approval:test", 100); err != nil {
		t.Fatalf("Dismiss() error = %v", err)
	}
	if err := controller.Dismiss(ctx, "approval:test", 200); err != nil {
		t.Fatalf("Dismiss() again error = %v", err)
	}
	ids, err := controller.ListDismissedIDs(ctx)
	if err != nil {
		t.Fatalf("ListDismissedIDs() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != "approval:test" {
		t.Fatalf("ListDismissedIDs() = %v, want [approval:test]", ids)
	}
}
