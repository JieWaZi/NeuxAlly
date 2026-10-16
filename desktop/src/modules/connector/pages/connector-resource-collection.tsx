import { useQueryEvents } from "@/shared/api/use-query-events";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import { listLibraryConnectorsOptions } from "@/shared/api/generated/@tanstack/react-query.gen";
import {
  withLocalErrorSurface,
  withLocalErrorMutation,
  localApiErrorMessage,
} from "@/shared/api/local-error-surface";
import { ResourceCardGrid } from "@/shared/ui/resource-page";
import { Button } from "@/shared/ui/button";
import { QueryState } from "@/shared/ui/query-state";
import {
  listWakerConnectorsOptions,
  setWakerConnectorEnabledMutation,
  invalidateVisibleConnectorQueries,
} from "../api/connector-queries";
import { ConnectorCard } from "../components/connector-card";
import { ConnectorFormDialog } from "../components/connector-form-dialog";
import { ConnectorDeleteDialog } from "../components/connector-delete-dialog";
import { toConnectorSummary } from "../model/connector-view";
import type { ConnectorSummary } from "../model/types";

export function ConnectorResourceCollection({
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
  const [target, setTarget] = useState<ConnectorSummary | null>(null);
  const library = useQuery({
    ...listLibraryConnectorsOptions(
      withLocalErrorSurface({
        query: { query: search, pageNum: page, pageSize: 24 },
      }),
    ),
    enabled: active && !wakerId,
  });
  const installed = useQuery({
    ...listWakerConnectorsOptions(
      withLocalErrorSurface({ path: { wakerId }, query: { query: search } }),
    ),
    enabled: active && Boolean(wakerId),
  });
  useQueryEvents({
    topic: "resources",
    resource: { kind: "global" },
    queryKeys: [
      listLibraryConnectorsOptions({
        query: { query: search, pageNum: page, pageSize: 24 },
      }).queryKey,
      listWakerConnectorsOptions({
        path: { wakerId },
        query: { query: search },
      }).queryKey,
    ],
    active,
  });
  const query = wakerId ? installed : library;
  const records = wakerId
    ? (installed.data ?? [])
    : (library.data?.resources ?? []);
  const connectors = records.flatMap((item) => toConnectorSummary(item) ?? []);
  const total = wakerId ? connectors.length : (library.data?.totalSize ?? 0);
  const visible = wakerId
    ? connectors.slice((page - 1) * 24, page * 24)
    : connectors;
  const toggle = useMutation({
    ...withLocalErrorMutation(setWakerConnectorEnabledMutation()),
    onSuccess: () => invalidateVisibleConnectorQueries(client),
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
          {visible.map((connector) => (
            <ConnectorCard
              key={connector.id}
              connector={connector}
              wakerId={wakerId}
              disabled={toggle.isPending}
              onEnabledChange={(enabled) =>
                toggle.mutate({
                  path: { wakerId, connectorId: connector.id },
                  body: { enabled },
                })
              }
              onInstall={
                connector.scope === "built_in"
                  ? undefined
                  : () => onInstall(connector.id, connector.name)
              }
              onDelete={
                connector.scope === "built_in"
                  ? undefined
                  : () => setTarget(connector)
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
      <ConnectorFormDialog
        open={add}
        wakerId={wakerId}
        onOpenChange={setAdd}
        onSaved={() => {
          void invalidateVisibleConnectorQueries(client);
          setAdd(false);
        }}
      />
      {target && (
        <ConnectorDeleteDialog
          open
          wakerId={wakerId}
          connectorId={target.id}
          name={target.name}
          affectedWakerCount={target.affectedWakerCount}
          onOpenChange={(open) => {
            if (!open) setTarget(null);
          }}
          onDeleted={() => {
            void invalidateVisibleConnectorQueries(client);
            setTarget(null);
          }}
        />
      )}
    </div>
  );
}
