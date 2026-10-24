import { MessageCircle, Plus } from "lucide-react";
import { useState } from "react";
import { useQueries } from "@tanstack/react-query";
import { imConnectionDependencyOptions } from "./queries";
import {
  ConnectionDependencies,
  connectionDeletionBlocked,
} from "./components/connection-dependencies";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import { deleteImConnection } from "@/shared/api/generated/sdk.gen";
import { useResourceSelection } from "@/shared/hooks/use-resource-selection";
import {
  ResourceSelection,
  ResourceSelectionActions,
} from "@/shared/ui/resource-selection";
import { ResourceBatchDeleteDialog } from "@/shared/ui/resource-batch-delete-dialog";
import type { IMConnection, IMPlatform } from "./types";
import {
  localApiErrorMessage,
  withLocalErrorSurface,
} from "@/shared/api/local-error-surface";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogContent } from "@/shared/ui/dialog";
import { EmptyState } from "@/shared/ui/empty-state";
import {
  PageAction,
  PageActionLabel,
  ResourceCardGrid,
  ResourcePageHeader,
  ResourcePageLibraryLabel,
  ResourcePageSurface,
  ResourcePageSearchField,
  ResourcePageToolbar,
} from "@/shared/ui/resource-page";
import { ConnectionCard } from "./components/connection-card";
import { ConnectionDialog } from "./components/connection-dialog";
import { PlatformIcon } from "./components/platform-icon";
import { useIMBindings, useIMConnections } from "./queries";

