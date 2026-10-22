import { BookOpen, Settings, Upload } from "lucide-react";
import { Button } from "@/shared/ui/button";
import { IconAction } from "./knowledge-controls";
import type { Library } from "./knowledge-model";
import { useCopy } from "./use-copy";

/** 文档操作与筛选并列，权限和任务状态仍由库详情提供。 */
export function KnowledgeLibraryActions({
  library,
  busy,
  onUpload,
  onOrganize,
  organizeDisabled,
  onSettings,
}: {
  library?: Library;
  busy: boolean;
  onUpload: () => void;
  onOrganize: () => void;
  organizeDisabled: boolean;
  onSettings: () => void;
}) {
  const t = useCopy();
  return (
    <div
      role="group"
      aria-label={t("libraryActions")}
      className="flex shrink-0 items-center gap-2"
    >
      <IconAction
        icon={Settings}
        label={t("settings")}
        disabled={!library}
        onClick={onSettings}
      />
      <IconAction
        icon={BookOpen}
        label={t("organizeDocuments")}
        disabled={organizeDisabled}
        onClick={onOrganize}
      />
      <Button
        aria-label={t("upload")}
        disabled={busy || !library}
        onClick={onUpload}
      >
        <Upload />
        {t("upload")}
      </Button>
    </div>
  );
}
