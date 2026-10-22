import { useQuery } from "@tanstack/react-query";
import {
  knowledgeAdapter,
  type KnowledgeSourceFilters,
} from "@/shared/api/knowledge";
import { withLocalErrorSurface } from "@/shared/api/local-error-surface";
import type { ResourceRef } from "./knowledge-transfer";
import {
  knowledgeKeys,
  librariesOptions,
  libraryOptions,
  sourcesOptions,
} from "./knowledge-queries";
import type { LibraryLocation } from "./knowledge-model";

/** 只读取列表、文档与真实整理任务；页面可见时才对账。 */
export function useKnowledgeWorkspaceQueries({
  ref,
  detail,
  documentsVisible = true,
  active,
  listOffset,
  sourceOffset,
  sourcePageSize,
  sourceFilters,
  location,
}: {
  ref: ResourceRef;
  detail: boolean;
  documentsVisible?: boolean;
  active: boolean;
  listOffset: number;
  sourceOffset: number;
  sourcePageSize: number;
  sourceFilters: KnowledgeSourceFilters;
  location: LibraryLocation;
}) {
  const libraries = useQuery({
    ...librariesOptions(listOffset, location),
    enabled: active && !detail,
  });
  const libraryQuery = useQuery({
    ...libraryOptions(ref),
    enabled: active && detail,
  });
  const sources = useQuery(
    sourcesOptions(
      ref,
      sourceOffset,
      active && detail && documentsVisible,
      sourcePageSize,
      sourceFilters,
    ),
  );
  const jobs = useQuery({
    queryKey: knowledgeKeys.jobs(ref),
    queryFn: async ({ signal }) =>
      (
        await knowledgeAdapter(ref).listJobs(
          withLocalErrorSurface({
            path: { libraryId: ref.knowledgeBaseId },
            query: { offset: 0, limit: 1 },
            signal,
            throwOnError: true,
          }),
        )
      ).data,
    enabled:
      active &&
      detail &&
      documentsVisible &&
      knowledgeAdapter(ref).supportsJobs,
  });
  const activeJob = jobs.data?.activeJob;
  return { libraries, libraryQuery, sources, jobs, activeJob };
}
