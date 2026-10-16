import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/shared/lib/cn";

type ToolbarProps = ComponentProps<"div"> & {
  leading?: ReactNode;
  trailing?: ReactNode;
  divided?: boolean;
};

export function Toolbar({
  children,
  className,
  divided = true,
  leading,
  trailing,
  ...props
}: ToolbarProps) {
  return (
    <div
      className={cn(
        "pb-3",
        divided && "border-border-subtle border-b",
        className,
      )}
      {...props}
    >
      <div
        className={cn(
          trailing
            ? "flex min-w-0 flex-wrap items-center gap-x-4 gap-y-2"
            : "min-h-control-sm flex items-center",
        )}
      >
        <div className="min-h-control-sm flex min-w-max shrink-0 flex-wrap items-center gap-3">
          {leading ?? children}
        </div>
        {trailing ? (
          <div className="ml-auto flex max-w-full min-w-0 flex-1 flex-wrap items-center justify-end gap-2">
            {trailing}
          </div>
        ) : null}
      </div>
    </div>
  );
}
