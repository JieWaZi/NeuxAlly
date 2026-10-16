import { useTranslation } from "react-i18next";

import { localApiErrorMessage } from "@/shared/api/local-error-surface";
import { Button } from "./button";
import { ContentLoadingState } from "./content-loading-state";

/** 读取区域的互斥状态；请求失败不能被解释为空数据。 */
export function QueryState({
  pending,
  error,
  emptyLabel,
  onRetry,
}: {
  pending?: boolean;
  error?: unknown;
  emptyLabel?: string;
  onRetry: () => void;
}) {
  const { t } = useTranslation("common");
  if (pending)
    return (
      <ContentLoadingState
        className="min-h-64"
        label={t("detailState.loading")}
      />
    );
  return (
    <div
      className="type-body grid min-h-64 min-w-0 place-items-center text-center"
      role={error ? "alert" : undefined}
    >
      <div className="min-w-0 space-y-4">
        <p className="break-words">
          {error
            ? localApiErrorMessage(error, t("detailState.failed"))
            : emptyLabel}
        </p>
        {error ? (
          <Button variant="secondary" onClick={onRetry}>
            {t("retry")}
          </Button>
        ) : null}
      </div>
    </div>
  );
}
