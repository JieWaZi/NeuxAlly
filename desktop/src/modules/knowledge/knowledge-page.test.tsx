import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  fireEvent,
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Link,
  MemoryRouter,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import {
  afterAll,
  afterEach,
  beforeAll,
  expect,
  it,
  onTestFinished,
  vi,
} from "vitest";
import { client } from "@/shared/api/generated/client.gen";
import { TooltipProvider } from "@/shared/ui/tooltip";
import { KnowledgePage } from "./knowledge-page";
import { saveOrganizePreferences } from "./knowledge-organize-preferences";

vi.mock("@/shared/api/use-query-events", () => ({ useQueryEvents: vi.fn() }));
const server = setupServer();
beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
  client.setConfig({ baseUrl: "http://ally.test" });
});
afterEach(() => {
  server.resetHandlers();
  saveOrganizePreferences(library.ref, {
    selection: undefined,
    requirements: undefined,
  });
});
afterAll(() => server.close());
const library = {
  id: "kb",
  name: "通用资料库",
  kind: "general",
  ref: { backend: "local", connectionId: "local", knowledgeBaseId: "kb" },
  version: 1,
  structureVersion: 1,
  policyVersion: 1,
};
function acceptedReceipt(body: Record<string, unknown>) {
  return {
    requestId: body.requestId,
    libraryId: "kb",
    resourceId: "job",
    jobId: "job",
    version: 1,
    state: "accepted",
  };
}
const firstSource = {
  id: "source-1",
  sourceId: "raw-1",
  filename: "原始资料.md",
  state: "pending",
  size: 2,
  createdAt: 1,
};
function Location() {
  const value = useLocation();
  return (
    <>
      <Link to="/knowledge">测试切换到列表</Link>
      <output aria-label="当前路由">
        {value.pathname}
        {value.search}
      </output>
    </>
  );
}
function show(initial = "/knowledge/kb") {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <MemoryRouter initialEntries={[initial]}>
          <Location />
          <Routes>
            <Route path="/knowledge/:libraryId?" element={<KnowledgePage />} />
          </Routes>
        </MemoryRouter>
      </TooltipProvider>
    </QueryClientProvider>,
  );
  return queryClient;
}
function fixtures() {
  server.use(
    http.get(
      "http://ally.test/api/v1/knowledge/libraries/kb/source-purges",
      () => HttpResponse.json({ items: [], total: 0 }),
    ),
    http.get(
      "http://ally.test/api/v1/knowledge/libraries/kb/execution-defaults",
      () =>
        HttpResponse.json({
          libraryId: "kb",
          version: 0,
          wakerId: "",
          runtimeId: "",
          accountId: "",
          model: "",
          reasoning: "",
        }),
    ),
    http.put(
      "http://ally.test/api/v1/knowledge/libraries/kb/execution-defaults",
      async ({ request }) =>
        HttpResponse.json({
          ...((await request.json()) as object),
          libraryId: "kb",
          version: 1,
        }),
    ),
    http.get("http://ally.test/api/v1/memory/settings", () =>
      HttpResponse.json({}),
    ),
    http.get("http://ally.test/api/v1/knowledge/libraries/kb", () =>
      HttpResponse.json(library),
    ),
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/sources", () =>
      HttpResponse.json({ items: [firstSource], total: 1 }),
    ),
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/jobs", () =>
      HttpResponse.json({ items: [], total: 0 }),
    ),
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/jobs/job", () =>
      HttpResponse.json({
        job: { id: "job", libraryId: "kb", state: "processing" },
        session: { id: "session", chatId: "" },
      }),
    ),
    http.get("http://ally.test/api/v1/runtimes", () =>
      HttpResponse.json({
        resources: [
          {
            id: "codex",
            name: "Codex",
            enabled: true,
            state: "ready",
            models: [
              { id: "model", name: "Model", permissionLevels: ["default"] },
            ],
            defaultModel: "model",
          },
        ],
      }),
    ),
    http.get("http://ally.test/api/v1/ai-accounts", () =>
      HttpResponse.json({ resources: [] }),
    ),
  );
}

it("文档页操作并入筛选栏，内容连续排列且无重复页头", async () => {
  fixtures();
  const job = {
    id: "job",
    libraryId: "kb",
    state: "completed",
    sourceVersionIds: ["source-1"],
  };
  server.use(
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/jobs", () =>
      HttpResponse.json({ items: [job], total: 1 }),
    ),
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/jobs/job", () =>
      HttpResponse.json({ job }),
    ),
  );
  show();
  await screen.findByText("原始资料.md");
  expect(
    screen.queryByRole("heading", { name: "通用资料库" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "返回列表" }),
  ).not.toBeInTheDocument();
  const actions = screen.getByRole("group", { name: "知识库操作" });
  expect(
    within(actions)
      .getAllByRole("button")
      .map((action) => action.getAttribute("aria-label")),
  ).toEqual(["设置", "整理文档", "上传资料"]);
  expect(
    within(actions).getByRole("button", { name: "设置" }),
  ).toHaveTextContent("");
  expect(
    within(actions).getByRole("button", { name: "整理文档" }),
  ).toHaveTextContent("");
  expect(actions.parentElement).toContainElement(
    screen.getByRole("searchbox", { name: "搜索原件…" }),
  );
  expect(screen.queryByText("整理完成")).not.toBeInTheDocument();
  expect(screen.queryByText(/本次 .*份文档/)).not.toBeInTheDocument();
  expect(screen.queryByText("查看整理对话")).not.toBeInTheDocument();
  expect(screen.queryByText("1 份资料")).not.toBeInTheDocument();
  expect(screen.queryByText("整理本页待处理文档")).not.toBeInTheDocument();
  const table = screen.getByRole("table");
  const flow = table.closest(
    '[data-slot="resource-page-surface"]',
  )?.firstElementChild;
  // 页面仍可占满滚动区，但内容不能用隐式网格行分配剩余高度。
  expect(flow).toHaveClass("flex", "flex-col");
  const raw = table.parentElement?.parentElement?.parentElement;
  expect(raw).toHaveClass("flex", "flex-col", "shrink-0");
  expect(table).toHaveClass("table-fixed");
  const columns = table.querySelectorAll("colgroup > col");
  expect(columns).toHaveLength(6);
  expect(
    screen.queryByRole("heading", { name: "文档" }),
  ).not.toBeInTheDocument();
  expect(actions.parentElement?.parentElement?.parentElement).toHaveClass(
    "border-b",
  );
  expect(
    screen.getByRole("columnheader", { name: "操作" }),
  ).toBeInTheDocument();
  expect(columns[0]).toHaveClass("w-12");
});

