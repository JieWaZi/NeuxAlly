import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  localApiErrorMessage,
  withLocalErrorMutation,
  withLocalErrorSurface,
} from "@/shared/api/local-error-surface";
import { Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";

import { WakerDetailLayout } from "@/modules/waker";
import { Button } from "@/shared/ui/button";
import { PageHeader } from "@/shared/ui/page-header";
import { PageAction, ResourcePageSurface } from "@/shared/ui/resource-page";
import { SegmentedControl } from "@/shared/ui/segmented-control";
import { Skeleton } from "@/shared/ui/skeleton";
import { Switch } from "@/shared/ui/switch";
import { useTheme } from "@/shared/theme";

import {
  createSkillVersionMutation,
  getSkillVersionOptions,
  getWakerSkillOptions,
  invalidateVisibleSkillQueries,
  listSkillVersionsOptions,
  pullSkillUpdateMutation,
  restoreSkillVersionMutation,
  setWakerSkillEnabledMutation,
  updateWakerSkillMutation,
} from "../api/skill-queries";
import { SkillDeleteDialog } from "../components/skill-delete-dialog";
import { SkillDisplayNameEditor } from "../components/skill-display-name-editor";
import { SkillGitPullDialog } from "../components/skill-git-pull-dialog";
import { SkillIcon } from "../components/skill-icon";
import { SkillInstructionsPanel } from "../components/skill-instructions-panel";
import { SkillMetadataCard } from "../components/skill-metadata-card";
import { SkillResourceBrowser } from "../components/skill-resource-browser";
import {
  SkillVersionDiffDialog,
  type DiffSkillVersion,
} from "../components/skill-version-diff-dialog";
import { SkillVersionHistory } from "../components/skill-version-history";
import {
  buildSkillResourceTree,
  countSkillResourceFiles,
} from "../model/skill-detail";

type DetailTab = "explanation" | "resources" | "version";

export function SkillDetailPage() {
  const { t } = useTranslation("skill");
  const { skillId = "", wakerId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { resolvedTheme } = useTheme();
  const [tab, setTab] = useState<DetailTab>("explanation");
  const [diffVersionId, setDiffVersionId] = useState<string | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const detailOptions = getWakerSkillOptions(
    withLocalErrorSurface({
      path: { wakerId, skillId },
    }),
  );
  const detailQuery = useQuery({
    ...detailOptions,
    enabled: Boolean(wakerId && skillId),
  });
  const versionsQuery = useQuery({
    ...listSkillVersionsOptions(
      withLocalErrorSurface({ path: { wakerId, skillId } }),
    ),
    enabled: Boolean(wakerId && skillId),
  });
  const skill = detailQuery.data?.skill;
  const currentVersion = detailQuery.data?.currentVersion;
  const currentVersionId = currentVersion?.id ?? "";
  const currentVersionQuery = useQuery({
    ...getSkillVersionOptions(
      withLocalErrorSurface({
        path: {
          wakerId,
          skillId,
          versionId: currentVersionId || "pending",
        },
      }),
    ),
    enabled: Boolean(wakerId && skillId && currentVersionId),
  });
  const diffVersionQuery = useQuery({
    ...getSkillVersionOptions(
      withLocalErrorSurface({
        path: {
          wakerId,
          skillId,
          versionId: diffVersionId ?? "pending",
        },
      }),
    ),
    enabled: Boolean(diffVersionId),
  });

  const refresh = async () => {
    await invalidateVisibleSkillQueries(queryClient);
  };
  const toggleSkill = useMutation({
    ...withLocalErrorMutation(setWakerSkillEnabledMutation()),
    onSuccess: refresh,
  });
  const createVersion = useMutation({
    ...withLocalErrorMutation(createSkillVersionMutation()),
    onSuccess: refresh,
  });
  const restoreVersion = useMutation({
    ...withLocalErrorMutation(restoreSkillVersionMutation()),
    onSuccess: async () => {
      setDiffVersionId(null);
      await refresh();
    },
  });
  const pullUpdate = useMutation({
    ...withLocalErrorMutation(pullSkillUpdateMutation()),
    onSuccess: refresh,
  });
  const updateSkill = useMutation({
    ...withLocalErrorMutation(updateWakerSkillMutation()),
    onSuccess: refresh,
  });
  const resources = useMemo(
    () => buildSkillResourceTree(currentVersionQuery.data?.resources ?? []),
    [currentVersionQuery.data?.resources],
  );
  const selectedDiffVersion = diffVersionQuery.data
    ? ({
        id: diffVersionQuery.data.version?.id ?? "",
        version: diffVersionQuery.data.version?.version ?? "",
        instructions: diffVersionQuery.data.content ?? "",
        resources: diffVersionQuery.data.resources ?? [],
      } satisfies DiffSkillVersion)
    : null;
  const backAction = {
    label: t("detail.backToList"),
    onClick: () => navigate(`/wakers/${wakerId}/skills`),
  };

  if (detailQuery.isPending) {
    return (
      <WakerDetailLayout active="skills">
        <ResourcePageSurface>
          <PageHeader
            layout="detail"
            title={t("title")}
            backAction={backAction}
          />
          <div className="mt-6 grid gap-4">
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-40 w-full" />
            <Skeleton className="h-80 w-full" />
          </div>
        </ResourcePageSurface>
      </WakerDetailLayout>
    );
  }
  if (detailQuery.isError || !skill) {
    return (
      <WakerDetailLayout active="skills">
        <ResourcePageSurface>
          <PageHeader
            layout="detail"
            title={t("title")}
            backAction={backAction}
          />
          <div className="mt-6 grid place-items-center p-6 text-center">
            <div>
              <p className="type-label font-medium">{t("detail.notFound")}</p>
              <Button
                size="sm"
                variant="secondary"
                className="mt-3"
                onClick={() => void detailQuery.refetch()}
              >
                {t("common:retry")}
              </Button>
            </div>
          </div>
        </ResourcePageSurface>
      </WakerDetailLayout>
    );
  }

  const currentVersionNumber = currentVersion?.version ?? "";
  const sourceLabel =
    skill.source === "git"
      ? t("source.publicGit")
      : skill.source === "zip"
        ? t("source.zipImport")
        : t("source.allyBuiltIn");
  const scopeLabel =
    skill.builtIn || skill.source === "builtin"
      ? t("scope.built_in")
      : skill.scope === "public"
        ? t("scope.public")
        : t("scope.private");
  const instructions = currentVersionQuery.data?.content ?? "";
  const metadataName = currentVersion?.name ?? "";
  const metadataDescription =
    currentVersion?.description ?? skill.description ?? "";
  const builtIn = Boolean(skill.builtIn || skill.source === "builtin");
  const saveVersion = (
    body: Parameters<typeof createVersion.mutate>[0]["body"],
    onSuccess?: () => void,
  ) =>
    createVersion.mutate({ path: { wakerId, skillId }, body }, { onSuccess });

  return (
    <WakerDetailLayout active="skills">
      <ResourcePageSurface>
        <PageHeader
          layout="detail"
          backAction={backAction}
          visual={<SkillIcon />}
          title={skill.name ?? ""}
          actions={
            <div className="grid max-w-full justify-items-end gap-2">
              <div className="flex max-w-full flex-wrap items-center gap-2">
                <SkillDisplayNameEditor
                  name={skill.name ?? ""}
                  pending={updateSkill.isPending}
                  error={updateSkill.error}
                  onSave={(name, onSuccess) =>
                    updateSkill.mutate(
                      { path: { wakerId, skillId }, body: { name } },
                      { onSuccess },
                    )
                  }
                />

                {skill.source === "git" ? (
                  <SkillGitPullDialog
                    currentVersion={currentVersionNumber}
                    error={pullUpdate.isError}
                    pending={pullUpdate.isPending}
                    onPull={(version, notes, onSuccess) =>
                      pullUpdate.mutate(
                        {
                          path: { wakerId, skillId },
                          body: { version, notes },
                        },
                        { onSuccess },
                      )
                    }
                  />
                ) : null}
                {!builtIn ? (
                  <PageAction
                    intent="delete"
                    label={t("detail.uninstall")}
                    onClick={() => setDeleteOpen(true)}
                  >
                    <Trash2 />
                  </PageAction>
                ) : null}
                <Switch
                  hitArea="comfortable"
                  checked={detailQuery.data?.enabled ?? false}
                  disabled={toggleSkill.isPending}
                  aria-label={t(
                    detailQuery.data?.enabled
                      ? "detail.disable"
                      : "detail.enable",
                    { name: skill.name },
                  )}
                  onClick={() =>
                    toggleSkill.mutate({
                      path: { wakerId, skillId },
                      body: { enabled: !(detailQuery.data?.enabled ?? false) },
                    })
                  }
                />
                <span className="text-content-secondary type-body">
                  {detailQuery.data?.enabled ? t("enabled") : t("disabled")}
                </span>
              </div>
              {toggleSkill.isError ? (
                <p role="alert" className="text-destructive type-caption">
                  {localApiErrorMessage(toggleSkill.error, t("toggleFailed"))}
                </p>
              ) : null}
            </div>
          }
        >
          <div className="mt-2 flex items-center gap-2">
            <span className="bg-muted text-muted-foreground type-caption shrink-0 rounded-full px-2 py-1">
              v{currentVersionNumber}
            </span>
            <span className="bg-muted text-muted-foreground type-caption shrink-0 rounded-full px-2 py-1">
              {scopeLabel}
            </span>
          </div>
        </PageHeader>

        <main className="min-w-0">
          <SkillMetadataCard
            name={metadataName}
            description={metadataDescription}
            source={sourceLabel}
          />
          <section className="mt-8">
            <SegmentedControl
              value={tab}
              onValueChange={setTab}
              ariaLabel={t("detail.tabs")}
              size="sm"
              options={[
                { value: "explanation", label: t("detail.explanation") },
                {
                  value: "resources",
                  label: `${t("detail.resources")} ${countSkillResourceFiles(resources)}`,
                },
                {
                  value: "version",
                  label: t("detail.currentVersionTab", {
                    version: currentVersionNumber,
                  }),
                },
              ]}
            />
            <div className="mt-4">
              <div hidden={tab !== "explanation"}>
                <SkillInstructionsPanel content={instructions} />
              </div>
              <div hidden={tab !== "resources"}>
                {currentVersionQuery.data ? (
                  <SkillResourceBrowser
                    key={`${skillId}:${currentVersionId}`}
                    resources={resources}
                    instructions={instructions}
                    wakerId={wakerId}
                    skillId={skillId}
                    versionId={currentVersionId}
                    currentVersion={currentVersionNumber}
                    pending={createVersion.isPending}
                    error={createVersion.isError}
                    onSave={saveVersion}
                  />
                ) : (
                  <Skeleton className="h-96 w-full" />
                )}
              </div>
              <div hidden={tab !== "version"}>
                <SkillVersionHistory
                  currentVersionId={currentVersionId}
                  currentVersion={currentVersionNumber}
                  onVersionClick={(version) =>
                    version.id && setDiffVersionId(version.id)
                  }
                  versions={versionsQuery.data ?? []}
                />
              </div>
            </div>
          </section>
        </main>
      </ResourcePageSurface>

      <SkillVersionDiffDialog
        key={diffVersionId ?? "closed"}
        applying={restoreVersion.isPending}
        currentContent={instructions}
        currentVersionId={currentVersionId}
        currentResources={currentVersionQuery.data?.resources ?? []}
        currentVersion={currentVersionNumber}
        onApply={() => {
          if (!diffVersionId) return;
          restoreVersion.mutate({
            path: {
              wakerId,
              skillId,
              versionId: diffVersionId,
            },
            body: {
              notes: "",
            },
          });
        }}
        onOpenChange={(open) => {
          if (!open) setDiffVersionId(null);
        }}
        open={Boolean(diffVersionId && selectedDiffVersion)}
        selectedVersion={selectedDiffVersion}
        skillId={skillId}
        wakerId={wakerId}
        theme={resolvedTheme}
        t={t}
      />
      {deleteOpen ? (
        <SkillDeleteDialog
          key={skillId}
          open
          skills={[
            {
              affectedWakerCount: detailQuery.data?.affectedWakerCount ?? 1,
              id: skillId,
              name: skill.name ?? "",
              scope: skill.scope === "public" ? "public" : "private",
            },
          ]}
          wakerId={wakerId}
          onOpenChange={setDeleteOpen}
          onDeleted={() => navigate(`/wakers/${wakerId}/skills`)}
        />
      ) : null}
    </WakerDetailLayout>
  );
}
