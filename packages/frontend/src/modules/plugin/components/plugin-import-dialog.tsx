import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { selectDirectory, selectFiles } from "@/shared/platform";
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
  const hasMCP = (pkg?.servers?.length ?? 0) > 0;
  const choose = async (kind: "folder" | "archive") => {
    try {
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
        title={t("importLocal")}
        description={t("chooseImport")}
        size="lg"
      >
        <div className="min-h-0 space-y-4 overflow-y-auto px-6 pb-5">
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={() => void choose("folder")}>
              {t("folder")}
            </Button>
            <Button variant="secondary" onClick={() => void choose("archive")}>
              {t("archive")}
            </Button>
          </div>
          <Input
            value={location}
            aria-label={t("source")}
            onChange={(event) => {
              setLocation(event.target.value);
              scan.reset();
              setTrusted(false);
            }}
          />
          <Button
            variant="secondary"
            disabled={!location.trim() || scan.isPending}
            onClick={() => {
              setTrusted(false);
              scan.mutate({ body: { location: location.trim() } });
            }}
          >
            {t("scan")}
          </Button>
          {pkg && (
            <>
              <h3 className="type-title">
                {pkg.manifest?.name} · {pkg.manifest?.version}
              </h3>
              <p className="type-body text-muted-foreground">
                {pkg.manifest?.description}
              </p>
              <ul className="type-body space-y-2">
                {pkg.skills?.map((skill) => (
                  <li key={skill.key}>
                    {skill.name} · {skill.description}
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
                      {item.component} · {item.code}
                    </p>
                  ))}
                </section>
              )}
              {hasMCP && (
                <label className="type-body flex items-start gap-2">
                  <Checkbox
                    checked={trusted}
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
              (hasMCP && !trusted)
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