it("空文档页不以固定最小高度撑开内容", async () => {
  fixtures();
  server.use(
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/sources", () =>
      HttpResponse.json({ items: [], total: 0 }),
    ),
  );
  show();
  const empty = await screen.findByText("还没有原始资料");
  expect(empty.parentElement?.parentElement).toHaveClass("min-h-0");
  expect(empty.parentElement?.parentElement).not.toHaveClass("min-h-64");
});

it.each(["relations", "history", "source", "project", "chat"])(
  "旧 %s 入口只读取文档并规范化 URL",
  async (view) => {
    fixtures();
    show(`/knowledge/kb?view=${view}&page=old&source=old&themeId=old`);
    expect(await screen.findByText("原始资料.md")).toBeVisible();
    await waitFor(() =>
      expect(screen.getByLabelText("当前路由")).toHaveTextContent(
        "/knowledge/kb",
      ),
    );
    expect(screen.getByLabelText("当前路由").textContent).not.toContain(
      "view=",
    );
    expect(
      screen.queryByRole("button", { name: /历史|详情|归档/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Wiki 已保存")).not.toBeInTheDocument();
  },
);

it("Dock 图谱保留直链，读取完整分页关系并直接展示正文", async () => {
  // jsdom 不执行布局；图谱外部资源须等实际容器尺寸可用后创建。
  const width = vi
    .spyOn(HTMLElement.prototype, "clientWidth", "get")
    .mockReturnValue(1000);
  const height = vi
    .spyOn(HTMLElement.prototype, "clientHeight", "get")
    .mockReturnValue(600);
  onTestFinished(() => {
    width.mockRestore();
    height.mockRestore();
  });
  fixtures();
  const offsets: string[] = [];
  server.use(
    http.get(
      "http://ally.test/api/v1/knowledge/libraries/kb/pages",
      ({ request }) => {
        const offset = new URL(request.url).searchParams.get("offset") ?? "0";
        offsets.push(offset);
        return HttpResponse.json({
          items:
            offset === "0"
              ? [
                  {
                    id: "first",
                    slug: "concept/first",
                    title: "起点知识",
                    pageType: "concept",
                    outLinks: ["concept/last"],
                  },
                ]
              : [
                  {
                    id: "last",
                    slug: "concept/last",
                    title: "末页知识",
                    pageType: "entity",
                    outLinks: [],
                  },
                ],
          total: 2,
        });
      },
    ),
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/themes", () =>
      HttpResponse.json({ items: [] }),
    ),
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/pages/last", () =>
      HttpResponse.json({
        id: "last",
        slug: "concept/last",
        title: "末页知识",
        content: "真实 Wiki 正文",
      }),
    ),
  );
  const user = userEvent.setup();
  show("/knowledge/kb?view=graph");
  const canvas = await screen.findByRole("group", { name: "知识关系图" });
  expect(canvas.closest('[data-slot="resource-page-surface"]')).toBeNull();
  expect(
    canvas.closest('section[aria-label="图谱"]')?.parentElement,
  ).toHaveClass("h-full");
  expect(offsets).toEqual(["0", "1"]);
  expect(canvas.querySelectorAll("line[data-edge]")).toHaveLength(1);
  expect(screen.getByLabelText("当前路由")).toHaveTextContent("view=graph");
  expect(
    screen.queryByRole("button", { name: "上传资料" }),
  ).not.toBeInTheDocument();
  await user.click(within(canvas).getByRole("button", { name: "末页知识" }));
  expect(await screen.findByText("真实 Wiki 正文")).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "打开 Wiki 页面" }),
  ).not.toBeInTheDocument();
  expect(screen.getByLabelText("当前路由")).toHaveTextContent("view=graph");
});

it("上传后只显示待整理，点击整理才接纳一次冻结来源任务", async () => {
  fixtures();
  let uploaded = false;
  const organized: Record<string, unknown>[] = [];
  server.use(
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/sources", () =>
      HttpResponse.json({
        items: uploaded
          ? [
              firstSource,
              { ...firstSource, id: "source-2", filename: "新资料.md" },
            ]
          : [firstSource],
        total: uploaded ? 2 : 1,
      }),
    ),
    http.post(
      "http://ally.test/api/v1/knowledge/libraries/kb/sources",
      async ({ request }) => {
        const body = await request.json();
        expect(body).toMatchObject({ filename: "新资料.md", bytes: "YWI=" });
        uploaded = true;
        return HttpResponse.json({
          resourceId: "source-2",
          state: "committed",
        });
      },
    ),
    http.post(
      "http://ally.test/api/v1/knowledge/libraries/kb/organize",
      async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        organized.push(body);
        return HttpResponse.json({
          receipt: acceptedReceipt(body),
        });
      },
    ),
  );
  const user = userEvent.setup();
  show();
  await user.click(await screen.findByRole("button", { name: "上传资料" }));
  const file = new File(["ab"], "新资料.md", { type: "text/markdown" });
  Object.defineProperty(file, "arrayBuffer", {
    value: async () => new Uint8Array([97, 98]).buffer,
  });
  fireEvent.change(await screen.findByLabelText<HTMLInputElement>("选择文件"), {
    target: { files: [file] },
  });
  await user.click(screen.getByRole("button", { name: "上传文档" }));
  expect(
    await within(screen.getByRole("dialog")).findByText("待整理"),
  ).toBeVisible();
  expect(organized).toHaveLength(0);
  await user.click(screen.getByText("关闭", { selector: "button" }));
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  const start = await screen.findByRole("button", { name: "开始整理" });
  await waitFor(() => expect(start).toBeEnabled());
  await user.dblClick(start);
  await waitFor(() => expect(organized).toHaveLength(1));
  expect(organized[0]).toMatchObject({
    sourceVersionIds: ["source-1", "source-2"],
    runtimeId: "codex",
    model: "model",
  });
});

