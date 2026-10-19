import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import { useQueryEvents } from "@/shared/api/use-query-events";
import {
  withLocalErrorMutation,
  withLocalErrorSurface,
  localApiErrorMessage,
} from "@/shared/api/local-error-surface";
import {
  ResourceCardGrid,
  ResourcePageToolbar,
  ResourcePageLibraryLabel,
} from "@/shared/ui/resource-page";
import {
  listPluginCatalog,
  listPlugins,
  listWakerPlugins,
} from "@/shared/api/generated/sdk.gen";
import {
  useResourcePages,
  RESOURCE_BATCH_SIZE,
} from "@/shared/hooks/use-resource-pages";
import { useResourceSelection } from "@/shared/hooks/use-resource-selection";
import {
  ResourceSelection,
  ResourceSelectionActions,
} from "@/shared/ui/resource-selection";
import { ResourceInfiniteScroll } from "@/shared/ui/resource-infinite-scroll";
import { ResourceBatchDeleteDialog } from "@/shared/ui/resource-batch-delete-dialog";
import { SegmentedControl } from "@/shared/ui/segmented-control";
import { QueryRefreshError, QueryState } from "@/shared/ui/query-state";
import {
  listPluginCatalogQueryKey,
  listPluginsQueryKey,
  listWakerPluginsQueryKey,
  setWakerPluginEnabledMutation,
  invalidatePluginQueries,
  deletePluginMutation,
  uninstallWakerPluginMutation,
} from "../api/plugin-queries";
import {
  pluginNeedsAuthorization,
  pluginDisplayName,
  type PluginCatalogEntry,
  type PluginDetail,
} from "../model/plugin-view";
import { usePluginResources } from "../api/use-plugin-resources";
import { StatusNotice } from "@/shared/ui/status-notice";
import { PluginRemovalDialog } from "../components/plugin-removal-dialog";
import { PluginIcon } from "../components/plugin-icon";
import { PluginCard } from "../components/plugin-card";
import { PluginInstallDialog } from "../components/plugin-install-dialog";

