import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, it, expect, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";
import "@/shared/i18n";
import { PluginInstallDialog } from "./plugin-install-dialog";
const fixture = vi.hoisted(() => ({
  detail: {
    plugin: { id: "plugin-1", name: "办公" },
    versions: [
      {
        id: "version-1",
        version: "1.0.0",
        package: { manifest: { name: "office" }, skills: [] },
      },
    ],
    installations: [],
    configurationRevision: 3,
    ready: true,
  },
  install: vi
    .fn<
      (
        input: unknown,
      ) => Promise<{ results: { wakerId: string; installed: boolean }[] }>
    >()
    .mockResolvedValue({ results: [{ wakerId: "p1", installed: true }] }),
}));
vi.mock("@/shared/api/use-query-events", () => ({
  useQueryEvents: () => undefined,
}));
vi.mock("../api/plugin-queries", () => ({
  getPluginOptions: () => ({
    queryKey: [{ _id: "getPlugin", path: { pluginId: "plugin-1" } }],
    queryFn: async () => fixture.detail,
  }),
  listWakersOptions: () => ({
    queryKey: [{ _id: "listWakers" }],
    queryFn: async () => ({
      resources: [
        { id: "p1", name: "伙伴一" },
        { id: "p2", name: "伙伴二" },
      ],
      totalSize: 2,
    }),
  }),
  getWakerOptions: () => ({
    queryKey: [{ _id: "getWaker" }],
    queryFn: async () => ({ id: "p1", name: "伙伴一" }),
  }),
  installPluginMutation: () => ({ mutationFn: fixture.install }),
  prepareCatalogPluginMutation: () => ({ mutationFn: vi.fn() }),
  updatePluginConfigurationMutation: () => ({ mutationFn: vi.fn() }),
  invalidatePluginQueries: async () => undefined,
}));
vi.mock("./plugin-auth-panel", () => ({ PluginAuthPanel: () => null }));
afterEach(cleanup);
beforeEach(() => {
  fixture.install.mockReset();
  fixture.install.mockResolvedValue({
    results: [{ wakerId: "p1", installed: true }],
  });
  fixture.detail.ready = true;
});
function show(onClose = vi.fn()) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <PluginInstallDialog detail={fixture.detail} onClose={onClose} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return onClose;
}
describe("plugin installation", () => {
  it("retries only failed partners after a partial installation", async () => {
    fixture.install
      .mockResolvedValueOnce({
        results: [
          { wakerId: "p1", installed: true },
          { wakerId: "p2", installed: false },
        ],
      })
      .mockResolvedValueOnce({ results: [{ wakerId: "p2", installed: true }] });
    show();
    fireEvent.click(await screen.findByRole("checkbox", { name: "伙伴一" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "伙伴二" }));
    fireEvent.click(screen.getByRole("button", { name: "安装" }));
    fireEvent.click(await screen.findByRole("button", { name: "重试" }));
    await waitFor(() => expect(fixture.install).toHaveBeenCalledTimes(2));
    expect(fixture.install.mock.calls[1]?.[0]).toMatchObject({
      body: { wakerIds: ["p2"] },
    });
  });
  it("selects partners only after readiness and submits real version/revision", async () => {
    show();
    const checkbox = await screen.findByRole("checkbox", { name: "伙伴一" });
    fireEvent.click(checkbox);
    fireEvent.click(screen.getByRole("button", { name: "安装" }));
    await waitFor(() => expect(fixture.install).toHaveBeenCalledTimes(1));
    expect(fixture.install.mock.calls[0]?.[0]).toMatchObject({
      path: { pluginId: "plugin-1" },
      body: {
        versionId: "version-1",
        expectedConfigurationRevision: 3,
        wakerIds: ["p1"],
      },
    });
    expect(await screen.findByText(/伙伴一 · 安装成功/)).toBeVisible();
  });
  it("does not create an installation when the user closes the prepared resource", async () => {
    const close = show();
    fireEvent.click(
      await screen.findByRole("button", { name: "仅保存到本机" }),
    );
    expect(close).toHaveBeenCalledTimes(1);
    expect(fixture.install).not.toHaveBeenCalled();
  });
  it("hides partner selection and installation action until shared setup completes", async () => {
    fixture.detail.ready = false;
    show();
    expect(await screen.findByText("需要完成设置")).toBeVisible();
    expect(
      screen.queryByRole("checkbox", { name: "伙伴一" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "安装" }),
    ).not.toBeInTheDocument();
    expect(fixture.install).not.toHaveBeenCalled();
  });
});
