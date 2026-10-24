import type {
  GithubComJieWaZiNeuxAllyServerApisPluginsPluginDetailResponse as PluginDetail,
  GithubComJieWaZiNeuxAllyServerRepositoriesModelsPluginVersion as PluginVersion,
  GithubComJieWaZiNeuxAllyServerInternalPluginsCatalogEntry as PluginCatalogEntry,
} from "@/shared/api/generated/types.gen";
export type { PluginDetail, PluginVersion, PluginCatalogEntry };
export type ResourceCategory = "plugins" | "skills" | "connectors";
export function resourceCategory(value: string | null): ResourceCategory {
  return value === "skills" || value === "connectors" ? value : "plugins";
}
/** 扩展只影响客户端设置，不重写标准组件。 */
export function pluginProvider(version: PluginVersion | undefined): string {
  const extension = version?.package?.manifest?.extensions?.["com.ally"];
  return extension &&
    typeof extension === "object" &&
    "provider" in extension &&
    typeof extension.provider === "string"
    ? extension.provider
    : "";
}
