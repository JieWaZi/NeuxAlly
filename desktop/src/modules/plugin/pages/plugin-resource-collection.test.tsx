import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { it, expect, vi, afterEach, beforeEach } from "vitest";
import "@/shared/i18n";
import type * as ResourceSdk from "@/shared/api/generated/sdk.gen";
import type { PluginDetail } from "../model/plugin-view";
import { PluginResourceCollection } from "./plugin-resource-collection";

const fixture = vi.hoisted(() => ({
  available: true,
  sourceKey: "catalog:office",
  entrySourceKey: undefined as string | undefined,
  sourceState: "available",
}));
vi.mock("@/shared/api/use-query-events", () => ({
  useQueryEvents: () => undefined,
}));
vi.mock("../api/use-plugin-resources", () => ({
  usePluginResources: () => ({
    data: [
      {
        plugin: { id: "existing", sourceKey: fixture.sourceKey },
        account: { available: fixture.available },
        installations: [{ wakerId: "w1", versionId: "v1" }],
      },
    ],
    isPending: false,
    isError: false,
  }),
}));
vi.mock("@/shared/api/generated/sdk.gen", async (original) => ({
  ...(await original<typeof ResourceSdk>()),
  listPluginCatalog: vi.fn(async () => ({
    data: {
      source: { state: fixture.sourceState },
      entries: [
        {
          key: "office",
          sourceKey: fixture.entrySourceKey,
          package: {
            manifest: {
              name: "office",
              extensions: { "com.ally": { provider: "feishu" } },
            },
            presentation: { displayName: "飞书办公" },
          },
        },
      ],
      totalSize: 1,
    },
  })),
}));
vi.mock("../components/plugin-install-dialog", () => ({
  PluginInstallDialog: ({
    detail,
    authorizeOnly,
  }: {
    detail?: PluginDetail;
    authorizeOnly?: boolean;
  }) => (
    <div role="dialog">
      {detail?.plugin?.id}:{detail?.installations?.[0]?.wakerId}:
      {authorizeOnly ? "reauthorize" : "install"}
    </div>
  ),
}));
afterEach(cleanup);
beforeEach(() => {
  fixture.available = true;
  fixture.sourceKey = "catalog:office";
  fixture.entrySourceKey = undefined;
  fixture.sourceState = "available";
});

function showCatalog() {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <MemoryRouter>
        <PluginResourceCollection
          search=""
          view="catalog"
          onViewChange={() => {}}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

it("does not merge the same catalog key from another source into an installed resource", async () => {
  fixture.entrySourceKey = "catalog:another-source:office";
  showCatalog();
  fireEvent.click(await screen.findByRole("button", { name: "安装 飞书办公" }));
  expect(screen.getByRole("dialog")).toHaveTextContent("::install");
});

it("keeps cached catalog cards visible and explains a stale source", async () => {
  fixture.sourceState = "stale";
  showCatalog();
  expect(
    await screen.findByText(
      "插件库暂未连接，当前显示缓存内容，可能不是最新版本。",
    ),
  ).toBeVisible();
  expect(screen.getByRole("button", { name: "安装 飞书办公" })).toBeVisible();
});
it.each([
  [true, "安装 飞书办公", "install"],
  [false, "授权 飞书办公", "reauthorize"],
] as const)(
  "uses the existing API snapshot when opening a catalog action (%s)",
  async (available, label, mode) => {
    fixture.available = available;
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <PluginResourceCollection
            search=""
            view="catalog"
            onViewChange={() => {}}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    fireEvent.click(await screen.findByRole("button", { name: label }));
    expect(screen.getByRole("dialog")).toHaveTextContent(`existing:w1:${mode}`);
  },
);
