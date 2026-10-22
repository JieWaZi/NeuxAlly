import { lazy, Suspense, useRef } from "react";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogContent, DialogFooter } from "@/shared/ui/dialog";
import { KnowledgeWikiContentLoading } from "./knowledge-loading";
import type { KnowledgeUploadQueue } from "./knowledge-upload-queue";
import { useCopy } from "./use-copy";

const Content = lazy(() =>
  import("./knowledge-dialogs").then((module) => ({
    default: module.UploadSourcesContent,
  })),
);

/** 加载与上传共享同一外壳，关闭行为由实际上传队列决定。 */
export function KnowledgeUploadDialog({
  opened,
  close,
  ...props
}: {
  opened: boolean;
  close: () => void;
  queue?: KnowledgeUploadQueue;
  upload: (
    file: File,
    requestId: string,
    signal: AbortSignal,
  ) => Promise<unknown>;
}) {
  const t = useCopy();
  const content = useRef<{ close: () => void }>(null);
  const requestClose = () => {
    if (content.current) content.current.close();
    else close();
  };
  return (
    <Dialog
      open={opened}
      onOpenChange={(open) => {
        if (!open) requestClose();
      }}
    >
      <DialogContent title={t("upload")} description={t("uploadHelp")}>
        <Suspense
          fallback={
            <>
              <div className="px-6 pb-5">
                <KnowledgeWikiContentLoading />
              </div>
              <DialogFooter>
                <Button variant="ghost" onClick={close}>
                  {t("close")}
                </Button>
              </DialogFooter>
            </>
          }
        >
          <Content {...props} close={close} dialogRef={content} />
        </Suspense>
      </DialogContent>
    </Dialog>
  );
}
