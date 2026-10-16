import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  expect,
  it,
  vi,
} from "vitest";
import type * as SDK from "@/shared/api/generated/sdk.gen";
import "@/shared/i18n";
import { IMPage } from "./im-page";
import { IMConnectionDetailPage } from "./im-connection-detail-page";

const api = vi.hoisted(() => ({
  connections: vi.fn<(options?: unknown) => Promise<{ data: object }>>(),
  remove: vi.fn<(options: unknown) => Promise<{ data: object }>>(async () => ({
    data: {},
  })),
  update: vi.fn<(options: unknown) => Promise<{ data: object }>>(async () => ({
    data: {},
  })),
  bindingUpdate: vi.fn<(options: unknown) => Promise<{ data: object }>>(
    async () => ({ data: {} }),
  ),
  conversations: vi.fn<(options: unknown) => Promise<{ data: object }>>(
    async () => ({ data: { resources: [] } }),
  ),
  selectConversation: vi.fn<(options: unknown) => Promise<{ data: object }>>(
    async () => ({ data: {} }),
  ),
  bindings: vi.fn<(options: unknown) => Promise<{ data: object }>>(
    async () => ({ data: { resources: [], totalSize: 0 } }),
  ),
  project: vi.fn<(options: unknown) => Promise<{ data: object }>>(async () => ({
    data: { id: "project", name: "资讯整理" },
  })),
}));
vi.mock("@/shared/api/generated/sdk.gen", async (original) => ({
  ...(await original<typeof SDK>()),
  listImConnections: api.connections,
  listImBindings: api.bindings,
  getProject: api.project,
  listImConversations: api.conversations,
  selectImConversation: api.selectConversation,
  updateImConnection: api.update,
  updateImBinding: api.bindingUpdate,
  deleteImConnection: api.remove,
  getImConnectionDependencies: async () => ({
    data: { workflows: [], pendingDeliveries: 0, digest: "empty" },
  }),
  createImConnection: async () => ({ data: { id: "new", revision: 1 } }),
  startImLogin: async () => ({
    data: { id: "login", status: "wait", qrContent: "https://example.invalid" },
  }),
  getImLogin: async () => ({
    data: { id: "login", status: "wait", qrContent: "https://example.invalid" },
  }),
  cancelImLogin: async () => ({ data: {} }),
  listWakers: async () => ({
    data: { resources: [{ id: "waker", name: "日常助理" }], totalSize: 1 },
  }),
  listChannels: async () => ({
    data: {
      resources: [
        { channel: { id: "team", name: "研发团队", leadWakerId: "waker" } },
      ],
      totalSize: 1,
    },
  }),
}));
beforeAll(() => {
  Element.prototype.hasPointerCapture = () => false;
});
afterAll(() => {
  Reflect.deleteProperty(Element.prototype, "hasPointerCapture");
});
const clients: QueryClient[] = [];
beforeEach(() => {
  api.connections.mockReset().mockResolvedValue({
    data: {
      resources: [
        {
          id: "connection",
          appId: "bot-account",
          name: "我的微信",
          platform: "weixin",
          enabled: true,
          keepAwake: true,
          state: "connected",
          revision: 3,
          conversationCount: 2,
        },
      ],
    },
  });
  api.remove.mockReset().mockResolvedValue({ data: {} });
  api.update.mockClear();
  api.bindingUpdate.mockReset().mockResolvedValue({ data: {} });
  api.remove.mockClear();
  api.bindings
    .mockReset()
    .mockResolvedValue({ data: { resources: [], totalSize: 0 } });
  api.conversations.mockReset().mockResolvedValue({ data: { resources: [] } });
  api.selectConversation.mockReset().mockResolvedValue({ data: {} });
  api.project
    .mockReset()
    .mockResolvedValue({ data: { id: "project", name: "资讯整理" } });
});
afterEach(() => {
  cleanup();
  clients.splice(0).forEach((client) => client.clear());
});
function show(path = "/im") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/im" element={<IMPage />} />
          <Route
            path="/im/:connectionId"
            element={<IMConnectionDetailPage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}
