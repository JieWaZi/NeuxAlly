import type { QueryClient } from "@tanstack/react-query";

export {
  createSkillVersionMutation,
  getSkillVersionOptions,
  getSkillVersionQueryKey,
  getSkillVersionFileOptions,
  getSkillVersionFileQueryKey,
  getWakerSkillOptions,
  getWakerSkillQueryKey,
  importWakerSkillMutation,
  listSkillVersionsOptions,
  listSkillVersionsQueryKey,
  listWakerSkillsOptions,
  listWakerSkillsQueryKey,
  pullSkillUpdateMutation,
  restoreSkillVersionMutation,
  scanWakerSkillImportMutation,
  setWakerSkillEnabledMutation,
  uninstallSkillMutation,
  updateWakerSkillMutation,
} from "@/shared/api/generated/@tanstack/react-query.gen";

const visibleSkillQueryIds = new Set([
  "listWakerSkills",
  "getWakerSkill",
  "listSkillVersions",
  "getSkillVersion",
  "getSkillVersionFile",
]);

export async function invalidateVisibleSkillQueries(
  queryClient: QueryClient,
): Promise<void> {
  await queryClient.invalidateQueries({
    predicate: (query) => {
      const descriptor = query.queryKey[0] as { _id?: string } | undefined;
      return Boolean(
        descriptor?._id && visibleSkillQueryIds.has(descriptor._id),
      );
    },
  });
}
