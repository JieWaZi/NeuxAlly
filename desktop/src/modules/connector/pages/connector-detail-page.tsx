import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import { useQueryEvents } from "@/shared/api/use-query-events";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  localApiErrorMessage,
  withLocalErrorMutation,
  withLocalErrorSurface,
} from "@/shared/api/local-error-surface";
import { Cable, Pencil, Play, Terminal, Trash2, Wrench } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams, useLocation } from "react-router-dom";

import {
  getWakerConnectorOptions,
  invalidateVisibleConnectorQueries,
  setWakerConnectorEnabledMutation,
  testWakerConnectorMutation,
} from "../api/connector-queries";
import { ConnectorDeleteDialog } from "../components/connector-delete-dialog";
import { ConnectorFormDialog } from "../components/connector-form-dialog";
import { connectorTransportOf } from "../model/connector-view";
import { WakerDetailLayout } from "@/modules/waker";
import { runtimeDisplayName } from "@/shared/lib/runtime-display-name";
import { Badge } from "@/shared/ui/badge";
import { Card } from "@/shared/ui/card";
import { PageHeader } from "@/shared/ui/page-header";
import { PageAction } from "@/shared/ui/resource-page";
import { RuntimeCliIcon } from "@/shared/ui/runtime-cli-icon";
import { DetailSection } from "@/shared/ui/detail-section";
import { QueryState } from "@/shared/ui/query-state";
import { ReadOnlyField } from "@/shared/ui/read-only-field";
import { ResourcePageSurface } from "@/shared/ui/resource-page";
import { Switch } from "@/shared/ui/switch";

