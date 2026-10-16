import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import { useQueryEvents } from "@/shared/api/use-query-events";
import {
  withLocalErrorMutation,
  withLocalErrorSurface,
  localApiErrorMessage,
} from "@/shared/api/local-error-surface";
import { ResourceCardGrid } from "@/shared/ui/resource-page";
import { Button } from "@/shared/ui/button";
import { SegmentedControl } from "@/shared/ui/segmented-control";
import { QueryState } from "@/shared/ui/query-state";
import { Dialog, DialogContent } from "@/shared/ui/dialog";
import { Input } from "@/shared/ui/input";
import {
  listPluginCatalogOptions,
  listPluginsOptions,
  listWakerPluginsOptions,
  setWakerPluginEnabledMutation,
  invalidatePluginQueries,
} from "../api/plugin-queries";
import type { PluginCatalogEntry, PluginDetail } from "../model/plugin-view";
import { PluginCard } from "../components/plugin-card";
import { PluginInstallDialog } from "../components/plugin-install-dialog";
import { PluginImportDialog } from "../components/plugin-import-dialog";

type Props = {
  wakerId?: string;
  targetPartnerId?: string;
  search: string;
  page: number;
  view: "catalog" | "mine";
  onViewChange: (view: "catalog" | "mine") => void;
  onPageChange: (page: number) => void;
  selecting?: boolean;
};
export function PluginResourceCollection({
  wakerId = "",
  targetPartnerId,
  search,
  page,
  view,
  onViewChange,
  onPageChange,
  selecting = false,
}: Props) {
  const { t } = useTranslation("plugin");
  const client = useQueryClient();
  const active = useRouteSceneActive();
  const [adding, setAdding] = useState(false);
  const [library, setLibrary] = useState(false);
  const [installation, setInstallation] = useState<{
    catalog?: PluginCatalogEntry;
    detail?: PluginDetail;
  } | null>(null);
  const catalogOptions = listPluginCatalogOptions(
    withLocalErrorSurface({
      query: { query: search, pageNum: page, pageSize: 24 },
    }),
  );
  const libraryOptions = listPluginsOptions(
    withLocalErrorSurface({
      query: { query: search, pageNum: page, pageSize: 24 },
    }),
  );
  const installedOptions = listWakerPluginsOptions(
    withLocalErrorSurface({
      path: { wakerId },
      query: { query: search, pageNum: page, pageSize: 24 },
    }),
  );
  const catalog = useQuery({
    ...catalogOptions,
    enabled: active && !wakerId && view === "catalog",
  });
  const resources = useQuery({
    ...libraryOptions,
    enabled: active && !wakerId && view === "mine",
  });
  const installed = useQuery({
    ...installedOptions,
    enabled: active && Boolean(wakerId),
  });
  useQueryEvents({
    topic: "plugins",
    resource: { kind: "global" },
    queryKeys: [libraryOptions.queryKey, installedOptions.queryKey],
    active,
  });
  const toggle = useMutation({
    ...withLocalErrorMutation(setWakerPluginEnabledMutation()),
    onSuccess: () => invalidatePluginQueries(client),
  });
  const query = wakerId ? installed : view === "catalog" ? catalog : resources;
  const entries = catalog.data?.entries ?? [];
  const details = (wakerId ? installed.data : resources.data)?.resources ?? [];
  const total = query.data?.totalSize ?? 0;
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        {!wakerId ? (
          <SegmentedControl
            ariaLabel={t("plugins")}
            value={view}
            options={[
              { value: "catalog", label: t("catalog") },
              { value: "mine", label: t("mine") },
            ]}
            onValueChange={onViewChange}
          />
        ) : (
          <span className="type-label">{t("partnerResources")}</span>
        )}
        <div className="flex gap-2">
          {wakerId && (
            <Button variant="secondary" onClick={() => setLibrary(true)}>
              {t("install")}
            </Button>
          )}
          {!selecting && (
            <Button variant="secondary" onClick={() => setAdding(true)}>
              {t("importLocal")}
            </Button>
          )}
        </div>
      </div>
      {query.isPending || query.isError || total === 0 ? (
        <QueryState
          pending={query.isPending}
          error={query.error}
          emptyLabel={t(search ? "noResults" : "empty")}
          onRetry={() => void query.refetch()}
        />
      ) : (
        <ResourceCardGrid>
          {!wakerId && view === "catalog"
            ? entries.map((entry) => (
                <PluginCard
                  key={entry.key}
                  catalog={entry}
                  onInstall={() => setInstallation({ catalog: entry })}
                />
              ))
            : details.map((detail) => (
                <PluginCard
                  key={detail.plugin?.id}
                  detail={detail}
                  wakerId={wakerId}
                  pending={toggle.isPending}
                  onInstall={() => setInstallation({ detail })}
                  onToggle={
                    wakerId
                      ? (enabled) =>
                          toggle.mutate({
                            path: {
                              wakerId,
                              pluginId: detail.plugin?.id ?? "",
                            },
                            body: { enabled },
                          })
                      : undefined
                  }
                />
              ))}
        </ResourceCardGrid>
      )}
      {toggle.error && (
        <p role="alert" className="type-body text-destructive">
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
      {adding && (
        <PluginImportDialog
          wakerId={wakerId}
          onClose={() => setAdding(false)}
          onPrepared={(detail) => {
            setAdding(false);
            setInstallation({ detail });
          }}
        />
      )}
      {installation && (
        <PluginInstallDialog
          {...installation}
          wakerId={targetPartnerId ?? wakerId}
          onClose={() => setInstallation(null)}
        />
      )}
      {library && (
        <Dialog open onOpenChange={setLibrary}>
          <DialogContent
            title={t("installToPartners")}
            description={t("installHelp")}
            size="xl"
            layout="form"
          >
            <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-5">
              <PluginLibraryPicker partnerId={wakerId} />
            </div>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
function PluginLibraryPicker({ partnerId }: { partnerId: string }) {
  const { t } = useTranslation("plugin");
  const [view, setView] = useState<"catalog" | "mine">("catalog");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  return (
    <div className="space-y-4">
      <Input
        aria-label={t("search")}
        value={search}
        onChange={(event) => {
          setSearch(event.target.value);
          setPage(1);
        }}
      />
      <PluginResourceCollection
        selecting
        targetPartnerId={partnerId}
        search={search}
        page={page}
        view={view}
        onViewChange={(value) => {
          setView(value);
          setPage(1);
        }}
        onPageChange={setPage}
      />
    </div>
  );
}
