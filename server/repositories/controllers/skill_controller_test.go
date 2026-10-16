package controllers

import (
	"context"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
)

func TestSkillControllerVisibleStateAndEnabledSkills(t *testing.T) {
	controller := NewSkillController(newTestDB(t))
	ctx := context.Background()
	public := models.SkillPackage{ID: "skill-public-000000000000000000000000", Name: "审查", Scope: constants.SkillScopePublic, Source: constants.SkillSourceGit, BuiltIn: true, CurrentVersionID: "version-public-0000000000000000"}
	private := models.SkillPackage{ID: "skill-private-000000000000000000000000", Name: "私有", Scope: constants.SkillScopePrivate, Source: constants.SkillSourceZip, OwnerWakerID: "waker-0000000000000000000000000000", CurrentVersionID: "version-private-000000000000000"}
	if err := controller.CreatePackage(ctx, public); err != nil {
		t.Fatal(err)
	}
	if err := controller.CreatePackage(ctx, private); err != nil {
		t.Fatal(err)
	}
	if err := controller.CreateVersion(ctx, models.SkillVersion{ID: public.CurrentVersionID, SkillPackageID: public.ID, Version: "1.0.0", ContentPath: "skills/versions/public/1", ContentDigest: "digest"}, true, public.CurrentVersionID); err != nil {
		t.Fatal(err)
	}
	if err := controller.CreateVersion(ctx, models.SkillVersion{ID: private.CurrentVersionID, SkillPackageID: private.ID, Version: "1.0.0", ContentPath: "skills/versions/private/1", ContentDigest: "digest"}, true, private.CurrentVersionID); err != nil {
		t.Fatal(err)
	}
	items, err := controller.ListVisible(ctx, private.OwnerWakerID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Enabled {
		t.Fatalf("visible skill state = %+v", items)
	}
	if err := controller.EnsureWakerSkill(ctx, models.WakerSkill{
		ID: "default-relation", WakerID: private.OwnerWakerID, SkillPackageID: public.ID, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	defaults, err := controller.ListEnabled(ctx, private.OwnerWakerID)
	if err != nil || len(defaults) != 1 || defaults[0].SkillPackage.ID != public.ID {
		t.Fatalf("explicit default skills = %+v, error=%v", defaults, err)
	}
	if err := controller.UpsertWakerSkill(ctx, models.WakerSkill{ID: "relation-000000000000000000000000000", WakerID: private.OwnerWakerID, SkillPackageID: public.ID, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	enabled, err := controller.ListEnabled(ctx, private.OwnerWakerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(enabled) != 0 {
		t.Fatalf("enabled skills = %d, want 0", len(enabled))
	}
	if err := controller.EnsureWakerSkill(ctx, models.WakerSkill{
		ID: "another-default", WakerID: private.OwnerWakerID, SkillPackageID: public.ID, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	enabled, err = controller.ListEnabled(ctx, private.OwnerWakerID)
	if err != nil || len(enabled) != 0 {
		t.Fatalf("bootstrap overwrote disabled preference: %+v %v", enabled, err)
	}
}

func TestSkillControllerCurrentVersionSynchronizesVersionedMetadata(t *testing.T) {
	controller := NewSkillController(newTestDB(t))
	ctx := context.Background()
	skill := models.SkillPackage{
		ID:          "skill-metadata-0000000000000000000000",
		Name:        "old-name",
		Description: "旧说明",
		Scope:       constants.SkillScopePublic,
		Source:      constants.SkillSourceZip,
	}
	if err := controller.CreatePackage(ctx, skill); err != nil {
		t.Fatal(err)
	}
	version := models.SkillVersion{
		ID:             "version-metadata-000000000000000000",
		SkillPackageID: skill.ID,
		Version:        "1.1.0",
		Name:           "new-name",
		Description:    "新说明",
		ContentPath:    "skills/versions/metadata/1",
		ContentDigest:  "digest",
	}
	if err := controller.CreateVersion(ctx, version, true, ""); err != nil {
		t.Fatal(err)
	}
	updated, err := controller.GetPackage(ctx, skill.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CurrentVersionID != version.ID || updated.Name != skill.Name || updated.Description != version.Description {
		t.Fatalf("updated skill = %+v", updated)
	}
}

func TestSkillControllerRejectsStaleCurrentVersionUpdate(t *testing.T) {
	controller := NewSkillController(newTestDB(t))
	ctx := context.Background()
	skill := models.SkillPackage{
		ID:               "skill-concurrency-000000000000000000",
		Name:             "review",
		Scope:            constants.SkillScopePublic,
		Source:           constants.SkillSourceZip,
		CurrentVersionID: "version-1-00000000000000000000000",
	}
	if err := controller.CreatePackage(ctx, skill); err != nil {
		t.Fatal(err)
	}
	first := models.SkillVersion{
		ID: skill.CurrentVersionID, SkillPackageID: skill.ID, Version: "1.0.0",
		Name: skill.Name, ContentPath: "skills/versions/concurrency/1.0.0", ContentDigest: "one",
	}
	if err := controller.CreateVersion(ctx, first, true, first.ID); err != nil {
		t.Fatal(err)
	}
	second := models.SkillVersion{
		ID: "version-2-00000000000000000000000", SkillPackageID: skill.ID, Version: "2.0.0",
		Name: skill.Name, ContentPath: "skills/versions/concurrency/2.0.0", ContentDigest: "two",
	}
	if err := controller.CreateVersion(ctx, second, true, first.ID); err != nil {
		t.Fatal(err)
	}
	stale := models.SkillVersion{
		ID: "version-stale-00000000000000000000", SkillPackageID: skill.ID, Version: "1.5.0",
		Name: skill.Name, ContentPath: "skills/versions/concurrency/1.5.0", ContentDigest: "stale",
	}
	if err := controller.CreateVersion(ctx, stale, true, first.ID); err == nil {
		t.Fatal("stale current version update succeeded")
	}
	updated, err := controller.GetPackage(ctx, skill.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CurrentVersionID != second.ID {
		t.Fatalf("current version = %q, want %q", updated.CurrentVersionID, second.ID)
	}
	if _, err := controller.GetVersion(ctx, stale.ID); err == nil {
		t.Fatal("stale version record was not rolled back")
	}
}
