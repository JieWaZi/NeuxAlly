import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@/shared/i18n";
import { PluginAuthPanel } from "./plugin-auth-panel";
const fixture = vi.hoisted(() => ({
  status: {
    provider: "feishu",
    account: {
      configured: true,
      available: true,
      appId: "cli_app",
      name: "用户",
    },
    session: {
      id: "session",
      state: "waiting",
      userCode: "",
      verificationUrl: "https://example.com/device?code=a%2Fb&x=1+2",
    },
    activeUses: [] as {
      chatId: string;
      executionId: string;
      wakerId: string;
      title: string;
    }[],
    missingScopes: [] as string[],
  },
  login: vi.fn<(input: unknown) => Promise<unknown>>().mockResolvedValue({}),
  configure: vi
    .fn<(input: unknown) => Promise<unknown>>()
    .mockResolvedValue({}),
  cancel: vi.fn().mockResolvedValue({}),
  open: vi.fn().mockResolvedValue(undefined),
}));
vi.mock("@/shared/api/use-query-events", () => ({
  useQueryEvents: () => undefined,
}));
vi.mock("@/shared/workspace/app-scene-workspace", () => ({
  useRouteSceneActive: () => true,
}));
vi.mock("@/shared/platform", () => ({ openExternalURL: fixture.open }));
vi.mock("qrcode.react", () => ({
  QRCodeSVG: ({ value }: { value: string }) => (
    <output aria-label="二维码内容">{value}</output>
  ),
}));
vi.mock("../api/plugin-queries", () => ({
  getPluginAuthorizationOptions: () => ({
    queryKey: [{ _id: "getPluginAuthorization" }],
    queryFn: async () => fixture.status,
  }),
  startPluginAuthorizationMutation: () => ({ mutationFn: fixture.login }),
  configurePluginAuthorizationMutation: () => ({
    mutationFn: fixture.configure,
  }),
  cancelPluginAuthorizationMutation: () => ({ mutationFn: fixture.cancel }),
  logoutPluginAuthorizationMutation: () => ({ mutationFn: vi.fn() }),
  invalidatePluginQueries: async () => undefined,
}));
afterEach(cleanup);
beforeEach(() => {
  fixture.login.mockClear();
  fixture.configure.mockClear();
  fixture.open.mockClear();
  fixture.status.activeUses = [];
  fixture.status.provider = "feishu";
  fixture.status.account.available = true;
  fixture.status.session.userCode = "";
  fixture.cancel.mockClear();
  fixture.status.session.state = "waiting";
});
function show() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <PluginAuthPanel pluginId="p" />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return client;
}
describe("shared plugin authorization", () => {
  it("preserves the original URL in QR and external link", async () => {
    show();
    fireEvent.click(
      await screen.findByRole("button", { name: "打开授权页面" }),
    );
    expect(fixture.open).toHaveBeenCalledWith(
      fixture.status.session.verificationUrl,
    );
    fireEvent.click(screen.getByRole("button", { name: "显示二维码" }));
    expect(await screen.findByLabelText("二维码内容")).toHaveTextContent(
      fixture.status.session.verificationUrl,
    );
  });
  it("requests explicit supplemental scopes without replacing the app", async () => {
    fixture.status.session.state = "idle";
    show();
    fireEvent.change(await screen.findByLabelText("补充权限（可选）"), {
      target: { value: "docx:document, bitable:app" },
    });
    fireEvent.click(screen.getByRole("button", { name: "开始授权" }));
    await waitFor(() =>
      expect(fixture.login.mock.calls[0]?.[0]).toMatchObject({
        body: { additionalScopes: ["docx:document", "bitable:app"] },
      }),
    );
    expect(fixture.configure).not.toHaveBeenCalled();
  });
  it("shows account leases as work links and blocks identity changes", async () => {
    fixture.status.session.state = "idle";
    fixture.status.activeUses = [
      {
        chatId: "chat-1",
        executionId: "exec-1",
        wakerId: "partner-1",
        title: "读取会议文档",
      },
    ];
    show();
    expect(
      await screen.findByRole("link", { name: "读取会议文档" }),
    ).toHaveAttribute("href", "/chats/chat-1");
    expect(screen.getByRole("button", { name: "更换飞书应用" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "开始授权" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "退出共享账号" })).toBeDisabled();
  });
  it("submits app secrets only from the explicit app configuration form", async () => {
    fixture.status.session.state = "idle";
    show();
    fireEvent.click(
      await screen.findByRole("button", { name: "更换飞书应用" }),
    );
    fireEvent.change(screen.getByLabelText("飞书应用 App ID"), {
      target: { value: "cli_next" },
    });
    fireEvent.change(screen.getByLabelText("飞书应用 App Secret"), {
      target: { value: "test-secret" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存应用配置" }));
    await waitFor(() =>
      expect(fixture.configure.mock.calls[0]?.[0]).toMatchObject({
        body: { appId: "cli_next", appSecret: "test-secret" },
      }),
    );
    await waitFor(() =>
      expect(
        screen.queryByLabelText("飞书应用 App Secret"),
      ).not.toBeInTheDocument(),
    );
  });
});

it("offers GitHub code and QR dialog without an app form; closing keeps authorization alive", async () => {
  fixture.status.provider = "github";
  fixture.status.account.available = false;
  fixture.status.session.userCode = "ABCD-EFGH";
  fixture.status.session.verificationUrl = "https://github.com/login/device";
  const client = show();
  expect(await screen.findByText("ABCD-EFGH")).toBeInTheDocument();
  expect(screen.queryByLabelText("飞书应用 App ID")).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "更换飞书应用" }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "显示二维码" }));
  expect(
    await screen.findByRole("dialog", { name: "扫码授权" }),
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "关闭" }));
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(fixture.cancel).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "显示二维码" }));
  expect(await screen.findByRole("dialog")).toBeInTheDocument();
  client.setQueryData([{ _id: "getPluginAuthorization" }], {
    ...fixture.status,
    session: {
      ...fixture.status.session,
      state: "ready",
      verificationUrl: "",
      userCode: "",
    },
  });
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
});

it("copies the GitHub device code and reports clipboard failures locally", async () => {
  const user = userEvent.setup();
  fixture.status.provider = "github";
  fixture.status.session.userCode = "ABCD-EFGH";
  show();
  await user.click(await screen.findByRole("button", { name: "复制验证码" }));
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "已复制" })).toBeInTheDocument(),
  );
  expect(await navigator.clipboard.readText()).toBe("ABCD-EFGH");
  const write = vi
    .spyOn(navigator.clipboard, "writeText")
    .mockRejectedValue(new Error("denied"));
  await user.click(screen.getByRole("button", { name: "已复制" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "复制未完成，请手动输入验证码。",
  );
  write.mockRestore();
});