it("默认整理全库，不受列表翻页与表格选中范围限制", async () => {
  fixtures();
  const organized: unknown[] = [];
  server.use(
    http.get(
      "http://ally.test/api/v1/knowledge/libraries/kb/sources",
      ({ request }) =>
        HttpResponse.json({
          items:
            new URL(request.url).searchParams.get("offset") === "50"
              ? [{ ...firstSource, id: "source-51", filename: "下一页.md" }]
              : [
                  firstSource,
                  ...Array.from({ length: 49 }, (_, index) => ({
                    ...firstSource,
                    id: `processed-${index}`,
                    filename: `已整理-${index}.md`,
                    state: "processed",
                  })),
                ],
          total: 51,
        }),
    ),
    http.post(
      "http://ally.test/api/v1/knowledge/libraries/kb/organize",
      async ({ request }) => {
        organized.push(await request.json());
        return HttpResponse.json({ receipt: { jobId: "job" } });
      },
    ),
  );
  const user = userEvent.setup();
  show("/knowledge/kb?sourcesPageSize=50");
  await user.click(
    await screen.findByRole("checkbox", { name: "选择 原始资料.md" }),
  );
  await user.click(screen.getByRole("button", { name: "下一页" }));
  await screen.findByText("下一页.md");
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  const start = await screen.findByRole("button", { name: "开始整理" });
  await waitFor(() => expect(start).toBeEnabled());
  await user.click(start);
  await waitFor(() => expect(organized).toHaveLength(1));
  expect(organized[0]).toMatchObject({
    sourceVersionIds: ["source-1", "source-51"],
  });
});

it.each(["network", "partial500"])(
  "新建%s未知结果关闭重开后沿用请求身份，且不会自动上传或整理",
  async (failure) => {
    fixtures();
    const requests: Record<string, unknown>[] = [];
    server.use(
      http.get("http://ally.test/api/v1/knowledge/connections/libraries", () =>
        HttpResponse.json({ items: [], total: 0 }),
      ),
      http.post(
        "http://ally.test/api/v1/knowledge/libraries",
        async ({ request }) => {
          requests.push((await request.json()) as Record<string, unknown>);
          return requests.length === 1
            ? failure === "network"
              ? HttpResponse.error()
              : HttpResponse.json(
                  {
                    message: "默认配置保存待补齐",
                    code: "KnowledgeDefaultsPersistenceFailed",
                  },
                  { status: 500 },
                )
            : HttpResponse.json({ resourceId: "kb", libraryId: "kb" });
        },
      ),
    );
    const user = userEvent.setup();
    show("/knowledge");
    await user.click(screen.getByRole("button", { name: "新建知识库" }));
    await user.type(
      await screen.findByRole("textbox", { name: "名称" }),
      "通用资料库",
    );
    await user.click(screen.getByRole("button", { name: "创建知识库" }));
    await screen.findByRole("alert");
    await user.click(screen.getByRole("button", { name: "取消" }));
    await user.click(screen.getByRole("button", { name: "新建知识库" }));
    expect(screen.getByRole("textbox", { name: "名称" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "重试并核验结果" }));
    await waitFor(() => expect(requests).toHaveLength(2));
    expect(requests[1]).toEqual(requests[0]);
    expect(requests[0]).not.toHaveProperty("kind");
    expect(requests[0]).toMatchObject({
      executionDefaults: { runtimeId: "codex", model: "model" },
    });
    expect(requests[1]).toMatchObject({ name: "通用资料库" });
    expect(await screen.findByText("原始资料.md")).toBeVisible();
  },
);

it("后台运行只保护操作按钮，不展示任务状态或请求任务详情", async () => {
  fixtures();
  let detailReads = 0;
  server.use(
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/jobs", () =>
      HttpResponse.json({
        items: [{ id: "job", libraryId: "kb", state: "processing" }],
        total: 1,
        activeJob: { id: "job", libraryId: "kb", state: "processing" },
      }),
    ),
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/jobs/job", () => {
      detailReads++;
      return HttpResponse.json({
        job: { id: "job", libraryId: "kb", state: "processing" },
      });
    }),
  );
  show();
  await screen.findByRole("button", { name: "设置" });
  await vi.dynamicImportSettled();
  await screen.findByText("原始资料.md", {}, { timeout: 4000 });
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "整理文档" })).toBeDisabled(),
  );
  expect(screen.queryByText("正在整理")).not.toBeInTheDocument();
  expect(detailReads).toBe(0);
});

