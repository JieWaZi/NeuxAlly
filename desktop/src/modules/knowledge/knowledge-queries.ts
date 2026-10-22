import { queryOptions } from "@tanstack/react-query";
import {
  knowledgeClient,
  KNOWLEDGE_SOURCE_PAGE_SIZE,
} from "@/shared/api/knowledge";
import {
  knowledgeAdapter,
  type KnowledgeSourceFilters,
} from "@/shared/api/knowledge";
import { withLocalErrorSurface } from "@/shared/api/local-error-surface";
import type { ResourceRef } from "./knowledge-transfer";
import {
  emptyLibrary,
  type Library,
  type LibraryLocation,
  type Source,
  isOrganizableSourceState,
} from "./knowledge-model";
import {
  readEmbeddingModelId,
  readLibraryIndexingStrategy,
  readLibraryWikiSettings,
  readLibraryChunkingConfig,
} from "./knowledge-library-config";
import type {
  GithubComJieWaZiNeuxAllyServerApisKnowledgeLibrary as LibraryDTO,
  GithubComJieWaZiNeuxAllyServerApisKnowledgeSource as SourceDTO,
} from "@/shared/api/generated/types.gen";
export const knowledgeKeys = {
  all: ["knowledge"] as const,
  libraryLists: ["knowledge", "libraries"] as const,
  jobs: (ref: ResourceRef) => [...knowledgeKeys.resource(ref), "jobs"] as const,
  job: (ref: ResourceRef, id: string) =>
    [...knowledgeKeys.resource(ref), "job", id] as const,
  libraries: (offset = 0, location = "all") =>
    ["knowledge", "libraries", location, offset] as const,
  resource: (ref: ResourceRef) =>
    ["knowledge", ref.backend, ref.connectionId, ref.knowledgeBaseId] as const,
};
export function librariesOptions(
  offset: number,
  location: LibraryLocation = "all",
) {
  return queryOptions({
    queryKey: knowledgeKeys.libraries(offset, location),
    // 返回列表时读取详情已更新的统计摘要，不等待全局缓存新鲜期。
    staleTime: 0,
    queryFn: async ({ signal }) =>
      (
        await knowledgeAdapter({
          backend: "local",
          connectionId: "local",
          knowledgeBaseId: "",
        }).listAvailableLibraries(
          withLocalErrorSurface({
            query: { offset, limit: 50, location },
            signal,
            throwOnError: true,
          }),
        )
      ).data,
  });
}
export function libraryOptions(ref: ResourceRef) {
  return queryOptions({
    queryKey: [...knowledgeKeys.resource(ref), "detail"],
    queryFn: ({ signal }) => knowledgeClient(ref).getLibrary(signal),
    enabled: Boolean(ref.knowledgeBaseId),
  });
}
export function sourcesOptions(
  ref: ResourceRef,
  offset: number,
  enabled: boolean,
  pageSize = KNOWLEDGE_SOURCE_PAGE_SIZE,
  filters: KnowledgeSourceFilters = {},
) {
  return queryOptions({
    queryKey: [
      ...knowledgeKeys.resource(ref),
      "sources",
      offset,
      pageSize,
      filters,
    ],
    queryFn: ({ signal }) =>
      knowledgeClient(ref).listSources(offset, signal, pageSize, filters),
    enabled: enabled && Boolean(ref.knowledgeBaseId),
    placeholderData: undefined,
  });
}

/** 打开整理表单时完整读取来源，默认范围不受列表翻页与筛选影响。 */
export function organizableSourcesOptions(ref: ResourceRef) {
  return queryOptions({
    queryKey: [...knowledgeKeys.resource(ref), "sources", "organizable"],
    staleTime: 0,
    queryFn: async ({ signal }) => {
      const sources: Source[] = [];
      let offset = 0;
      while (true) {
        const page = await knowledgeClient(ref).listSources(offset, signal);
        const items = page.items ?? [];
        sources.push(...items.map((item) => toSource(item, ref.backend)));
        offset += items.length;
        if (offset >= (page.total ?? offset)) break;
        if (!items.length) throw new Error("Incomplete knowledge source list");
      }
      return sources.filter((source) => isOrganizableSourceState(source.state));
    },
  });
}
export function toLibrary(v: LibraryDTO): Library {
  return {
    ...emptyLibrary(
      v.id ?? "",
      v.name ?? "",
      v.kind === "software"
        ? "software"
        : v.kind === "general"
          ? "general"
          : "official",
    ),
    ref: {
      backend: v.ref?.backend ?? "local",
      connectionId: v.ref?.connectionId ?? "local",
      knowledgeBaseId: v.ref?.knowledgeBaseId ?? v.id ?? "",
    },
    storage: v.ref?.backend === "local" ? "local" : "remote",
    description: v.purpose ?? "",
    pageTotal: v.pageTotal,
    pageTotalObservedAt: v.pageTotalObservedAt,
    language: v.wikiLanguage === "en-US" ? "en-US" : "zh-CN",
    revision: v.version ?? 0,
    structureVersion: v.structureVersion ?? 0,
    policyVersion: v.policyVersion ?? 1,
    writable: v.ref?.backend === "local" && !v.archived,
    wiki: readLibraryWikiSettings(v),
    indexing: readLibraryIndexingStrategy(v),
    embeddingModelId: readEmbeddingModelId(v),
    chunkingConfig: readLibraryChunkingConfig(v),
  };
}
export function toSource(v: SourceDTO, backend = "local"): Source {
  return {
    sourceId: v.sourceId,
    id: v.id ?? "",
    name: v.filename || v.id || "",
    size:
      backend === "weknora"
        ? "—"
        : v.size === 0
          ? "0 B"
          : `${Math.ceil((v.size ?? 0) / 1024)} KB`,
    state:
      backend === "weknora"
        ? "unknown"
        : v.archived
          ? "archived"
          : v.state === "queued"
            ? "queued"
            : v.state === "processing"
              ? "processing"
              : v.state === "processed"
                ? "processed"
                : v.state === "no-change"
                  ? "unchanged"
                  : v.state === "cancelled"
                    ? "cancelled"
                    : v.state === "failed"
                      ? "failed"
                      : v.state === "pending"
                        ? "pending"
                        : "unknown",
    createdAt: v.createdAt,
    date: v.createdAt ? new Date(v.createdAt).toLocaleDateString() : "—",
    diagnostic: v.diagnostic ?? "",
  };
}
