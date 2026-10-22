import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { localApiErrorMessage } from "@/shared/api/local-error-surface";
import { QueryState } from "@/shared/ui/query-state";
import type { KnowledgeReadState } from "./knowledge-model";
import { useCopy } from "./use-copy";

/** 后台刷新保留已有内容，失败与陈旧提示留在所属读取区域。 */
export function KnowledgeReadFeedback({
  state,
  hasData = false,
  loading,
}: {
  state?: KnowledgeReadState;
  hasData?: boolean;
  loading?: ReactNode;
}) {
  const t = useCopy();
  const { t: common } = useTranslation("common");
  if (!state) return null;
  if (state.pending && !hasData && !state.error && loading) return loading;
  if (state.error && hasData)
    return (
      <div
        role="alert"
        className="flex min-w-0 flex-wrap items-center gap-3 px-3 py-2"
      >
        <p className="type-body text-destructive min-w-0 break-words">
          {localApiErrorMessage(state.error, t("knowledgeReadFailed"))}
        </p>
        <Button size="sm" variant="secondary" onClick={state.retry}>
          {common("retry")}
        </Button>
      </div>
    );
  if (state.error || (state.pending && !hasData))
    return (
      <QueryState
        pending={!state.error && state.pending && !hasData}
        error={state.error}
        onRetry={state.retry}
      />
    );
  if (state.stale)
    return (
      <p role="status" className="text-content-muted type-caption px-3 py-2">
        {t("staleData")}
      </p>
    );
  return null;
}