type Props = {
  wakerId?: string;
  targetPartnerId?: string;
  search: string;
  view: "catalog" | "mine";
  onViewChange: (view: "catalog" | "mine") => void;
  searchField?: ReactNode;
};
export function PluginResourceCollection({
  wakerId = "",
  targetPartnerId,
  search,
  view,
  onViewChange,
  searchField,
}: Props) {
  const { t } = useTranslation("plugin");
  const client = useQueryClient();
  const active = useRouteSceneActive();
  const [batchTargets, setBatchTargets] = useState<
    (PluginDetail & { id: string })[]
  >([]);
  const remove = useMutation(withLocalErrorMutation(deletePluginMutation()));
  const uninstall = useMutation(
    withLocalErrorMutation(uninstallWakerPluginMutation()),
  );
  const [removing, setRemoving] = useState<PluginDetail | null>(null);
  const [installation, setInstallation] = useState<{
    catalog?: PluginCatalogEntry;
    detail?: PluginDetail;
    authorizeOnly?: boolean;
  } | null>(null);
  const catalogKey = listPluginCatalogQueryKey(
    withLocalErrorSurface({
      query: { query: search, pageSize: RESOURCE_BATCH_SIZE },
    }),
  );
  const libraryKey = listPluginsQueryKey(
    withLocalErrorSurface({
      query: { query: search, pageSize: RESOURCE_BATCH_SIZE },
    }),
  );
  const installedKey = listWakerPluginsQueryKey(
    withLocalErrorSurface({
      path: { wakerId },
      query: { query: search, pageSize: RESOURCE_BATCH_SIZE },
    }),
  );
  const catalogResources = usePluginResources(
    undefined,
    active && !wakerId && view === "catalog",
  );
  const catalog = useResourcePages({
    queryKey: catalogKey,
    fetchPage: async (pageNum, signal) => {
      const { data } = await listPluginCatalog(
        withLocalErrorSurface({
          query: { query: search, pageNum, pageSize: RESOURCE_BATCH_SIZE },
          signal,
          throwOnError: true as const,
        }),
      );
      return {
        items: data.entries ?? [],
        total: data.totalSize ?? 0,
        metadata: data.source,
      };
    },
    enabled: active && !wakerId && view === "catalog",
  });
  const resources = useResourcePages({
    queryKey: libraryKey,
    fetchPage: async (pageNum, signal) => {
      const { data } = await listPlugins(
        withLocalErrorSurface({
          query: { query: search, pageNum, pageSize: RESOURCE_BATCH_SIZE },
          signal,
          throwOnError: true as const,
        }),
      );
      return { items: data.resources ?? [], total: data.totalSize ?? 0 };
    },
    enabled: active && !wakerId && view === "mine",
  });
  const installed = useResourcePages({
    queryKey: installedKey,
    fetchPage: async (pageNum, signal) => {
      const { data } = await listWakerPlugins(
        withLocalErrorSurface({
          path: { wakerId },
          query: { query: search, pageNum, pageSize: RESOURCE_BATCH_SIZE },
          signal,
          throwOnError: true as const,
        }),
      );
      return { items: data.resources ?? [], total: data.totalSize ?? 0 };
    },
    enabled: active && Boolean(wakerId),
  });
  useQueryEvents({
    topic: "plugins",
    resource: { kind: "global" },
    queryKeys: [
      [...catalogKey, "infinite"],
      [...libraryKey, "infinite"],
      [...installedKey, "infinite"],
    ],
    active,
  });
  const toggle = useMutation({
    ...withLocalErrorMutation(setWakerPluginEnabledMutation()),
    onSuccess: () => invalidatePluginQueries(client),
  });
  const query = wakerId ? installed : view === "catalog" ? catalog : resources;
  const entries = catalog.items;
  const details = (wakerId ? installed.items : resources.items).flatMap(
    (item) => (item.plugin?.id ? [{ ...item, id: item.plugin.id }] : []),
  );
  const total = query.total;
  const selection = useResourceSelection(
    details,
    JSON.stringify([wakerId, search, view]),
  );
  return (
    <div className="space-y-5">
      <ResourcePageToolbar
        filters={searchField}
        appearance={wakerId ? "default" : "navigation"}
      >
        {wakerId ? (
          <ResourcePageLibraryLabel
            installed={t("allPluginsTitle")}
            count={t("resourceCount", { count: total })}
          />
        ) : (
          <>
            <SegmentedControl
              appearance="underline"
              ariaLabel={t("plugins")}
              value={view}
              options={[
                { value: "catalog", label: t("catalog") },
                { value: "mine", label: t("mine") },
              ]}
              onValueChange={onViewChange}
            />
            <span className="type-caption text-content-secondary whitespace-nowrap">
              {t("resourceCount", { count: total })}
            </span>
          </>
        )}
        {(wakerId || view === "mine") && (
          <ResourceSelection
            checked={selection.checked}
            disabled={selection.disabled}
            onChange={selection.selectAll}
          />
        )}
      </ResourcePageToolbar>
      {!wakerId &&
        view === "catalog" &&
        catalog.metadata?.state === "stale" && (
          <StatusNotice>{t("catalogStale")}</StatusNotice>
        )}
      <ResourceSelectionActions
        count={selection.selected.length}
        onClear={selection.clear}
        onDelete={() => setBatchTargets(selection.selected)}
      />
      {query.isPending || (query.isError && total === 0) || total === 0 ? (
        <QueryState
          loadingLayout="cards"
          pending={query.isPending}
          error={query.error}
          emptyLabel={t(search ? "noResults" : "empty")}
          onRetry={() => void query.refetch()}
        />
      ) : (
        <ResourceCardGrid>
          {!wakerId && view === "catalog"
            ? entries.map((entry) => {
                const detail = catalogResources.data?.find(
                  (item) =>
                    item.plugin?.sourceKey ===
                    (entry.sourceKey ?? `catalog:${entry.key}`),
                );
                return (
                  <PluginCard
                    key={entry.key}
                    catalog={entry}
                    detail={detail}
                    pending={
                      catalogResources.isPending || catalogResources.isError
                    }
                    onInstall={() =>
                      setInstallation({
                        catalog: entry,
                        detail,
                        authorizeOnly: pluginNeedsAuthorization(
                          entry.package,
                          detail,
                        ),
                      })
                    }
                  />
                );
              })
            : details.map((detail) => (
                <PluginCard
                  key={detail.plugin?.id}
                  detail={detail}
                  selected={selection.selectedIds.has(detail.id)}
                  onSelectedChange={(selected) =>
                    selection.setSelected(detail.id, selected)
                  }
                  wakerId={wakerId}
                  pending={toggle.isPending}
                  onInstall={() =>
                    setInstallation({
                      detail,
                      authorizeOnly: pluginNeedsAuthorization(
                        detail.versions?.[0]?.package,
                        detail,
                      ),
                    })
                  }
                  onRemove={() => setRemoving(detail)}
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
      {total > 0 && query.isError && !query.isFetchNextPageError && (
        <QueryRefreshError
          error={query.error}
          pending={query.isFetching}
          onRetry={() => void query.refetch()}
        />
      )}
      {!wakerId && view === "catalog" && catalogResources.isError && (
        <QueryState
          error={catalogResources.error}
          onRetry={() => void catalogResources.refetch()}
        />
      )}
      {toggle.error && (
        <p role="alert" className="type-body text-destructive">
          {localApiErrorMessage(toggle.error, t("failed"))}
        </p>
      )}
      <ResourceInfiniteScroll
        hasMore={active && Boolean(query.hasNextPage) && !query.isRefetchError}
        pending={query.isFetching}
        error={query.isFetchNextPageError ? query.error : undefined}
        onLoadMore={() => void query.fetchNextPage()}
      />
      {batchTargets.length > 0 && (
        <ResourceBatchDeleteDialog
          key={batchTargets.map((item) => item.id).join(":")}
          items={batchTargets.map((detail) => ({
            id: detail.id,
            name: pluginDisplayName(
              detail.versions?.[0]?.package,
              detail.plugin?.name,
            ),
            icon: <PluginIcon pkg={detail.versions?.[0]?.package} />,
            description: t("partnerUsage", {
              count: detail.installations?.length ?? 0,
            }),
          }))}
          title={t(wakerId ? "batchUninstallTitle" : "batchDeleteTitle")}
          warning={
            wakerId
              ? t("uninstallHelp")
              : t("deleteHelp", {
                  count: new Set(
                    batchTargets.flatMap((item) =>
                      (item.installations ?? []).map(
                        (relation) => relation.wakerId,
                      ),
                    ),
                  ).size,
                })
          }
          onDelete={(pluginId) => {
            if (wakerId)
              return uninstall.mutateAsync({ path: { pluginId, wakerId } });
            const detail = batchTargets.find((item) => item.id === pluginId);
            return remove.mutateAsync({
              path: { pluginId },
              body: {
                confirmed: true,
                installations: (detail?.installations ?? []).map((item) => ({
                  wakerId: item.wakerId ?? "",
                  expectedVersionId: item.versionId ?? "",
                  versionId: item.versionId ?? "",
                })),
              },
            });
          }}
          onDeleted={async (ids) => {
            selection.remove(ids);
            await invalidatePluginQueries(client);
          }}
          onOpenChange={(open) => {
            if (!open) setBatchTargets([]);
          }}
        />
      )}
      {removing && (
        <PluginRemovalDialog
          detail={removing}
          mode={wakerId ? "installation" : "resource"}
          wakerId={wakerId}
          onClose={() => setRemoving(null)}
        />
      )}
      {installation && (
        <PluginInstallDialog
          {...installation}
          wakerId={targetPartnerId ?? wakerId}
          onClose={() => setInstallation(null)}
        />
      )}
    </div>
  );
}
