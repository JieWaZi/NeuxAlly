import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";

import {
  chatListKey,
  chatMessagesKey,
  evictDeletedChatsFromCache,
  nextChatCursor,
  nextMessageCursor,
} from "./chat-queries";
import { getChatQueryKey } from "@/shared/api/generated/@tanstack/react-query.gen";

describe("chat query cursors", () => {
  it("按伙伴和 Chat 隔离分页缓存", () => {
    expect(chatListKey("waker-1")).not.toEqual(chatListKey("waker-2"));
    expect(chatMessagesKey("chat-1")).not.toEqual(chatMessagesKey("chat-2"));
  });

  it("只在服务端确认还有下一页时返回稳定游标", () => {
    expect(
      nextChatCursor({
        hasMore: true,
        nextBeforeUpdatedAt: 100,
        nextBeforeId: "chat-50",
      }),
    ).toEqual({ beforeUpdatedAt: 100, beforeId: "chat-50" });
    expect(
      nextChatCursor({
        hasMore: false,
        nextBeforeUpdatedAt: 100,
        nextBeforeId: "chat-50",
      }),
    ).toBeUndefined();
    expect(
      nextMessageCursor({ hasMore: true, nextBeforeSequence: 81 }),
    ).toEqual({ beforeSequence: 81 });
    expect(nextMessageCursor({ hasMore: true })).toBeUndefined();
  });

  it("evicts deleted chats from list, detail and message caches", () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(chatMessagesKey("chat-1"), {
      pages: [{ resources: [{ id: "message-1" }], hasMore: false }],
      pageParams: [undefined],
    });
    queryClient.setQueryData(getChatQueryKey({ path: { chatId: "chat-1" } }), {
      chat: { id: "chat-1", wakerId: "waker-1" },
    });
    queryClient.setQueryData(chatListKey("waker-1"), {
      pages: [
        {
          resources: [
            { id: "chat-1", wakerId: "waker-1" },
            { id: "chat-2", wakerId: "waker-1" },
          ],
          hasMore: false,
        },
      ],
      pageParams: [undefined],
    });

    evictDeletedChatsFromCache(queryClient, ["chat-1"]);

    expect(queryClient.getQueryData(chatMessagesKey("chat-1"))).toBeUndefined();
    expect(
      queryClient.getQueryData(getChatQueryKey({ path: { chatId: "chat-1" } })),
    ).toBeUndefined();
    expect(queryClient.getQueryData(chatListKey("waker-1"))).toEqual({
      pages: [
        {
          resources: [{ id: "chat-2", wakerId: "waker-1" }],
          hasMore: false,
        },
      ],
      pageParams: [undefined],
    });
  });
});
