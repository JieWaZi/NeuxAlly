import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode } from "react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type * as SDK from "@/shared/api/generated/sdk.gen";
import "@/shared/i18n";
import { ConnectionDialog } from "./connection-dialog";

const api = vi.hoisted(() => ({
  connections: vi.fn(async () => ({
    data: { resources: [{ id: "new", revision: 1, enabled: false }] },
  })),
  create: vi.fn<
    (options: unknown) => Promise<{ data: { id: string; revision: number } }>
  >(async () => ({ data: { id: "new", revision: 1 } })),
  start: vi.fn<
    (
      options: unknown,
    ) => Promise<{ data: { id: string; status: string; qrContent: string } }>
  >(async () => ({
    data: {
      id: "login",
      status: "wait",
      qrContent: "https://example.invalid/qr",
    },
  })),
  bindings: vi.fn<(options: unknown) => Promise<{ data: object }>>(
    async () => ({ data: { resources: [], totalSize: 0 } }),
  ),
  bindingUpdate: vi.fn<(options: unknown) => Promise<{ data: object }>>(
    async () => ({ data: {} }),
  ),
  progress: vi.fn(async () => ({
    data: {
      id: "login",
      status: "wait",
      qrContent: "https://example.invalid/qr",
    },
  })),
  cancel: vi.fn<(options: unknown) => Promise<{ data: object }>>(async () => ({
    data: {},
  })),
  update: vi.fn<
    (options: unknown) => Promise<{ data: { id: string; revision: number } }>
  >(async () => ({ data: { id: "new", revision: 2 } })),
}));
vi.mock("@/shared/api/generated/sdk.gen", async (original) => ({
  ...(await original<typeof SDK>()),
  createImConnection: api.create,
  startImLogin: api.start,
  cancelImLogin: api.cancel,
  updateImConnection: api.update,
  getImLogin: api.progress,
  listWakers: async () => ({ data: { resources: [], totalSize: 0 } }),
  listChannels: async () => ({
    data: {
      resources: [{ channel: { id: "team", name: "研发团队" } }],
      totalSize: 1,
    },
  }),
  listImBindings: api.bindings,
  updateImBinding: api.bindingUpdate,
  listImConnections: () => api.connections(),
}));
const clients: QueryClient[] = [];
beforeEach(() => {
  Object.values(api).forEach((mock) => mock.mockClear());
  api.connections.mockResolvedValue({
    data: { resources: [{ id: "new", revision: 1, enabled: false }] },
  });
  api.bindings.mockResolvedValue({ data: { resources: [], totalSize: 0 } });
  api.progress.mockResolvedValue({
    data: {
      id: "login",
      status: "wait",
      qrContent: "https://example.invalid/qr",
    },
  });
});
afterEach(() => {
  cleanup();
  clients.splice(0).forEach((client) => client.clear());
});
function show(platform: "weixin" | "feishu", existing = false) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  render(
    <StrictMode>
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <ConnectionDialog
            platform={platform}
            connection={
              existing
                ? {
                    id: "new",
                    platform,
                    appId: "same-account",
                    enabled: true,
                    state: "connected",
                    name: "现有连接",
                    revision: 1,
                  }
                : undefined
            }
            onClose={vi.fn()}
            onSaved={vi.fn()}
          />
        </MemoryRouter>
      </QueryClientProvider>
    </StrictMode>,
  );
}
it("选定微信后自动生成二维码，刷新复用同一连接且不重复创建", async () => {
  const user = userEvent.setup();
  show("weixin");
  expect(await screen.findByTitle("扫码连接")).toBeInTheDocument();
  expect(screen.getByLabelText(/名称/)).toHaveValue("微信");
  expect(screen.queryByRole("switch")).not.toBeInTheDocument();
  expect(
    screen.queryByRole("heading", { name: "聊天接入" }),
  ).not.toBeInTheDocument();
  expect(api.create).toHaveBeenCalledTimes(1);
  expect(api.start).toHaveBeenCalledTimes(1);
  expect(
    screen.queryByRole("button", { name: "扫码连接" }),
  ).not.toBeInTheDocument();
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "刷新二维码" })).toBeEnabled(),
  );
  await user.click(screen.getByRole("button", { name: "刷新二维码" }));
  await waitFor(() => expect(api.start).toHaveBeenCalledTimes(2));
  expect(api.create).toHaveBeenCalledTimes(1);
  expect(api.start.mock.calls[1]?.[0]).toMatchObject({
    path: { connectionId: "new" },
  });
});
it("飞书可从扫码切换手动凭证，保存后自动启用", async () => {
  const user = userEvent.setup();
  show("feishu");
  await screen.findByTitle("扫码连接");
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "手工填写飞书应用凭据" }),
    ).toBeEnabled(),
  );
  await user.click(
    screen.getByRole("button", { name: "手工填写飞书应用凭据" }),
  );
  await user.type(screen.getByLabelText(/应用 ID/), "cli_example");
  await user.type(screen.getByLabelText(/应用密钥/), "test-secret");
  await user.click(screen.getByRole("button", { name: "保存" }));
  await waitFor(() => expect(api.update).toHaveBeenCalledTimes(1));
  expect(api.update.mock.calls[0]?.[0]).toMatchObject({
    body: { appId: "cli_example", appSecret: "test-secret", enabled: true },
  });
  expect(api.cancel).toHaveBeenCalledTimes(1);
});
it("管理已连接账号只读取设置，重新扫码由明确入口触发", async () => {
  const user = userEvent.setup();
  show("weixin", true);
  expect(
    await screen.findByRole("button", { name: "重新扫码" }),
  ).toBeInTheDocument();
  expect(screen.getByLabelText(/名称/)).toHaveValue("现有连接");
  expect(
    screen.queryByRole("switch", { name: /连接/ }),
  ).not.toBeInTheDocument();
  expect(api.start).not.toHaveBeenCalled();
  expect(api.create).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "重新扫码" }));
  await waitFor(() => expect(api.start).toHaveBeenCalledTimes(1));
  expect(api.create).not.toHaveBeenCalled();
});

