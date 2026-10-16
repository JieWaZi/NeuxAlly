import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { Input } from "@/shared/ui/input";
import { Switch } from "@/shared/ui/switch";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/shared/ui/select";
import {
  ResourcePageSurface,
  ResourcePageHeader,
} from "@/shared/ui/resource-page";
import { DetailSection } from "@/shared/ui/detail-section";
import { SegmentedControl } from "@/shared/ui/segmented-control";
import { Dialog, DialogContent, DialogFooter } from "@/shared/ui/dialog";
import { QueryState } from "@/shared/ui/query-state";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import { useQueryEvents } from "@/shared/api/use-query-events";
import {
  localApiErrorMessage,
  withLocalErrorSurface,
  withLocalErrorMutation,
} from "@/shared/api/local-error-surface";
import {
  getPluginOptions,
  updatePluginMutation,
  setWakerPluginEnabledMutation,
  uninstallWakerPluginMutation,
  deletePluginMutation,
  invalidatePluginQueries,
} from "../api/plugin-queries";
import { PluginInstallDialog } from "../components/plugin-install-dialog";
import { PluginImportDialog } from "../components/plugin-import-dialog";
import { PluginAuthPanel } from "../components/plugin-auth-panel";
import { PluginConfigurationEditor } from "../components/plugin-configuration-editor";
import { pluginProvider, type PluginDetail } from "../model/plugin-view";

