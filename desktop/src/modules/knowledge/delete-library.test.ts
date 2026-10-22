import { beforeEach, describe, expect, it, vi } from "vitest";
import { emptyLibrary } from "./knowledge-model";
import { createLibraryDeleter } from "./delete-library";
import "./use-copy";
import { ApiError } from "@/shared/api/errors";
let deleteLibraryPermanently: ReturnType<typeof createLibraryDeleter>;
const api = vi.hoisted(() => ({
  getRemoteCapabilities: vi.fn(),
  getRemoteOperation: vi.fn(),
  deleteLibrary: vi.fn(),
  deleteRemoteLibrary: vi.fn(),
  getLibraryDeletion: vi.fn(),
  getRemoteLibraryDeletion: vi.fn(),
  recoverLibraryDeletion: vi.fn(),
  recoverRemoteLibraryDeletion: vi.fn(),
}));
vi.mock("@/shared/api/generated/sdk.gen", () => api);
const library = () => ({
  ...emptyLibrary("kb", "Knowledge", "general"),
  revision: 4,
});
describe("整库删除确认", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    deleteLibraryPermanently = createLibraryDeleter();
    api.getRemoteOperation.mockResolvedValue({
      data: undefined,
      response: { status: 404 },
    });
    api.getRemoteCapabilities.mockResolvedValue({
      data: {
        operations: { libraryDelete: true },
        details: {
          libraryDelete: {
            supported: true,
            permission: "allowed",
            condition: "ready",
          },
        },
      },
    });
    api.deleteLibrary.mockResolvedValue({ data: { state: "accepted" } });
    api.deleteRemoteLibrary.mockResolvedValue({ data: { state: "accepted" } });
  });
  it("接纳回执不会冒充原件清理完成", async () => {
    api.getLibraryDeletion.mockResolvedValue({
      data: { state: "blocked", version: 1 },
    });
    api.recoverLibraryDeletion.mockResolvedValue({
      data: { state: "accepted" },
    });
    await expect(
      deleteLibraryPermanently(library(), "stable"),
    ).rejects.toMatchObject({ code: "KNOWLEDGE_DELETION_INCOMPLETE" });
    expect(api.recoverLibraryDeletion).toHaveBeenCalledWith(
      expect.objectContaining({
        body: { requestId: expect.any(String), expectedVersion: 1 },
      }),
    );
  });
  it("远端清理完成才返回成功，重试始终使用同一删除请求", async () => {
    const value = library();
    value.ref = {
      backend: "custom",
      connectionId: "remote",
      knowledgeBaseId: "kb",
    };
    api.getRemoteLibraryDeletion.mockResolvedValue({
      data: { state: "completed", version: 2 },
    });
    await deleteLibraryPermanently(value, "same-request");
    await deleteLibraryPermanently(value, "same-request");
    expect(api.deleteRemoteLibrary).toHaveBeenCalledTimes(2);
    for (const [options] of api.deleteRemoteLibrary.mock.calls)
      expect(options).toMatchObject({
        path: { connectionId: "remote", libraryId: "kb" },
        body: { requestId: "same-request", expectedVersion: 4 },
      });
    expect(api.deleteLibrary).not.toHaveBeenCalled();
    expect(api.recoverRemoteLibraryDeletion).not.toHaveBeenCalled();
  });
  it("远端清理明确失败后用新请求重试，未知结果保留原请求", async () => {
    const value = library();
    value.ref = {
      backend: "custom",
      connectionId: "remote",
      knowledgeBaseId: "kb",
    };
    api.getRemoteLibraryDeletion.mockResolvedValue({
      data: { libraryId: "kb", policyVersion: 1, state: "pending", version: 1 },
    });
    api.recoverRemoteLibraryDeletion
      .mockRejectedValueOnce(
        new ApiError("version conflict", "KnowledgeConflict", 409),
      )
      .mockRejectedValueOnce(
        new ApiError("unknown", "KnowledgeRemoteOutcomeUnknown", 504),
      )
      .mockRejectedValueOnce(
        new ApiError("unknown", "KnowledgeRemoteOutcomeUnknown", 504),
      );
    for (let i = 0; i < 3; i++)
      await expect(
        deleteLibraryPermanently(value, "stable"),
      ).rejects.toBeInstanceOf(ApiError);
    const ids = api.recoverRemoteLibraryDeletion.mock.calls.map(
      ([options]) => options.body.requestId,
    );
    expect(ids[0]).not.toBe(ids[1]);
    expect(ids[1]).toBe(ids[2]);
  });
  it.each([408, 500, 503])(
    "清理恢复 HTTP %i 未知结果重用原请求身份",
    async (status) => {
      api.getLibraryDeletion.mockResolvedValue({
        data: { state: "pending", version: 1 },
      });
      api.recoverLibraryDeletion.mockRejectedValue(
        new ApiError("response lost", "HTTP_ERROR", status),
      );
      const remove = createLibraryDeleter();
      await expect(remove(library(), "original-delete")).rejects.toBeInstanceOf(
        ApiError,
      );
      await expect(remove(library(), "reopened-delete")).rejects.toBeInstanceOf(
        ApiError,
      );
      const [first, second] = api.recoverLibraryDeletion.mock.calls;
      expect(first?.[0].body.requestId).toBeTruthy();
      expect(second?.[0].body).toEqual(first?.[0].body);
    },
  );
  it("关闭后重开确认框仍核对原删除及其冻结版本", async () => {
    api.deleteLibrary.mockRejectedValueOnce(new ApiError("unknown", "NETWORK"));
    await expect(
      deleteLibraryPermanently(library(), "first-dialog"),
    ).rejects.toBeInstanceOf(ApiError);
    api.getLibraryDeletion.mockResolvedValue({
      data: { state: "completed", version: 2 },
    });
    await deleteLibraryPermanently(
      { ...library(), revision: 5 },
      "reopened-dialog",
    );
    expect(api.deleteLibrary.mock.calls[1]?.[0]).toMatchObject({
      body: { requestId: "first-dialog", expectedVersion: 4 },
    });
  });
  it("响应丢失时保留未知错误，不能宣称删除完成", async () => {
    api.deleteLibrary.mockRejectedValue(new Error("unknown outcome"));
    await expect(deleteLibraryPermanently(library(), "stable")).rejects.toThrow(
      "unknown outcome",
    );
    expect(api.getLibraryDeletion).not.toHaveBeenCalled();
  });
});

it("核心已删库但聊天清理失败时，关闭重开继续原删除身份", async () => {
  vi.resetAllMocks();
  const remove = createLibraryDeleter();
  api.deleteLibrary
    .mockRejectedValueOnce(
      new ApiError("chat log cleanup failed", "ChatLogError", 500),
    )
    .mockResolvedValueOnce({ data: { state: "accepted" } });
  api.getLibraryDeletion.mockResolvedValue({
    data: { state: "completed", version: 2 },
  });
  await expect(remove(library(), "original-delete")).rejects.toBeInstanceOf(
    ApiError,
  );
  await remove({ ...library(), revision: 5 }, "reopened-delete");
  expect(api.deleteLibrary.mock.calls[1]?.[0]).toMatchObject({
    body: { requestId: "original-delete", expectedVersion: 4 },
  });
});
