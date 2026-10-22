import { knowledgeAdapter } from "@/shared/api/knowledge";
import { ApiError } from "@/shared/api/errors";
import { isKnowledgeWriteOutcomeUnknown } from "./knowledge-write-outcome";
import { withLocalErrorSurface } from "@/shared/api/local-error-surface";
import { i18n } from "@/shared/i18n";
import type { Library } from "./knowledge-model";

/** 重试复用删除身份；只有持久清理状态完成才向界面返回成功。 */
export function createLibraryDeleter() {
  const recoveries = new Map<string, { version: number; requestId: string }>();
  const deletions = new Map<string, { library: Library; requestId: string }>();
  return async (inputLibrary: Library, inputRequestId: string) => {
    const identity = JSON.stringify([
      inputLibrary.ref.backend,
      inputLibrary.ref.connectionId,
      inputLibrary.ref.knowledgeBaseId,
    ]);
    // 关闭后重新打开确认框也继续核对原删除，避免未知结果生成第二个写请求。
    let deletion = deletions.get(identity);
    if (!deletion) {
      deletion = { library: inputLibrary, requestId: inputRequestId };
      deletions.set(identity, deletion);
    }
    const { library, requestId } = deletion;
    const path = {
      libraryId: library.id,
      connectionId: library.ref.connectionId,
    };
    const body = { requestId, expectedVersion: library.revision };
    const options = withLocalErrorSurface({
      path,
      body,
      throwOnError: true as const,
    });
    try {
      await knowledgeAdapter(library.ref).deleteLibrary(options);
    } catch (cause) {
      if (!isKnowledgeWriteOutcomeUnknown(cause)) {
        deletions.delete(identity);
        recoveries.delete(requestId);
      }
      throw cause;
    }
    const read = async () =>
      (
        await knowledgeAdapter(library.ref).getLibraryDeletion(
          withLocalErrorSurface({ path, throwOnError: true }),
        )
      ).data;
    let status = await read();
    if (status?.state !== "completed" && status?.version) {
      let attempt = recoveries.get(requestId);
      if (!attempt || attempt.version !== status.version) {
        attempt = { version: status.version, requestId: crypto.randomUUID() };
        recoveries.set(requestId, attempt);
      }
      const recovery = withLocalErrorSurface({
        path,
        body: {
          requestId: attempt.requestId,
          expectedVersion: status.version,
        },
        throwOnError: true as const,
      });
      try {
        await knowledgeAdapter(library.ref).recoverLibraryDeletion(recovery);
        recoveries.delete(requestId);
      } catch (cause) {
        // 网络与未知结果继续核对原请求；明确拒绝后才允许新的清理尝试。
        if (!isKnowledgeWriteOutcomeUnknown(cause)) {
          recoveries.delete(requestId);
        }
        throw cause;
      }
      status = await read();
    }
    if (status?.state !== "completed") {
      throw new ApiError(
        i18n.t(
          status?.state === "blocked"
            ? "knowledge:libraryDeletionBlocked"
            : "knowledge:libraryDeletionPending",
        ),
        "KNOWLEDGE_DELETION_INCOMPLETE",
      );
    }
    recoveries.delete(requestId);
    deletions.delete(identity);
  };
}