type Tab = "components" | "configuration" | "partners" | "source";
export function PluginDetailPage() {
  const { t } = useTranslation("plugin");
  const { pluginId = "", wakerId = "" } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const [params, setParams] = useSearchParams();
  const active = useRouteSceneActive();
  const client = useQueryClient();
  const [install, setInstall] = useState(false);
  const [importing, setImporting] = useState(false);
  const [removing, setRemoving] = useState<"resource" | "installation" | null>(
    null,
  );
  const [updates, setUpdates] = useState<string[]>([]);
  const rawTab = params.get("tab");
  const tab: Tab =
    rawTab === "configuration" || rawTab === "partners" || rawTab === "source"
      ? rawTab
      : "components";
  const selectedVersion = params.get("version") ?? "";
  const options = getPluginOptions(
    withLocalErrorSurface({
      path: { pluginId },
      query: { wakerId, versionId: selectedVersion },
    }),
  );
  const query = useQuery({ ...options, enabled: active && Boolean(pluginId) });
  useQueryEvents({
    topic: "plugins",
    resource: { kind: "global" },
    queryKeys: [options.queryKey],
    active,
  });
  const refresh = () => invalidatePluginQueries(client);
  const toggle = useMutation({
    ...withLocalErrorMutation(setWakerPluginEnabledMutation()),
    onSuccess: refresh,
  });
  const uninstall = useMutation({
    ...withLocalErrorMutation(uninstallWakerPluginMutation()),
    onSuccess: async () => {
      await refresh();
      navigate(back);
    },
  });
  const remove = useMutation({
    ...withLocalErrorMutation(deletePluginMutation()),
    onSuccess: async () => {
      await refresh();
      navigate(back);
    },
  });
  const detail = query.data;
  const resource = detail?.plugin;
  const relation = detail?.installations?.find(
    (item) => item.wakerId === wakerId,
  );
  const version =
    detail?.versions?.find((item) => item.id === detail.resolvedVersionId) ??
    detail?.versions?.[0];
  const pkg = version?.package;
  const setParam = (key: string, value: string) =>
    setParams(
      (current) => {
        const next = new URLSearchParams(current);
        next.set(key, value);
        return next;
      },
      { state: location.state },
    );
  const back =
    (location.state as { resourceBack?: string } | null)?.resourceBack ??
    (wakerId ? `/wakers/${wakerId}/resources` : "/resources?view=mine");
  const error = toggle.error ?? remove.error ?? uninstall.error;
  const switches = (detail?.installations ?? [])
    .filter(
      (item) =>
        item.wakerId &&
        updates.includes(item.wakerId) &&
        item.versionId !== version?.id,
    )
    .map((item) => ({
      wakerId: item.wakerId ?? "",
      expectedVersionId: item.versionId ?? "",
      versionId: version?.id ?? "",
    }));
  return (
    <main
      className="h-full min-w-0 overflow-hidden"
      data-partner-section={wakerId ? "resources" : undefined}
    >
      <ResourcePageSurface>
        <ResourcePageHeader
          title={resource?.name ?? t("plugins")}
          description={resource?.description ?? ""}
          actions={
            <div className="flex flex-wrap gap-2">
              <Button variant="ghost" onClick={() => navigate(back)}>
                {t("back")}
              </Button>
              {resource && (
                <>
                  <Button variant="secondary" onClick={() => setInstall(true)}>
                    {t("installToPartners")}
                  </Button>
                  <Button
                    variant="secondary"
                    onClick={() => setImporting(true)}
                  >
                    {t("add")} {t("version")}
                  </Button>
                  {relation && (
                    <Button
                      variant="ghost"
                      onClick={() => setRemoving("installation")}
                    >
                      {t("uninstall")}
                    </Button>
                  )}
                  {(!wakerId || resource.ownerWakerId === wakerId) && (
                    <Button
                      variant="danger-quiet"
                      onClick={() => setRemoving("resource")}
                    >
                      {t("remove")}
                    </Button>
                  )}
                </>
              )}
            </div>
          }
        />
        {query.isPending || query.isError || !detail ? (
          <QueryState
            pending={query.isPending}
            error={query.error}
            onRetry={() => void query.refetch()}
          />
        ) : (
          <div className="space-y-6">
            <div className="flex flex-wrap items-center justify-between gap-4">
              <label className="type-body flex items-center gap-3">
                {t("version")}
                <Select
                  value={version?.id ?? ""}
                  onValueChange={(value) => {
                    setParam("version", value);
                    setUpdates([]);
                  }}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {detail.versions?.map((item) => (
                      <SelectItem key={item.id} value={item.id ?? ""}>
                        {item.version}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </label>
              {relation && (
                <label className="type-body flex items-center gap-2">
                  <Switch
                    checked={relation.enabled ?? false}
                    disabled={toggle.isPending}
                    onClick={() =>
                      toggle.mutate({
                        path: { wakerId, pluginId },
                        body: { enabled: !relation.enabled },
                      })
                    }
                  />
                  {t(relation.enabled ? "enabled" : "disabled")}
                </label>
              )}
            </div>
            <SegmentedControl
              ariaLabel={t("details")}
              value={tab}
              onValueChange={(value) => setParam("tab", value)}
              options={[
                { value: "components", label: t("components") },
                { value: "configuration", label: t("configuration") },
                { value: "partners", label: t("partnerResources") },
                { value: "source", label: t("source") },
              ]}
            />
            {tab === "components" && (
              <DetailSection title={t("components")}>
                <div className="space-y-5">
                  {pkg?.skills?.map((skill) => (
                    <article key={skill.key} className="space-y-1">
                      <h3 className="type-label font-medium">{skill.name}</h3>
                      <p className="type-body text-muted-foreground">
                        {skill.description}
                      </p>
                    </article>
                  ))}
                  {pkg?.servers?.map((server) => (
                    <article key={server.key} className="space-y-1">
                      <h3 className="type-label font-medium">
                        {server.key} · {t("connectors")}
                      </h3>
                      <p className="type-caption text-muted-foreground break-all">
                        {server.type} · {server.command || server.url}
                      </p>
                    </article>
                  ))}
                  {(pkg?.diagnostics?.length ?? 0) > 0 && (
                    <section>
                      <h3 className="type-label">{t("diagnostics")}</h3>
                      {pkg?.diagnostics?.map((item) => (
                        <p
                          key={`${item.component}:${item.code}`}
                          className="type-caption text-muted-foreground"
                        >
                          {item.component} · {item.code}
                        </p>
                      ))}
                    </section>
                  )}
                </div>
              </DetailSection>
            )}
            {tab === "configuration" && (
              <div className="space-y-6">
                <PluginNameEditor
                  key={resource?.updatedAt}
                  detail={detail}
                  wakerId={wakerId}
                />
                {pluginProvider(version) === "feishu" && (
                  <PluginAuthPanel
                    pluginId={pluginId}
                    wakerId={wakerId}
                    versionId={version?.id}
                    onReady={() => void query.refetch()}
                  />
                )}
                <p className="type-body text-muted-foreground">
                  {t("updateHelp")}
                </p>
                <InstallationRows
                  detail={detail}
                  selected={updates}
                  onChange={setUpdates}
                />
                <PluginConfigurationEditor
                  key={`${detail.configurationRevision}:${version?.id}`}
                  detail={detail}
                  wakerId={wakerId}
                  switches={switches}
                  onSaved={() => setUpdates([])}
                />
              </div>
            )}
            {tab === "partners" && (
              <DetailSection
                title={t("installedCount", {
                  count: detail.installations?.length ?? 0,
                })}
              >
                <InstallationRows detail={detail} />
              </DetailSection>
            )}
            {tab === "source" && (
              <DetailSection title={t("source")}>
                <dl className="type-body grid gap-3">
                  <dt className="text-muted-foreground">
                    {t("upstreamAuthor")}
                  </dt>
                  <dd>{pkg?.manifest?.author?.name ?? t("unknown")}</dd>
                  <dt className="text-muted-foreground">{t("license")}</dt>
                  <dd>{pkg?.manifest?.license ?? t("unknown")}</dd>
                  <dt className="text-muted-foreground">{t("source")}</dt>
                  <dd className="break-all">
                    {pkg?.manifest?.repository ??
                      pkg?.manifest?.homepage ??
                      t("unknown")}
                  </dd>
                  <dt className="text-muted-foreground">{t("maintainer")}</dt>
                  <dd>
                    {String(
                      (
                        version?.provenance?.wrapper as
                          { maintainer?: string } | undefined
                      )?.maintainer ?? t("unknown"),
                    )}
                  </dd>
                </dl>
              </DetailSection>
            )}
            {error && (
              <p role="alert" className="type-body text-destructive">
                {localApiErrorMessage(error, t("failed"))}
              </p>
            )}
          </div>
        )}
        {install && detail && (
          <PluginInstallDialog
            detail={detail}
            wakerId={wakerId}
            onClose={() => setInstall(false)}
          />
        )}
        {importing && (
          <PluginImportDialog
            wakerId={resource?.ownerWakerId ?? ""}
            targetPluginId={pluginId}
            onClose={() => setImporting(false)}
            onPrepared={() => {
              setImporting(false);
              void refresh();
            }}
          />
        )}
        {removing && detail && (
          <Dialog
            open
            onOpenChange={(open) => {
              if (!open) setRemoving(null);
            }}
          >
            <DialogContent
              title={t(
                removing === "resource" ? "deleteTitle" : "uninstallTitle",
              )}
              description={
                removing === "resource"
                  ? t("deleteHelp", {
                      count: detail.installations?.length ?? 0,
                    })
                  : t("uninstallHelp")
              }
            >
              <DialogFooter>
                <Button variant="ghost" onClick={() => setRemoving(null)}>
                  {t("cancel")}
                </Button>
                <Button
                  variant="danger"
                  disabled={remove.isPending || uninstall.isPending}
                  onClick={() => {
                    if (removing === "installation")
                      uninstall.mutate({ path: { wakerId, pluginId } });
                    else
                      remove.mutate({
                        path: { pluginId },
                        query: { wakerId },
                        body: {
                          confirmed: true,
                          installations: (detail.installations ?? []).map(
                            (item) => ({
                              wakerId: item.wakerId ?? "",
                              expectedVersionId: item.versionId ?? "",
                              versionId: item.versionId ?? "",
                            }),
                          ),
                        },
                      });
                  }}
                >
                  {t("confirmDelete")}
                </Button>
              </DialogFooter>
              {(remove.error || uninstall.error) && (
                <p
                  role="alert"
                  className="text-destructive type-body px-6 pb-4"
                >
                  {localApiErrorMessage(
                    remove.error ?? uninstall.error,
                    t("failed"),
                  )}
                </p>
              )}
            </DialogContent>
          </Dialog>
        )}
      </ResourcePageSurface>
    </main>
  );
}
function InstallationRows({
  detail,
  selected,
  onChange,
}: {
  detail: PluginDetail;
  selected?: string[];
  onChange?: (ids: string[]) => void;
}) {
  const { t } = useTranslation("plugin");
  if (!detail.installations?.length)
    return (
      <p className="type-body text-muted-foreground">{t("notInstalled")}</p>
    );
  return (
    <ul className="space-y-2">
      {detail.installations.map((item) => {
        const id = item.wakerId ?? "";
        const version = detail.versions?.find(
          (version) => version.id === item.versionId,
        );
        return (
          <li
            key={id}
            className="type-body rounded-control bg-surface-subtle flex flex-wrap items-center gap-3 p-3"
          >
            {onChange && (
              <Checkbox
                aria-label={detail.partnerNames?.[id] ?? t("partners")}
                checked={selected?.includes(id) ?? false}
                onCheckedChange={(checked) =>
                  onChange(
                    checked
                      ? [...(selected ?? []), id]
                      : (selected ?? []).filter((value) => value !== id),
                  )
                }
              />
            )}
            <span className="min-w-0 flex-1 truncate">
              {detail.partnerNames?.[id] || t("unknown")}
            </span>
            <span>{version?.version ?? t("unknown")}</span>
            <span className="text-muted-foreground">
              {t(item.enabled ? "enabled" : "disabled")}
            </span>
          </li>
        );
      })}
    </ul>
  );
}
function PluginNameEditor({
  detail,
  wakerId,
}: {
  detail: PluginDetail;
  wakerId: string;
}) {
  const { t } = useTranslation("plugin");
  const client = useQueryClient();
  const [name, setName] = useState(detail.plugin?.name ?? "");
  const update = useMutation({
    ...withLocalErrorMutation(updatePluginMutation()),
    onSuccess: () => invalidatePluginQueries(client),
  });
  return (
    <section className="space-y-3">
      <label className="type-body grid gap-2">
        {t("name")}
        <Input value={name} onChange={(event) => setName(event.target.value)} />
      </label>
      <Button
        variant="secondary"
        disabled={!name.trim() || update.isPending}
        onClick={() =>
          update.mutate({
            path: { pluginId: detail.plugin?.id ?? "" },
            query: { wakerId },
            body: {
              name: name.trim(),
              description: detail.plugin?.description ?? "",
            },
          })
        }
      >
        {t("rename")}
      </Button>
      {update.error && (
        <p role="alert" className="type-body text-destructive">
          {localApiErrorMessage(update.error, t("failed"))}
        </p>
      )}
    </section>
  );
}