it("详情工具栏不再提供返回列表的页头按钮", async () => {
  fixtures();
  show();
  await screen.findByRole("button", { name: "设置" });
  expect(
    screen.queryByRole("button", { name: "返回列表" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("heading", { name: "通用资料库" }),
  ).not.toBeInTheDocument();
});

it("整理默认选中待处理和失败文件，不混入已整理原件", async () => {
  fixtures();
  const requests: Array<{ sourceVersionIds: string[] }> = [];
  server.use(
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/sources", () =>
      HttpResponse.json({
        items: [
          firstSource,
          {
            ...firstSource,
            id: "failed",
            filename: "失败原件.md",
            state: "failed",
          },
          {
            ...firstSource,
            id: "processed",
            filename: "已整理原件.md",
            state: "processed",
          },
        ],
        total: 3,
      }),
    ),
    http.post(
      "http://ally.test/api/v1/knowledge/libraries/kb/organize",
      async ({ request }) => {
        requests.push((await request.json()) as { sourceVersionIds: string[] });
        return HttpResponse.json({
          receipt: { jobId: "job", state: "accepted" },
        });
      },
    ),
  );
  const user = userEvent.setup();
  show();
  await screen.findByText("失败原件.md");
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  expect(await screen.findByText("已选 2 个文件")).toBeVisible();
  const start = screen.getByRole("button", { name: "开始整理" });
  await waitFor(() => expect(start).toBeEnabled());
  await user.click(start);
  await waitFor(() => expect(requests).toHaveLength(1));
  expect(requests[0]?.sourceVersionIds).toEqual(["source-1", "failed"]);
});

it.each(["failed", "cancelled"] as const)(
  "没有pending时默认整理%s原件，终态旧任务不占槽且提交新任务身份",
  async (state) => {
    fixtures();
    const requests: Array<{ requestId: string; sourceVersionIds: string[] }> =
      [];
    server.use(
      http.get("http://ally.test/api/v1/knowledge/libraries/kb/sources", () =>
        HttpResponse.json({
          items: [
            { ...firstSource, id: "retry-source", state },
            {
              ...firstSource,
              id: "processed",
              filename: "已整理原件.md",
              state: "processed",
            },
          ],
          total: 2,
        }),
      ),
      http.get("http://ally.test/api/v1/knowledge/libraries/kb/jobs", () =>
        HttpResponse.json({
          items: [
            {
              id: "old-job",
              libraryId: "kb",
              requestId: "old-request",
              state,
              attempt: 2,
            },
          ],
          total: 1,
        }),
      ),
      http.post(
        "http://ally.test/api/v1/knowledge/libraries/kb/organize",
        async ({ request }) => {
          requests.push(
            (await request.json()) as {
              requestId: string;
              sourceVersionIds: string[];
            },
          );
          return HttpResponse.json({
            receipt: { jobId: "job", state: "accepted" },
          });
        },
      ),
    );
    const user = userEvent.setup();
    show();
    await screen.findByText("已整理原件.md");
    const organize = screen.getByRole("button", { name: "整理文档" });
    await waitFor(() => expect(organize).toBeEnabled());
    await user.click(organize);
    expect(await screen.findByText("已选 1 个文件")).toBeVisible();
    const start = screen.getByRole("button", { name: "开始整理" });
    await waitFor(() => expect(start).toBeEnabled());
    await user.click(start);
    await waitFor(() => expect(requests).toHaveLength(1));
    expect(requests[0]?.sourceVersionIds).toEqual(["retry-source"]);
    expect(requests[0]?.requestId).toBeTruthy();
    expect(requests[0]?.requestId).not.toBe("old-request");
  },
);

it("真实运行中的任务仍阻止默认整理失败来源", async () => {
  fixtures();
  server.use(
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/sources", () =>
      HttpResponse.json({
        items: [{ ...firstSource, state: "failed" }],
        total: 1,
      }),
    ),
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/jobs", () =>
      HttpResponse.json({
        items: [{ id: "active-job", libraryId: "kb", state: "processing" }],
        activeJob: { id: "active-job", libraryId: "kb", state: "processing" },
        total: 1,
      }),
    ),
  );
  show();
  await screen.findByText("原始资料.md");
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "整理文档" })).toBeDisabled(),
  );
});

it("整理可用性读取失败就近重试，不恢复任务状态展示", async () => {
  fixtures();
  let failed = true;
  server.use(
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/jobs", () =>
      failed
        ? HttpResponse.error()
        : HttpResponse.json({ items: [], total: 0 }),
    ),
  );
  const user = userEvent.setup();
  show();
  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("无法确认是否有整理任务正在运行，请重试。");
  expect(screen.getByRole("button", { name: "整理文档" })).toBeDisabled();
  failed = false;
  await user.click(within(alert).getByRole("button", { name: "重试" }));
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "整理文档" })).toBeEnabled(),
  );
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(screen.queryByText("查看整理对话")).not.toBeInTheDocument();
});

it("确认整理取得真实任务回执后关闭弹窗，尚无Execution时也不显示失败", async () => {
  fixtures();
  const user = userEvent.setup();
  let finish: (() => void) | undefined;
  const requests: Record<string, unknown>[] = [];
  server.use(
    http.post(
      "http://ally.test/api/v1/knowledge/libraries/kb/organize",
      async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        requests.push(body);
        await new Promise<void>((resolve) => {
          finish = resolve;
        });
        return HttpResponse.json({
          receipt: {
            requestId: body.requestId,
            libraryId: "kb",
            resourceId: "job",
            jobId: "job",
            version: 1,
            state: "accepted",
          },
          noop: false,
          session: { chatId: "" },
        });
      },
    ),
  );
  show();
  await screen.findByText("原始资料.md");
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  await user.click(await screen.findByRole("button", { name: "开始整理" }));
  expect(
    screen.queryByRole("region", { name: "悬浮对话" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("dialog")).toBeVisible();
  expect(screen.queryByText("正在连接整理对话…")).not.toBeInTheDocument();
  await waitFor(() => expect(requests).toHaveLength(1));
  expect(requests[0]).toMatchObject({
    message: "整理指定资料，连续完成全部阶段。",
  });
  finish?.();
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(
    screen.queryByRole("region", { name: "悬浮对话" }),
  ).not.toBeInTheDocument();
  expect(requests).toHaveLength(1);
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});
it("离开详情后迟到的接纳不会在列表页弹出聊天", async () => {
  fixtures();
  const user = userEvent.setup();
  let finish: (() => void) | undefined;
  server.use(
    http.get("http://ally.test/api/v1/knowledge/connections/libraries", () =>
      HttpResponse.json({ items: [], total: 0 }),
    ),
    http.post(
      "http://ally.test/api/v1/knowledge/libraries/kb/organize",
      async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        await new Promise<void>((resolve) => {
          finish = resolve;
        });
        return HttpResponse.json({ receipt: acceptedReceipt(body) });
      },
    ),
  );
  show();
  await screen.findByText("原始资料.md");
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  await user.click(await screen.findByRole("button", { name: "开始整理" }));
  await waitFor(() => expect(finish).toBeTypeOf("function"));
  await user.click(screen.getByRole("button", { name: "取消" }));
  await user.click(screen.getByRole("link", { name: "测试切换到列表" }));
  finish?.();
  await waitFor(() =>
    expect(screen.getByLabelText("当前路由").textContent).toBe("/knowledge"),
  );
  expect(
    screen.queryByRole("region", { name: "悬浮对话" }),
  ).not.toBeInTheDocument();
});

it("后台接纳只使用知识任务回执且不挂载聊天", async () => {
  fixtures();
  server.use(
    http.post(
      "http://ally.test/api/v1/knowledge/libraries/kb/organize",
      async ({ request }) =>
        HttpResponse.json({
          receipt: acceptedReceipt(
            (await request.json()) as Record<string, unknown>,
          ),
        }),
    ),
  );
  const user = userEvent.setup();
  show();
  await screen.findByText("原始资料.md");
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  const start = await screen.findByRole("button", { name: "开始整理" });
  await waitFor(() => expect(start).toBeEnabled());
  await user.click(start);
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(
    screen.queryByRole("region", { name: "悬浮对话" }),
  ).not.toBeInTheDocument();
});

