import type { ComponentProps, ReactNode } from "react";
import { Slottable } from "@radix-ui/react-slot";

import { cn } from "@/shared/lib/cn";
import { Button } from "./button";
import { PageHeader } from "./page-header";
import { SearchField } from "./search-field";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  TooltipProvider,
} from "./tooltip";
import { Toolbar } from "./toolbar";

export function PageActionLabel({
  className,
  ...props
}: ComponentProps<"span">) {
  return (
    <span
      className={cn("hidden @[32rem]/page-header:inline", className)}
      {...props}
    />
  );
}

/** 页面操作按语义决定文案，通用维护操作只显示图标，业务动作保留短文案。 */
export function PageAction({
  asChild = false,
  hitArea = "default",
  intent,
  label,
  children,
  ...props
}: Omit<ComponentProps<typeof Button>, "size"> & {
  hitArea?: "default" | "comfortable";
  intent?: "primary" | "secondary" | "edit" | "delete" | "revoke";
  label?: string;
}) {
  const accessibleLabel = label ?? props["aria-label"];
  const comfortable = Boolean(label) || hitArea === "comfortable";
  const iconOnly = intent === "edit" || intent === "delete";
  const variant =
    intent === "delete" || intent === "revoke"
      ? "danger-quiet"
      : intent === "primary"
        ? "primary"
        : intent || label
          ? "secondary"
          : props.variant;
  const action = (
    <Button
      {...props}
      asChild={asChild}
      variant={variant}
      aria-label={accessibleLabel}
      data-page-action={comfortable ? "true" : undefined}
      data-variant={comfortable ? (variant ?? "primary") : undefined}
      className={cn(
        iconOnly
          ? "w-control-md gap-0 px-0"
          : "gap-2 px-2 @[32rem]/page-header:px-3",
        comfortable && "page-action min-w-9",
        props.className,
      )}
      size="md"
    >
      {asChild ? <Slottable>{children}</Slottable> : children}
      {label && !iconOnly ? (
        <span className="whitespace-nowrap">{label}</span>
      ) : null}
    </Button>
  );
  return accessibleLabel && comfortable ? (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>{action}</TooltipTrigger>
        <TooltipContent>{accessibleLabel}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  ) : (
    action
  );
}

export function ResourcePageHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description: ReactNode;
  actions: ReactNode;
}) {
  return (
    <PageHeader title={title} description={description} actions={actions} />
  );
}

export function ResourcePageToolbar({
  children,
  filters,
}: {
  children: ReactNode;
  filters?: ReactNode;
}) {
  return <Toolbar leading={children} trailing={filters} />;
}

export function ResourcePageSearchField(
  props: Omit<
    ComponentProps<typeof SearchField>,
    "containerClassName" | "size" | "width"
  >,
) {
  return <SearchField {...props} size="sm" width="toolbar" />;
}

export function ResourcePageSurface({
  children,
  className,
  contentClassName,
  ...props
}: ComponentProps<"div"> & { contentClassName?: string }) {
  return (
    <div
      data-slot="resource-page-surface"
      className={cn(
        "bg-surface-scene @container/resource-page h-full min-w-0 overflow-hidden",
        className,
      )}
      {...props}
    >
      <div
        className={cn(
          "scrollbar-hidden pt-shell-content-top pb-scene-bottom scroll-pb-scene-bottom h-full overflow-y-auto px-4 @[42.5rem]/resource-page:px-6",
          contentClassName,
        )}
      >
        {children}
      </div>
    </div>
  );
}

export function ResourceCardGrid({
  className,
  ...props
}: ComponentProps<"div">) {
  return (
    <div
      className={cn(
        "grid grid-cols-1 gap-3 @[36rem]/resource-page:grid-cols-2 @[56rem]/resource-page:grid-cols-3",
        className,
      )}
      {...props}
    />
  );
}

export function ResourcePageLibraryLabel({
  installed,
  count,
}: {
  installed: string;
  count: string;
}) {
  return (
    <div className="flex min-w-max items-center gap-3 whitespace-nowrap">
      <h2 className="type-body font-medium">{installed}</h2>
      <span className="text-content-secondary type-caption">{count}</span>
    </div>
  );
}
