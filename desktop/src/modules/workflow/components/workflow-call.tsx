import { lazy, Suspense, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowUpRight, Workflow } from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  useOpenWorkbenchContent,
  type ChatWorkflowRunContributionProps,
  type WorkbenchContributionProps,
} from "@/modules/chat";
import type { GlobalEventState } from "@/shared/api/global-events";
import { useQueryEvents } from "@/shared/api/use-query-events";
import { withLocalErrorSurface } from "@/shared/api/local-error-surface";
import { Card } from "@/shared/ui/card";
import { ContentLoadingState } from "@/shared/ui/content-loading-state";
import {
  getWorkflowRunOptions,
  getWorkflowRunQueryKey,
} from "../api/workflow-queries";
import { WorkflowQueryError } from "./query-feedback";
import { ResultOpenScope } from "../presentation/node-result";

const RunView = lazy(async () => ({
  default: (await import("./run-view")).WorkflowRunView,
}));
type RunReference = { workflowId: string; runId: string; name: string };
function runReferences(value: unknown): RunReference[] {
  if (!Array.isArray(value)) return [];
  const refs = new Map<string, RunReference>();
  for (const item of value) {
    if (
      !item ||
      typeof item !== "object" ||
      typeof item.workflowId !== "string" ||
      !item.workflowId ||
      typeof item.runId !== "string" ||
      !item.runId ||
      typeof item.name !== "string"
    )
      continue;
    refs.set(`${item.workflowId}:${item.runId}`, {
      workflowId: item.workflowId,
      runId: item.runId,
      name: item.name,
    });
  }
  return [...refs.values()];
}
function WorkflowCallItem({
  reference,
  scopeKey,
  isSummarizing,
  active = true,
}: { reference: RunReference } & Omit<
  ChatWorkflowRunContributionProps,
  "runs"
>) {
  const { t } = useTranslation("workflow");
  const open = useOpenWorkbenchContent(scopeKey);
  const options = getWorkflowRunOptions(
    withLocalErrorSurface({
      path: { id: reference.workflowId, runId: reference.runId },
    }),
  );
  const [stream, setStream] = useState<GlobalEventState>("connecting");
  const query = useQuery({
    ...options,
    queryKey: getWorkflowRunQueryKey({
      path: { id: reference.workflowId, runId: reference.runId },
    }),
    enabled: active,
  });
  useQueryEvents({
    topic: "workflow",
    resource: {
      kind: "workflow",
      id: reference.workflowId,
      parentId: reference.runId,
    },
    queryKeys: [
      getWorkflowRunQueryKey({
        path: { id: reference.workflowId, runId: reference.runId },
      }),
    ],
    active,
    enabled: active,
    onState: setStream,
  });
  const run = query.data?.run;
  const status = run?.id === reference.runId ? run.status : undefined;
  return (
    <Card
      variant="resource"
      className="my-2 gap-2 p-3"
      data-workflow-call={reference.runId}
    >
      <button
        type="button"
        className="focus-visible:ring-ring type-body min-h-control-md flex min-w-0 flex-wrap items-center gap-2 text-left outline-none focus-visible:ring-2"
        onClick={() =>
          open(
            `workflow-run:${reference.workflowId}:${reference.runId}`,
            reference.name,
            {
              type: "workflow-run",
              ids: { workflowId: reference.workflowId, runId: reference.runId },
            },
          )
        }
      >
        <Workflow className="text-primary size-icon-sm shrink-0" />
        <span className="min-w-0 flex-1 break-words">{reference.name}</span>
        <span className="text-muted-foreground type-caption">
          {status
            ? t(`status.${status}`)
            : t(
                query.isError || query.isSuccess
                  ? "callUnavailable"
                  : "loading",
              )}
        </span>
        <ArrowUpRight className="size-icon-sm shrink-0" />
      </button>
      {["succeeded", "failed", "cancelled"].includes(status ?? "") &&
      isSummarizing ? (
        <p className="text-muted-foreground type-caption">
          {t("agentSummarizing")}
        </p>
      ) : null}
      {stream === "unavailable" || stream === "reconnecting" ? (
        <p role="status" className="text-muted-foreground type-caption">
          {t("runStateStale")}
        </p>
      ) : null}
      {query.isError ? (
        <WorkflowQueryError
          error={query.error}
          retrying={query.isFetching}
          onRetry={() => void query.refetch()}
        />
      ) : null}
    </Card>
  );
}
/** 消息只消费服务端关联，每个真实运行形成一个独立业务项。 */
export function WorkflowCall({
  runs,
  ...props
}: ChatWorkflowRunContributionProps) {
  return runReferences(runs).map((reference) => (
    <WorkflowCallItem
      key={`${reference.workflowId}:${reference.runId}`}
      reference={reference}
      {...props}
    />
  ));
}
/** 工作台引用固定原运行身份，关闭和切换只释放观察。 */
export function WorkflowCallContent({
  reference,
  active,
  scopeKey,
}: WorkbenchContributionProps) {
  const { t } = useTranslation("workflow");
  if (reference.type !== "workflow-run") return null;
  const { workflowId, runId } = reference.ids;
  if (!workflowId || !runId) return <p role="alert">{t("callUnavailable")}</p>;
  return (
    <ResultOpenScope scopeKey={scopeKey}>
      <Suspense fallback={<ContentLoadingState label={t("loading")} />}>
        <RunView
          key={`${workflowId}:${runId}`}
          id={workflowId}
          runId={runId}
          active={active}
          embedded
        />
      </Suspense>
    </ResultOpenScope>
  );
}