it("关闭重开上传弹窗后用原请求身份核对未知回执", async () => {
  fixtures();
  const requests: Array<{ requestId?: string }> = [];
  server.use(
    http.post(
      "http://ally.test/api/v1/knowledge/libraries/kb/sources",
      async ({ request }) => {
        requests.push((await request.json()) as { requestId?: string });
        return requests.length === 1
          ? HttpResponse.error()
          : HttpResponse.json({ resourceId: "source-2", state: "saved" });
      },
    ),
  );
  const user = userEvent.setup();
  show();
  await user.click(await screen.findByRole("button", { name: "上传资料" }));
  const file = new File(["ab"], "恢复资料.md", { type: "text/markdown" });
  Object.defineProperty(file, "arrayBuffer", {
    value: async () => new Uint8Array([97, 98]).buffer,
  });
  fireEvent.change(await screen.findByLabelText<HTMLInputElement>("选择文件"), {
    target: { files: [file] },
  });
  await user.click(screen.getByRole("button", { name: "上传文档" }));
  await screen.findByRole("alert");
  await user.click(screen.getByText("关闭", { selector: "button" }));
  await user.click(screen.getByRole("button", { name: "上传资料" }));
  expect(screen.getByText("恢复资料.md")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "重试失败文档" }));
  await waitFor(() => expect(requests).toHaveLength(2));
  expect(requests[1]?.requestId).toBe(requests[0]?.requestId);
  expect(
    await within(screen.getByRole("dialog")).findByText("待整理"),
  ).toBeVisible();
});

vi.stubGlobal(
  "ResizeObserver",
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
);

it("整理从库配置初始化，临时修改直接执行且不写回库配置", async () => {
  fixtures();
  const user = userEvent.setup();
  const profileRequests: Record<string, unknown>[] = [];
  const organized: Record<string, unknown>[] = [];
  const profile = {
    libraryId: "kb",
    version: 2,
    wakerId: "configured-waker",
    runtimeId: "codex",
    accountId: "system",
    model: "saved-model",
    reasoning: "",
  };
  server.use(
    http.get("http://ally.test/api/v1/runtimes", () =>
      HttpResponse.json({
        resources: [
          {
            id: "codex",
            name: "Codex",
            enabled: true,
            state: "ready",
            models: [
              { id: "model", name: "Model", permissionLevels: ["default"] },
              {
                id: "saved-model",
                name: "已存模型",
                permissionLevels: ["default"],
              },
            ],
            defaultModel: "model",
          },
        ],
      }),
    ),
    http.get(
      "http://ally.test/api/v1/knowledge/libraries/kb/execution-defaults",
      () => HttpResponse.json(profile),
    ),
    http.put(
      "http://ally.test/api/v1/knowledge/libraries/kb/execution-defaults",
      async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        profileRequests.push(body);
        return HttpResponse.json(profile);
      },
    ),
    http.post(
      "http://ally.test/api/v1/knowledge/libraries/kb/organize",
      async ({ request }) => {
        organized.push((await request.json()) as Record<string, unknown>);
        return HttpResponse.json({ noop: true });
      },
    ),
  );
  show();
  await screen.findByText("原始资料.md", {}, { timeout: 10000 });
  await user.click(await screen.findByRole("button", { name: "整理文档" }));
  const model = await screen.findByRole(
    "combobox",
    { name: "模型" },
    { timeout: 10000 },
  );
  expect(model).toHaveTextContent("已存模型");
  fireEvent.keyDown(model, { key: "ArrowDown" });
  await user.click(await screen.findByRole("option", { name: "Model" }));
  await user.click(screen.getByRole("button", { name: "开始整理" }));
  await waitFor(() => expect(organized).toHaveLength(1));
  expect(profileRequests).toHaveLength(0);
  expect(organized[0]).toMatchObject({
    sourceVersionIds: ["source-1"],
    model: "model",
  });
  expect(organized[0]).not.toHaveProperty("wakerId");
  expect(organized[0]).not.toHaveProperty("permissionLevel");
});

it("后台接纳200缺完整任务回执时关闭重开保留原请求，不重复用户输入", async () => {
  fixtures();
  const user = userEvent.setup();
  const requests: Record<string, unknown>[] = [];
  server.use(
    http.post(
      "http://ally.test/api/v1/knowledge/libraries/kb/organize",
      async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        requests.push(body);
        return HttpResponse.json(
          requests.length === 1
            ? { receipt: { jobId: "job" } }
            : {
                receipt: acceptedReceipt(body),
              },
        );
      },
    ),
  );
  show();
  await screen.findByText("原始资料.md");
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  await user.type(
    await screen.findByRole("textbox", { name: "整理提取要求" }),
    "保留在线和离线适用条件",
  );
  await user.click(await screen.findByRole("button", { name: "开始整理" }));
  await screen.findByRole("alert");
  await user.click(screen.getByRole("button", { name: "取消" }));
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  const requirements = await screen.findByRole("textbox", {
    name: "整理提取要求",
  });
  expect(requirements).toHaveValue("保留在线和离线适用条件");
  expect(requirements).toBeDisabled();
  expect(screen.getByRole("button", { name: "已选 1 个文件" })).toBeDisabled();
  await user.click(await screen.findByRole("button", { name: "开始整理" }));
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(
    screen.queryByRole("region", { name: "悬浮对话" }),
  ).not.toBeInTheDocument();
  expect(requests).toHaveLength(2);
  expect(requests[1]).toEqual(requests[0]);
});

