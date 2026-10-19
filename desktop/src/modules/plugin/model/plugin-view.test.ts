import { expect, it } from "vitest";
import {
  pluginSummary,
  pluginExplanation,
  pluginHasAuthorization,
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

it("keeps the selected source README and summary when a generated guide has matching capabilities", () => {
  expect(pluginSummary(pkg, "本地摘要", "catalog:github")).toBe("旧版摘要");
  expect(pluginExplanation(pkg, "catalog:github")).toBe("旧版构建与隔离细节");
  expect(pkg.readme).toBe("旧版构建与隔离细节");
  expect(pkg.manifest?.version).toBe("1.0.1");
});

it("recognizes authorization declarations without a provider allowlist", () => {
  expect(
    pluginHasAuthorization({
      integration: {
        authentication: {
          strategy: "plugin-command",
          protocol: "neuxally-auth/1",
          command: "./.neuxally/auth/authorize",
        },
      },
    }),
  ).toBe(true);
  expect(pluginHasAuthorization({ manifest: { name: "skills-only" } })).toBe(
    false,
  );
});

it("keeps third-party and mismatching snapshots on their own documentation", () => {
  expect(pluginExplanation(pkg)).toBe(pkg.readme);
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