export function IMPage() {
  const { t } = useTranslation("im");
  const query = useIMConnections();
  const bindings = useIMBindings();
  const [selectingPlatform, setSelectingPlatform] = useState(false);
  const [platform, setPlatform] = useState<IMPlatform>();
  const [params, setParams] = useSearchParams();
  const [batchTargets, setBatchTargets] = useState<IMConnection[]>([]);
  const deletionDependencies = useQueries({
    queries: batchTargets.map((item) =>
      imConnectionDependencyOptions(item.id || ""),
    ),
  });
  const search = params.get("search") || "";
  const connections = (query.data?.resources || []).filter((connection) =>
    `${connection.name} ${connection.appId || ""}`
      .toLocaleLowerCase()
      .includes(search.trim().toLocaleLowerCase()),
  );
  const selection = useResourceSelection(
    connections.flatMap((connection) =>
      connection.id ? [{ ...connection, id: connection.id }] : [],
    ),
    search,
  );

  return (
    <ResourcePageSurface>
      <ResourcePageHeader
        title={t("title")}
        description={t("description")}
        actions={
          <PageAction
            aria-label={t("addConnection")}
            onClick={() => setSelectingPlatform(true)}
          >
            <Plus />
            <PageActionLabel>{t("addConnection")}</PageActionLabel>
          </PageAction>
        }
      />
      <ResourcePageToolbar
        filters={
          <ResourcePageSearchField
            value={search}
            onChange={(event) => {
              const value = event.target.value;
              setParams((current) => {
                const next = new URLSearchParams(current);
                if (value) next.set("search", value);
                else next.delete("search");
                return next;
              });
            }}
            placeholder={t("searchConnections")}
            aria-label={t("searchConnections")}
          />
        }
      >
        <ResourcePageLibraryLabel
          installed={t("connections")}
          count={t("connectionCount", { count: connections.length })}
        />
        <ResourceSelection
          checked={selection.checked}
          disabled={selection.disabled}
          onChange={selection.selectAll}
        />
      </ResourcePageToolbar>
      <ResourceSelectionActions
        count={selection.selected.length}
        onClear={selection.clear}
        onDelete={() => setBatchTargets(selection.selected)}
      />
      {query.error ? (
        <div role="alert" className="text-destructive type-body mb-4">
          {localApiErrorMessage(query.error, t("operationFailed"))}
          <Button
            size="sm"
            variant="ghost"
            onClick={() => void query.refetch()}
          >
            {t("reload")}
          </Button>
        </div>
      ) : null}
      {query.isPending ? (
        <p role="status" className="text-content-secondary type-body py-8">
          {t("loading")}
        </p>
      ) : !connections.length ? (
        <EmptyState
          className="mt-6"
          icon={MessageCircle}
          title={t(search ? "empty.search" : "empty.connections")}
        />
      ) : (
        <ResourceCardGrid className="mt-6">
          {connections.map((connection) => (
            <ConnectionCard
              key={connection.id}
              connection={connection}
              selected={Boolean(
                connection.id && selection.selectedIds.has(connection.id),
              )}
              onSelectedChange={
                connection.id
                  ? (selected) =>
                      selection.setSelected(connection.id || "", selected)
                  : undefined
              }
              bindings={
                bindings.data?.filter(
                  (binding) => binding.connectionId === connection.id,
                ) || []
              }
              onSaved={() => void query.refresh()}
            />
          ))}
        </ResourceCardGrid>
      )}
      {bindings.error ? (
        <p role="alert" className="text-destructive type-body mt-4">
          {localApiErrorMessage(bindings.error, t("authorizationReadFailed"))}
          <Button
            size="sm"
            variant="ghost"
            onClick={() => void bindings.refetch()}
          >
            {t("reload")}
          </Button>
        </p>
      ) : null}
      {batchTargets.length ? (
        <ResourceBatchDeleteDialog
          items={batchTargets.map((item) => ({
            id: item.id || "",
            name: item.name || "",
          }))}
          title={t("deleteConnections")}
          description={t("deleteConnectionsHint")}
          warning={t("deleteConnectionsWarning")}
          disabled={deletionDependencies.some(
            (item) =>
              item.isPending ||
              item.error ||
              connectionDeletionBlocked(item.data),
          )}
          onDelete={(id) =>
            deleteImConnection(
              withLocalErrorSurface({
                path: { connectionId: id },
                query: {
                  revision:
                    batchTargets.find((item) => item.id === id)?.revision || 0,
                  dependencyDigest:
                    deletionDependencies[
                      batchTargets.findIndex((item) => item.id === id)
                    ]?.data?.digest,
                },
                throwOnError: true,
              }),
            )
          }
          onDeleted={async (ids) => {
            selection.remove(ids);
            setBatchTargets((targets) =>
              targets.filter((item) => !ids.includes(item.id || "")),
            );
            await query.refresh();
          }}
          onOpenChange={(open) => {
            if (!open) setBatchTargets([]);
          }}
        >
          {deletionDependencies.map((item, index) => (
            <div key={batchTargets[index]?.id} className="mb-4 grid gap-2">
              {item.data?.workflows?.length ||
              connectionDeletionBlocked(item.data) ||
              item.error ? (
                <p className="type-label font-medium">
                  {batchTargets[index]?.name}
                </p>
              ) : null}
              <ConnectionDependencies
                data={item.data}
                loading={item.isPending}
                error={item.error}
                onRetry={() => void item.refetch()}
              />
            </div>
          ))}
        </ResourceBatchDeleteDialog>
      ) : null}
      <Dialog open={selectingPlatform} onOpenChange={setSelectingPlatform}>
        <DialogContent
          title={t("addConnection")}
          description={t("selectPlatform")}
        >
          <div className="grid gap-3 px-6 pb-6">
            {(["weixin", "feishu", "wecom"] as const).map((item) => (
              <div
                key={item}
                className="bg-surface-secondary flex items-center gap-4 rounded-xl p-4"
              >
                <PlatformIcon platform={item} />
                <div className="min-w-0 flex-1">
                  <h2 className="type-label font-medium">{t(item)}</h2>
                  <p className="text-content-secondary type-caption mt-1">
                    {t("platformDescription." + item)}
                  </p>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  aria-label={t("addPlatformBot", { platform: t(item) })}
                  onClick={() => {
                    setSelectingPlatform(false);
                    setPlatform(item);
                  }}
                >
                  <Plus />
                  {t("addBot")}
                </Button>
              </div>
            ))}
          </div>
        </DialogContent>
      </Dialog>
      {platform ? (
        <ConnectionDialog
          platform={platform}
          onClose={() => setPlatform(undefined)}
          onSaved={() => void query.refresh()}
        />
      ) : null}
    </ResourcePageSurface>
  );
}
