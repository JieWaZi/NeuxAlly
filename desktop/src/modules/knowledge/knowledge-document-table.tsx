import { KnowledgePendingSourceCleanups } from "./knowledge-source-cleanup";
import { useState, type ComponentProps, type ReactNode } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { MoreHorizontal, Download, RotateCcw, Trash2 } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { Button } from "@/shared/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";
import { localApiErrorMessage } from "@/shared/api/local-error-surface";
import { KnowledgeSourceTable } from "./knowledge-source-table";
import { KnowledgeDocumentPreview } from "./knowledge-document-preview";
import {
  KnowledgeSourceDelete,
  createSourceDeleter,
} from "./knowledge-source-delete";
import { knowledgeAdapter } from "@/shared/api/knowledge";
import { downloadSource } from "./knowledge-source-document";
import { knowledgeKeys } from "./knowledge-queries";
import { useDocumentCopy } from "./use-document-copy";
import type { Library, Source } from "./knowledge-model";

type Props = ComponentProps<typeof KnowledgeSourceTable> & {
  library: Library;
  onRebuild: (source: Source) => void;
  onDeleted: (source: Source) => void;
  emptyContent?: ReactNode;
};
/** 文档操作与预览按需加载，读写均使用当前库的完整身份。 */
export function KnowledgeDocumentTable({
  library,
  onRebuild,
  onDeleted,
  emptyContent,
  ...table
}: Props) {
  const t = useDocumentCopy();
  const client = useQueryClient();
  const [preview, setPreview] = useState<Source>();
  const [deleting, setDeleting] = useState<Source>();
  const [remove] = useState(createSourceDeleter);
  const download = useMutation({
    mutationFn: (source: Source) => downloadSource(library.ref, source.id),
    retry: false,
  });
  const action = useMutation({
    mutationFn: async ({
      capability,
      run,
    }: {
      capability: string;
      run: () => void;
    }) => {
      await knowledgeAdapter(library.ref).assertCapability(capability);
      run();
    },
    retry: false,
  });
  return (
    <>
      {(download.error || action.error) && (
        <p role="alert" className="type-body text-destructive">
          {localApiErrorMessage(
            action.error || download.error,
            t("downloadFailed"),
          )}
        </p>
      )}
      {emptyContent ?? (
        <KnowledgeSourceTable
          {...table}
          open={setPreview}
          actions={(source) => (
            <DropdownMenu>
              <Tooltip>
                <TooltipTrigger asChild>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={t("actions")}
                    >
                      <MoreHorizontal />
                    </Button>
                  </DropdownMenuTrigger>
                </TooltipTrigger>
                <TooltipContent>{t("actions")}</TooltipContent>
              </Tooltip>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  disabled={download.isPending}
                  onSelect={() => download.mutate(source)}
                >
                  <Download />
                  {t("download")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={table.disabled || action.isPending}
                  onSelect={() =>
                    action.mutate({
                      capability: "rebuild",
                      run: () => onRebuild(source),
                    })
                  }
                >
                  <RotateCcw />
                  {t("rebuild")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={table.disabled || action.isPending}
                  onSelect={() =>
                    action.mutate({
                      capability: "sourcePurge",
                      run: () => setDeleting(source),
                    })
                  }
                >
                  <Trash2 />
                  {t("delete")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        />
      )}
      {knowledgeAdapter(library.ref).supportsSourceCleanup && (
        <KnowledgePendingSourceCleanups library={library} />
      )}
      {preview && (
        <KnowledgeDocumentPreview
          key={preview.id}
          resource={library.ref}
          source={preview}
          downloadError={
            download.variables?.id === preview.id ? download.error : undefined
          }
          downloading={download.isPending}
          onClose={() => setPreview(undefined)}
          onDownload={() => download.mutate(preview)}
        />
      )}
      {deleting && (
        <KnowledgeSourceDelete
          key={deleting.id}
          library={library}
          source={deleting}
          remove={remove}
          onClose={() => setDeleting(undefined)}
          onDeleted={async () => {
            onDeleted(deleting);
            await Promise.all([
              client.invalidateQueries({
                queryKey: knowledgeKeys.resource(library.ref),
              }),
              client.invalidateQueries({
                queryKey: knowledgeKeys.libraryLists,
              }),
            ]);
          }}
        />
      )}{" "}
    </>
  );
}