it("文档初次加载显示内容骨架，后台刷新仍保留已有文档与筛选", async () => {
  fixtures();
  let release!: () => void;
  let pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  server.use(
    http.get(
      "http://ally.test/api/v1/knowledge/libraries/kb/sources",
      async () => {
        await pending;
        return HttpResponse.json({ items: [firstSource], total: 1 });
      },
    ),
  );
  const queryClient = show();
  try {
    const loading = await screen.findByRole("status", {
      name: "正在读取内容…",
    });
    expect(loading).toHaveAttribute("aria-busy", "true");
    expect(loading.querySelector("[aria-hidden=true]")).toBeInTheDocument();
    expect(screen.getByRole("searchbox", { name: "搜索原件…" })).toBeEnabled();
    expect(screen.queryByText("还没有原始资料")).not.toBeInTheDocument();
    release();
    await screen.findByText("原始资料.md");
    pending = new Promise<void>((resolve) => {
      release = resolve;
    });
    void queryClient.invalidateQueries({
      predicate: (query) => query.queryKey.includes("sources"),
    });
    await waitFor(() => expect(queryClient.isFetching()).toBeGreaterThan(0));
    expect(screen.getByText("原始资料.md")).toBeVisible();
    expect(
      screen.queryByRole("status", { name: "正在读取内容…" }),
    ).not.toBeInTheDocument();
  } finally {
    release();
  }
});

it("后台接纳中关闭重开配置复用同一个待定请求，不取消任务或重复发送", async () => {
  fixtures();
  const user = userEvent.setup();
  let finish: (() => void) | undefined;
  const requests: Record<string, unknown>[] = [];
  server.use(
    http.post(
      "http://ally.test/api/v1/knowledge/libraries/kb/organize",
      async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        requests.push(body);
        await new Promise<void>((resolve) => {
          finish = resolve;
        });
        return HttpResponse.json({
          receipt: acceptedReceipt(body),
        });
      },
    ),
  );
  show();
  await screen.findByText("原始资料.md");
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  await user.click(await screen.findByRole("button", { name: "开始整理" }));
  await waitFor(() => expect(requests).toHaveLength(1));
  await user.click(screen.getByRole("button", { name: "取消" }));
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  await user.click(await screen.findByRole("button", { name: "开始整理" }));
  expect(requests).toHaveLength(1);
  finish?.();
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(requests).toHaveLength(1);
  expect(
    screen.queryByRole("region", { name: "悬浮对话" }),
  ).not.toBeInTheDocument();
});

it("更早任务重试仍按activeJob保护操作，不把最新历史完成记录当成空闲", async () => {
  fixtures();
  server.use(
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/jobs", () =>
      HttpResponse.json({
        items: [{ id: "newer-completed", libraryId: "kb", state: "completed" }],
        total: 2,
        activeJob: {
          id: "old-retry",
          libraryId: "kb",
          state: "queued",
          attempt: 2,
        },
      }),
    ),
  );
  show();
  await screen.findByRole("button", { name: "设置" });
  await vi.dynamicImportSettled();
  await screen.findByText("原始资料.md", {}, { timeout: 4000 });
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "整理文档" })).toBeDisabled(),
  );
  expect(screen.getByRole("button", { name: "上传资料" })).toBeDisabled();
});

it("整理全库待处理文件，可搜索取消选择并提交仅本次的提取要求", async () => {
  fixtures();
  const requests: Record<string, unknown>[] = [];
  const firstPage = Array.from({ length: 50 }, (_, index) => ({
    ...firstSource,
    id: `source-${index}`,
    filename: `资料-${index}.md`,
  }));
  server.use(
    http.get(
      "http://ally.test/api/v1/knowledge/libraries/kb/sources",
      ({ request }) =>
        HttpResponse.json({
          items:
            new URL(request.url).searchParams.get("offset") === "50"
              ? [
                  {
                    ...firstSource,
                    id: "failed-next-page",
                    filename: "失败资料.md",
                    state: "failed",
                  },
                ]
              : firstPage,
          total: 51,
        }),
    ),
    http.post(
      "http://ally.test/api/v1/knowledge/libraries/kb/organize",
      async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        requests.push(body);
        return HttpResponse.json({
          receipt: acceptedReceipt(body),
        });
      },
    ),
  );
  const user = userEvent.setup();
  show();
  await screen.findByText("资料-0.md");
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  await user.click(
    await screen.findByRole("button", { name: "已选 51 个文件" }),
  );
  await user.type(
    screen.getByRole("searchbox", { name: "搜索文件…" }),
    "失败资料",
  );
  const failed = screen.getByRole("checkbox", { name: "失败资料.md" });
  expect(failed).toBeChecked();
  await user.click(failed);
  await user.keyboard("{Escape}");
  expect(screen.getByRole("button", { name: "已选 50 个文件" })).toBeVisible();
  await user.type(
    screen.getByRole("textbox", { name: "整理提取要求" }),
    "重点提取部署步骤和限制条件",
  );
  expect(
    screen.queryByText("账号决定可用模型；模型决定推理选项和支持的执行权限。"),
  ).not.toBeInTheDocument();
  const start = screen.getByRole("button", { name: "开始整理" });
  await waitFor(() => expect(start).toBeEnabled());
  await user.click(start);
  await waitFor(() => expect(requests).toHaveLength(1));
  expect(requests[0]?.sourceVersionIds).toEqual(
    firstPage.map((source) => source.id),
  );
  expect(requests[0]?.message).toContain("重点提取部署步骤和限制条件");
});

it("上传资料从点击到表单就绪保持同一个弹窗", async () => {
  fixtures();
  show();
  await screen.findByText("原始资料.md");
  fireEvent.click(screen.getByRole("button", { name: "上传资料" }));
  const shell = screen.getByRole("dialog", { name: "上传资料" });
  await screen.findByRole("button", { name: "选择文件" });
  expect(screen.getByRole("dialog", { name: "上传资料" })).toBe(shell);
});

it("清空整理文件后不可提交，重新勾选才能开始", async () => {
  fixtures();
  const user = userEvent.setup();
  show();
  await screen.findByText("原始资料.md");
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  await user.click(
    await screen.findByRole("button", { name: "已选 1 个文件" }),
  );
  await user.click(screen.getByRole("button", { name: "清空选择" }));
  await user.keyboard("{Escape}");
  expect(screen.getByRole("button", { name: "开始整理" })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "已选 0 个文件" }));
  await user.click(screen.getByRole("checkbox", { name: "原始资料.md" }));
  await user.keyboard("{Escape}");
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "开始整理" })).toBeEnabled(),
  );
});

