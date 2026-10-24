import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams, useSearchParams } from "react-router-dom";
import {
  PluginResourceCollection,
  PartnerPicker,
  resourceCategory,
} from "@/modules/plugin";
import { SkillResourceCollection } from "@/modules/skill";
import { ConnectorResourceCollection } from "@/modules/connector";
import {
  ResourcePageSurface,
  ResourcePageHeader,
  ResourcePageToolbar,
  ResourcePageSearchField,
} from "@/shared/ui/resource-page";
import { SegmentedControl } from "@/shared/ui/segmented-control";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogContent, DialogFooter } from "@/shared/ui/dialog";
import {
  installLibrarySkillMutation,
  installLibraryConnectorMutation,
} from "@/shared/api/generated/@tanstack/react-query.gen";
import {
  withLocalErrorMutation,
  localApiErrorMessage,
} from "@/shared/api/local-error-surface";

type Installation = { kind: "skill" | "connector"; id: string; name: string };
export function ResourcesPage() {
  const { t } = useTranslation("plugin");
  const { wakerId = "" } = useParams();
  const [params, setParams] = useSearchParams();
  const category = resourceCategory(params.get("category"));
  const search = params.get("search") ?? "";
  const page = Math.max(1, Number(params.get("page")) || 1);
  const view = params.get("view") === "mine" ? "mine" : "catalog";
  const [installation, setInstallation] = useState<Installation | null>(null);
  const setParam = (key: string, value: string) =>
    setParams((current) => {
      const next = new URLSearchParams(current);
      if (value) next.set(key, value);
      else next.delete(key);
      if (key !== "page") next.delete("page");
      return next;
    });
  const onPageChange = (next: number) =>
    setParam("page", next === 1 ? "" : String(next));
  const targetPartner = params.get("partner") || undefined;
  return (
    <main
      className="bg-surface-scene h-full min-w-0 overflow-hidden"
      data-partner-section={wakerId ? "resources" : undefined}
    >
      <ResourcePageSurface>
        <ResourcePageHeader
          title={wakerId ? t("partnerResources") : t("title")}
          description={t("description")}
          actions={null}
        />
        <div className="mb-5">
          <SegmentedControl
            ariaLabel={t("title")}
            value={category}
            options={[
              { value: "plugins", label: t("plugins") },
              { value: "skills", label: t("skills") },
              { value: "connectors", label: t("connectors") },
            ]}
            onValueChange={(value) =>
              setParams((current) => {
                const next = new URLSearchParams(current);
                next.set("category", value);
                next.delete("page");
                next.delete("search");
                next.delete("view");
                return next;
              })
            }
          />
        </div>
        <ResourcePageToolbar>
          <ResourcePageSearchField
            value={search}
            aria-label={t("search")}
            placeholder={t("search")}
            onChange={(event) => setParam("search", event.target.value)}
          />
        </ResourcePageToolbar>
        {wakerId && category !== "plugins" && (
          <div className="mb-4 flex justify-end">
            <Button asChild variant="secondary">
              <Link to={`/resources?category=${category}&partner=${wakerId}`}>
                {t("install")}
              </Link>
            </Button>
          </div>
        )}
        <div key={category}>
          {category === "plugins" ? (
            <PluginResourceCollection
              wakerId={wakerId}
              targetPartnerId={targetPartner}
              search={search}
              page={page}
              view={view}
              onViewChange={(value) => setParam("view", value)}
              onPageChange={onPageChange}
            />
          ) : category === "skills" ? (
            <SkillResourceCollection
              wakerId={wakerId}
              search={search}
              page={page}
              onPageChange={onPageChange}
              onInstall={(id, name) =>
                setInstallation({ kind: "skill", id, name })
              }
            />
          ) : (
            <ConnectorResourceCollection
              wakerId={wakerId}
              search={search}
              page={page}
              onPageChange={onPageChange}
              onInstall={(id, name) =>
                setInstallation({ kind: "connector", id, name })
              }
            />
          )}
        </div>
        {installation && (
          <IndependentInstallDialog
            resource={installation}
            targetPartner={targetPartner}
            onClose={() => setInstallation(null)}
          />
        )}
      </ResourcePageSurface>
    </main>
  );
}
function IndependentInstallDialog({
  resource,
  targetPartner,
  onClose,
}: {
  resource: Installation;
  targetPartner?: string;
  onClose: () => void;
}) {
  const { t } = useTranslation("plugin");
  const client = useQueryClient();
  const [selected, setSelected] = useState<string[]>(
    targetPartner ? [targetPartner] : [],
  );
  const [done, setDone] = useState(false);
  const skill = useMutation(
    withLocalErrorMutation(installLibrarySkillMutation()),
  );
  const connector = useMutation(
    withLocalErrorMutation(installLibraryConnectorMutation()),
  );
  const pending = skill.isPending || connector.isPending;
  const error = skill.error ?? connector.error;
  const install = async () => {
    try {
      if (resource.kind === "skill")
        await skill.mutateAsync({
          path: { skillId: resource.id },
          body: { wakerIds: selected },
        });
      else
        await connector.mutateAsync({
          path: { connectorId: resource.id },
          body: { wakerIds: selected },
        });
      await client.invalidateQueries({
        predicate: (query) =>
          /Skill|Connector/.test(
            (query.queryKey[0] as { _id?: string } | undefined)?._id ?? "",
          ),
      });
      setDone(true);
    } catch {
      /* 错误由安装对话框就近显示。 */
    }
  };
  return (
    <Dialog
      open
      onOpenChange={(value) => {
        if (!value && !pending) onClose();
      }}
    >
      <DialogContent
        title={t("installIndependent")}
        description={resource.name}
      >
        <div className="min-h-0 space-y-4 overflow-y-auto px-6 pb-5">
          {done ? (
            <p role="status" className="type-body">
              {t("succeeded")}
            </p>
          ) : (
            <PartnerPicker
              selected={selected}
              onChange={setSelected}
              onlyPartner={targetPartner}
            />
          )}{" "}
          {error && (
            <p role="alert" className="type-body text-destructive">
              {localApiErrorMessage(error, t("failed"))}
            </p>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" disabled={pending} onClick={onClose}>
            {t(done ? "close" : "cancel")}
          </Button>
          {!done && (
            <Button
              disabled={pending || selected.length === 0}
              onClick={() => void install()}
            >
              {t("install")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
