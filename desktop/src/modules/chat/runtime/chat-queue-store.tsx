import type { JSONContent } from "@tiptap/core";
import type { AppendMessage } from "@assistant-ui/react";
import {
  createContext,
  useContext,
  useState,
  useSyncExternalStore,
  type PropsWithChildren,
} from "react";

import type { AllyChatMessage } from "./chat-projection";
import type { PermissionLevel } from "@/shared/lib/runtime-permissions";

type RunConfig = NonNullable<AppendMessage["runConfig"]>;

/** 仅供本地失败恢复的编辑器快照，不进入 Core 消息请求。 */
export type ChatComposerDraft = { document: JSONContent };
export type ChatComposerRunConfig = RunConfig & {
  custom?: Record<string, unknown> & { chatComposerDraft?: ChatComposerDraft };
};

/** 快照由真实输入组件产生，只读取对应发送项的类型化文档。 */
export function chatComposerDraftOf(
  config?: RunConfig,
): ChatComposerDraft | undefined {
  const value = config?.custom?.chatComposerDraft;
  if (!value || typeof value !== "object" || !("document" in value))
    return undefined;
  const document = value.document;
  if (
    !document ||
    typeof document !== "object" ||
    !("type" in document) ||
    document.type !== "doc"
  )
    return undefined;
  return { document: { ...document, type: "doc" } };
}

/** 入队和直接发送都冻结自己的文档，后续输入不能改写旧项。 */
export function snapshotComposerMessage(message: AppendMessage): AppendMessage {
  const draft = chatComposerDraftOf(message.runConfig);
  if (!draft) return message;
  const runConfig: ChatComposerRunConfig = {
    ...message.runConfig,
    custom: {
      ...message.runConfig?.custom,
      chatComposerDraft: structuredClone(draft),
    },
  };
  return { ...message, runConfig };
}

/** 拒绝恢复的数据固定来自该发送项，页面只负责把它交还输入。 */
export type ChatComposerRecovery = { text: string; runConfig?: RunConfig };
export type RejectedChatSelection = ChatSendSelection & {
  composerRecovery: ChatComposerRecovery;
};

export type ChatSendSelection = {
  accountId?: string;
  runtimeId: string;
  model: string;
  reasoning: string;
  permissionLevel: PermissionLevel;
  skillIds?: string[];
  selectedSkills?: Array<{
    id: string;
    name: string;
    version?: string;
  }>;
  connectorIds?: string[];
  workflowIds?: string[];
  selectedWorkflows?: Array<{ id: string; name: string }>;
  selectedConnectors?: Array<{
    id: string;
    name: string;
  }>;
  codeReferences?: ChatCodeReference[];
};

export type ChatCodeReference = {
  id?: string;
  path?: string;
  startLine?: number;
  endLine?: number;
  content?: string;
};

export type QueuedChatEntry = {
  id: string;
  prompt: string;
  message: AppendMessage;
  chatId?: string;
  selection: ChatSendSelection;
};

type Listener = () => void;
type QueueState = Record<string, readonly QueuedChatEntry[]>;
type HandoffState = Record<string, readonly AllyChatMessage[]>;

const emptyEntries: readonly QueuedChatEntry[] = [];
const emptyHandoffMessages: readonly AllyChatMessage[] = [];

export class ChatQueueStore {
  private state: QueueState = {};
  private handoffs: HandoffState = {};
  private readonly listeners = new Set<Listener>();

  subscribe = (listener: Listener) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  getEntries = (queueKey: string): readonly QueuedChatEntry[] =>
    this.state[queueKey] ?? emptyEntries;

  getHandoff = (chatId?: string): readonly AllyChatMessage[] =>
    chatId
      ? (this.handoffs[chatId] ?? emptyHandoffMessages)
      : emptyHandoffMessages;

  enqueue(queueKey: string, entry: QueuedChatEntry, first = false) {
    const current = this.getEntries(queueKey);
    this.update(queueKey, first ? [entry, ...current] : [...current, entry]);
  }

  take(queueKey: string): QueuedChatEntry | undefined {
    const current = this.getEntries(queueKey);
    const head = current[0];
    if (!head) return undefined;
    this.update(queueKey, current.slice(1));
    return head;
  }

  promote(queueKey: string, entryId: string) {
    const current = this.getEntries(queueKey);
    const target = current.find((entry) => entry.id === entryId);
    if (!target || current[0]?.id === entryId) return;
    this.update(queueKey, [
      target,
      ...current.filter((entry) => entry.id !== entryId),
    ]);
  }

  remove(queueKey: string, entryId: string) {
    this.update(
      queueKey,
      this.getEntries(queueKey).filter((entry) => entry.id !== entryId),
    );
  }

  clear(queueKey: string) {
    this.update(queueKey, emptyEntries);
  }

  setHandoff(chatId: string, messages: readonly AllyChatMessage[]) {
    this.handoffs = { ...this.handoffs, [chatId]: messages };
    this.emit();
  }

  clearHandoff(chatId: string) {
    if (!(chatId in this.handoffs)) return;
    this.handoffs = Object.fromEntries(
      Object.entries(this.handoffs).filter(([key]) => key !== chatId),
    );
    this.emit();
  }

  migrate(fromQueueKey: string, toChatId: string) {
    if (fromQueueKey === toChatId) return;
    const source = this.getEntries(fromQueueKey);
    if (source.length === 0) return;
    const migrated = source.map((entry) => ({
      ...entry,
      chatId: toChatId,
    }));
    const next = this.without(fromQueueKey);
    next[toChatId] = [...(next[toChatId] ?? emptyEntries), ...migrated];
    this.state = next;
    this.emit();
  }

  private update(queueKey: string, entries: readonly QueuedChatEntry[]) {
    if (entries.length === 0) {
      if (!(queueKey in this.state)) return;
      this.state = this.without(queueKey);
    } else {
      this.state = { ...this.state, [queueKey]: entries };
    }
    this.emit();
  }

  private emit() {
    for (const listener of this.listeners) listener();
  }

  private without(queueKey: string): QueueState {
    return Object.fromEntries(
      Object.entries(this.state).filter(([key]) => key !== queueKey),
    );
  }
}

const ChatQueueContext = createContext<ChatQueueStore | null>(null);

export function ChatQueueProvider({ children }: PropsWithChildren) {
  const [store] = useState(() => new ChatQueueStore());
  return (
    <ChatQueueContext.Provider value={store}>
      {children}
    </ChatQueueContext.Provider>
  );
}

export function useChatQueueStore(): ChatQueueStore {
  const store = useContext(ChatQueueContext);
  if (!store) {
    throw new Error("ChatQueueProvider is required");
  }
  return store;
}

export function useChatQueueEntries(queueKey: string) {
  const store = useChatQueueStore();
  return useSyncExternalStore(
    store.subscribe,
    () => store.getEntries(queueKey),
    () => store.getEntries(queueKey),
  );
}

export function useChatHandoff(chatId?: string) {
  const store = useChatQueueStore();
  return useSyncExternalStore(
    store.subscribe,
    () => store.getHandoff(chatId),
    () => store.getHandoff(chatId),
  );
}
