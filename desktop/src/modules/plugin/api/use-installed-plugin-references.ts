import { useQuery } from "@tanstack/react-query";
import { listWakerPlugins } from "@/shared/api/generated/sdk.gen";
import { useQueryEvents } from "@/shared/api/use-query-events";
import type { PluginDetail } from "../model/plugin-view";

/** 引用选择读取同一安装事实，逐页加载，避免只允许引用第一页的插件。 */
export function useInstalledPluginReferences(
  wakerId: string | undefined,
  active = true,
) {
  const queryKey = [
    { _id: "listWakerPluginReferences", path: { wakerId } },
  ] as const;
  const query = useQuery({
    queryKey,
    enabled: active && Boolean(wakerId),
    queryFn: async ({ signal }) => {
      if (!wakerId) return [];
      const resources: PluginDetail[] = [];
      for (let page = 1; ; page++) {
        const response = await listWakerPlugins({
          path: { wakerId },
          query: { pageNum: page, pageSize: 100 },
          signal,
          throwOnError: true,
        });
        const batch = response.data.resources ?? [];
        resources.push(...batch);
        if (
          resources.length >= (response.data.totalSize ?? 0) ||
          batch.length === 0
        )
          break;
      }
      return resources;
    },
  });
  useQueryEvents({
    topic: "plugins",
    resource: { kind: "global" },
    queryKeys: [queryKey],
    active,
    enabled: Boolean(wakerId),
  });
  const plugins = (query.data ?? []).flatMap((detail) => {
    const relation = detail.installations?.find(
      (item) => item.wakerId === wakerId,
    );
    const version = detail.versions?.find(
      (item) => item.id === relation?.versionId,
    );
    if (!relation?.enabled || !detail.ready || !detail.plugin?.id || !version)
      return [];
    return [
      {
        id: detail.plugin.id,
        name: detail.plugin.name ?? "",
        description: detail.plugin.description ?? "",
        version: version.version,
        versionId: version.id,
      },
    ];
  });
  return { ...query, plugins };
}
