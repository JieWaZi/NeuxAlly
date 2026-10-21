import { act, renderHook } from "@testing-library/react";
import { QueryClient } from "@tanstack/react-query";
import { beforeEach, expect, it, vi } from "vitest";
import { cancelChatExecutionMutation } from "./chat-queries";
import {
  dismissErrorNotice,
  runErrorNoticeAction,
  useErrorNotices,
} from "@/shared/api/error-notices";
import { ApiError } from "@/shared/api/errors";

const { stop } = vi.hoisted(() => ({ stop: vi.fn() }));
vi.mock(
  "@/shared/api/generated/@tanstack/react-query.gen",
  async (importOriginal) => ({
    ...(await importOriginal<object>()),
    cancelChatExecutionMutation: () => ({ mutationFn: stop }),
  }),
);
beforeEach(() => {
  stop.mockReset();
  const notices = renderHook(useErrorNotices);
  act(() => {
    for (const notice of notices.result.current) dismissErrorNotice(notice.id);
  });
  notices.unmount();
});

it("停止失败提示重试原请求，下一轮请求不覆盖旧执行身份", async () => {
  stop
    .mockRejectedValueOnce(new ApiError("数据库暂不可用", "DATABASE_ERROR"))
    .mockResolvedValue({});
  const { result } = renderHook(useErrorNotices);
  const mutation = cancelChatExecutionMutation();
  const original = { path: { chatId: "chat", executionId: "old" } };
  const context = {
    client: new QueryClient(),
    meta: undefined,
    mutationKey: undefined,
  };
  await act(async () => {
    await expect(mutation.mutationFn?.(original, context)).rejects.toThrow();
  });
  const notice = result.current.find((item) => item.action === "retry");
  expect(notice).toMatchObject({ scope: "chat:chat", action: "retry" });
  await act(async () => {
    await mutation.mutationFn?.(
      { path: { chatId: "chat", executionId: "new" } },
      context,
    );
  });
  expect(result.current).toHaveLength(1);
  await act(async () => {
    if (notice) await runErrorNoticeAction(notice.id);
  });
  expect(stop.mock.calls.map(([request]) => request.path)).toEqual([
    original.path,
    { chatId: "chat", executionId: "new" },
    original.path,
  ]);
  expect(result.current).toHaveLength(0);
});
