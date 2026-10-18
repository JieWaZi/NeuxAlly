import type {
  GithubComJieWaZiNeuxAllyServerApisPluginsPluginDetailResponse as PluginDetail,
  GithubComJieWaZiNeuxAllyServerRepositoriesModelsPluginVersion as PluginVersion,
  GithubComJieWaZiNeuxAllyServerInternalPluginsCatalogEntry as PluginCatalogEntry,
} from "@/shared/api/generated/types.gen";
import builtinGuides from "./builtin-plugin-guides.gen.json";
export type { PluginDetail, PluginVersion, PluginCatalogEntry };
export type ResourceCategory = "plugins" | "skills" | "connectors";

/** 阅读资料可独立完善；来源、包身份与能力集合不匹配时保留原版本说明。 */
export function builtinPluginGuide(
  pkg: PluginCatalogEntry["package"],
  sourceKey?: string,
) {
  return builtinGuides.find(
    (guide) =>
      sourceKey === `catalog:${guide.key}` &&
      pkg?.manifest?.name === guide.name &&
      pkg.manifest.repository === guide.repository &&
      (pkg.skills ?? [])
        .map((item) => item.key ?? "")
        .sort()
        .join("\n") === guide.skills.join("\n") &&
      (pkg.servers ?? [])
        .map((item) => item.key ?? "")
        .sort()
        .join("\n") === guide.connectors.join("\n"),
  );
}

/** 三种详情和列表共用同一简介来源，作者与名称不再占用简介位置。 */
export function pluginSummary(
  pkg: PluginCatalogEntry["package"],
  fallback?: string,
  sourceKey?: string,
): string {
  return (
    builtinPluginGuide(pkg, sourceKey)?.summary ||
    pkg?.presentation?.shortDescription?.trim() ||
    pkg?.manifest?.description?.trim() ||
    fallback?.trim() ||
    ""
  );
}

export function pluginExplanation(
  pkg: PluginCatalogEntry["package"],
  sourceKey?: string,
): string {
  return (
    builtinPluginGuide(pkg, sourceKey)?.readme ||
    pkg?.readme?.trim() ||
    pkg?.presentation?.longDescription?.trim() ||
    pluginSummary(pkg)
  );
}
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

/** 仅已存在实际安装且共享身份明确失效时，目录操作才是重新授权。 */
export function pluginNeedsAuthorization(
  pkg: PluginCatalogEntry["package"],
  detail?: PluginDetail,
): boolean {
  return Boolean(
    detail?.installations?.length &&
    detail.account?.available === false &&
    ["feishu", "github"].includes(pluginProvider({ package: pkg })),
  );
}

/** 展示名来自插件声明；用户重命名始终优先。 */
export function pluginDisplayName(
  pkg: PluginCatalogEntry["package"],
  name?: string,
): string {
  if (name && name !== pkg?.manifest?.name) return name;
  if (pkg?.presentation?.displayName) return pkg.presentation.displayName;
  const extension = pkg?.manifest?.extensions?.["com.ally"];
  return extension &&
    typeof extension === "object" &&
    "displayName" in extension &&
    typeof extension.displayName === "string"
    ? extension.displayName
    : (name ?? pkg?.manifest?.name ?? "");
}

/** 官方远程服务未固定发布版本，不将本机包装版本标为上游版本。 */
export function pluginUsesHostedVersion(
  pkg: PluginCatalogEntry["package"],
): boolean {
  return (
    pkg?.manifest?.repository ===
      "https://github.com/github/github-mcp-server" &&
    Boolean(
      pkg.servers?.some((server) =>
        server.url?.startsWith("https://api.githubcopilot.com/"),
      ),
    )
  );
}
