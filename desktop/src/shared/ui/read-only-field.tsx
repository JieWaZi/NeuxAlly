import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";

/** 详情中的只读字段：标签 12/16、正文 13/20，长路径与标识可完整换行。 */
export function ReadOnlyField({
  label,
  children,
  className,
}: {
  label: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("min-w-0", className)}>
      <dt className="text-content-secondary type-ui">{label}</dt>
      <dd className="type-body mt-1 min-w-0 [overflow-wrap:anywhere] whitespace-pre-wrap">
        {children}
      </dd>
    </div>
  );
}
