export { listRuntimesOptions } from "@/shared/api/runtime-queries";
export {
  cancelChatExecutionMutation,
  createChatMutation,
  deleteChatMutation,
  getChatOptions,
  getChatQueryKey,
  getChatMessageFileChangeOptions,
  getChatMessageFileChangeQueryKey,
  listChatsQueryKey,
  listChatMessagesOptions,
  listChatMessagesQueryKey,
  listRuntimesQueryKey,
  resolveApprovalMutation,
  resolveElicitationMutation,
  listWakerSkillsOptions,
  listWakerSkillsQueryKey,
  listWakerChatsOptions,
  listWakerChatsQueryKey,
  sendMessageMutation,
  updateChatMutation,
} from "@/shared/api/generated/@tanstack/react-query.gen";

import {
  infiniteQueryOptions,
  type InfiniteData,
  type QueryClient,
} from "@tanstack/react-query";

import { clearChatLastActiveIfMatches } from "@/shared/lib/chat-navigation";

import { getChatQueryKey } from "@/shared/api/generated/@tanstack/react-query.gen";
import { listChatMessages, listChats } from "@/shared/api/generated/sdk.gen";
import type {
  ListChatMessagesResponse,
  ListChatsResponse,
} from "@/shared/api/generated/types.gen";

type ChatCursor = {
  beforeUpdatedAt: number;
  beforeId: string;
};

type MessageCursor = {
  beforeSequence: number;
};

export const chatListKey = (wakerId: string) =>
  ["chats", "list", wakerId] as const;

export const chatMessagesKey = (chatId: string) =>
  ["chats", "messages", chatId] as const;

export function nextChatCursor(
  page: ListChatsResponse,
): ChatCursor | undefined {
  if (!page.hasMore || !page.nextBeforeUpdatedAt || !page.nextBeforeId) {
    return undefined;
  }
  return {
    beforeUpdatedAt: page.nextBeforeUpdatedAt,
    beforeId: page.nextBeforeId,
  };
}

export function nextMessageCursor(
  page: ListChatMessagesResponse,
): MessageCursor | undefined {
  if (!page.hasMore || !page.nextBeforeSequence) return undefined;
  return { beforeSequence: page.nextBeforeSequence };
}

export function chatListInfiniteOptions(wakerId: string) {
  return infiniteQueryOptions({
    queryKey: chatListKey(wakerId),
    initialPageParam: undefined as ChatCursor | undefined,
    queryFn: async ({ pageParam, signal }) => {
      const { data } = await listChats({
        query: { wakerId, limit: 50, ...pageParam },
        signal,
        throwOnError: true,
      });
      return data;
    },
    getNextPageParam: nextChatCursor,
  });
}

export function chatMessagesInfiniteOptions(chatId: string) {
  return infiniteQueryOptions({
    queryKey: chatMessagesKey(chatId),
    initialPageParam: undefined as MessageCursor | undefined,
    queryFn: async ({ pageParam, signal }) => {
      const { data } = await listChatMessages({
        path: { chatId },
        query: { limit: 50, ...pageParam },
        signal,
        throwOnError: true,
      });
      return data;
    },
    getNextPageParam: nextMessageCursor,
    staleTime: 5 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
  });
}

function filterDeletedChatsFromListPages<
  TPage extends { resources?: readonly { id?: string }[] },
>(pages: readonly TPage[], deletedIds: ReadonlySet<string>): TPage[] {
  return pages.map((page) => ({
    ...page,
    resources: page.resources?.filter((chat) => !deletedIds.has(chat.id ?? "")),
  }));
}

export function evictDeletedChatsFromCache(
  queryClient: QueryClient,
  chatIds: readonly string[],
) {
  const deletedIds = new Set(
    chatIds.map((chatId) => chatId.trim()).filter(Boolean),
  );
  if (deletedIds.size === 0) return;

  for (const chatId of deletedIds) {
    queryClient.removeQueries({
      queryKey: chatMessagesKey(chatId),
    });
    queryClient.removeQueries({
      queryKey: getChatQueryKey({ path: { chatId } }),
    });
    clearChatLastActiveIfMatches(chatId);
  }

  queryClient.setQueriesData<InfiniteData<ListChatsResponse>>(
    { queryKey: ["chats", "list"] },
    (current) => {
      if (!current) return current;
      return {
        ...current,
        pages: filterDeletedChatsFromListPages(current.pages, deletedIds),
      };
    },
  );
}