it("连接卡片进入真实详情，记录在页面表格内展示且不提供投递记录", async () => {
  const user = userEvent.setup();
  show();
  const card = await screen.findByRole("link", {
    name: "查看我的微信连接详情",
  });
  expect(card.closest("article")).toHaveAttribute("data-slot", "card");
  expect(screen.queryByText("投递记录")).not.toBeInTheDocument();
  expect(screen.queryByText("聊天授权")).not.toBeInTheDocument();
  expect(api.bindings.mock.calls[0]?.[0]).toMatchObject({
    query: { connectionId: "", offset: 0 },
  });
  await user.click(card);
  expect(
    await screen.findByRole("heading", { name: "连接信息" }),
  ).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "聊天接入" })).toBeInTheDocument();
  expect(screen.getByText("bot-account")).toBeInTheDocument();
  await waitFor(() => expect(api.bindings).toHaveBeenCalled());
  expect(
    api.bindings.mock.calls.some(([options]) =>
      JSON.stringify(options).includes("connection"),
    ),
  ).toBe(true);
  expect(screen.getByRole("table", { name: "对话记录" })).toBeInTheDocument();
  expect(
    screen.queryByRole("link", { name: "返回连接列表" }),
  ).not.toBeInTheDocument();
  expect(screen.queryByText("连接开启时维持应用运行")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "编辑连接" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "删除连接" })).toBeEnabled();
  await user.hover(screen.getByRole("heading", { name: /我的微信/, level: 1 }));
  await user.click(screen.getByRole("button", { name: "返回列表" }));
  expect(
    await screen.findByRole("link", { name: "查看我的微信连接详情" }),
  ).toBeInTheDocument();
});
it("卡片更多菜单提供编辑与删除，确认删除使用当前版本且不导航", async () => {
  const user = userEvent.setup();
  show();
  await user.click(
    await screen.findByRole("button", { name: "我的微信连接操作" }),
  );
  expect(screen.getByRole("menuitem", { name: "编辑" })).toBeInTheDocument();
  await user.click(screen.getByRole("menuitem", { name: "删除" }));
  expect(api.remove).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "删除" }));
  await waitFor(() => expect(api.remove).toHaveBeenCalledTimes(1));
  expect(api.remove.mock.calls[0]?.[0]).toMatchObject({
    path: { connectionId: "connection" },
    query: { revision: 3 },
  });
  expect(
    screen.getByRole("link", { name: "查看我的微信连接详情" }),
  ).toBeInTheDocument();
});

it("先展示支持平台，添加机器人替换为单独扫码弹窗", async () => {
  const user = userEvent.setup();
  show();
  await user.click(screen.getByRole("button", { name: "添加连接" }));
  expect(screen.getByRole("dialog", { name: "添加连接" })).toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "添加微信机器人" }),
  ).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "添加飞书机器人" }));
  expect(await screen.findByTitle("扫码连接")).toBeInTheDocument();
  expect(
    screen.queryByRole("dialog", { name: "添加连接" }),
  ).not.toBeInTheDocument();
  expect(screen.getAllByRole("dialog")).toHaveLength(1);
});

it("待授权标识进入所属连接，授权入口不自动批准聊天", async () => {
  const user = userEvent.setup();
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "pending",
          connectionId: "connection",
          name: "待接入私聊",
          kind: "direct",
          state: "pending",
        },
      ],
      totalSize: 1,
    },
  });
  show();
  await user.click(await screen.findByRole("link", { name: "待授权" }));
  expect(await screen.findByRole("button", { name: /^授权$/ })).toBeEnabled();
  expect(screen.getByText("待接入私聊")).toBeInTheDocument();
  expect(api.update).not.toHaveBeenCalled();
});
it("详情表格展示实际对话和当前指针，切换操作保留聊天边界", async () => {
  const user = userEvent.setup();
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "binding",
          connectionId: "connection",
          name: "我的聊天",
          kind: "direct",
          state: "approved",
          activeLinkId: "old",
        },
      ],
      totalSize: 1,
    },
  });
  api.conversations.mockResolvedValue({
    data: {
      resources: [
        {
          id: "old",
          chatId: "chat",
          title: "实际对话",
          turnCount: 3,
          createdAt: 1791450000000,
        },
        {
          id: "another",
          chatId: "another-chat",
          title: "历史对话",
          turnCount: 1,
        },
      ],
    },
  });
  show("/im/connection");
  expect(await screen.findByText("实际对话")).toBeInTheDocument();
  expect(
    screen.getByRole("columnheader", { name: "轮数" }),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("columnheader", { name: "聊天接入" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("cell", { name: "3" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "撤销授权" })).toBeEnabled();
  expect(
    screen.queryByRole("button", { name: "编辑操作" }),
  ).not.toBeInTheDocument();
  const currentRow = screen.getByRole("row", { name: /实际对话/ });
  expect(within(currentRow).getByRole("status")).toHaveTextContent("当前对话");
  expect(within(currentRow).getByRole("status")).toHaveClass("text-success");
  expect(
    within(currentRow).queryByRole("button", { name: "当前对话" }),
  ).not.toBeInTheDocument();
  expect(
    within(currentRow).getByRole("link", { name: "查看执行过程" }),
  ).toHaveAttribute("href", "/chats/chat");
  expect(api.conversations.mock.calls[0]?.[0]).toMatchObject({
    path: { bindingId: "binding" },
  });
  await user.click(screen.getByRole("button", { name: "切换到此对话" }));
  await waitFor(() =>
    expect(api.selectConversation).toHaveBeenCalledWith(
      expect.objectContaining({
        path: { bindingId: "binding" },
        body: { linkId: "another" },
      }),
    ),
  );
});