it("已有飞书应用可返回扫码接入，仍使用原连接", async () => {
  const user = userEvent.setup();
  show("feishu", true);
  const back = await screen.findByRole("button", { name: "返回扫码接入" });
  expect(back).toBeEnabled();
  await user.click(back);
  expect(await screen.findByTitle("扫码连接")).toBeInTheDocument();
  expect(api.start).toHaveBeenCalledTimes(1);
  expect(api.start.mock.calls[0]?.[0]).toMatchObject({
    path: { connectionId: "new" },
  });
  expect(api.create).not.toHaveBeenCalled();
});

it("扫码成功保留二维码，并在下方展示绿色成功状态", async () => {
  api.progress.mockResolvedValue({
    data: {
      id: "login",
      status: "confirmed",
      qrContent: "https://example.invalid/qr",
    },
  });
  show("weixin");
  const success = await screen.findByText("连接已保存并启用");
  expect(success).toHaveClass("text-success");
  expect(screen.getByTitle("扫码连接")).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "刷新二维码" }),
  ).not.toBeInTheDocument();
});

for (const platform of ["weixin", "feishu"] as const) {
  it(`${platform} 既有连接可改名，保存实际输入并保留凭据`, async () => {
    const user = userEvent.setup();
    show(platform, true);
    const name = screen.getByLabelText(/名称/);
    await user.clear(name);
    await user.type(name, "  日常助理  ");
    await user.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(api.update).toHaveBeenCalledTimes(1));
    expect(api.update.mock.calls[0]?.[0]).toMatchObject({
      path: { connectionId: "new" },
      body: { name: "日常助理", appSecret: "", revision: 1 },
    });
  });
}
it("已有聊天配置的连接合并保存名称与聊天设置", async () => {
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "binding",
          connectionId: "new",
          name: "测试群",
          state: "approved",
          channelId: "team",
          revision: 4,
        },
      ],
      totalSize: 1,
    },
  });
  const user = userEvent.setup();
  show("weixin", true);
  await screen.findByText("测试群");
  const name = screen.getByLabelText(/名称/);
  await user.clear(name);
  await user.type(name, "团队机器人");
  await user.click(screen.getByRole("button", { name: "保存" }));
  await waitFor(() => expect(api.bindingUpdate).toHaveBeenCalledTimes(1));
  expect(api.update.mock.calls[0]?.[0]).toMatchObject({
    body: { name: "团队机器人" },
  });
  expect(api.bindingUpdate.mock.calls[0]?.[0]).toMatchObject({
    path: { bindingId: "binding" },
    body: { channelId: "team", revision: 4 },
  });
});