it("当前页全部已整理时仍可选择其他页待整理文件", async () => {
  fixtures();
  server.use(
    http.get(
      "http://ally.test/api/v1/knowledge/libraries/kb/sources",
      ({ request }) =>
        HttpResponse.json({
          items:
            new URL(request.url).searchParams.get("offset") === "50"
              ? [
                  {
                    ...firstSource,
                    id: "next-pending",
                    filename: "待整理资料.md",
                  },
                ]
              : Array.from({ length: 50 }, (_, index) => ({
                  ...firstSource,
                  id: `processed-${index}`,
                  filename: `已整理-${index}.md`,
                  state: "processed",
                })),
          total: 51,
        }),
    ),
  );
  const user = userEvent.setup();
  show();
  await screen.findByText("已整理-0.md");
  const organize = screen.getByRole("button", { name: "整理文档" });
  await waitFor(() => expect(organize).toBeEnabled());
  await user.click(organize);
  await user.click(
    await screen.findByRole("button", { name: "已选 1 个文件" }),
  );
  expect(screen.getByRole("checkbox", { name: "待整理资料.md" })).toBeChecked();
});

it("普通取消后重新打开会读取新增文件，不复用旧来源草稿", async () => {
  fixtures();
  let items = [firstSource];
  server.use(
    http.get("http://ally.test/api/v1/knowledge/libraries/kb/sources", () =>
      HttpResponse.json({ items, total: items.length }),
    ),
  );
  const user = userEvent.setup();
  show();
  await screen.findByText("原始资料.md");
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  await screen.findByRole("button", { name: "已选 1 个文件" });
  await user.click(screen.getByRole("button", { name: "取消" }));
  items = [
    firstSource,
    { ...firstSource, id: "new-upload", filename: "新增资料.md" },
  ];
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  await user.click(
    await screen.findByRole("button", { name: "已选 2 个文件" }),
  );
  expect(screen.getByRole("checkbox", { name: "新增资料.md" })).toBeChecked();
});

it("取消后重新挂载页面仍恢复本库的模型和整理要求，清空要求也会记住", async () => {
  fixtures();
  server.use(
    http.get("http://ally.test/api/v1/runtimes", () =>
      HttpResponse.json({
        resources: [
          {
            id: "codex",
            name: "Codex",
            enabled: true,
            state: "ready",
            models: [
              { id: "model", name: "Model", permissionLevels: ["default"] },
              { id: "other", name: "Other", permissionLevels: ["default"] },
            ],
            defaultModel: "model",
          },
        ],
      }),
    ),
  );
  const user = userEvent.setup();
  show();
  await screen.findByText("原始资料.md", {}, { timeout: 10000 });
  await user.click(await screen.findByRole("button", { name: "整理文档" }));
  fireEvent.keyDown(
    await screen.findByRole("combobox", { name: "模型" }, { timeout: 10000 }),
    { key: "ArrowDown" },
  );
  await user.click(await screen.findByRole("option", { name: "Other" }));
  await user.type(
    screen.getByRole("textbox", { name: "整理提取要求" }),
    "保留部署限制",
  );
  await user.click(screen.getByRole("button", { name: "取消" }));
  cleanup();
  show();
  await screen.findByText("原始资料.md", {}, { timeout: 10000 });
  await user.click(await screen.findByRole("button", { name: "整理文档" }));
  expect(
    await screen.findByRole("combobox", { name: "模型" }, { timeout: 10000 }),
  ).toHaveTextContent("Other");
  expect(screen.getByRole("textbox", { name: "整理提取要求" })).toHaveValue(
    "保留部署限制",
  );
  await user.clear(screen.getByRole("textbox", { name: "整理提取要求" }));
  await user.click(screen.getByRole("button", { name: "取消" }));
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  expect(
    await screen.findByRole("textbox", { name: "整理提取要求" }),
  ).toHaveValue("");
});

