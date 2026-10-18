import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate, useParams, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { ResourcePageSurface, PageAction } from "@/shared/ui/resource-page";
import { Plus, Pencil } from "lucide-react";
import { PluginDetailHeader } from "../components/plugin-detail-header";
import { PluginSettingsDialog } from "../components/plugin-settings-dialog";
import { PluginOverview } from "../components/plugin-overview";
import { pluginNeedsAuthorization } from "../model/plugin-view";
import { useQueryEvents } from "@/shared/api/use-query-events";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import { QueryState } from "@/shared/ui/query-state";
import { withLocalErrorSurface } from "@/shared/api/local-error-surface";
import {
  listPluginCatalogOptions,
  listPluginsOptions,
} from "../api/plugin-queries";
import { PluginRemovalDialog } from "../components/plugin-removal-dialog";
import { PluginInstallDialog } from "../components/plugin-install-dialog";
export function PluginCatalogDetailPage() {
  const { t } = useTranslation("plugin");
  const [removingPartner, setRemovingPartner] = useState("");
  const { catalogKey = "" } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const back =
    (location.state as { resourceBack?: string } | null)?.resourceBack ??
    "/resources";
  const [install, setInstall] = useState(false);
  const [editing, setEditing] = useState(false);
  const query = useQuery(
    listPluginCatalogOptions(
      withLocalErrorSurface({
        query: { query: catalogKey, pageNum: 1, pageSize: 100 },
      }),
    ),
  );
  const entry = query.data?.entries?.find((item) => item.key === catalogKey);
  const pkg = entry?.package;
  const active = useRouteSceneActive();
  const installedOptions = listPluginsOptions(
    withLocalErrorSurface({
      query: { query: `catalog:${catalogKey}`, pageNum: 1, pageSize: 100 },
    }),
  );
  const installed = useQuery({
    ...installedOptions,
    enabled: active && Boolean(entry),
  });
  useQueryEvents({
    topic: "plugins",
    resource: { kind: "global" },
    queryKeys: [installedOptions.queryKey],
    active,
  });
  const detail = installed.data?.resources?.find(
    (item) => item.plugin?.sourceKey === `catalog:${catalogKey}`,
  );

  return (
    <ResourcePageSurface>
      <PluginDetailHeader
        pkg={pkg}
        sourceKey={`catalog:${catalogKey}`}
        backAction={{ label: t("back"), onClick: () => navigate(back) }}
        actions={
          entry && (
            <>
              {detail?.plugin?.id && (
                <PageAction
                  intent="edit"
                  label={t("edit")}
                  onClick={() => setEditing(true)}
                >
                  <Pencil />
                </PageAction>
              )}
              <PageAction
                intent="primary"
                disabled={installed.isPending || installed.isError}
                onClick={() => setInstall(true)}
              >
                <Plus />
                {t(
                  pluginNeedsAuthorization(pkg, detail)
                    ? "authorize"
                    : "installToPartners",
                )}
              </PageAction>
            </>
          )
        }
      />
      <div className="mt-8 space-y-6">
        {entry && query.isError && (
          <QueryState
            error={query.error}
            onRetry={() => void query.refetch()}
          />
        )}
        {!entry ? (
          <QueryState
            loadingLayout="detail"
            pending={query.isPending}
            error={query.error}
            emptyLabel={t("noResults")}
            onRetry={() => void query.refetch()}
          />
        ) : (
          <PluginOverview
            pkg={pkg}
            sourceKey={`catalog:${catalogKey}`}
            skillContext={{ catalogKey }}
            detail={detail}
            onUninstall={setRemovingPartner}
            installationStatus={
              installed.isPending || installed.isError ? (
                <QueryState
                  loadingLayout="content"
                  pending={installed.isPending}
                  error={installed.error}
                  onRetry={() => void installed.refetch()}
                />
              ) : undefined
            }
          />
        )}
      </div>
      {editing && detail && (
        <PluginSettingsDialog
          key={`${detail.plugin?.id}:${detail.resolvedVersionId}`}
          detail={detail}
          versionId={detail.resolvedVersionId}
          onClose={() => setEditing(false)}
        />
      )}
      {removingPartner && detail && (
        <PluginRemovalDialog
          detail={detail}
          mode="installation"
          wakerId={removingPartner}
          onClose={() => setRemovingPartner("")}
        />
      )}
      {install && entry && (
        <PluginInstallDialog
          catalog={entry}
          detail={detail}
          authorizeOnly={pluginNeedsAuthorization(pkg, detail)}
          onClose={() => setInstall(false)}
        />
      )}
    </ResourcePageSurface>
  );
}