it("待授权聊天可单独修改连接名称，不要求配置伙伴或批准聊天", async () => {
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "binding",
          connectionId: "new",
          name: "待授权聊天",
          state: "pending",
          revision: 1,
        },
      ],
      totalSize: 1,
    },
  });
  const user = userEvent.setup();
  show("feishu", true);
  await screen.findByText("待授权聊天");
  const name = screen.getByLabelText(/名称/);
  await user.clear(name);
  await user.type(name, "独立连接名称");
  await user.click(screen.getByRole("button", { name: "保存" }));
  await waitFor(() => expect(api.update).toHaveBeenCalledTimes(1));
  expect(api.update.mock.calls[0]?.[0]).toMatchObject({
    body: { name: "独立连接名称" },
  });
  expect(api.bindingUpdate).not.toHaveBeenCalled();
});

it("新建机器人名称位于二维码上方，改名保存到同一连接且不提前启用", async () => {
  const user = userEvent.setup();
  show("weixin");
  const qr = await screen.findByTitle("扫码连接");
  const name = screen.getByLabelText(/名称/);
  expect(
    name.compareDocumentPosition(qr) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  expect(api.create.mock.calls[0]?.[0]).toMatchObject({
    body: { name: "微信", enabled: false },
  });
  await user.clear(name);
  await user.type(name, "研发微信机器人");
  await user.click(screen.getByRole("button", { name: "取消" }));
  await waitFor(() => expect(api.update).toHaveBeenCalledTimes(1));
  expect(api.update.mock.calls[0]?.[0]).toMatchObject({
    path: { connectionId: "new" },
    body: { name: "研发微信机器人", enabled: false },
  });
  expect(api.create).toHaveBeenCalledTimes(1);
});

it("扫码成功后完成新建，保存自定义名称并保留已确认的启用状态", async () => {
  api.progress.mockResolvedValue({
    data: {
      id: "login",
      status: "confirmed",
      qrContent: "https://example.invalid/qr",
    },
  });
  api.connections.mockResolvedValue({
    data: { resources: [{ id: "new", revision: 5, enabled: true }] },
  });
  const user = userEvent.setup();
  show("feishu");
  await screen.findByText("连接已保存并启用");
  const name = screen.getByLabelText(/名称/);
  expect(name).toHaveValue("飞书");
  await user.clear(name);
  await user.type(name, "客服飞书机器人");
  await user.click(screen.getByRole("button", { name: "完成" }));
  await waitFor(() => expect(api.update).toHaveBeenCalledTimes(1));
  expect(api.update.mock.calls[0]?.[0]).toMatchObject({
    path: { connectionId: "new" },
    body: { name: "客服飞书机器人", revision: 5, enabled: true },
  });
  expect(api.cancel).not.toHaveBeenCalled();
});

vi.mock("@/shared/api/global-events", () => ({
  registerCoreConnectionRecovery: () => () => {},
  subscribeGlobalEvents: () => () => {},
}));
