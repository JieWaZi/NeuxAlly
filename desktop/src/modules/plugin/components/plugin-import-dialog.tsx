import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { selectDirectory, selectFiles } from "@/shared/desktop";
import { Button } from "@/shared/ui/button";
import { Input } from "@/shared/ui/input";
import { Checkbox } from "@/shared/ui/checkbox";
import { Dialog, DialogContent, DialogFooter } from "@/shared/ui/dialog";
import {
  localApiErrorMessage,
  withLocalErrorMutation,
} from "@/shared/api/local-error-surface";
import {
  scanPluginMutation,
  preparePluginMutation,
  invalidatePluginQueries,
} from "../api/plugin-queries";
import type { PluginDetail } from "../model/plugin-view";
import { pluginDisplayName } from "../model/plugin-view";
import { PluginIcon } from "./plugin-icon";

const diagnosticLabels = {
  unsupported_component: "diagnosticUnsupported",
  invalid_asset: "diagnosticAsset",
  invalid_documentation: "diagnosticDocumentation",
  unsupported_schema: "diagnosticSchema",
  unknown_field: "diagnosticUnknown",
} as const;
export function PluginImportDialog({
  wakerId = "",
  targetPluginId,
  onClose,
  onPrepared,
}: {
  wakerId?: string;
  targetPluginId?: string;
  onClose: () => void;
  onPrepared: (value: PluginDetail) => void;
}) {
  const { t } = useTranslation("plugin");
  const client = useQueryClient();
  const [location, setLocation] = useState("");
  const [trusted, setTrusted] = useState(false);
  const [selectionError, setSelectionError] = useState("");
  const scan = useMutation(withLocalErrorMutation(scanPluginMutation()));
  const prepare = useMutation({
    ...withLocalErrorMutation(preparePluginMutation()),
    onSuccess: async (detail) => {
      await invalidatePluginQueries(client);
      onPrepared(detail);
    },
  });
  const pkg = scan.data?.package;
  const requiresTrust = scan.data?.trustRequired ?? false;
  const busy = scan.isPending || prepare.isPending;
  const choose = async (kind: "folder" | "archive") => {
    try {
      setSelectionError("");
      const selected =
        kind === "folder" ? await selectDirectory() : (await selectFiles())[0];
      if (selected) {
        setLocation(selected);
        scan.reset();
        setTrusted(false);
      }
    } catch {
      setSelectionError(t("failed"));
    }
  };
  return (
    <Dialog
      open
      onOpenChange={(value) => {
        if (!value) onClose();
      }}
    >
      <DialogContent
        title={t(targetPluginId ? "addPluginVersion" : "importPlugin")}
        description={t("importCompatibility")}
        size="lg"
      >
        <div className="min-h-0 space-y-4 overflow-y-auto px-6 pb-5">
          <div className="flex flex-wrap gap-2">
            <Button
              variant="secondary"
              disabled={busy}
              onClick={() => void choose("folder")}
            >
              {t("folder")}
            </Button>
            <Button
              variant="secondary"
              disabled={busy}
              onClick={() => void choose("archive")}
            >
              {t("archive")}
            </Button>
          </div>
          <label className="type-body grid gap-2">
            {t("importLocation")}
            <Input
              value={location}
              disabled={busy}
              placeholder={t("importLocationExample")}
              onChange={(event) => {
                setLocation(event.target.value);
                scan.reset();
                prepare.reset();
                setSelectionError("");
                setTrusted(false);
              }}
            />
            <span className="type-caption text-muted-foreground">
              {t("chooseImport")}
            </span>
          </label>
          <Button
            variant="secondary"
            disabled={!location.trim() || busy}
            onClick={() => {
              setTrusted(false);
              scan.mutate({ body: { location: location.trim() } });
            }}
          >
            {t("scan")}
          </Button>
          {pkg && (
            <>
              <div className="flex items-start gap-3">
                <PluginIcon pkg={pkg} />
                <div className="min-w-0 space-y-1">
                  <h3 className="type-title font-medium break-words">
                    {pluginDisplayName(pkg)}
                  </h3>
                  <p className="type-body text-muted-foreground">
                    {pkg.presentation?.shortDescription ||
                      pkg.manifest?.description}
                  </p>
                  <p className="type-caption text-muted-foreground">
                    {t("packageSummary", {
                      skills: pkg.skills?.length ?? 0,
                      connectors: pkg.servers?.length ?? 0,
                    })}
                  </p>
                </div>
              </div>
              <dl className="type-body grid gap-2">
                {[
                  [t("packageFormat"), pkg.format],
                  [t("version"), pkg.manifest?.version || t("unknown")],
                  [
                    t("developer"),
                    pkg.presentation?.developerName ||
                      pkg.manifest?.author?.name ||
                      t("unknown"),
                  ],
                  [t("license"), pkg.manifest?.license || t("unknown")],
                ].map(([label, value]) => (
                  <div
                    key={label}
                    className="flex flex-wrap justify-between gap-x-4 gap-y-1"
                  >
                    <dt className="text-muted-foreground">{label}</dt>
                    <dd className="min-w-0 break-words">{value}</dd>
                  </div>
                ))}
              </dl>
              <ul className="type-body space-y-2">
                {pkg.skills?.map((skill) => (
                  <li
                    key={skill.key}
                    className="flex min-w-0 items-center gap-3"
                    title={skill.description}
                  >
                    <span className="w-40 shrink-0 truncate font-medium">
                      {skill.name}
                    </span>
                    <span className="text-content-secondary min-w-0 truncate">
                      {skill.description}
                    </span>
                  </li>
                ))}
                {pkg.servers?.map((server) => (
                  <li key={server.key}>
                    {server.key} · {server.type} ·{" "}
                    {server.command || server.url}
                    <pre className="type-caption overflow-x-auto">
                      {JSON.stringify(
                        {
                          args: server.args,
                          cwd: server.cwd,
                          env: server.env,
                          headers: server.headers,
                        },
                        null,
                        2,
                      )}
                    </pre>
                  </li>
                ))}
              </ul>
              {(pkg.diagnostics?.length ?? 0) > 0 && (
                <section>
                  <h3 className="type-label">{t("diagnostics")}</h3>
                  {pkg.diagnostics?.map((item) => (
                    <p
                      className="type-caption text-muted-foreground"
                      key={`${item.component}:${item.code}`}
                    >
                      <span className="font-medium">{item.component}</span>
                      {" · "}
                      <span>
                        {t(
                          diagnosticLabels[
                            item.code as keyof typeof diagnosticLabels
                          ] ?? "diagnosticInvalid",
                        )}
                      </span>
                    </p>
                  ))}
                </section>
              )}
              {requiresTrust && (
                <label className="type-body flex items-start gap-2">
                  <Checkbox
                    checked={trusted}
                    disabled={busy}
                    onCheckedChange={(value) => setTrusted(value === true)}
                  />
                  {t("trust")}
                </label>
              )}
            </>
          )}
          {(scan.error || prepare.error || selectionError) && (
            <p role="alert" className="type-body text-destructive">
              {selectionError ||
                localApiErrorMessage(scan.error ?? prepare.error, t("failed"))}
            </p>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button
            disabled={
              !scan.data?.token ||
              scan.isPending ||
              prepare.isPending ||
              (requiresTrust && !trusted)
            }
            onClick={() =>
              prepare.mutate({
                body: {
                  token: scan.data?.token ?? "",
                  ownerWakerId: wakerId,
                  pluginId: targetPluginId,
                  trustConfirmed: trusted,
                },
              })
            }
          >
            {t("prepare")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