it("卡片展示静态伙伴与智能体模型，移除聊天授权计数，列表与详情不重复平台文字", async () => {
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "b",
          connectionId: "connection",
          state: "approved",
          wakerId: "waker",
          workingDirectory: "/Users/example/workspace",
          options: {
            runtimeId: "codex",
            model: "gpt-6-luna",
            permissionLevel: "full-access",
          },
        },
        { id: "p", connectionId: "connection", state: "pending" },
      ],
      totalSize: 2,
    },
  });
  const user = userEvent.setup();
  show();
  expect(await screen.findByText("日常助理")).toBeInTheDocument();
  expect(screen.getByText("Codex", { selector: "span" })).toBeInTheDocument();
  expect(screen.getByText("gpt-6-luna")).toBeInTheDocument();
  expect(screen.getByText("/Users/example/workspace")).toBeInTheDocument();
  expect(screen.getByText("完全访问")).toBeInTheDocument();
  expect(api.project).not.toHaveBeenCalled();
  expect(
    document.querySelector('[data-companion-animated="false"]'),
  ).toBeInTheDocument();
  expect(screen.queryByText("伙伴：日常助理")).not.toBeInTheDocument();
  expect(screen.queryByText(/个聊天|个已授权/)).not.toBeInTheDocument();
  expect(screen.queryByText("微信")).not.toBeInTheDocument();
  await user.click(screen.getByRole("link", { name: "查看我的微信连接详情" }));
  await screen.findByRole("heading", { name: "连接信息" });
  expect(screen.queryByText("微信")).not.toBeInTheDocument();
});

it("卡片读取所属伙伴的项目名称，空工作范围显示默认", async () => {
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "binding",
          connectionId: "connection",
          state: "approved",
          wakerId: "waker",
          projectId: "project",
          options: { runtimeId: "codex", model: "gpt-6-luna" },
        },
      ],
      totalSize: 1,
    },
  });
  show();
  expect(await screen.findByText("资讯整理")).toBeInTheDocument();
  expect(api.project.mock.calls[0]?.[0]).toMatchObject({
    path: { wakerId: "waker", projectId: "project" },
  });
  expect(screen.getByText("默认权限")).toBeInTheDocument();
});

it("团队连接按负责人读取项目，权限显示按成员配置", async () => {
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "binding",
          connectionId: "connection",
          state: "approved",
          channelId: "team",
          projectId: "project",
          options: { permissionLevel: "full-access" },
        },
      ],
      totalSize: 1,
    },
  });
  show();
  expect(await screen.findByText("资讯整理")).toBeInTheDocument();
  expect(api.project.mock.calls[0]?.[0]).toMatchObject({
    path: { wakerId: "waker", projectId: "project" },
  });
  expect(screen.getByText("按成员配置")).toBeInTheDocument();
  expect(screen.queryByText("完全访问")).not.toBeInTheDocument();
});

