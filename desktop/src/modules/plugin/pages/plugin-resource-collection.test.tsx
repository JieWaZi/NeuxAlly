import {
  act,
  render,
  screen,
  fireEvent,
  cleanup,
  waitFor,
} from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { it, expect, vi, afterEach, beforeEach } from "vitest";
import "@/shared/i18n";
import type { GlobalEventObserver } from "@/shared/api/global-events";
import { listPluginCatalog } from "@/shared/api/generated/sdk.gen";
import type * as ResourceSdk from "@/shared/api/generated/sdk.gen";
import type { PluginDetail } from "../model/plugin-view";
import { PluginResourceCollection } from "./plugin-resource-collection";

const fixture = vi.hoisted(() => ({
  available: true,
  sourceKey: "catalog:office",
  entrySourceKey: undefined as string | undefined,
  sourceState: "available",
  displayName: "飞书办公",
  listeners: new Set<GlobalEventObserver>(),
}));
vi.mock("@/shared/api/global-events", () => ({
  subscribeGlobalEvents: (
    _topic: string,
    _resource: unknown,
    observer: GlobalEventObserver,
  ) => {
    fixture.listeners.add(observer);
    return () => fixture.listeners.delete(observer);
  },
}));
vi.mock("../api/use-plugin-resources", () => ({
  usePluginResources: () => ({
    data: [
      {
        plugin: { id: "existing", sourceKey: fixture.sourceKey },
        account: { available: fixture.available, checkedAt: 123 },
        installations: [{ wakerId: "w1", versionId: "v1" }],
      },
    ],
    isPending: false,
    isError: false,
  }),
}));
vi.mock("@/shared/api/generated/sdk.gen", async (original) => ({
  ...(await original<typeof ResourceSdk>()),
  listPlugins: vi.fn(
    async (options?: { query?: { installedOnly?: boolean } }) => {
      const installed = {
        plugin: { id: "installed", name: "实际安装插件" },
        installations: [{ wakerId: "w1", versionId: "v1", enabled: false }],
      };
      const resources = options?.query?.installedOnly
        ? [installed]
        : [
            installed,
            {
              plugin: { id: "prepared", name: "尚未安装插件" },
              installations: [],
            },
          ];
      return { data: { resources, totalSize: resources.length } };
    },
  ),
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
            presentation: { displayName: fixture.displayName },
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
afterEach(() => {
  cleanup();
  fixture.listeners.clear();
});
beforeEach(() => {
  vi.clearAllMocks();
  fixture.displayName = "飞书办公";
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

it("refreshes the active catalog when a background catalog update emits plugins.changed", async () => {
  showCatalog();
  expect(
    await screen.findByRole("button", { name: "安装 飞书办公" }),
  ).toBeVisible();
  expect(listPluginCatalog).toHaveBeenCalledTimes(1);
  fixture.displayName = "更新后的插件";
  act(() => {
    for (const listener of fixture.listeners)
      listener.onEvent({
        protocolVersion: 1,
        epoch: "epoch",
        connectionId: "connection",
        deliverySequence: 1,
        topic: "plugins",
        type: "plugins.changed",
        resource: { kind: "global" },
        eventId: "catalog-update",
        transient: false,
        payload: {},
      });
  });
  await waitFor(() => expect(listPluginCatalog).toHaveBeenCalledTimes(2));
  expect(
    await screen.findByRole("button", { name: "安装 更新后的插件" }),
  ).toBeVisible();
});

it("shows installed resources and the installed total in my plugins", async () => {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <MemoryRouter>
        <PluginResourceCollection
          search=""
          view="mine"
          onViewChange={() => {}}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  expect(await screen.findByText("实际安装插件")).toBeVisible();
  expect(screen.queryByText("尚未安装插件")).not.toBeInTheDocument();
  expect(screen.getByText("1 项资源")).toBeVisible();
});
