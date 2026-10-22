import type { ResourceRef } from "./knowledge-adapter-types";

/** 静态策略仅描述适配实现；真实操作许可仍从服务器逐次核验。 */
export function knowledgeAdapterMetadata(ref: ResourceRef) {
  const local = ref.backend === "local";
  const official = ref.backend === "weknora";
  return {
    ref,
    graphMode: official ? ("remote" as const) : ("pages" as const),
    revisionsIncludeContent: local,
    localConfiguration: local,
    supportsJobs: !official,
    supportsSourceCleanup: !official,
  };
}