it("同平台机器人按名称和账号搜索，选择只作用于当前结果且不跳详情", async () => {
  api.connections.mockResolvedValue({
    data: {
      resources: [
        {
          id: "first",
          name: "研发机器人",
          appId: "cli_first",
          platform: "feishu",
          revision: 2,
        },
        {
          id: "second",
          name: "客服机器人",
          appId: "cli_second",
          platform: "feishu",
          revision: 4,
        },
      ],
    },
  });
  const user = userEvent.setup();
  show();
  await screen.findByRole("link", { name: "查看研发机器人连接详情" });
  await user.click(
    screen.getByRole("checkbox", { name: "选择研发机器人连接" }),
  );
  expect(screen.getByRole("button", { name: "删除所选" })).toBeEnabled();
  expect(
    screen.queryByRole("heading", { name: "连接信息" }),
  ).not.toBeInTheDocument();
  await user.type(
    screen.getByRole("searchbox", { name: "搜索机器人名称或账号" }),
    "second",
  );
  expect(
    screen.queryByRole("link", { name: "查看研发机器人连接详情" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "删除所选" }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole("checkbox", { name: "选择" }));
  await user.click(screen.getByRole("button", { name: "删除所选" }));
  const dialog = screen.getByRole("dialog", { name: "删除所选连接" });
  expect(within(dialog).getByText("客服机器人")).toBeInTheDocument();
  expect(within(dialog).queryByText("研发机器人")).not.toBeInTheDocument();
  expect(api.remove).not.toHaveBeenCalled();
  await user.click(within(dialog).getByRole("button", { name: "取消" }));
  await user.clear(
    screen.getByRole("searchbox", { name: "搜索机器人名称或账号" }),
  );
  await user.type(
    screen.getByRole("searchbox", { name: "搜索机器人名称或账号" }),
    "研发",
  );
  expect(
    screen.getByRole("link", { name: "查看研发机器人连接详情" }),
  ).toBeInTheDocument();
});
it("批量删除逐个携带版本，部分失败仅保留失败项供重试", async () => {
  api.connections.mockResolvedValue({
    data: {
      resources: [
        { id: "first", name: "研发机器人", platform: "feishu", revision: 2 },
        { id: "second", name: "客服机器人", platform: "feishu", revision: 4 },
      ],
    },
  });
  api.remove
    .mockResolvedValueOnce({ data: {} })
    .mockRejectedValueOnce(new Error("busy"));
  const user = userEvent.setup();
  show();
  await screen.findByRole("link", { name: "查看研发机器人连接详情" });
  await user.click(screen.getByRole("checkbox", { name: "选择" }));
  await user.click(screen.getByRole("button", { name: "删除所选" }));
  const dialog = screen.getByRole("dialog", { name: "删除所选连接" });
  expect(api.remove).not.toHaveBeenCalled();
  await user.click(within(dialog).getByRole("button", { name: "确认删除" }));
  await within(dialog).findByRole("alert");
  expect(api.remove.mock.calls.map(([options]) => options)).toMatchObject([
    { path: { connectionId: "first" }, query: { revision: 2 } },
    { path: { connectionId: "second" }, query: { revision: 4 } },
  ]);
  expect(within(dialog).queryByText("研发机器人")).not.toBeInTheDocument();
  expect(within(dialog).getByText("客服机器人")).toBeInTheDocument();
  await user.click(within(dialog).getByRole("button", { name: "重试失败项" }));
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(api.remove).toHaveBeenCalledTimes(3);
  expect(api.remove.mock.calls[2]?.[0]).toMatchObject({
    path: { connectionId: "second" },
  });
});

it("响应配置区分工作范围和权限，多种配置以首项和计数展示", async () => {
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "one",
          connectionId: "connection",
          state: "approved",
          wakerId: "waker",
          options: {
            runtimeId: "codex",
            model: "model-one",
            permissionLevel: "default",
          },
        },
        {
          id: "two",
          connectionId: "connection",
          state: "approved",
          wakerId: "waker",
          options: {
            runtimeId: "codex",
            model: "model-one",
            permissionLevel: "full-access",
          },
        },
        {
          id: "three",
          connectionId: "connection",
          state: "approved",
          wakerId: "waker",
          options: { runtimeId: "codex", model: "model-two" },
        },
      ],
      totalSize: 3,
    },
  });
  show();
  expect(await screen.findByText("model-one")).toBeInTheDocument();
  expect(screen.getAllByText("model-one")).toHaveLength(1);
  expect(screen.queryByText("model-two")).not.toBeInTheDocument();
  expect(screen.getByText("3 个响应配置")).toBeInTheDocument();
  expect(screen.getByText("默认")).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: "查看我的微信连接详情" }),
  ).toHaveAttribute("title", expect.stringContaining("model-two"));
});

