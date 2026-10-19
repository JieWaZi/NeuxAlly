import { i18n } from "@/shared/i18n";
import type {
  GithubComJieWaZiNeuxAllyServerApisPluginsPluginDetailResponse as PluginDetail,
  GithubComJieWaZiNeuxAllyServerRepositoriesModelsPluginVersion as PluginVersion,
  GithubComJieWaZiNeuxAllyServerInternalPluginsCatalogEntry as PluginCatalogEntry,
} from "@/shared/api/generated/types.gen";
export type { PluginDetail, PluginVersion, PluginCatalogEntry };
export type ResourceCategory = "plugins" | "skills" | "connectors";

/** 三种详情和列表共用同一简介来源，作者与名称不再占用简介位置。 */
export function pluginSummary(
  pkg: PluginCatalogEntry["package"],
  fallback?: string,
  _sourceKey?: string,
  locale = pluginLocale(),
): string {
  // 来源身份不参与替换包内的标准简介。
  void _sourceKey;
  return (
    pkg?.presentation?.localizedDescriptions?.[pluginLocale(locale)]?.trim() ||
    pkg?.presentation?.localizedDescriptions?.en?.trim() ||
    pkg?.presentation?.shortDescription?.trim() ||
    pkg?.manifest?.description?.trim() ||
    fallback?.trim() ||
    ""
  );
}

export function pluginExplanation(
  pkg: PluginCatalogEntry["package"],
  _sourceKey?: string,
): string {
  // 保留调用入口的来源参数，正文始终来自当前包。
  void _sourceKey;
  return (
    pkg?.readme ||
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

/** 新包按授权声明接入，旧包沿原有扩展读取，不维护供应商名单。 */
export function pluginHasAuthorization(
  pkg: PluginCatalogEntry["package"],
): boolean {
  return Boolean(
    pkg?.integration?.authentication?.strategy ||
    pluginProvider({ package: pkg }),
  );
}

/** 仅已存在实际安装且共享身份明确失效时，目录操作才是重新授权。 */
export function pluginNeedsAuthorization(
  _pkg: PluginCatalogEntry["package"],
  detail?: PluginDetail,
): boolean {
  return Boolean(
    detail?.installations?.length && detail.account?.available === false,
  );
}

/** 展示名来自插件声明；用户重命名始终优先。 */
export function pluginDisplayName(
  pkg: PluginCatalogEntry["package"],
  name?: string,
  locale = pluginLocale(),
): string {
  const authoredNames = [pkg?.manifest?.name, pkg?.presentation?.displayName];
  if (name && !authoredNames.includes(name)) return name;
  const localized =
    pkg?.presentation?.displayNames?.[pluginLocale(locale)]?.trim() ||
    pkg?.presentation?.displayNames?.en?.trim();
  if (localized) return localized;
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

/** 展示与正文请求共用同一语言选择，未知界面语言回退英文。 */
export function pluginLocale(
  locale = i18n.resolvedLanguage ?? i18n.language,
): "en" | "zh-CN" {
  return locale.toLowerCase().startsWith("zh") ? "zh-CN" : "en";
}
