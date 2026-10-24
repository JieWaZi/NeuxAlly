import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate, useParams, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  ResourcePageSurface,
  ResourcePageHeader,
} from "@/shared/ui/resource-page";
import { Button } from "@/shared/ui/button";
import { QueryState } from "@/shared/ui/query-state";
import { withLocalErrorSurface } from "@/shared/api/local-error-surface";
import { listPluginCatalogOptions } from "../api/plugin-queries";
import { PluginInstallDialog } from "../components/plugin-install-dialog";
export function PluginCatalogDetailPage() {
  const { t } = useTranslation("plugin");
  const { catalogKey = "" } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const back =
    (location.state as { resourceBack?: string } | null)?.resourceBack ??
    "/resources";
  const [install, setInstall] = useState(false);
  const query = useQuery(
    listPluginCatalogOptions(
      withLocalErrorSurface({
        query: { query: catalogKey, pageNum: 1, pageSize: 100 },
      }),
    ),
  );
  const entry = query.data?.entries?.find((item) => item.key === catalogKey);
  const pkg = entry?.package;
  return (
    <ResourcePageSurface>
      <ResourcePageHeader
        title={pkg?.manifest?.name ?? t("catalog")}
        description={pkg?.manifest?.description ?? ""}
        actions={
          <>
            <Button variant="ghost" onClick={() => navigate(back)}>
              {t("back")}
            </Button>
            {entry && (
              <Button onClick={() => setInstall(true)}>{t("install")}</Button>
            )}
          </>
        }
      />
      {query.isPending || query.isError || !entry ? (
        <QueryState
          pending={query.isPending}
          error={query.error}
          emptyLabel={t("noResults")}
          onRetry={() => void query.refetch()}
        />
      ) : (
        <div className="space-y-5">
          <p className="type-body">
            {t("version")} {pkg?.manifest?.version}
          </p>
          <p className="type-body">
            {t("source")} · {pkg?.manifest?.author?.name} ·{" "}
            {pkg?.manifest?.license}
          </p>
          <div className="space-y-3">
            {pkg?.skills?.map((skill) => (
              <article key={skill.key}>
                <h2 className="type-label">{skill.name}</h2>
                <p className="text-muted-foreground type-body">
                  {skill.description}
                </p>
              </article>
            ))}
          </div>
        </div>
      )}
      {install && entry && (
        <PluginInstallDialog
          catalog={entry}
          onClose={() => setInstall(false)}
        />
      )}
    </ResourcePageSurface>
  );
}