export function ConnectorDetailPage() {
  const { t } = useTranslation("connector");
  const { connectorId = "", wakerId = "" } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const active = useRouteSceneActive();
  const back =
    (location.state as { resourceBack?: string } | null)?.resourceBack ??
    (wakerId
      ? `/wakers/${wakerId}/resources?category=connectors`
      : "/resources?category=connectors");
  const queryClient = useQueryClient();
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const detailOptions = getWakerConnectorOptions(
    withLocalErrorSurface({ path: { connectorId, wakerId } }),
  );
  const detailQuery = useQuery({
    ...detailOptions,
    enabled: active && Boolean(connectorId),
  });
  useQueryEvents({
    topic: "resources",
    resource: { kind: "global" },
    queryKeys: [detailOptions.queryKey],
    active,
  });
  const refresh = () => invalidateVisibleConnectorQueries(queryClient);
  const toggleConnector = useMutation({
    ...withLocalErrorMutation(setWakerConnectorEnabledMutation()),
    onSuccess: refresh,
  });
  const testConnector = useMutation({
    ...withLocalErrorMutation(testWakerConnectorMutation()),
    onSuccess: refresh,
  });
  const connector = detailQuery.data?.connector?.id
    ? detailQuery.data.connector
    : undefined;
  const builtIn = connector?.scope === "built_in";
  const systemManaged = connector?.activationPolicy === "workflow_context";
  const transport = connectorTransportOf(connector?.transport);
  const definitions =
    transport === "stdio"
      ? (connector?.environment ?? [])
      : (connector?.headers ?? []);
  const canEdit = !builtIn || definitions.length > 0;
  const testedAt = connector?.testedAt
    ? new Intl.DateTimeFormat("zh-CN", {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(new Date(connector.testedAt))
    : t("detail.neverTested");
  const testStatus = testConnector.isPending
    ? "testing"
    : (connector?.testStatus ?? "untested");
  const testStatusClassName =
    testStatus === "succeeded"
      ? "text-success"
      : testStatus === "failed"
        ? "text-destructive"
        : "text-content-secondary";
  const compatibility = detailQuery.data?.compatibility ?? [];
  const compatibilityGroups = [
    {
      supported: true,
      runtimes: compatibility.filter((runtime) => runtime.supported),
    },
    {
      supported: false,
      runtimes: compatibility.filter((runtime) => !runtime.supported),
    },
  ].filter((group) => group.runtimes.length > 0);

  return (
    <WakerDetailLayout active="connectors">
      <ResourcePageSurface>
        <PageHeader
          layout="detail"
          backAction={{
            label: t("common:backToList"),
            onClick: () => navigate(back),
          }}
          title={
            <span className="inline-flex max-w-full flex-wrap items-center gap-2">
              <span className="min-w-0 [overflow-wrap:anywhere]">
                {connector?.name ?? t("title")}
              </span>
              {connector ? (
                <Badge variant="secondary">
                  {t(`scope.${builtIn ? "built_in" : "public"}`)}
                </Badge>
              ) : null}
            </span>
          }
          description={connector?.description || undefined}
          visual={
            <span className="bg-muted text-muted-foreground grid size-10 place-items-center rounded-md">
              {transport === "stdio" ? (
                <Terminal className="size-icon-md" strokeWidth={1.5} />
              ) : (
                <Cable className="size-icon-md" strokeWidth={1.5} />
              )}
            </span>
          }
          actions={
            <div className="grid max-w-full justify-items-end gap-2">
              <div className="flex max-w-full flex-wrap items-center gap-2">
                {wakerId && !systemManaged ? (
                  <Switch
                    checked={detailQuery.data?.enabled ?? false}
                    disabled={!connector || toggleConnector.isPending}
                    aria-label={t(
                      detailQuery.data?.enabled
                        ? "detail.disable"
                        : "detail.enable",
                      { name: connector?.name ?? t("title") },
                    )}
                    onClick={() =>
                      toggleConnector.mutate({
                        path: { connectorId, wakerId },
                        body: {
                          enabled: !(detailQuery.data?.enabled ?? false),
                        },
                      })
                    }
                  />
                ) : null}
                {connector && wakerId ? (
                  <span className="text-content-secondary type-body">
                    {systemManaged
                      ? t("systemManaged")
                      : detailQuery.data?.enabled
                        ? t("enabled")
                        : t("disabled")}
                  </span>
                ) : null}
                {canEdit ? (
                  <PageAction
                    intent="edit"
                    disabled={!connector}
                    label={t("detail.edit")}
                    onClick={() => setEditOpen(true)}
                  >
                    <Pencil />
                  </PageAction>
                ) : null}
                {!builtIn ? (
                  <PageAction
                    intent="delete"
                    disabled={!connector}
                    label={t("detail.delete")}
                    onClick={() => setDeleteOpen(true)}
                  >
                    <Trash2 />
                  </PageAction>
                ) : null}
              </div>
              {toggleConnector.isError ? (
                <p role="alert" className="text-destructive type-body">
                  {localApiErrorMessage(
                    toggleConnector.error,
                    t("toggleFailed"),
                  )}
                </p>
              ) : null}
            </div>
          }
        />

        <main className="mt-6 grid min-w-0 gap-8">
          {!connector ? (
            <QueryState
              loadingLayout="detail"
              pending={detailQuery.isPending}
              error={detailQuery.error}
              emptyLabel={t("detail.notFound")}
              onRetry={() => void detailQuery.refetch()}
            />
          ) : (
            <>
              {detailQuery.isError ? (
                <QueryState
                  error={detailQuery.error}
                  onRetry={() => void detailQuery.refetch()}
                />
              ) : null}
              <DetailSection title={t("detail.configuration")} surface="wash">
                <div className="@container/connector-config grid min-w-0 gap-5">
                  <dl className="grid min-w-0 grid-cols-1 gap-x-8 gap-y-4 @[24rem]/connector-config:grid-cols-2 @[40rem]/connector-config:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,2fr)] @[24rem]/connector-config:[&>div:last-child]:col-span-2 @[40rem]/connector-config:[&>div:last-child]:col-span-1">
                    <ReadOnlyField label={t("form.transport")}>
                      {t(`transport.${transport}`)}
                    </ReadOnlyField>
                    <ReadOnlyField label={t("detail.timeout")}>
                      {connector.timeoutMs ?? 10000} ms
                    </ReadOnlyField>
                    {transport === "stdio" ? (
                      <ReadOnlyField label={t("form.command")}>
                        <code className="type-body">
                          {connector.command || "—"}
                        </code>
                      </ReadOnlyField>
                    ) : (
                      <ReadOnlyField label={t("form.url")}>
                        {connector.url || "—"}
                      </ReadOnlyField>
                    )}
                  </dl>
                  <dl className="grid min-w-0 gap-5">
                    {transport === "stdio" ? (
                      <ReadOnlyField label={t("form.arguments")}>
                        <code className="type-body">
                          {(connector.arguments ?? []).join(" ") || "—"}
                        </code>
                      </ReadOnlyField>
                    ) : null}
                    <ReadOnlyField
                      label={
                        transport === "stdio"
                          ? t("form.environment")
                          : t("form.headers")
                      }
                    >
                      {definitions.length ? (
                        <ul className="grid min-w-0 gap-3">
                          {definitions.map((definition) => (
                            <li
                              key={definition.id ?? definition.name}
                              className="bg-muted/60 min-h-row-default grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-md px-3 py-2"
                            >
                              <code className="min-w-0 [overflow-wrap:anywhere]">
                                {definition.name}
                              </code>
                              <span className="text-content-secondary whitespace-nowrap">
                                {definition.configured
                                  ? t("detail.configured")
                                  : t("detail.notConfigured")}
                              </span>
                            </li>
                          ))}
                        </ul>
                      ) : (
                        <span className="text-content-secondary">
                          {t("form.noSecrets")}
                        </span>
                      )}
                    </ReadOnlyField>
                  </dl>
                </div>
              </DetailSection>

              <DetailSection
                title={t("detail.connectionAndCompatibility")}
                actions={
                  <PageAction
                    disabled={testConnector.isPending}
                    intent="primary"
                    label={
                      testConnector.isPending
                        ? t("detail.testing")
                        : t("detail.test")
                    }
                    onClick={() =>
                      testConnector.mutate({
                        path: { connectorId, wakerId },
                      })
                    }
                  >
                    <Play />
                  </PageAction>
                }
              >
                <div className="grid min-w-0 gap-6">
                  <Card asChild variant="section">
                    <section
                      aria-label={t("detail.testTitle")}
                      className="grid min-w-0 gap-4"
                    >
                      <div className="@container/connector-test min-w-0">
                        <dl className="grid min-w-0 grid-cols-1 gap-x-8 gap-y-4 @[24rem]/connector-test:grid-cols-2 @[36rem]/connector-test:grid-cols-3">
                          <ReadOnlyField label={t("detail.testStatus")}>
                            <span
                              className={testStatusClassName}
                              aria-live="polite"
                            >
                              {t(`testStatus.${testStatus}`)}
                            </span>
                          </ReadOnlyField>
                          <ReadOnlyField label={t("detail.testedAt")}>
                            {testedAt}
                          </ReadOnlyField>
                          <ReadOnlyField label={t("detail.testDuration")}>
                            {connector.testedAt
                              ? `${connector.testDurationMs ?? 0} ms`
                              : "—"}
                          </ReadOnlyField>
                        </dl>
                      </div>
                      {connector.testError || testConnector.isError ? (
                        <p
                          role="alert"
                          className="text-destructive type-body min-w-0 [overflow-wrap:anywhere]"
                        >
                          {connector.testError ||
                            localApiErrorMessage(
                              testConnector.error,
                              t("detail.testFailed"),
                            )}
                        </p>
                      ) : null}
                    </section>
                  </Card>

                  <Card asChild variant="section">
                    <section aria-label={t("detail.compatibility")}>
                      {compatibility.length ? (
                        <div className="grid min-w-0 gap-4">
                          {compatibilityGroups.map((group) => (
                            <div
                              key={
                                group.supported ? "supported" : "unsupported"
                              }
                              className="grid min-w-0 gap-2"
                            >
                              <h3 className="text-content-secondary type-ui font-medium">
                                {group.supported
                                  ? t("compatibility.supported")
                                  : t("compatibility.unsupported")}
                              </h3>
                              <ul className="flex min-w-0 flex-wrap gap-x-6 gap-y-3">
                                {group.runtimes.map((runtime) => (
                                  <li
                                    key={runtime.runtimeId}
                                    className="type-body inline-flex min-w-0 flex-wrap items-center gap-2"
                                  >
                                    <RuntimeCliIcon
                                      runtimeId={runtime.runtimeId}
                                      size={14}
                                    />
                                    <span className="font-medium [overflow-wrap:anywhere]">
                                      {runtimeDisplayName({
                                        id: runtime.runtimeId,
                                      }) || "Runtime"}
                                    </span>
                                  </li>
                                ))}
                              </ul>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <p className="text-content-secondary type-body">
                          {t("detail.noCompatibility")}
                        </p>
                      )}
                    </section>
                  </Card>
                </div>
              </DetailSection>

              <DetailSection
                title={t("detail.tools")}
                actions={
                  connector.testedAt &&
                  connector.testToolCount !== undefined ? (
                    <span className="text-content-secondary type-ui">
                      {t("card.toolCount", { count: connector.testToolCount })}
                    </span>
                  ) : undefined
                }
              >
                {connector.testTools?.length ? (
                  <ul className="grid min-w-0 gap-2">
                    {connector.testTools.map((tool) => (
                      <li
                        key={tool.name}
                        className="bg-muted/60 min-h-row-rich grid min-w-0 grid-cols-[20px_minmax(0,1fr)] items-center gap-3 rounded-md px-3 py-2"
                      >
                        <Wrench
                          className="text-content-secondary size-icon-md"
                          aria-hidden="true"
                        />
                        <div className="min-w-0">
                          <h3
                            className={`type-label font-medium ${builtIn ? "truncate" : "[overflow-wrap:anywhere]"}`}
                          >
                            {tool.name}
                          </h3>
                          <p
                            className={`text-content-secondary type-body mt-1 ${builtIn ? "truncate" : "max-w-[72ch] [overflow-wrap:anywhere] whitespace-pre-wrap"}`}
                          >
                            {tool.description || t("detail.noToolDescription")}
                          </p>
                        </div>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <div className="grid min-w-0 gap-1">
                    <p className="type-body font-medium">
                      {t("detail.noTools")}
                    </p>
                    <p className="text-content-secondary type-body">
                      {t("detail.noToolsDescription")}
                    </p>
                  </div>
                )}
              </DetailSection>
            </>
          )}
        </main>
      </ResourcePageSurface>

      {connector ? (
        <ConnectorFormDialog
          open={editOpen}
          connector={connector}
          wakerId={wakerId}
          onOpenChange={setEditOpen}
        />
      ) : null}
      {connector && !builtIn ? (
        <ConnectorDeleteDialog
          open={deleteOpen}
          affectedWakerCount={detailQuery.data?.affectedWakerCount ?? 0}
          connectorId={connectorId}
          name={connector.name ?? ""}
          wakerId={wakerId}
          onOpenChange={setDeleteOpen}
          onDeleted={() => navigate(back)}
        />
      ) : null}
    </WakerDetailLayout>
  );
}
