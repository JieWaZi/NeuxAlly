import { describe, expect, it } from "vitest";
import { settingsNavigation, settingsSectionOf } from "./settings-navigation";

describe("API 集成设置导航", () => {
  it("恢复 API 集成路由且只提供一个发布入口", () => {
    expect(settingsSectionOf("integration-api")).toBe("integration-api");
    expect(
      settingsNavigation.filter(
        (item) => String(item.key) === "integration-api",
      ),
    ).toHaveLength(1);
  });
});
