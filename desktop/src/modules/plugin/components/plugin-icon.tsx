import Github from "@lobehub/icons/es/Github/components/Mono";
import { Package } from "lucide-react";
import { cn } from "@/shared/lib/cn";
import type { PluginCatalogEntry } from "../model/plugin-view";
import feishuIcon from "@/shared/assets/brands/feishu.png";

/** 品牌来自本地资源，未知插件使用统一包图标。 */
export function PluginIcon({
  pkg,
  hero = false,
}: {
  pkg?: PluginCatalogEntry["package"];
  hero?: boolean;
}) {
  const provider = pkg?.manifest?.extensions?.["com.ally"];
  const name =
    provider && typeof provider === "object" && "provider" in provider
      ? provider.provider
      : "";
  return (
    <span
      aria-hidden="true"
      className={cn(
        "text-content-primary grid shrink-0 place-items-center",
        hero ? "[&_svg]:size-icon-xl size-14" : "[&_svg]:size-icon-lg size-10",
      )}
    >
      {pkg?.icon ? (
        <img
          src={pkg.icon}
          alt=""
          className={hero ? "size-10 object-contain" : "size-8 object-contain"}
        />
      ) : name === "feishu" ? (
        <img
          src={feishuIcon}
          alt=""
          className={hero ? "size-10 object-contain" : "size-8 object-contain"}
        />
      ) : name === "github" ? (
        <Github />
      ) : (
        <Package />
      )}
    </span>
  );
}
