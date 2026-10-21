import { useQueryEvents } from "@/shared/api/use-query-events";
import { useInfiniteQuery, useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { GithubComJieWaZiNeuxAllyServerServicesExecutionAcceptedExecution as AcceptedExecution } from "@/shared/api/generated/types.gen";
import { withLocalErrorMutation } from "@/shared/api/local-error-surface";
import { openExternalURL } from "@/shared/desktop";
import {
  useExecutionSelection,
  type ExecutionSelection,
} from "@/shared/hooks/use-execution-selection";
import { permissionLevelOf } from "@/shared/lib/runtime-permissions";
import { ExecutionConfigurationControls } from "@/shared/ui/execution-configuration-controls";
import { ExecutionPermissionControl } from "@/shared/ui/execution-selection-controls";
import {
  ChatComposerWorkspaceStrip,
  composerContextControlClassName,
} from "./chat-composer-workspace-strip";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import {
  cancelChatExecutionMutation,
  chatMessagesInfiniteOptions,
  getChatOptions,
  resolveApprovalMutation,
  resolveElicitationMutation,
} from "../api/chat-queries";
import { conversationKeyOf } from "../model/conversation-identity";
import { useChatRuntime } from "../runtime/use-chat-runtime";
import type {
  ChatSendSelection,
  ChatComposerRecovery,
} from "../runtime/chat-queue-store";
import { ChatThread } from "./chat-thread";
import { ChatHistoryLoadingState } from "./chat-history-loading-state";
import { getWakerOptions } from "@/shared/api/generated/@tanstack/react-query.gen";

/** 上层工作入口只接管发送接纳；消息、过程、队列、原生审批和输入仍走普通 Chat。 */
export function ChatPane({
  scopeKey,
  chatId,
  title,
  defaults,
  draft,
  emptyState,
  centered = false,
  showComposer = true,
  isHistoryLoading = false,
  onSend,
  onAccepted,
  onRefresh,
  onConfigurationChange,
  onRunningChange,
}: {
  scopeKey: string;
  chatId?: string;
  title: string;
  defaults?: ExecutionSelection;
  draft?: string;
  emptyState?: ReactNode;
  centered?: boolean;
  showComposer?: boolean;
  /** 上层入口尚在解析 Chat 身份时，只延长消息区的加载状态。 */
  isHistoryLoading?: boolean;
  onSend: (
    text: string,
    attachments: string[],
    selection: ChatSendSelection,
  ) => Promise<AcceptedExecution>;
  onAccepted?: (result: AcceptedExecution) => void;
  onRefresh?: () => void;
  onConfigurationChange?: (selection: ChatSendSelection | undefined) => void;
  onRunningChange?: (running: boolean) => void;
}) {
  const { t } = useTranslation("chat");
  const active = useRouteSceneActive();
  const [selection, setSelection] = useState<ExecutionSelection>({});
  const [clearRevision, setClearRevision] = useState(0);
  const [restoreDraft, setRestoreDraft] = useState<ChatComposerRecovery>();
  const [restoreRevision, setRestoreRevision] = useState(0);
  const detail = useQuery({
    ...getChatOptions({ path: { chatId: chatId || "pending" } }),
    enabled: active && Boolean(chatId),
  });
  useQueryEvents({
    topic: "chat",
    resource: { kind: "chat", id: chatId || "" },
    queryKeys: [
      getChatOptions({ path: { chatId: chatId || "pending" } }).queryKey,
    ],
    active,
    enabled: Boolean(chatId),
  });
  const history = useInfiniteQuery({
    ...chatMessagesInfiniteOptions(chatId || "pending"),
    enabled: active && Boolean(chatId),
  });
  const wakerId = detail.data?.chat?.wakerId;
  const partner = useQuery({
    ...getWakerOptions({ path: { wakerId: wakerId || "pending" } }),
    enabled: active && Boolean(wakerId),
  });
  const messages = useMemo(() => {
    const seen = new Set<string>();
    return [...(history.data?.pages ?? [])].reverse().flatMap((page) =>
      (page.resources ?? []).filter((message) => {
        if (!message.id || seen.has(message.id)) return false;
        seen.add(message.id);
        return true;
      }),
    );
  }, [history.data]);
  const execution = useExecutionSelection({
    value: detail.data?.chat?.activeExecutionId
      ? (defaults ?? {})
      : { ...defaults, ...selection },
    defaults: true,
    enabled: active,
  });
  const config = useMemo<ChatSendSelection>(
    () => ({
      runtimeId: execution.selection.runtimeId || "",
      accountId: execution.selection.accountId,
      model: execution.selection.model || "",
      reasoning: execution.selection.reasoning || "",
      permissionLevel:
        permissionLevelOf(execution.selection.permissionLevel) ?? "default",
    }),
    [
      execution.selection.runtimeId,
      execution.selection.accountId,
      execution.selection.model,
      execution.selection.reasoning,
      execution.selection.permissionLevel,
    ],
  );
  useEffect(() => {
    onConfigurationChange?.(
      active && !execution.loading && config.runtimeId ? config : undefined,
    );
  }, [active, config, execution.loading, onConfigurationChange]);
  const refresh = () => {
    if (chatId) {
      void detail.refetch();
      void history.refetch();
    }
    onRefresh?.();
  };
  const cancel = useMutation(
    withLocalErrorMutation(cancelChatExecutionMutation()),
  );
  const approval = useMutation(
    withLocalErrorMutation(resolveApprovalMutation()),
  );
  const elicitation = useMutation(
    withLocalErrorMutation(resolveElicitationMutation()),
  );
  const state = useChatRuntime({
    conversationKey: conversationKeyOf(scopeKey, chatId),
    queueKey: chatId || scopeKey,
    queueActive: active,
    chatId,
    messages,
    latestSequence: history.data?.pages[0]?.latestSequence ?? 0,
    activeExecutionId: detail.data?.chat?.activeExecutionId,
    initialPendingApprovals: active ? detail.data?.chat?.pendingApprovals : [],
    initialPendingElicitations: active
      ? detail.data?.chat?.pendingElicitations
      : [],
    isHistoryReady: !chatId || history.isSuccess,
    isLoading: Boolean(chatId && (detail.isLoading || history.isLoading)),
    isSendDisabled:
      !active || !config.runtimeId || execution.loading || isHistoryLoading,
    attachmentOnlyText: t("attachmentOnlyMessage"),
    incompleteErrorText: t("executionIncomplete"),
    selection: config,
    send: (text, _id, paths = [], selected = config) =>
      onSend(text, paths, selected),
    cancel: async (executionId) => {
      if (chatId && executionId)
        await cancel.mutateAsync({ path: { chatId, executionId } });
    },
    onMessageAccepted: () => setClearRevision((value) => value + 1),
    onMessageRejected: (selection) => {
      setRestoreDraft(selection.composerRecovery);
      setRestoreRevision((value) => value + 1);
    },
    onStarted: (result) => {
      onAccepted?.(result);
      refresh();
    },
    onSettled: refresh,
    onTerminal: refresh,
  });
  useEffect(() => {
    onRunningChange?.(state.isRunning);
  }, [onRunningChange, state.isRunning]);
  const historyLoading = Boolean(
    isHistoryLoading ||
    (chatId && chatId !== state.boundChatId) ||
    (chatId && !state.isHistorySynchronized && !state.hasVisibleMessages),
  );
  return (
    <ChatThread
      runtime={state.runtime}
      wakerName={partner.data?.name || title}
      wakerAvatar={partner.data?.avatar}
      isRunning={state.isRunning}
      emptyState={historyLoading ? <ChatHistoryLoadingState /> : emptyState}
      emptyStatePosition={centered ? "center" : "fill"}
      isHistoryLoading={historyLoading}
      composerSettings={
        <ExecutionConfigurationControls
          state={execution}
          onChange={setSelection}
          showPermissions={false}
        />
      }
      renderComposerContextStrip={() => (
        <ChatComposerWorkspaceStrip
          permissionControl={
            <ExecutionPermissionControl
              selectedPermissionLevel={execution.selection.permissionLevel}
              triggerClassName={composerContextControlClassName}
              supportedPermissions={execution.supportedPermissions}
              disabled={execution.loading}
              onChange={(permissionLevel) =>
                setSelection({ ...execution.selection, permissionLevel })
              }
            />
          }
        />
      )}
      composerDraft={draft}
      showComposer={
        showComposer ||
        state.pendingApprovals.length > 0 ||
        state.pendingElicitations.length > 0
      }
      composerClearRevision={clearRevision}
      composerRestoreRevision={restoreRevision}
      composerRestoreDraft={restoreDraft}
      fileScopeChatId={chatId}
      resultSource={
        chatId ? { chatId, owner: { kind: "chat", id: chatId } } : undefined
      }
      linkWorkspaceRoot={detail.data?.chat?.workspaceDirectory}
      onOpenLink={(url) => {
        void openExternalURL(url);
        return true;
      }}
      sendFailure={state.sendFailure}
      errorScopes={[scopeKey, `chat:${chatId ?? "pending"}`]}
      onRetryExecution={state.retryExecution}
      contextMetrics={state.contextMetrics}
      hasEarlierMessages={Boolean(history.hasNextPage)}
      isLoadingEarlier={history.isFetchingNextPage}
      onLoadEarlier={() => history.fetchNextPage().then(() => {})}
      pendingApprovals={active ? state.pendingApprovals : []}
      approvalSubmissionState={approval.isPending ? "submitting" : "idle"}
      onResolveApproval={(approvalId, optionId) => {
        if (active && chatId)
          approval.mutate(
            { path: { chatId, approvalId }, body: { optionId } },
            { onSuccess: refresh },
          );
      }}
      pendingElicitations={active ? state.pendingElicitations : []}
      elicitationSubmissionState={elicitation.isPending ? "submitting" : "idle"}
      onResolveElicitation={(elicitationId, content, declined) => {
        if (active && chatId)
          elicitation.mutate(
            { path: { chatId, elicitationId }, body: { content, declined } },
            { onSuccess: refresh },
          );
      }}
    />
  );
}
