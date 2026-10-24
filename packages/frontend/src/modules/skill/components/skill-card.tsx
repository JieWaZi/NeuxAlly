import { Card, CardFooter } from "@/shared/ui/card";
import { GitBranch, Package, Trash2, Upload } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link, useLocation } from "react-router-dom";

import { SkillIcon } from "./skill-icon";
import type { SkillSummary } from "../model/types";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { Switch } from "@/shared/ui/switch";

type Props = {
  onInstall?: () => void;
  disabled?: boolean;
  onDelete?: () => void;
  skill: SkillSummary;
  wakerId: string;
  onEnabledChange: (enabled: boolean) => void;
  onSelectedChange?: (selected: boolean) => void;
  selected?: boolean;
};

const sourceIcons = {
  ally: Package,
  git: GitBranch,
  zip: Upload,
} as const;

export function SkillCard({
  disabled,
  onInstall,
  onDelete,
  onEnabledChange,
  onSelectedChange,
  selected = false,
  skill,
  wakerId,
}: Props) {
  const location = useLocation();
  const { t } = useTranslation("skill");
  const SourceIcon = sourceIcons[skill.source];

  return (
    <Card
      asChild
      variant="resource"
      data-state={selected ? "selected" : undefined}
    >
      <article>
        <div className="hover:bg-surface-subtle relative transition-colors">
          <Link
            state={{ resourceBack: location.pathname + location.search }}
            to={
              wakerId
                ? `/wakers/${wakerId}/skills/${skill.id}`
                : `/resources/skills/${skill.id}`
            }
            className="focus-visible:ring-ring absolute inset-0 z-10 rounded-t-xl outline-none focus-visible:ring-2 focus-visible:ring-inset"
            aria-label={t("card.view", { name: skill.name })}
            title={[skill.name, skill.description].filter(Boolean).join("\n")}
          />
          <div className="grid min-h-32 grid-cols-[40px_minmax(0,1fr)_auto] gap-4 p-5">
            <SkillIcon />
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <h2 className="type-label min-w-0 truncate font-medium">
                  {skill.name}
                </h2>
                <span className="bg-muted text-muted-foreground type-caption shrink-0 rounded-xs px-1.5 py-0.5">
                  {t(`scope.${skill.scope}`)}
                </span>
              </div>
              <p className="text-muted-foreground type-body mt-2 line-clamp-3">
                {skill.description}
              </p>
            </div>
            {onSelectedChange ? (
              <Checkbox
                className="relative z-20"
                size="sm"
                checked={selected}
                aria-label={t("card.select", { name: skill.name })}
                onCheckedChange={(checked) =>
                  onSelectedChange(checked === true)
                }
              />
            ) : null}
          </div>
        </div>

        <CardFooter>
          <div className="text-muted-foreground type-caption flex min-w-0 items-center gap-3">
            <span className="inline-flex items-center gap-1">
              <SourceIcon className="size-icon-xs" strokeWidth={1.5} />
              {skill.source === "ally"
                ? t("source.ally")
                : skill.source === "git"
                  ? t("source.git")
                  : t("source.zip")}
            </span>
            <span>v{skill.version}</span>
          </div>
          <div className="ml-auto flex items-center gap-2">
            {onDelete ? (
              <Button
                type="button"
                size="icon"
                variant="danger-quiet"
                aria-label={t("card.delete", { name: skill.name })}
                onClick={onDelete}
              >
                <Trash2 />
              </Button>
            ) : null}
            {wakerId ? (
              <Switch
                hitArea="comfortable"
                checked={skill.enabled}
                disabled={disabled}
                aria-label={t(skill.enabled ? "card.disable" : "card.enable", {
                  name: skill.name,
                })}
                onClick={() => onEnabledChange(!skill.enabled)}
              />
            ) : onInstall ? (
              <Button size="sm" variant="secondary" onClick={onInstall}>
                {t("plugin:install")}
              </Button>
            ) : null}
          </div>
        </CardFooter>
      </article>
    </Card>
  );
}
