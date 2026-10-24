import { useQueryEvents } from "@/shared/api/use-query-events";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import { listLibrarySkillsOptions } from "@/shared/api/generated/@tanstack/react-query.gen";
import {
  withLocalErrorSurface,
  withLocalErrorMutation,
  localApiErrorMessage,
} from "@/shared/api/local-error-surface";
import { ResourceCardGrid } from "@/shared/ui/resource-page";
import { Button } from "@/shared/ui/button";
import { QueryState } from "@/shared/ui/query-state";
import {
  listWakerSkillsOptions,
  setWakerSkillEnabledMutation,
  invalidateVisibleSkillQueries,
} from "../api/skill-queries";
import { SkillCard } from "../components/skill-card";
import { SkillImportDialog } from "../components/skill-import-dialog";
import {
  SkillDeleteDialog,
  type DeletableSkill,
} from "../components/skill-delete-dialog";
import { toSkillSummary } from "../model/skill-view";

export function SkillResourceCollection({
  wakerId = "",
  search,
  page,
  onPageChange,
  onInstall,
}: {
  wakerId?: string;
  search: string;
  page: number;
  onPageChange: (page: number) => void;
  onInstall: (id: string, name: string) => void;
}) {
  const { t } = useTranslation("plugin");
  const active = useRouteSceneActive();
  const client = useQueryClient();
  const [add, setAdd] = useState(false);
  const [targets, setTargets] = useState<DeletableSkill[]>([]);
  const library = useQuery({
    ...listLibrarySkillsOptions(
      withLocalErrorSurface({
        query: { query: search, pageNum: page, pageSize: 24 },
      }),
    ),
    enabled: active && !wakerId,
  });
  const installed = useQuery({
    ...listWakerSkillsOptions(
      withLocalErrorSurface({ path: { wakerId }, query: { query: search } }),
    ),
    enabled: active && Boolean(wakerId),
  });
  useQueryEvents({
    topic: "resources",
    resource: { kind: "global" },
    queryKeys: [
      listLibrarySkillsOptions({
        query: { query: search, pageNum: page, pageSize: 24 },
      }).queryKey,
      listWakerSkillsOptions({ path: { wakerId }, query: { query: search } })
        .queryKey,
    ],
    active,
  });
  const query = wakerId ? installed : library;
  const records = wakerId
    ? (installed.data ?? [])
    : (library.data?.resources ?? []);
  const skills = records.flatMap((item) => toSkillSummary(item) ?? []);
  const total = wakerId ? skills.length : (library.data?.totalSize ?? 0);
  const visible = wakerId ? skills.slice((page - 1) * 24, page * 24) : skills;
  const toggle = useMutation({
    ...withLocalErrorMutation(setWakerSkillEnabledMutation()),
    onSuccess: () => invalidateVisibleSkillQueries(client),
  });
  return (
    <div className="space-y-5">
      <div className="flex justify-end">
        <Button variant="secondary" onClick={() => setAdd(true)}>
          {t("add")}
        </Button>
      </div>
      {query.isPending || query.isError || visible.length === 0 ? (
        <QueryState
          pending={query.isPending}
          error={query.error}
          emptyLabel={t(search ? "noResults" : "empty")}
          onRetry={() => void query.refetch()}
        />
      ) : (
        <ResourceCardGrid>
          {visible.map((skill) => (
            <SkillCard
              key={skill.id}
              skill={skill}
              wakerId={wakerId}
              disabled={toggle.isPending}
              onEnabledChange={(enabled) =>
                toggle.mutate({
                  path: { wakerId, skillId: skill.id },
                  body: { enabled },
                })
              }
              onInstall={
                skill.scope === "built_in"
                  ? undefined
                  : () => onInstall(skill.id, skill.name)
              }
              onDelete={
                skill.scope === "built_in"
                  ? undefined
                  : () =>
                      setTargets([
                        {
                          ...skill,
                          scope:
                            skill.scope === "public" ? "public" : "private",
                        },
                      ])
              }
            />
          ))}
        </ResourceCardGrid>
      )}
      {toggle.error && (
        <p role="alert" className="text-destructive type-body">
          {localApiErrorMessage(toggle.error, t("failed"))}
        </p>
      )}
      {(total > 24 || page > 1) && (
        <div className="flex items-center justify-between gap-3">
          <Button
            variant="ghost"
            disabled={page === 1}
            onClick={() => onPageChange(page - 1)}
          >
            {t("previous")}
          </Button>
          <span className="type-caption">
            {t("page", { page, count: total })}
          </span>
          <Button
            variant="ghost"
            disabled={page * 24 >= total}
            onClick={() => onPageChange(page + 1)}
          >
            {t("next")}
          </Button>
        </div>
      )}
      <SkillImportDialog
        open={add}
        wakerId={wakerId}
        onOpenChange={setAdd}
        onImported={() => {
          void invalidateVisibleSkillQueries(client);
          setAdd(false);
        }}
      />
      <SkillDeleteDialog
        open={targets.length > 0}
        onOpenChange={(open) => {
          if (!open) setTargets([]);
        }}
        skills={targets}
        wakerId={wakerId}
        onDeleted={() => {
          void invalidateVisibleSkillQueries(client);
          setTargets([]);
        }}
      />
    </div>
  );
}
