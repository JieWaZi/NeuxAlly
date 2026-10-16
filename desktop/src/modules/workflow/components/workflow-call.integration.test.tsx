import userEvent from "@testing-library/user-event";
import type * as GlobalEventsModule from "@/shared/api/global-events";
import type * as EventsModule from "@/shared/api/events";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import "@/shared/i18n";
import { WorkbenchProvider } from "@/modules/chat";
import { useWorkbench } from "@/modules/chat/workbench/workbench-store";
import { getWorkflowRunQueryKey } from "../api/workflow-queries";
import { WorkflowCall, WorkflowCallContent } from "./workflow-call";

import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { MemoryRouter } from "react-router-dom";
import { client as apiClient } from "@/shared/api/generated/client.gen";
import type {
  GlobalEventObserver,
  GlobalEventResource,
} from "@/shared/api/global-events";
const watching = vi.hoisted(() => ({
  global: vi.fn(),
  run: vi.fn(),
  globalClose: vi.fn(),
  runClose: vi.fn(),
  observer: undefined as GlobalEventObserver | undefined,
}));
vi.mock("@/shared/api/global-events", async (original) => ({
  ...(await original<typeof GlobalEventsModule>()),
  subscribeGlobalEvents: (
    topic: string,
    resource: GlobalEventResource,
    observer: GlobalEventObserver,
  ) => {
    watching.global(topic, resource);
    watching.observer = observer;
    return watching.globalClose;
  },
}));
vi.mock("@/shared/api/events", async (original) => ({
  ...(await original<typeof EventsModule>()),
  subscribeWorkflow: (id: string, options: { runId?: string }) => {
    watching.run(id, options.runId);
    return watching.runClose;
  },
}));
const origin = "http://workflow-call.test";
const server = setupServer();
beforeAll(() => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
  apiClient.setConfig({ baseUrl: origin });
  server.listen({ onUnhandledRequest: "error" });
});
afterAll(() => {
  server.close();
  vi.unstubAllGlobals();
});
beforeEach(() => vi.clearAllMocks());
afterEach(() => {
  localStorage.clear();
  server.resetHandlers();
});
function Tabs() {
  const { snapshot } = useWorkbench("chat:test");
  return (
    <output aria-label="工作台引用">
      {JSON.stringify(snapshot.tabs.map((tab) => tab.contentReference))}
    </output>
  );
}
describe("聊天工作流执行项", () => {
  it.each(["light", "dark"])(
    "%s 主题窄容器中长中文运行项保持键盘打开能力",
    async (theme) => {
      document.documentElement.dataset.theme = theme;
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
      });
      queryClient.setQueryData(
        getWorkflowRunQueryKey({ path: { id: "flow", runId: "narrow-run" } }),
        { run: { id: "narrow-run", status: "running" } },
      );
      render(
        <QueryClientProvider client={queryClient}>
          <WorkbenchProvider>
            <div style={{ width: 280 }}>
              <WorkflowCall
                runs={[
                  {
                    workflowId: "flow",
                    runId: "narrow-run",
                    name: "非常长的中文工作流评审名称用于窄容器操作验证",
                  },
                ]}
                scopeKey="chat:test"
                isSummarizing={false}
                active
              />
              <Tabs />
            </div>
          </WorkbenchProvider>
        </QueryClientProvider>,
      );
      const user = userEvent.setup();
      await user.tab();
      expect(
        screen.getByRole("button", { name: /非常长的中文/ }),
      ).toHaveFocus();
      await user.keyboard("{Enter}");
      expect(screen.getByLabelText("工作台引用")).toHaveTextContent(
        "narrow-run",
      );
      delete document.documentElement.dataset.theme;
    },
  );

  it("准确资源事件更新原运行，隐藏和卸载释放观察而不取消", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    });
    queryClient.setQueryData(
      getWorkflowRunQueryKey({ path: { id: "flow", runId: "old-run" } }),
      { run: { id: "old-run", status: "running" } },
    );
    const reads = vi.fn();
    server.use(
      http.get(`${origin}/api/v1/workflows/flow/runs/old-run`, () => {
        reads();
        return HttpResponse.json({
          run: { id: "old-run", status: "succeeded" },
        });
      }),
    );
    const view = (active: boolean) => (
      <QueryClientProvider client={queryClient}>
        <WorkbenchProvider>
          <WorkflowCall
            runs={[{ workflowId: "flow", runId: "old-run", name: "原运行" }]}
            scopeKey="chat:test"
            isSummarizing
            active={active}
          />
        </WorkbenchProvider>
      </QueryClientProvider>
    );
    const rendered = render(view(true));
    expect(watching.global).toHaveBeenCalledWith("workflow", {
      kind: "workflow",
      id: "flow",
      parentId: "old-run",
    });
    act(() => watching.observer?.onState?.("unavailable"));
    expect(screen.getByRole("status")).toHaveTextContent(
      "连接中断，显示上次读取的运行状态",
    );
    expect(screen.getByRole("button", { name: /原运行/ })).toHaveTextContent(
      "执行中",
    );
    act(() => watching.observer?.onState?.("open"));
    act(() => {
      watching.observer?.onResync?.("gap");
    });
    await waitFor(() =>
      expect(screen.getByRole("button", { name: /原运行/ })).toHaveTextContent(
        "已完成",
      ),
    );
    expect(reads).toHaveBeenCalledTimes(1);
    rendered.rerender(view(false));
    expect(watching.globalClose).toHaveBeenCalledTimes(1);
    rendered.unmount();
    expect(watching.run).not.toHaveBeenCalled();
  });

  it("右侧复用原运行停止，关闭仅释放观察", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const cancelled = vi.fn();
    let status = "running";
    server.use(
      http.get(`${origin}/api/v1/workflows/flow/runs/old-run`, () =>
        HttpResponse.json({
          run: { id: "old-run", status },
          instances: [],
          humanRequests: [],
          operationRequests: [],
        }),
      ),
      http.post(`${origin}/api/v1/workflows/flow/runs/old-run/cancel`, () => {
        cancelled();
        status = "cancelled";
        return HttpResponse.json({ run: { id: "old-run", status } });
      }),
    );
    const rendered = render(
      <QueryClientProvider client={queryClient}>
        <WorkbenchProvider>
          <MemoryRouter>
            <WorkflowCallContent
              reference={{
                type: "workflow-run",
                ids: { workflowId: "flow", runId: "old-run" },
              }}
              active
              scopeKey="chat:test"
            />
          </MemoryRouter>
        </WorkbenchProvider>
      </QueryClientProvider>,
    );
    fireEvent.click(await screen.findByRole("button", { name: "停止运行" }));
    fireEvent.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "停止运行",
      }),
    );
    await waitFor(() => expect(cancelled).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    rendered.unmount();
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(watching.runClose).toHaveBeenCalledTimes(1);
    expect(cancelled).toHaveBeenCalledTimes(1);
  });

  it("右侧返回其他运行时拒绝呈现并保留原引用", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    server.use(
      http.get(`${origin}/api/v1/workflows/flow/runs/missing-run`, () =>
        HttpResponse.json({
          run: { id: "latest-run", status: "running" },
          manifest: { nodes: [], edges: [] },
        }),
      ),
    );
    render(
      <QueryClientProvider client={queryClient}>
        <WorkbenchProvider>
          <MemoryRouter>
            <WorkflowCallContent
              reference={{
                type: "workflow-run",
                ids: { workflowId: "flow", runId: "missing-run" },
              }}
              active
              scopeKey="chat:test"
            />
          </MemoryRouter>
        </WorkbenchProvider>
      </QueryClientProvider>,
    );
    expect(await screen.findByRole("alert")).toHaveTextContent("运行暂不可用");
    expect(
      screen.queryByRole("button", { name: "停止运行" }),
    ).not.toBeInTheDocument();
  });

  it.each([404, 500])(
    "原运行查询 %s 在所属区域报错且不显示另一运行",
    async (status) => {
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });
      server.use(
        http.get(`${origin}/api/v1/workflows/flow/runs/deleted-run`, () =>
          HttpResponse.json({ message: "原运行不可用" }, { status }),
        ),
      );
      render(
        <QueryClientProvider client={queryClient}>
          <WorkbenchProvider>
            <MemoryRouter>
              <WorkflowCallContent
                reference={{
                  type: "workflow-run",
                  ids: { workflowId: "flow", runId: "deleted-run" },
                }}
                active
                scopeKey="chat:test"
              />
            </MemoryRouter>
          </WorkbenchProvider>
        </QueryClientProvider>,
      );
      expect(await screen.findByRole("alert")).toBeVisible();
      expect(
        screen.queryByRole("button", { name: "停止运行" }),
      ).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: "重试" })).toBeEnabled();
    },
  );

  it.each(["running", "failed", "cancelled", "succeeded"])(
    "%s 读取准确旧运行，重复关联只显示一项且打开右侧原身份",
    (status) => {
      const client = new QueryClient({
        defaultOptions: { queries: { retry: false, staleTime: Infinity } },
      });
      client.setQueryData(
        getWorkflowRunQueryKey({ path: { id: "flow", runId: "run-old" } }),
        { run: { id: "run-old", workflowId: "flow", status } },
      );
      const ref = {
        workflowId: "flow",
        runId: "run-old",
        revisionId: "rev-old",
        name: "代码评审",
      };
      render(
        <QueryClientProvider client={client}>
          <WorkbenchProvider>
            <WorkflowCall
              runs={[ref, ref]}
              scopeKey="chat:test"
              isSummarizing={status !== "running"}
              active
            />
            <Tabs />
          </WorkbenchProvider>
        </QueryClientProvider>,
      );
      const item = screen.getByRole("button", { name: /代码评审/ });
      expect(screen.getAllByRole("button", { name: /代码评审/ })).toHaveLength(
        1,
      );
      expect(item).toHaveTextContent(
        {
          running: "执行中",
          failed: "失败",
          cancelled: "失败（已停止）",
          succeeded: "已完成",
        }[status] ?? "",
      );
      if (status !== "running")
        expect(screen.getByText("智能体正在总结")).toBeVisible();
      fireEvent.click(item);
      expect(
        JSON.parse(screen.getByLabelText("工作台引用").textContent || "[]"),
      ).toContainEqual({
        type: "workflow-run",
        ids: { workflowId: "flow", runId: "run-old" },
      });
    },
  );
});
