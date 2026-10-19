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
      challengeId: "challenge",
      actions: [] as { type: string; url?: string; value?: string }[],
      verificationUrl: "https://example.com/device?code=a%2Fb&x=1+2",
      expiresAt: Date.now() + 60_000,
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
  submit: vi.fn(),
  statusError: null as Error | null,
}));
vi.mock("@/shared/api/use-query-events", () => ({
  useQueryEvents: () => undefined,
}));
vi.mock("@/shared/workspace/app-scene-workspace", () => ({
  useRouteSceneActive: () => true,
}));
vi.mock("@/shared/desktop", () => ({ openExternalURL: fixture.open }));
vi.mock("qrcode.react", () => ({
  QRCodeSVG: ({ value }: { value: string }) => (
    <output aria-label="二维码内容">{value}</output>
  ),
}));
vi.mock("../api/plugin-queries", () => ({
  getPluginAuthorizationOptions: () => ({
    queryKey: [{ _id: "getPluginAuthorization" }],
    queryFn: async () => {
      if (fixture.statusError) throw fixture.statusError;
      return structuredClone(fixture.status);
    },
  }),
  startPluginAuthorizationMutation: () => ({ mutationFn: fixture.login }),
  configurePluginAuthorizationMutation: () => ({
    mutationFn: fixture.configure,
  }),
  cancelPluginAuthorizationMutation: () => ({ mutationFn: fixture.cancel }),
  logoutPluginAuthorizationMutation: () => ({ mutationFn: vi.fn() }),
  invalidatePluginQueries: async (client: QueryClient) =>
    client.invalidateQueries(),
}));
afterEach(cleanup);
beforeEach(() => {
  fixture.login.mockClear();
  fixture.submit.mockClear();
  fixture.statusError = null;
  fixture.configure.mockClear();
  fixture.open.mockClear();
  fixture.status.activeUses = [];
  fixture.status.missingScopes = [];
  fixture.status.provider = "feishu";
  fixture.status.account.available = true;
  fixture.status.account.configured = true;
  fixture.status.session.userCode = "";
  fixture.cancel.mockClear();
  fixture.status.session.state = "waiting";
  fixture.status.session.id = "session";
  fixture.status.session.expiresAt = Date.now() + 60_000;
  fixture.status.session.verificationUrl =
    "https://example.com/device?code=a%2Fb&x=1+2";
  fixture.status.session.challengeId = "challenge";
  fixture.status.session.actions = [
    { type: "show-qr", value: fixture.status.session.verificationUrl },
    { type: "open-url", url: fixture.status.session.verificationUrl },
  ];
});
function show(installation = false, onReady = vi.fn()) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            fixture.submit();
          }}
        >
          <PluginAuthPanel
            pluginId="p"
            installation={installation}
            onReady={onReady}
          />
        </form>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return client;
}
describe("shared plugin authorization", () => {
  it("retries account loading without submitting the surrounding settings form", async () => {
    fixture.statusError = new Error("offline");
    fixture.status.session.state = "ready";
    show();
    const retry = await screen.findByRole("button", { name: "重试" });
    fixture.statusError = null;
    fireEvent.click(retry);
    expect(await screen.findByText("已连接：用户")).toBeVisible();
    expect(fixture.submit).not.toHaveBeenCalled();
  });
  it("notifies completion only after an authoritative ready snapshot", async () => {
    const onReady = vi.fn();
    const client = show(true, onReady);
    await screen.findByLabelText("二维码内容");
    expect(onReady).not.toHaveBeenCalled();
    const completed = {
      ...fixture.status,
      session: {
        ...fixture.status.session,
        state: "ready",
        verificationUrl: "",
      },
    };
    client.setQueryData([{ _id: "getPluginAuthorization" }], completed);
    await waitFor(() => expect(onReady).toHaveBeenCalledOnce());
    client.setQueryData([{ _id: "getPluginAuthorization" }], completed);
    expect(onReady).toHaveBeenCalledOnce();
  });
  it("removes an expired QR without a server push, then waits for cancellation before refreshing", async () => {
    fixture.status.account.available = false;
    fixture.status.session.expiresAt = Date.now() + 250;
    let finishCancel: (() => void) | undefined;
    fixture.cancel.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          finishCancel = resolve;
        }),
    );
    fixture.login.mockImplementationOnce(async () => {
      fixture.status.session = {
        ...fixture.status.session,
        id: "new-session",
        verificationUrl: "https://example.com/new-device",
        actions: [{ type: "show-qr", value: "https://example.com/new-device" }],
        expiresAt: Date.now() + 60_000,
      };
      return {};
    });
    show(true);
    await screen.findByLabelText("二维码内容");
    await waitFor(() =>
      expect(screen.queryByLabelText("二维码内容")).not.toBeInTheDocument(),
    );
    expect(screen.getByRole("status")).toHaveTextContent("授权已过期");
    fireEvent.click(screen.getByRole("button", { name: "重新授权" }));
    await waitFor(() => expect(fixture.cancel).toHaveBeenCalledOnce());
    expect(fixture.login).not.toHaveBeenCalled();
    finishCancel?.();
    await waitFor(() => expect(fixture.login).toHaveBeenCalledOnce());
    expect(await screen.findByLabelText("二维码内容")).toHaveTextContent(
      "https://example.com/new-device",
    );
  });
  it("preserves the original URL in QR and external link", async () => {
    show();
    fireEvent.click(
      await screen.findByRole("button", { name: "打开授权页面" }),
    );
    expect(fixture.open).toHaveBeenCalledWith(
      fixture.status.session.verificationUrl,
    );
    expect(await screen.findByLabelText("二维码内容")).toHaveTextContent(
      fixture.status.session.verificationUrl,
    );
  });
  it("requests server-reported missing permissions without a manual scope field", async () => {
    fixture.status.session.state = "idle";
    fixture.status.account.available = false;
    fixture.status.missingScopes = ["docx:document", "bitable:app"];
    show();
    fireEvent.click(await screen.findByRole("button", { name: "重新授权" }));
    await waitFor(() =>
      expect(fixture.login.mock.calls[0]?.[0]).toMatchObject({
        body: { additionalScopes: ["docx:document", "bitable:app"] },
      }),
    );
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(fixture.configure).not.toHaveBeenCalled();
    expect(fixture.submit).not.toHaveBeenCalled();
  });
  it("shows a connected account without asking users to connect or enter permissions again", async () => {
    fixture.status.provider = "github";
    fixture.status.session.state = "ready";
    show();
    expect(await screen.findByText("已连接：用户")).toBeVisible();
    expect(screen.getByRole("heading", { name: "账号连接" })).toBeVisible();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "连接账号" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "重新授权" }),
    ).not.toBeInTheDocument();
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
    expect(
      screen.queryByRole("button", { name: "更换飞书应用" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "重新授权" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "断开连接" })).toBeDisabled();
  });
  it("starts first-time Feishu setup through QR authorization without app credentials", async () => {
    fixture.status.account.configured = false;
    fixture.status.account.available = false;
    fixture.status.session.state = "idle";
    show();
    fireEvent.click(await screen.findByRole("button", { name: "连接账号" }));
    await waitFor(() => expect(fixture.login).toHaveBeenCalledTimes(1));
    expect(screen.queryByLabelText("飞书应用 App ID")).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText("飞书应用 App Secret"),
    ).not.toBeInTheDocument();
    expect(fixture.configure).not.toHaveBeenCalled();
  });
});

