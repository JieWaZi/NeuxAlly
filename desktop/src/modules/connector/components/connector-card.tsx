import { Card, CardFooter } from "@/shared/ui/card";
import { Cable, Trash2, Wrench } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";

import type { ConnectorSummary } from "../model/types";
import { runtimeDisplayName } from "@/shared/lib/runtime-display-name";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { RuntimeCliIcon } from "@/shared/ui/runtime-cli-icon";
import { Checkbox } from "@/shared/ui/checkbox";
import { Switch } from "@/shared/ui/switch";

type Props = {
  connector: ConnectorSummary;
  selected?: boolean;
  onSelectedChange?: (selected: boolean) => void;
  disabled?: boolean;
  onDelete?: () => void;
  onEnabledChange: (enabled: boolean) => void;
  wakerId: string;
};

export function ConnectorCard({
  connector,
  selected = false,
  onSelectedChange,
  disabled,
  onDelete,
  onEnabledChange,
  wakerId,
}: Props) {
  const { t } = useTranslation("connector");
  const visibleCompatibility = connector.compatibility.slice(0, 5);
  const hiddenCompatibilityCount = Math.max(
    connector.compatibility.length - visibleCompatibility.length,
    0,
  );

  return (
    <Card
      asChild
      variant="resource"
      data-state={selected ? "selected" : undefined}
    >
      <article>
        <div className="hover:bg-surface-subtle h-connector-card-content relative shrink-0 overflow-hidden transition-colors">
          <Link
            to={`/wakers/${wakerId}/connectors/${connector.id}`}
            className="focus-visible:ring-ring absolute inset-0 z-10 rounded-t-xl outline-none focus-visible:ring-2 focus-visible:ring-inset"
            aria-label={t("card.view", { name: connector.name })}
            title={[connector.name, connector.description]
              .filter(Boolean)
              .join("\n")}
          />
          <div className="grid h-full min-h-0 grid-cols-[40px_minmax(0,1fr)_auto] gap-4 p-5">
            <span className="bg-muted text-muted-foreground grid size-10 place-items-center rounded-md">
              <Cable className="size-icon-md" strokeWidth={1.5} />
            </span>
            <div className="min-w-0">
              <div className="flex min-w-0 items-center gap-2">
                <h2
                  className="type-label min-w-0 truncate font-medium"
                  title={connector.name}
                >
                  {connector.name}
                </h2>
                <Badge variant="secondary">
                  {t(`scope.${connector.scope}`)}
                </Badge>
                <Badge variant="outline">
                  {t(`transport.${connector.transport}`)}
                </Badge>
              </div>
              <p className="text-muted-foreground type-body h-row-default mt-1 line-clamp-2">
                {connector.description || t("noDescription")}
              </p>
              <div className="text-muted-foreground type-caption mt-1 flex h-4 items-center gap-3 overflow-hidden">
                <span className="inline-flex shrink-0 items-center gap-1">
                  <Wrench className="size-icon-xs" strokeWidth={1.5} />
                  {t("card.toolCount", { count: connector.toolCount })}
                </span>
                <span className="min-w-0 truncate">
                  {t(`testStatus.${connector.testStatus}`)}
                </span>
              </div>
            </div>
            {onSelectedChange ? (
              <Checkbox
                className="relative z-20"
                size="sm"
                checked={selected}
                aria-label={t("card.select", { name: connector.name })}
                onCheckedChange={(checked) =>
                  onSelectedChange(checked === true)
                }
              />
            ) : null}
          </div>
        </div>

        <CardFooter className="h-row-navigation shrink-0">
          <div className="flex min-w-0 items-center gap-2">
            {visibleCompatibility.map((runtime) => {
              const runtimeName =
                runtimeDisplayName({ id: runtime.runtimeId }) || "Runtime";
              const status = runtime.supported
                ? t("compatibility.supported")
                : t("compatibility.unsupported");
              return (
                <span
                  key={runtime.runtimeId}
                  className={runtime.supported ? undefined : "opacity-35"}
                  aria-label={`${runtimeName} · ${status}`}
                  title={`${runtimeName} · ${status}`}
                >
                  <RuntimeCliIcon runtimeId={runtime.runtimeId} size={16} />
                </span>
              );
            })}
            {hiddenCompatibilityCount > 0 ? (
              <span
                className="text-muted-foreground type-caption"
                aria-label={t("card.moreRuntimes", {
                  count: hiddenCompatibilityCount,
                })}
              >
                …
              </span>
            ) : null}
          </div>
          <div className="ml-auto flex items-center gap-2">
            {onDelete ? (
              <Button
                type="button"
                size="icon"
                variant="danger-quiet"
                aria-label={t("card.delete", { name: connector.name })}
                onClick={onDelete}
              >
                <Trash2 />
              </Button>
            ) : null}
            {connector.activationPolicy === "workflow_context" ? (
              <span className="text-muted-foreground type-caption">
                {t("systemManaged")}
              </span>
            ) : (
              <Switch
                hitArea="comfortable"
                checked={connector.enabled}
                disabled={disabled}
                aria-label={t(
                  connector.enabled ? "card.disable" : "card.enable",
                  {
                    name: connector.name,
                  },
                )}
                onClick={() => onEnabledChange(!connector.enabled)}
              />
            )}
          </div>
        </CardFooter>
      </article>
    </Card>
  );
}