it("卡片展示实际对话数量，勾选不增加外轮廓", async () => {
  const user = userEvent.setup();
  show();
  expect(await screen.findByText("2 条对话记录")).toBeInTheDocument();
  const card = screen
    .getByRole("link", { name: "查看我的微信连接详情" })
    .closest("article");
  await user.click(screen.getByRole("checkbox", { name: "选择我的微信连接" }));
  expect(card).not.toHaveAttribute("data-state", "selected");
  expect(card).toHaveClass("bg-accent-surface");
});

it("登录失效错误替换配置摘要，底栏仍保留实际对话数", async () => {
  api.connections.mockResolvedValue({
    data: {
      resources: [
        {
          id: "connection",
          name: "我的微信",
          platform: "weixin",
          enabled: true,
          state: "login_required",
          errorCode: "IMLoginRequired",
          conversationCount: 2,
          revision: 3,
        },
      ],
    },
  });
  show();
  expect(
    await screen.findByText("登录已失效，请重新扫码。"),
  ).toBeInTheDocument();
  expect(screen.getByText("2 条对话记录")).toBeInTheDocument();
});

it("首次授权只展示响应配置，不包含连接名称和平台登录", async () => {
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "new-binding",
          connectionId: "connection",
          name: "新聊天",
          kind: "direct",
          state: "pending",
          revision: 7,
        },
      ],
      totalSize: 1,
    },
  });
  const user = userEvent.setup();
  show("/im/connection");
  await user.click(await screen.findByRole("button", { name: "授权" }));
  const dialog = screen.getByRole("dialog", { name: "授权" });
  expect(
    within(dialog).getByRole("combobox", { name: "执行目标" }),
  ).toBeInTheDocument();
  expect(
    within(dialog).getByRole("button", { name: "工作范围" }),
  ).toBeInTheDocument();
  expect(
    within(dialog).getByRole("switch", {
      name: "允许工作流查询并发送到此聊天",
    }),
  ).toBeInTheDocument();
  expect(within(dialog).queryByLabelText(/名称/)).not.toBeInTheDocument();
  expect(within(dialog).queryByText("聊天接入")).not.toBeInTheDocument();
  expect(
    within(dialog).queryByRole("button", { name: "重新扫码" }),
  ).not.toBeInTheDocument();
  expect(within(dialog).getByRole("button", { name: "授权" })).toBeDisabled();
  expect(api.bindingUpdate).not.toHaveBeenCalled();
  await user.click(within(dialog).getByRole("radio", { name: "团队" }));
  within(dialog).getByRole("combobox", { name: "执行目标" }).focus();
  await user.keyboard("{ArrowDown}");
  await screen.findByRole("option", { name: "研发团队" });
  await user.keyboard("{Enter}");
  await user.click(within(dialog).getByRole("button", { name: "授权" }));
  await waitFor(() => expect(api.bindingUpdate).toHaveBeenCalledTimes(1));
  expect(api.bindingUpdate.mock.calls[0]?.[0]).toMatchObject({
    path: { bindingId: "new-binding" },
    body: {
      state: "approved",
      channelId: "team",
      projectId: "",
      workingDirectory: "",
      revision: 7,
    },
  });
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(api.update).not.toHaveBeenCalled();
});

