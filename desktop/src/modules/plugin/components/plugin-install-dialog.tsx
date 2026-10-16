import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { Dialog, DialogContent, DialogFooter } from "@/shared/ui/dialog";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { QueryState } from "@/shared/ui/query-state";
import { useQueryEvents } from "@/shared/api/use-query-events";
import {
  localApiErrorMessage,
  withLocalErrorMutation,
  withLocalErrorSurface,
} from "@/shared/api/local-error-surface";
import {
  getPluginOptions,
  prepareCatalogPluginMutation,
  installPluginMutation,
  invalidatePluginQueries,
} from "../api/plugin-queries";
import {
  pluginProvider,
  type PluginDetail,
  type PluginCatalogEntry,
} from "../model/plugin-view";
import { PartnerPicker } from "./partner-picker";
import { PluginAuthPanel } from "./plugin-auth-panel";
import { PluginConfigurationEditor } from "./plugin-configuration-editor";

type Props = {
  catalog?: PluginCatalogEntry;
  detail?: PluginDetail;
  wakerId?: string;
  onClose: () => void;
};
export function PluginInstallDialog(props: Props) {
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose();
      }}
    >
      <Installation {...props} />
    </Dialog>
  );
}
function Installation({
  catalog,
  detail: initial,
  wakerId = "",
  onClose,
}: Props) {
  const [partnerNames, setPartnerNames] = useState<Record<string, string>>({});
  const rememberNames = useCallback(
    (next: Record<string, string>) =>
      setPartnerNames((previous) => ({ ...previous, ...next })),
    [],
  );
  const { t } = useTranslation("plugin");
  const client = useQueryClient();
  const [prepared, setPrepared] = useState(initial);
  const [trusted, setTrusted] = useState(false);
  const [selected, setSelected] = useState<string[]>(wakerId ? [wakerId] : []);
  const [results, setResults] = useState<
    { wakerId?: string; installed?: boolean; errorCode?: string }[] | null
  >(null);
  const pluginId = prepared?.plugin?.id ?? "";
  const options = getPluginOptions(
    withLocalErrorSurface({
      path: { pluginId },
      query: { wakerId, versionId: prepared?.resolvedVersionId },
    }),
  );
  const query = useQuery({ ...options, enabled: Boolean(pluginId) });
  useQueryEvents({
    topic: "plugins",
    resource: { kind: "global" },
    queryKeys: [options.queryKey],
    enabled: Boolean(pluginId),
  });
  const detail = query.data ?? prepared;
  const version =
    detail?.versions?.find((item) => item.id === detail.resolvedVersionId) ??
    detail?.versions?.[0];
  const provider = pluginProvider(version);
  const prepare = useMutation({
    ...withLocalErrorMutation(prepareCatalogPluginMutation()),
    onSuccess: async (value) => {
      setPrepared(value);
      await invalidatePluginQueries(client);
    },
  });
  const install = useMutation({
    ...withLocalErrorMutation(installPluginMutation()),
    onSuccess: async (value) => {
      setResults(value.results ?? []);
      setSelected(
        (value.results ?? [])
          .filter((result) => !result.installed && result.wakerId)
          .map((result) => result.wakerId ?? ""),
      );
      await invalidatePluginQueries(client);
    },
    onError: () => {
      void query.refetch();
    },
  });
  const hasMCP = (catalog?.package?.servers?.length ?? 0) > 0;
  const installed = (detail?.installations ?? []).map(
    (item) => item.wakerId ?? "",
  );
  const submit = () => {
    setResults(null);
    install.mutate({
      path: { pluginId },
      body: {
        versionId: version?.id ?? "",
        wakerIds: selected.filter((id) => !installed.includes(id)),
        expectedConfigurationRevision: detail?.configurationRevision ?? 0,
      },
    });
  };
  const pending = prepare.isPending || install.isPending;
  return (
    <DialogContent
      title={t("installToPartners")}
      description={t("installHelp")}
      size="lg"
      layout="form"
    >
      <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-6 pb-5">
        <h2 className="type-title font-medium">
          {detail?.plugin?.name ?? catalog?.package?.manifest?.name}
        </h2>
        {!pluginId ? (
          <>
            <p className="type-body text-muted-foreground">
              {catalog?.package?.manifest?.description}
            </p>
            <ul className="type-body space-y-2">
              {catalog?.package?.skills?.map((skill) => (
                <li key={skill.key}>
                  {skill.name} · {skill.description}
                </li>
              ))}
              {catalog?.package?.servers?.map((server) => (
                <li key={server.key}>
                  {server.key} · {server.type} · {server.command || server.url}
                </li>
              ))}
            </ul>
            {hasMCP && (
              <label className="type-body flex items-start gap-2">
                <Checkbox
                  checked={trusted}
                  onCheckedChange={(value) => setTrusted(value === true)}
                />
                {t("trust")}
              </label>
            )}
            <Button
              disabled={pending || (hasMCP && !trusted)}
              onClick={() =>
                prepare.mutate({
                  path: { key: catalog?.key ?? "" },
                  body: { ownerWakerId: "", trustConfirmed: trusted },
                })
              }
            >
              {t("prepare")}
            </Button>
          </>
        ) : query.isError ? (
          <QueryState
            error={query.error}
            onRetry={() => void query.refetch()}
          />
        ) : !detail?.ready ? (
          <>
            <p role="status" className="type-body">
              {t("notReady")}
            </p>
            {(provider === "feishu" || provider === "github") && (
              <PluginAuthPanel
                pluginId={pluginId}
                wakerId={wakerId}
                versionId={version?.id}
                onReady={() => void query.refetch()}
              />
            )}
            {(detail?.fields?.length ?? 0) > 0 && detail && (
              <PluginConfigurationEditor
                key={detail.configurationRevision}
                detail={detail}
                wakerId={wakerId}
                onSaved={() => void query.refetch()}
              />
            )}
          </>
        ) : (
          <>
            <p className="type-body text-muted-foreground">
              {t("partnerHelp")}
            </p>
            <PartnerPicker
              onNames={rememberNames}
              selected={selected}
              onChange={setSelected}
              installed={installed}
              onlyPartner={wakerId || undefined}
            />
          </>
        )}
        {results && (
          <section className="space-y-2" aria-live="polite">
            <h3 className="type-label">{t("installResult")}</h3>
            {results.map((result) => (
              <p key={result.wakerId} className="type-body">
                {partnerNames[result.wakerId ?? ""] ||
                  detail?.partnerNames?.[result.wakerId ?? ""] ||
                  t("partners")}{" "}
                · {result.installed ? t("succeeded") : t("failed")}
              </p>
            ))}
            {results.some((result) => !result.installed) && (
              <p className="type-body text-destructive">
                {t("partialFailure")}
              </p>
            )}
          </section>
        )}
        {(prepare.error || install.error) && (
          <p role="alert" className="type-body text-destructive">
            {localApiErrorMessage(
              prepare.error ?? install.error,
              t("authorizationRequired"),
            )}
          </p>
        )}
      </div>
      <DialogFooter>
        <Button variant="ghost" disabled={pending} onClick={onClose}>
          {pluginId ? t("keepOnly") : t("cancel")}
        </Button>
        {pluginId && detail?.ready && (
          <Button
            disabled={
              pending ||
              query.isError ||
              selected.filter((id) => !installed.includes(id)).length === 0
            }
            onClick={submit}
          >
            {install.isPending
              ? t("busy")
              : results
                ? t("retry")
                : t("install")}
          </Button>
        )}
      </DialogFooter>
    </DialogContent>
  );
}
