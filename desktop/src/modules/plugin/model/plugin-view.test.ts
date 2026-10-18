import { expect, it } from "vitest";
import guides from "./builtin-plugin-guides.gen.json";
import {
  builtinPluginGuide,
  pluginSummary,
  pluginExplanation,
  type PluginCatalogEntry,
} from "./plugin-view";

const pkg: PluginCatalogEntry["package"] = {
  manifest: {
    name: "github",
    repository: "https://github.com/github/github-mcp-server",
    description: "旧版摘要",
    version: "1.0.1",
  },
  skills: [],
  servers: [{ key: "github", url: "https://api.githubcopilot.com/mcp/" }],
  readme: "旧版构建与隔离细节",
};

it("uses source-generated user guides for matching installed capabilities without changing the package", () => {
  const guide = guides.find((item) => item.key === "github");
  expect(guide).toBeDefined();
  expect(pluginSummary(pkg, "本地摘要", "catalog:github")).toBe(guide?.summary);
  expect(pluginExplanation(pkg, "catalog:github")).toBe(guide?.readme);
  expect(pkg.readme).toBe("旧版构建与隔离细节");
  expect(pkg.manifest?.version).toBe("1.0.1");
});

it("keeps third-party and mismatching snapshots on their own documentation", () => {
  expect(pluginExplanation(pkg)).toBe(pkg.readme);
  expect(
    builtinPluginGuide({ ...pkg, servers: [] }, "catalog:github"),
  ).toBeUndefined();
  expect(
    builtinPluginGuide(
      {
        ...pkg,
        manifest: { ...pkg.manifest, repository: "https://example.com" },
      },
      "catalog:github",
    ),
  ).toBeUndefined();
  expect(pluginExplanation({ ...pkg, servers: [] }, "catalog:github")).toBe(
    pkg.readme,
  );
});

it("uses the same summary fallback for every entry and never substitutes the author", () => {
  const entry = {
    manifest: { description: "插件能力", author: { name: "作者" } },
    presentation: { shortDescription: " ", developerName: "开发者" },
  };
  expect(pluginSummary(entry)).toBe("插件能力");
  expect(
    pluginSummary({ presentation: { developerName: "开发者" } }, "保存的简介"),
  ).toBe("保存的简介");
});
