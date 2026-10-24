import { Package } from "lucide-react";
import { Link, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Card, CardFooter } from "@/shared/ui/card";
import { Button } from "@/shared/ui/button";
import { Switch } from "@/shared/ui/switch";
import type { PluginDetail, PluginCatalogEntry } from "../model/plugin-view";
export function PluginCard({
  detail,
  catalog,
  wakerId = "",
  onInstall,
  onToggle,
  pending = false,
}: {
  detail?: PluginDetail;
  catalog?: PluginCatalogEntry;
  wakerId?: string;
  onInstall: () => void;
  onToggle?: (enabled: boolean) => void;
  pending?: boolean;
}) {
  const location = useLocation();
  const { t } = useTranslation("plugin");
  const relation = detail?.installations?.find(
    (item) => item.wakerId === wakerId,
  );
  const version =
    detail?.versions?.find((item) => item.id === relation?.versionId) ??
    detail?.versions?.[0];
  const pkg = version?.package ?? catalog?.package;
  const name = detail?.plugin?.name ?? pkg?.manifest?.name ?? "";
  const to = catalog
    ? `/resources/plugins/catalog/${catalog.key}`
    : wakerId
      ? `/wakers/${wakerId}/plugins/${detail?.plugin?.id}`
      : `/resources/plugins/${detail?.plugin?.id}`;
  return (
    <Card variant="resource" asChild>
      <article>
        <Link
          state={{ resourceBack: location.pathname + location.search }}
          to={to}
          className="focus-visible:ring-ring rounded-control min-h-32 space-y-3 p-5 outline-none focus-visible:ring-2"
        >
          <div className="flex items-center gap-3">
            <Package className="size-icon-lg text-muted-foreground shrink-0" />
            <h2 className="type-label min-w-0 truncate font-medium">{name}</h2>
          </div>
          <p className="type-body text-muted-foreground line-clamp-3">
            {detail?.plugin?.description ?? pkg?.manifest?.description}
          </p>
          <p className="type-caption text-muted-foreground">
            {t("version")}{" "}
            {version?.version ?? pkg?.manifest?.version ?? t("unknown")}
          </p>
        </Link>
        <CardFooter>
          <span className="type-caption text-muted-foreground min-w-0 flex-1 truncate">
            {relation
              ? t(relation.enabled ? "enabled" : "disabled")
              : detail
                ? t("installedCount", {
                    count: detail.installations?.length ?? 0,
                  })
                : pkg?.manifest?.author?.name}
          </span>
          {relation && onToggle ? (
            <Switch
              checked={relation.enabled ?? false}
              disabled={pending}
              aria-label={t("enable")}
              onClick={() => onToggle(!relation.enabled)}
            />
          ) : (
            <Button size="sm" variant="secondary" onClick={onInstall}>
              {t("install")}
            </Button>
          )}
        </CardFooter>
      </article>
    </Card>
  );
}