it("已有伙伴配置直接授权，保留完整配置并刷新授权状态", async () => {
  const binding = {
    id: "configured",
    connectionId: "connection",
    name: "已有私聊",
    kind: "direct",
    state: "rejected",
    wakerId: "waker",
    channelId: "",
    projectId: "",
    workingDirectory: "/approved/work",
    options: {
      runtimeId: "codex",
      accountId: "saved-account",
      model: "saved-model",
      reasoning: "high",
      permissionLevel: "full-access",
    },
    controllerIds: ["owner"],
    allowWorkflowDelivery: false,
    activeLinkId: "old",
    revision: 9,
  };
  api.bindings.mockResolvedValue({
    data: { resources: [binding], totalSize: 1 },
  });
  api.bindingUpdate.mockImplementation(async () => {
    api.bindings.mockResolvedValue({
      data: {
        resources: [{ ...binding, state: "approved", revision: 10 }],
        totalSize: 1,
      },
    });
    return { data: {} };
  });
  const user = userEvent.setup();
  show("/im/connection");
  await user.click(await screen.findByRole("button", { name: "授权" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  await screen.findByRole("button", { name: "撤销授权" });
  expect(api.bindingUpdate).toHaveBeenCalledTimes(1);
  expect(api.bindingUpdate.mock.calls[0]?.[0]).toMatchObject({
    path: { bindingId: "configured" },
    body: {
      name: binding.name,
      state: "approved",
      wakerId: binding.wakerId,
      channelId: "",
      projectId: "",
      workingDirectory: binding.workingDirectory,
      options: binding.options,
      controllerIds: binding.controllerIds,
      allowWorkflowDelivery: false,
      revision: 9,
    },
  });
  expect(api.update).not.toHaveBeenCalled();
});

it("已有团队配置直接授权，请求期间阻止重复点击并就近展示失败", async () => {
  const binding = {
    id: "group",
    connectionId: "connection",
    name: "研发群",
    kind: "group",
    state: "rejected",
    channelId: "team",
    controllerIds: ["owner"],
    allowWorkflowDelivery: true,
    revision: 5,
  };
  api.bindings.mockResolvedValue({
    data: { resources: [binding], totalSize: 1 },
  });
  let fail: (error: Error) => void = () => {
    throw new Error("request not started");
  };
  api.bindingUpdate.mockImplementation(
    () =>
      new Promise((_resolve, reject) => {
        fail = reject;
      }),
  );
  const user = userEvent.setup();
  show("/im/connection");
  const authorize = await screen.findByRole("button", { name: "授权" });
  await user.click(authorize);
  expect(authorize).toBeDisabled();
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(api.bindingUpdate.mock.calls[0]?.[0]).toMatchObject({
    path: { bindingId: "group" },
    body: {
      state: "approved",
      channelId: "team",
      controllerIds: ["owner"],
      revision: 5,
    },
  });
  fail(new Error("授权请求失败"));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "操作失败，请检查连接或执行配置后重试。",
  );
  await waitFor(() => expect(authorize).toBeEnabled());
  expect(
    screen.queryByRole("button", { name: "撤销授权" }),
  ).not.toBeInTheDocument();
});

it("群聊缺少管理员时仍打开精简授权配置", async () => {
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "incomplete",
          connectionId: "connection",
          name: "未完整设置",
          kind: "group",
          state: "rejected",
          channelId: "team",
          controllerIds: [],
          revision: 2,
        },
      ],
      totalSize: 1,
    },
  });
  const user = userEvent.setup();
  show("/im/connection");
  await user.click(await screen.findByRole("button", { name: "授权" }));
  const dialog = screen.getByRole("dialog", { name: "授权" });
  expect(
    within(dialog).getByLabelText(/聊天管理员的平台身份/),
  ).toBeInTheDocument();
  expect(api.bindingUpdate).not.toHaveBeenCalled();
  expect(within(dialog).queryByLabelText(/名称/)).not.toBeInTheDocument();
});

it("已选择伙伴但未保存智能体时先补全配置，取消不授权", async () => {
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "incomplete-waker",
          connectionId: "connection",
          name: "未完成的伙伴配置",
          kind: "direct",
          state: "rejected",
          wakerId: "waker",
          options: {},
          revision: 2,
        },
      ],
      totalSize: 1,
    },
  });
  const user = userEvent.setup();
  show("/im/connection");
  await user.click(await screen.findByRole("button", { name: "授权" }));
  const dialog = screen.getByRole("dialog", { name: "授权" });
  await waitFor(() =>
    expect(
      within(dialog).getByRole("combobox", { name: "执行目标" }),
    ).toHaveTextContent("日常助理"),
  );
  expect(api.bindingUpdate).not.toHaveBeenCalled();
  await user.click(within(dialog).getByRole("button", { name: "取消" }));
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(api.bindingUpdate).not.toHaveBeenCalled();
});

