import type { ResourceRef } from "@/shared/api/knowledge";
export type { ResourceRef } from "@/shared/api/knowledge";
/** 原件大小先校验，再读取实际字节；请求身份由调用方保留以处理未知结果。 */
export async function encodeUpload(
  file: Pick<File, "name" | "type" | "size" | "arrayBuffer">,
  requestId: string,
): Promise<{
  requestId: string;
  filename: string;
  mime: string;
  bytes: string;
}> {
  if (file.size > 32 * 1024 * 1024) throw new Error("source_too_large");
  const bytes = new Uint8Array(await file.arrayBuffer());
  let binary = "";
  for (let offset = 0; offset < bytes.length; offset += 8192)
    binary += String.fromCharCode(...bytes.subarray(offset, offset + 8192));
  return {
    requestId,
    filename: file.name,
    mime: file.type,
    bytes: btoa(binary),
  };
}
export function resourceKey(ref: ResourceRef): string {
  return JSON.stringify([ref.backend, ref.connectionId, ref.knowledgeBaseId]);
}
/** 文档操作只由持久终态解除保护；未知状态继续按运行中处理。 */
export function isKnowledgeJobBusy(state: string) {
  return !["completed", "partial_failed", "failed", "cancelled"].includes(
    state,
  );
}
