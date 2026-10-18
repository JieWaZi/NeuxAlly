import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { expect, it, vi } from "vitest";
import "@/shared/i18n";
import { PluginCatalogDetailPage } from "./plugin-catalog-detail-page";
import type { PluginDetail } from "../model/plugin-view";

vi.mock("@/shared/api/use-query-events", () => ({
  useQueryEvents: () => undefined,
}));
vi.mock("../api/plugin-queries", () => {
  const pkg = {
    manifest: {
      name: "github",
      repository: "https://github.com/github/github-mcp-server",
      description: "旧简介",
      author: { name: "开发者署名" },
    },
    presentation: { displayName: "GitHub" },
    skills: [],
    servers: [{ key: "github", url: "https://api.githubcopilot.com/mcp/" }],
    readme: "源码与钥匙串隔离",
  };
  return {
    listPluginCatalogOptions: () => ({
      queryKey: ["catalog"],
      queryFn: async () => ({ entries: [{ key: "github", package: pkg }] }),
    }),
    listPluginsOptions: () => ({
      queryKey: ["installed"],
      queryFn: async () => ({
        resources: [
          {
            plugin: { id: "p", sourceKey: "catalog:github" },
            resolvedVersionId: "v1",
            versions: [{ id: "v1", package: pkg }],
            installations: [],
          },
        ],
      }),
    }),
  };
});
vi.mock("../components/plugin-settings-dialog", () => ({
  PluginSettingsDialog: ({ detail }: { detail: PluginDetail }) => (
    <div role="dialog">{detail.plugin?.id}</div>
  ),
}));

it("catalog details use the shared summary and explanation and edit the actual installed resource", async () => {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <MemoryRouter initialEntries={["/resources/plugins/catalog/github"]}>
        <Routes>
          <Route
            path="/resources/plugins/catalog/:catalogKey"
            element={<PluginCatalogDetailPage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  await screen.findByRole("heading", { name: "GitHub" });
  const summary = screen.getByText(
    "检索仓库代码、跟进 Issue，协助整理与评审 Pull Request。",
  );
  expect(summary).toHaveClass("truncate");
  expect(summary).toHaveAttribute("title", summary.textContent);
  expect(screen.queryByRole("radiogroup")).not.toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "说明" })).toBeVisible();
  expect(screen.queryByText("源码与钥匙串隔离")).not.toBeInTheDocument();
  fireEvent.click(await screen.findByRole("button", { name: "编辑插件" }));
  expect(screen.getByRole("dialog")).toHaveTextContent("p");
});