it("连接级操作留页头，撤销属于当前绑定且取消不提交", async () => {
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "current-binding",
          connectionId: "connection",
          name: "当前聊天",
          kind: "direct",
          state: "approved",
          wakerId: "waker",
          revision: 2,
        },
      ],
      totalSize: 1,
    },
  });
  const user = userEvent.setup();
  show("/im/connection");
  const revoke = await screen.findByRole("button", { name: "撤销授权" });
  const pageHeader = screen
    .getByRole("heading", { name: /^我的微信/ })
    .closest("header");
  expect(pageHeader).not.toContainElement(revoke);
  expect(pageHeader).toContainElement(
    screen.getByRole("button", { name: "编辑连接" }),
  );
  expect(revoke.closest("section")).toHaveTextContent("聊天 ID");
  await user.click(revoke);
  const dialog = screen.getByRole("dialog", { name: "撤销授权" });
  await user.click(within(dialog).getByRole("button", { name: "取消" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(api.bindingUpdate).not.toHaveBeenCalled();
  expect(api.remove).not.toHaveBeenCalled();
});

it("绑定与对话读取失败不会伪装为等待聊天或暂无对话", async () => {
  api.conversations.mockRejectedValue(new Error("会话读取失败"));
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "binding",
          connectionId: "connection",
          name: "当前聊天",
          state: "approved",
        },
      ],
      totalSize: 1,
    },
  });
  show("/im/connection");
  await waitFor(() =>
    expect(screen.getAllByRole("alert").length).toBeGreaterThan(0),
  );
  expect(screen.queryByText("暂无对话记录")).not.toBeInTheDocument();
  expect(
    screen.getByRole("heading", { name: /^我的微信/ }),
  ).toBeInTheDocument();
});

it("多个聊天绑定分别展示身份与授权，新建对话在弹窗明确选择目标", async () => {
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "first",
          name: "聊天一",
          externalChatId: "oc_first",
          kind: "direct",
          state: "approved",
          wakerId: "waker",
        },
        {
          id: "second",
          name: "聊天二",
          externalChatId: "oc_second",
          kind: "direct",
          state: "approved",
          wakerId: "waker",
        },
      ],
      totalSize: 2,
    },
  });
  show("/im/connection");
  await screen.findAllByText("oc_second");
  expect(screen.getByText("oc_first")).toBeInTheDocument();
  expect(screen.getAllByRole("button", { name: "撤销授权" })).toHaveLength(2);
  expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "新建对话" }));
  const dialog = screen.getByRole("dialog", { name: "新建对话" });
  await userEvent.click(
    within(dialog).getByRole("combobox", { name: "选择聊天接入" }),
  );
  await userEvent.click(screen.getByRole("option", { name: "聊天二" }));
  await userEvent.click(
    within(dialog).getByRole("button", { name: "新建对话" }),
  );
  await waitFor(() => expect(api.selectConversation).toHaveBeenCalledTimes(1));
  expect(api.selectConversation).toHaveBeenCalledWith(
    expect.objectContaining({
      path: { bindingId: "second" },
      body: { linkId: "" },
    }),
  );
});

it("连接删除和新建对话选择可以取消，不提交业务请求", async () => {
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "first",
          externalChatId: "oc_first",
          state: "approved",
          wakerId: "waker",
        },
        {
          id: "second",
          externalChatId: "oc_second",
          state: "approved",
          wakerId: "waker",
        },
      ],
      totalSize: 2,
    },
  });
  show("/im/connection");
  await screen.findAllByText("oc_second");
  await userEvent.click(screen.getByRole("button", { name: "删除连接" }));
  await userEvent.click(
    within(screen.getByRole("dialog")).getByRole("button", { name: "取消" }),
  );
  expect(api.remove).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "新建对话" }));
  await userEvent.click(
    within(screen.getByRole("dialog")).getByRole("button", { name: "取消" }),
  );
  expect(api.selectConversation).not.toHaveBeenCalled();
});

it("聊天绑定失败仍保留对话分区且禁止新建，重试可恢复", async () => {
  api.bindings.mockRejectedValue(new Error("绑定读取失败"));
  show("/im/connection");
  await screen.findAllByRole("alert");
  expect(screen.getByRole("heading", { name: "对话记录" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "新建对话" })).toBeDisabled();
  expect(screen.queryByText("尚无实际对话")).not.toBeInTheDocument();
  api.bindings.mockResolvedValue({
    data: {
      resources: [
        {
          id: "first",
          externalChatId: "oc_first",
          state: "approved",
          wakerId: "waker",
        },
      ],
      totalSize: 1,
    },
  });
  await userEvent.click(
    within(screen.getByRole("region", { name: "对话记录" })).getByRole(
      "button",
      { name: "重试读取" },
    ),
  );
  await screen.findByText("oc_first");
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "新建对话" })).toBeEnabled(),
  );
});

vi.mock("@/shared/api/global-events", () => ({
  registerCoreConnectionRecovery: () => () => {},
  subscribeGlobalEvents: () => () => {},
}));
