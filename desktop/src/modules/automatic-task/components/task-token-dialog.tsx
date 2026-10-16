import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogContent, DialogFooter } from "@/shared/ui/dialog";
import { Skeleton } from "@/shared/ui/skeleton";
import type { TaskView } from "../model/types";
import { shellQuote } from "../model/workflow-inputs";
import { CopyValue } from "./copy-value";
export { CopyValue } from "./copy-value";
import { WorkflowAPIContract } from "./workflow-api-contract";
import {
  getGlobalAutomaticTaskOptions,
  getAutomaticTaskApiOptions,
} from "../api/task-queries";

export function apiExamples(
  taskId: string,
  port: number,
  token = "<TASK_TOKEN>",
  ip = "127.0.0.1",
  workflow = false,
  contract?: TaskView["workflowInputContract"],
) {
  const host = ip === "0.0.0.0" ? "127.0.0.1" : ip === "::" ? "::1" : ip;
  const address = host.includes(":") ? `[${host}]` : host;
  const endpoint = `http://${address}:${port}/v1/automatic-tasks/${taskId}/runs`;
  const body = workflow ? { args: contract?.example ?? {} } : { input: "" };
  const examples = {
    endpoint,
    trigger: `curl -X POST ${shellQuote(endpoint)} \\\n  -H ${shellQuote(`Authorization: Bearer ${token}`)} \\\n  -H 'Idempotency-Key: <UNIQUE_REQUEST_KEY>' \\\n  -H 'Content-Type: application/json' \\\n  -d ${shellQuote(JSON.stringify(body))}`,
    status: `curl ${shellQuote(`${endpoint}/<RUN_ID>`)} \\\n  -H ${shellQuote(`Authorization: Bearer ${token}`)}`,
    upload: `curl -X POST ${shellQuote(endpoint.replace(/\/runs$/, "/files"))} -H ${shellQuote(`Authorization: Bearer ${token}`)} -F 'file=@<INPUT_FILE>'`,
    result: `curl ${shellQuote(`${endpoint}/<RUN_ID>/result`)} -H ${shellQuote(`Authorization: Bearer ${token}`)}`,
    download: `curl ${shellQuote(`${endpoint}/<RUN_ID>/files/<FILE_ID>`)} -H ${shellQuote(`Authorization: Bearer ${token}`)} --output '<OUTPUT_FILE>'`,
  };
  return examples;
}

export function TaskTokenDialog({
  receipt,
  onClose,
}: {
  receipt?: { taskId: string; token: string };
  onClose: () => void;
}) {
  const { t } = useTranslation("automaticTask");
  const api = useQuery({
    ...getAutomaticTaskApiOptions(),
    enabled: Boolean(receipt),
  });
  const task = useQuery({
    ...getGlobalAutomaticTaskOptions({
      path: { taskId: receipt?.taskId || "" },
    }),
    enabled: Boolean(receipt),
  });
  const isWorkflow = Boolean(task.data?.workflowId);
  const examples =
    api.data?.port &&
    task.data &&
    (!isWorkflow || task.data.workflowInputContract)
      ? apiExamples(
          receipt?.taskId || "",
          api.data.port,
          receipt?.token,
          api.data.ip,
          isWorkflow,
          task.data?.workflowInputContract,
        )
      : undefined;
  return (
    <Dialog
      open={Boolean(receipt)}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent
        size="lg"
        title={t("tokenTitle")}
        description={t("tokenHint")}
      >
        <div className="grid gap-4 overflow-y-auto px-6 pb-6">
          <CopyValue label={t("token")} value={receipt?.token || ""} />
          {examples ? (
            <>
              {isWorkflow && task.data?.workflowInputContract ? (
                <WorkflowAPIContract
                  contract={task.data.workflowInputContract}
                />
              ) : null}
              <CopyValue label={t("endpoint")} value={examples.endpoint} />
              <CopyValue label={t("triggerExample")} value={examples.trigger} />
              <CopyValue label={t("statusExample")} value={examples.status} />
              {isWorkflow ? (
                <>
                  <CopyValue
                    label={t("fileUploadExample")}
                    value={examples.upload}
                  />
                  <CopyValue
                    label={t("resultExample")}
                    value={examples.result}
                  />
                  <CopyValue
                    label={t("fileDownloadExample")}
                    value={examples.download}
                  />
                </>
              ) : null}
            </>
          ) : isWorkflow && task.data && !task.data.workflowInputContract ? (
            <p role="alert" className="text-destructive type-body">
              {t("configurationInvalid")}
            </p>
          ) : api.isError || task.isError ? (
            <Button
              variant="secondary"
              onClick={() => {
                void api.refetch();
                void task.refetch();
              }}
            >
              {t("retry")}
            </Button>
          ) : (
            <Skeleton className="h-24 w-full" />
          )}
        </div>
        <DialogFooter>
          <Button onClick={onClose}>{t("done")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
