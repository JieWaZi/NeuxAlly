import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import {
  ChatQueueStore,
  snapshotComposerMessage,
  ChatQueueProvider,
  useChatQueueEntries,
  useChatQueueStore,
  type QueuedChatEntry,
} from "./chat-queue-store";

function entry(id: string): QueuedChatEntry {
  return {
    id,
    prompt: `prompt-${id}`,
    message: {
      role: "user",
      content: [{ type: "text", text: id }],
      attachments: [],
      createdAt: new Date(0),
      metadata: { custom: {} },
      parentId: null,
      sourceId: null,
      runConfig: undefined,
    },
    chatId: "chat-1",
    selection: {
      runtimeId: "codex",
      model: "gpt-5",
      reasoning: "high",
      permissionLevel: "auto",
    },
  };
}

function QueueProbe() {
  const store = useChatQueueStore();
  const entries = useChatQueueEntries("chat-1");
  return (
    <div>
      <span data-testid="queue-size">{entries.length}</span>
      <button
        type="button"
        onClick={() => store.enqueue("chat-1", entry("queued-1"))}
      >
        enqueue
      </button>
    </div>
  );
}

function RendererShell() {
  const [mounted, setMounted] = useState(true);
  return (
    <ChatQueueProvider>
      <button type="button" onClick={() => setMounted((value) => !value)}>
        toggle route
      </button>
      {mounted && <QueueProbe />}
    </ChatQueueProvider>
  );
}

describe("chat queue store", () => {
  it("逐项深拷贝编辑器文档并保留原runConfig自定义字段", () => {
    const message = entry("draft").message;
    const attrs = { id: "flow-a", label: "甲", referenceType: "workflow" };
    const text = { type: "text", text: "甲正文" };
    message.runConfig = {
      custom: {
        existingOption: { enabled: true },
        chatComposerDraft: {
          document: {
            type: "doc",
            content: [
              {
                type: "paragraph",
                content: [{ type: "mention", attrs }, text],
              },
            ],
          },
        },
      },
    };
    const first = snapshotComposerMessage(message);
    attrs.id = "flow-b";
    attrs.label = "乙";
    text.text = "乙正文";
    const second = snapshotComposerMessage(message);
    expect(first.runConfig?.custom).toEqual({
      existingOption: { enabled: true },
      chatComposerDraft: {
        document: {
          type: "doc",
          content: [
            {
              type: "paragraph",
              content: [
                {
                  type: "mention",
                  attrs: {
                    id: "flow-a",
                    label: "甲",
                    referenceType: "workflow",
                  },
                },
                { type: "text", text: "甲正文" },
              ],
            },
          ],
        },
      },
    });
    expect(second.runConfig?.custom?.chatComposerDraft).toEqual({
      document: {
        type: "doc",
        content: [
          {
            type: "paragraph",
            content: [
              {
                type: "mention",
                attrs: { id: "flow-b", label: "乙", referenceType: "workflow" },
              },
              { type: "text", text: "乙正文" },
            ],
          },
        ],
      },
    });
  });

  it("在 Chat 路由子树卸载后仍保留临时队列", () => {
    render(<RendererShell />);
    fireEvent.click(screen.getByRole("button", { name: "enqueue" }));
    expect(screen.getByTestId("queue-size")).toHaveTextContent("1");

    fireEvent.click(screen.getByRole("button", { name: "toggle route" }));
    expect(screen.queryByTestId("queue-size")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "toggle route" }));

    expect(screen.getByTestId("queue-size")).toHaveTextContent("1");
  });

  it("按 Chat 隔离并在草稿取得稳定 ID 后迁移队列", () => {
    const store = new ChatQueueStore();
    const queued = { ...entry("draft-entry"), chatId: undefined };
    store.enqueue("draft:waker-1", queued);
    store.enqueue("chat-2", entry("other"));
    store.migrate("draft:waker-1", "chat-1");

    expect(store.getEntries("draft:waker-1")).toHaveLength(0);
    expect(store.getEntries("chat-1")[0]).toMatchObject({
      id: "draft-entry",
      chatId: "chat-1",
      selection: { runtimeId: "codex", model: "gpt-5", reasoning: "high" },
    });
    expect(store.getEntries("chat-2")).toHaveLength(1);
  });
});
