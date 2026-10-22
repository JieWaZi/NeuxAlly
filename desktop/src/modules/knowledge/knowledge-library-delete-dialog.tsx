import { useState } from "react";
import { localApiErrorMessage } from "@/shared/api/local-error-surface";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogContent, DialogFooter } from "@/shared/ui/dialog";
import type { Library } from "./knowledge-model";
import { useCopy } from "./use-extended-copy";

export function KnowledgeLibraryDeleteDialog({
  library,
  open,
  onOpenChange,
  onDelete,
}: {
  library: Library;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDelete: (library: Library, requestId: string) => Promise<void>;
}) {
  const t = useCopy();
  const [requestId] = useState(() => crypto.randomUUID());
  const [attempted, setAttempted] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const confirmDelete = async () => {
    if (pending) return;
    setAttempted(true);
    setPending(true);
    setError("");
    try {
      await onDelete(library, requestId);
      onOpenChange(false);
    } catch (cause) {
      setError(localApiErrorMessage(cause, t("deleteLibraryFailed")));
    } finally {
      setPending(false);
    }
  };
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        if (!pending) onOpenChange(value);
      }}
    >
      <DialogContent
        title={t(attempted ? "libraryDeletionStatus" : "deleteLibrary")}
        description={t(
          attempted ? "libraryDeletionStatusHelp" : "deleteLibraryHelp",
          { name: library.name },
        )}
      >
        {error ? (
          <p role="alert" className="text-destructive type-body px-6 pb-5">
            {error}
          </p>
        ) : null}
        <DialogFooter>
          <Button
            variant="secondary"
            disabled={pending}
            onClick={() => onOpenChange(false)}
          >
            {t(attempted ? "close" : "cancel")}
          </Button>
          <Button
            variant="danger"
            disabled={pending}
            onClick={() => void confirmDelete()}
          >
            {t(
              pending
                ? "deletingLibrary"
                : attempted
                  ? "retryLibraryDeletion"
                  : "deleteLibrary",
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
