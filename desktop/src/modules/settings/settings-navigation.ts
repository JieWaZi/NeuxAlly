import { Bot, Info, KeyRound, Settings2 } from "lucide-react";

export type SettingsSection =
  "general" | "ai-accounts" | "runtime-environments" | "about";

export const settingsNavigationGroups = [
  {
    titleKey: "groups.app",
    items: [
      { key: "general", labelKey: "sections.general", icon: Settings2 },
      { key: "about", labelKey: "sections.about", icon: Info },
    ],
  },
  {
    titleKey: "groups.ai",
    items: [
      { key: "ai-accounts", labelKey: "sections.accounts", icon: KeyRound },
      {
        key: "runtime-environments",
        labelKey: "sections.runtimeEnvironments",
        icon: Bot,
      },
    ],
  },
] as const satisfies ReadonlyArray<{
  titleKey: string;
  items: ReadonlyArray<{
    key: SettingsSection;
    labelKey: string;
    icon: typeof Settings2;
  }>;
}>;

export const settingsNavigation = settingsNavigationGroups.flatMap(
  (group) =>
    [...group.items] as ReadonlyArray<{
      key: SettingsSection;
      labelKey: string;
      icon: typeof Settings2;
    }>,
);

export function settingsSectionOf(value?: string): SettingsSection {
  return value === "runtime-environments" ||
    value === "ai-accounts" ||
    value === "about"
    ? value
    : "general";
}