it("文档默认每页十条，翻页与切换条数使用真实查询并回到第一页", async () => {
  fixtures();
  const all = Array.from({ length: 21 }, (_, index) => ({
    ...firstSource,
    id: `page-source-${index + 1}`,
    filename: `分页资料-${index + 1}.md`,
  }));
  server.use(
    http.get(
      "http://ally.test/api/v1/knowledge/libraries/kb/sources",
      ({ request }) => {
        const params = new URL(request.url).searchParams;
        const offset = Number(params.get("offset"));
        const limit = Number(params.get("limit"));
        return HttpResponse.json({
          items: all.slice(offset, offset + limit),
          total: all.length,
        });
      },
    ),
  );
  const user = userEvent.setup();
  show();
  await screen.findByText("分页资料-1.md");
  expect(screen.getByText("分页资料-10.md")).toBeVisible();
  expect(screen.queryByText("分页资料-11.md")).not.toBeInTheDocument();
  expect(screen.getByText("共 21 条")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "下一页" }));
  await screen.findByText("分页资料-11.md");
  expect(screen.queryByText("分页资料-1.md")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "第 2 页" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await user.click(screen.getByRole("button", { name: "每页数量" }));
  await user.click(screen.getByRole("menuitemradio", { name: "每页 30 条" }));
  await screen.findByText("分页资料-1.md");
  expect(screen.getByText("分页资料-21.md")).toBeVisible();
  expect(screen.getByRole("button", { name: "第 1 页" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  expect(screen.getByRole("button", { name: "下一页" })).toBeDisabled();
  expect(screen.getByLabelText("当前路由")).toHaveTextContent(
    "sourcesPageSize=30",
  );
  await user.click(screen.getByRole("button", { name: "每页数量" }));
  await user.click(screen.getByRole("menuitemradio", { name: "每页 10 条" }));
  await waitFor(() =>
    expect(screen.queryByText("分页资料-11.md")).not.toBeInTheDocument(),
  );
  expect(screen.getByRole("button", { name: "下一页" })).toBeEnabled();
});

it("跨页状态筛选、搜索与排序由后端执行并使用匹配总数", async () => {
  fixtures();
  const all = Array.from({ length: 44 }, (_, index) => ({
    ...firstSource,
    id: `filtered-source-${index + 1}`,
    filename: `${index < 10 ? "完成" : "排队"}-${index + 1}.md`,
    state: index < 10 ? "processed" : "queued",
    createdAt: index + 1,
  }));
  server.use(
    http.get(
      "http://ally.test/api/v1/knowledge/libraries/kb/sources",
      ({ request }) => {
        const params = new URL(request.url).searchParams;
        const offset = Number(params.get("offset"));
        const limit = Number(params.get("limit"));
        const state = params.get("state") || "all";
        const query = params.get("query") || "";
        const filtered = all.filter(
          (source) =>
            (state === "all" || source.state === state) &&
            source.filename.includes(query),
        );
        if (params.get("sort") !== "oldest") filtered.reverse();
        return HttpResponse.json({
          items: filtered.slice(offset, offset + limit),
          total: filtered.length,
        });
      },
    ),
  );
  const user = userEvent.setup();
  show("/knowledge/kb?sourcesOffset=10&sourceState=queued&sourceSort=oldest");
  await vi.dynamicImportSettled();
  await screen.findByText("排队-21.md", {}, { timeout: 5000 });
  expect(screen.getByText("共 34 条")).toBeVisible();
  expect(screen.queryByText("排队-11.md")).not.toBeInTheDocument();
  fireEvent.keyDown(screen.getByRole("combobox", { name: "排序" }), {
    key: "Enter",
  });
  fireEvent.keyDown(await screen.findByRole("option", { name: "最新上传" }), {
    key: "Enter",
  });
  await screen.findByText("排队-44.md");
  expect(screen.getByRole("button", { name: "第 1 页" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await user.type(
    screen.getByRole("searchbox", { name: "搜索原件…" }),
    "排队-1",
  );
  await screen.findByText("共 9 条");
  expect(screen.getByText("排队-19.md")).toBeVisible();
  expect(screen.queryByText("排队-44.md")).not.toBeInTheDocument();
  fireEvent.keyDown(screen.getByRole("combobox", { name: "资料状态" }), {
    key: "Enter",
  });
  fireEvent.keyDown(await screen.findByRole("option", { name: "已整理" }), {
    key: "Enter",
  });
  await screen.findByText("共 0 条");
  expect(screen.queryByText("排队-19.md")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "下一页" })).toBeDisabled();
});

it("状态更新使末页超出匹配总数时回退到有效页", async () => {
  fixtures();
  let all = Array.from({ length: 21 }, (_, index) => ({
    ...firstSource,
    id: `shrinking-${index + 1}`,
    filename: `进度资料-${index + 1}.md`,
  }));
  server.use(
    http.get(
      "http://ally.test/api/v1/knowledge/libraries/kb/sources",
      ({ request }) => {
        const query = new URL(request.url).searchParams;
        const offset = Number(query.get("offset"));
        const limit = Number(query.get("limit"));
        return HttpResponse.json({
          items: all.slice(offset, offset + limit),
          total: all.length,
        });
      },
    ),
  );
  const client = show("/knowledge/kb?sourcesOffset=20");
  await vi.dynamicImportSettled();
  await screen.findByText("进度资料-21.md", {}, { timeout: 5000 });
  all = all.slice(0, 5);
  await client.invalidateQueries({ queryKey: ["knowledge"] });
  await screen.findByText("进度资料-1.md");
  expect(screen.getByText("共 5 条")).toBeVisible();
  expect(screen.getByRole("button", { name: "第 1 页" })).toHaveAttribute(
    "aria-current",
    "page",
  );
});

it.each([
  "capability-denied",
  "capability-error",
  "jobs-error",
  "sources-error",
])("远端整理未知接纳后重开不被 %s 阻断原请求核对", async (failure) => {
  const user = userEvent.setup();
  const root =
    "http://ally.test/api/v1/knowledge/connections/remote/libraries/kb";
  let denied = false;
  const bodies: Record<string, unknown>[] = [];
  server.use(
    http.get(root, () =>
      HttpResponse.json({
        ...library,
        ref: {
          backend: "custom",
          connectionId: "remote",
          knowledgeBaseId: "kb",
        },
      }),
    ),
    http.get(`${root}/sources`, () =>
      denied && failure === "sources-error"
        ? new HttpResponse(null, { status: 503 })
        : HttpResponse.json({ items: [firstSource], total: 1 }),
    ),
    http.get(`${root}/source-purges`, () =>
      HttpResponse.json({ items: [], total: 0 }),
    ),
    http.get(`${root}/jobs`, () =>
      denied && failure === "jobs-error"
        ? new HttpResponse(null, { status: 503 })
        : HttpResponse.json({
            items: [],
            total: 0,
          }),
    ),
    http.get(`${root}/capabilities`, () =>
      denied && failure === "capability-error"
        ? new HttpResponse(null, { status: 503 })
        : HttpResponse.json({
            operations: {
              organize: !(denied && failure === "capability-denied"),
            },
            profiles: [{ id: "original-profile", name: "原配置" }],
            details: {
              organize: {
                supported: true,
                permission:
                  denied && failure === "capability-denied"
                    ? "denied"
                    : "allowed",
                condition: "ready",
              },
            },
          }),
    ),
    http.get(`${root}/operations/:requestId`, ({ params }) =>
      HttpResponse.json({
        requestId: params.requestId,
        operation: "job.accept",
        ref: {
          backend: "custom",
          connectionId: "remote",
          knowledgeBaseId: "kb",
        },
      }),
    ),
    http.post(`${root}/organize`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      bodies.push(body);
      if (bodies.length === 1) return new HttpResponse(null, { status: 503 });
      return HttpResponse.json({
        requestId: body.requestId,
        jobId: "job",
        state: "accepted",
      });
    }),
  );
  const queryClient = show("/knowledge/kb?backend=custom&connection=remote");
  await screen.findByText("原始资料.md", {}, { timeout: 5000 });
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  const start = await screen.findByRole("button", { name: "开始整理" });
  await waitFor(() => expect(start).toBeEnabled());
  await user.click(start);
  await waitFor(() => expect(screen.getByRole("alert")).toBeVisible());
  denied = true;
  await user.click(screen.getByRole("button", { name: "取消" }));
  await queryClient.invalidateQueries({
    queryKey: ["knowledge", "custom", "remote", "kb"],
  });
  expect(screen.getByRole("button", { name: "整理文档" })).toBeEnabled();
  await user.click(screen.getByRole("button", { name: "整理文档" }));
  await user.click(await screen.findByRole("button", { name: "开始整理" }));
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(bodies).toHaveLength(2);
  expect(bodies[1]).toEqual(bodies[0]);
});
