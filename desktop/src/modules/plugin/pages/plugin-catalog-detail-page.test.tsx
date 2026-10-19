import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";
import { i18n } from "@/shared/i18n";
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
    presentation: {
      displayName: "GitHub",
      displayNames: { en: "GitHub", "zh-CN": "GitHub" },
      localizedDescriptions: { en: "Manage repositories", "zh-CN": "旧简介" },
    },
    skills: [],
    servers: [{ key: "github", url: "https://api.githubcopilot.com/mcp/" }],
    readme: "源码与钥匙串隔离",
  };
  return {
    getCatalogPluginOptions: (options: { query?: { locale?: string } }) => ({
      queryKey: ["catalog", options.query?.locale],
      queryFn: async () => ({
        key: "github",
        package: {
          ...pkg,
          readme:
            options.query?.locale === "en" ? "Business guide" : "业务说明",
        },
      }),
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

afterEach(async () => {
  cleanup();
  await i18n.changeLanguage("zh-CN");
});

it("catalog details use the selected source summary and README and edit the actual installed resource", async () => {
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
  const summary = screen.getByText("旧简介");
  expect(summary).toHaveClass("truncate");
  expect(summary).toHaveAttribute("title", summary.textContent);
  expect(screen.queryByRole("radiogroup")).not.toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "说明" })).toBeVisible();
  expect(screen.getByText("业务说明")).toBeVisible();
  fireEvent.click(await screen.findByRole("button", { name: "编辑插件" }));
  expect(screen.getByRole("dialog")).toHaveTextContent("p");
});

it("switches catalog README and summary with the UI language without reusing the other locale query", async () => {
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
  expect(await screen.findByText("业务说明")).toBeVisible();
  await act(() => i18n.changeLanguage("en-US"));
  expect(await screen.findByText("Business guide")).toBeVisible();
  expect(screen.getByText("Manage repositories")).toBeVisible();
  expect(screen.queryByText("业务说明")).not.toBeInTheDocument();
});