it("offers GitHub code and QR dialog without an app form; closing keeps authorization alive", async () => {
  fixture.status.provider = "github";
  fixture.status.account.available = false;
  fixture.status.session.userCode = "ABCD-EFGH";
  fixture.status.session.actions.push({
    type: "copy-code",
    value: "ABCD-EFGH",
  });
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
  fixture.status.session.actions.push({
    type: "copy-code",
    value: "ABCD-EFGH",
  });
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

it("uses declared actions for an unknown plugin and never turns a browser URL into a QR", async () => {
  fixture.status.provider = "new-provider";
  fixture.status.account.available = false;
  fixture.status.session.actions = [
    {
      type: "open-url",
      url: "https://example.com/oauth?redirect_uri=http://localhost/callback",
    },
  ];
  show(true);
  fireEvent.click(await screen.findByRole("button", { name: "打开授权页面" }));
  expect(fixture.open).toHaveBeenCalledWith(
    "https://example.com/oauth?redirect_uri=http://localhost/callback",
  );
  expect(screen.queryByLabelText("二维码内容")).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "显示二维码" }),
  ).not.toBeInTheDocument();
  expect(screen.queryByText(/飞书/)).not.toBeInTheDocument();
});

it("replaces a challenge and removes all actions after cancellation", async () => {
  fixture.status.provider = "new-provider";
  fixture.status.account.available = false;
  fixture.status.session.actions = [
    { type: "show-qr", value: "custom://first" },
    { type: "copy-code", value: "ONE-TIME" },
  ];
  const client = show(true);
  expect(await screen.findByLabelText("二维码内容")).toHaveTextContent(
    "custom://first",
  );
  const changed = {
    ...fixture.status,
    session: {
      ...fixture.status.session,
      challengeId: "second",
      actions: [{ type: "show-qr", value: "custom://second" }],
    },
  };
  client.setQueryData([{ _id: "getPluginAuthorization" }], changed);
  await waitFor(() =>
    expect(screen.getByLabelText("二维码内容")).toHaveTextContent(
      "custom://second",
    ),
  );
  expect(screen.queryByText("ONE-TIME")).not.toBeInTheDocument();
  client.setQueryData([{ _id: "getPluginAuthorization" }], {
    ...changed,
    session: { ...changed.session, state: "cancelled" },
  });
  await waitFor(() =>
    expect(screen.queryByLabelText("二维码内容")).not.toBeInTheDocument(),
  );
});
