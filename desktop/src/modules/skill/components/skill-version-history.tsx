import { useTranslation } from "react-i18next";

import type { GithubComJieWaZiNeuxAllyServerRepositoriesModelsSkillVersion as ApiSkillVersion } from "@/shared/api/generated/types.gen";
import { cn } from "@/shared/lib/cn";
import { DetailSection } from "@/shared/ui/detail-section";

type SkillVersionHistoryProps = {
  currentVersion: string;
  currentVersionId: string;
  onVersionClick: (version: ApiSkillVersion) => void;
  versions: ApiSkillVersion[];
};

export function SkillVersionHistory({
  currentVersionId,
  onVersionClick,
  versions,
}: SkillVersionHistoryProps) {
  const { t } = useTranslation("skill");

  return (
    <DetailSection title={t("detail.versionHistory")}>
      <div className="grid gap-2">
        {versions.map((version) => {
          const current = version.id === currentVersionId;
          return (
            <button
              key={version.id}
              type="button"
              aria-label={t("detail.viewVersionDiff", {
                version: version.version,
              })}
              onClick={() => onVersionClick(version)}
              className={cn(
                "focus-visible:ring-ring duration-quick flex w-full flex-wrap items-center justify-between gap-3 rounded-md px-3 py-3 text-left transition-colors outline-none focus-visible:ring-2 motion-reduce:transition-none",
                current
                  ? "bg-primary/5 hover:bg-primary/10"
                  : "bg-muted/60 hover:bg-muted",
              )}
            >
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="type-label font-medium">
                    v{version.version}
                  </span>
                  {current ? (
                    <span className="bg-primary/10 text-primary type-caption rounded-full px-2 py-1">
                      {t("detail.current")}
                    </span>
                  ) : null}
                </div>
                <p className="text-content-secondary type-body mt-1 [overflow-wrap:anywhere] whitespace-pre-wrap">
                  {version.notes || operationLabel(version.operation, t)}
                </p>
              </div>
              <span className="text-muted-foreground type-caption shrink-0">
                {version.createdAt
                  ? new Intl.DateTimeFormat("zh-CN", {
                      month: "short",
                      day: "numeric",
                    }).format(new Date(version.createdAt))
                  : "-"}
              </span>
            </button>
          );
        })}
      </div>
    </DetailSection>
  );
}

function operationLabel(
  operation: string | undefined,
  t: (key: string) => string,
): string {
  if (operation === "edit") return t("detail.operation.edit");
  if (operation === "pull") return t("detail.operation.pull");
  if (operation === "restore") return t("detail.operation.restore");
  return t("detail.operation.import");
}
