import type { ReactNode } from "react";
import { useDocumentCopy } from "./use-document-copy";
import { Button } from "@/shared/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";
import { Checkbox } from "@/shared/ui/checkbox";
import {
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/shared/ui/table";
import { PaginatedTable } from "@/shared/ui/paginated-table";
import type { PaginationProps } from "@/shared/ui/pagination";
import { WorkspaceFileTypeIcon } from "@/shared/workspace/workspace-file-type-icon";
import { Pill } from "./knowledge-controls";
import { useCopy } from "./use-copy";
import { isOrganizableSourceState, type Source } from "./knowledge-model";

export function KnowledgeSourceTable({
  rows,
  selection,
  select,
  selectPage,
  disabled,
  pagination,
  open,
  actions,
}: {
  rows: Source[];
  selection: string[];
  select: (source: Source) => void;
  selectPage: (checked: boolean) => void;
  disabled: boolean;
  pagination: PaginationProps;
  open?: (source: Source) => void;
  actions?: (source: Source) => ReactNode;
}) {
  const t = useCopy();
  const documentCopy = useDocumentCopy();
  const selectable = rows.filter((source) =>
    isOrganizableSourceState(source.state),
  );
  return (
    <PaginatedTable
      aria-label={t("sources")}
      className="min-w-xl table-fixed"
      pagination={pagination}
    >
      <colgroup>
        <col className="w-12" />
        <col />
        <col className="w-24" />
        <col className="w-32" />
        <col className="w-40" />
        {actions && <col className="w-14" />}
      </colgroup>
      <TableHeader>
        <TableRow>
          <TableHead>
            <Checkbox
              size="sm"
              aria-label={t("selectAll")}
              disabled={disabled || !selectable.length}
              checked={
                selectable.length > 0 &&
                selectable.every((source) => selection.includes(source.id))
              }
              onCheckedChange={(checked) => selectPage(Boolean(checked))}
            />
          </TableHead>
          <TableHead>{t("filename")}</TableHead>
          <TableHead>{documentCopy("fileSize")}</TableHead>
          <TableHead>{t("state")}</TableHead>
          <TableHead>{t("date")}</TableHead>
          {actions && (
            <TableHead>
              <span className="sr-only">{documentCopy("actions")}</span>
            </TableHead>
          )}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((source) => (
          <TableRow key={source.id} className="hover:bg-action-quiet-hover">
            <TableCell>
              <Checkbox
                size="sm"
                aria-label={t("selectSource", { name: source.name })}
                disabled={disabled || !isOrganizableSourceState(source.state)}
                checked={selection.includes(source.id)}
                onCheckedChange={() => select(source)}
              />
            </TableCell>
            <TableCell>
              <div className="flex min-w-0 items-center gap-2">
                <WorkspaceFileTypeIcon
                  isDirectory={false}
                  name={source.name}
                  size={16}
                />
                <span className="min-w-0 flex-1">
                  {open ? (
                    <Button
                      variant="text"
                      size="row"
                      onClick={() => open(source)}
                    >
                      <span title={source.name} className="min-w-0 truncate">
                        {source.name}
                      </span>
                    </Button>
                  ) : (
                    <span
                      title={source.name}
                      className="type-body block truncate"
                    >
                      {source.name}
                    </span>
                  )}
                </span>
              </div>
            </TableCell>
            <TableCell className="text-content-secondary whitespace-nowrap tabular-nums">
              {source.size}
            </TableCell>
            <TableCell>
              {source.diagnostic && source.state === "failed" ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span tabIndex={0} className="inline-flex cursor-default">
                      <Pill tone={source.state}>{t(source.state)}</Pill>
                    </span>
                  </TooltipTrigger>
                  <TooltipContent className="max-w-sm break-words">
                    {source.diagnostic}
                  </TooltipContent>
                </Tooltip>
              ) : (
                <Pill tone={source.state}>{t(source.state)}</Pill>
              )}
            </TableCell>
            <TableCell className="text-content-secondary">
              {source.date}
            </TableCell>
            {actions && <TableCell>{actions(source)}</TableCell>}
          </TableRow>
        ))}
      </TableBody>
    </PaginatedTable>
  );
}
