import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  render,
  screen,
  within,
  waitFor,
  act,
  fireEvent,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { beforeAll, afterAll, afterEach, expect, it, vi } from "vitest";
import { client } from "@/shared/api/generated/client.gen";
import { toApiError } from "@/shared/api/errors";
import * as desktop from "@/shared/desktop";
import { TooltipProvider } from "@/shared/ui/tooltip";
import { emptyLibrary } from "./knowledge-model";
import type { Source } from "./knowledge-model";
import { KnowledgeResultDocument } from "./knowledge-result-document";
import { KnowledgeDocumentTable } from "./knowledge-document-table";

vi.mock("@/shared/api/use-query-events", () => ({ useQueryEvents: vi.fn() }));

const server = setupServer();
let errorInterceptor: number;
const root = "http://ally.test/api/v1/knowledge/libraries/kb";
const source = {
  id: "version",
  sourceId: "source",
  name: "evidence.md",
  size: "1 KB",
  date: "2026/10/20",
  state: "processed" as const,
  diagnostic: "",
};
beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
  client.setConfig({ baseUrl: "http://ally.test" });
  errorInterceptor = client.interceptors.error.use((error, response) =>
    toApiError(error, response?.status),
  );
});
afterEach(() => {
  server.resetHandlers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
afterAll(() => {
  server.close();
  client.interceptors.error.eject(errorInterceptor);
});
function show(
  ref = emptyLibrary("kb", "资料库", "general").ref,
  disabled = false,
  row: Source = source,
) {
  const rebuild = vi.fn();
  const deleted = vi.fn();
  let hideRows = () => {};
  function Subject() {
    const [rows, setRows] = useState([row]);
    hideRows = () => setRows([]);
    return (
      <KnowledgeDocumentTable
        library={{
          ...emptyLibrary("kb", "资料库", "general"),
          ref,
          writable: ref.backend === "local",
        }}
        rows={rows}
        emptyContent={rows.length ? undefined : <p>空文档区域</p>}
        selection={[]}
        select={vi.fn()}
        selectPage={vi.fn()}
        disabled={disabled}
        onRebuild={rebuild}
        onDeleted={deleted}
        pagination={{
          page: 1,
          pageSize: 50,
          totalItems: rows.length,
          onPageChange: vi.fn(),
          labels: {
            navigation: "分页",
            next: "下一页",
            previous: "上一页",
            page: String,
            pageSize: String,
            pageSizeSelection: "每页",
            total: String,
          },
        }}
      />
    );
  }
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <TooltipProvider>
        <Subject />
      </TooltipProvider>
    </QueryClientProvider>,
  );
  return {
    rebuild,
    deleted,
    hideRows: () => act(() => hideRows()),
  };
}
function fixtures() {
  server.use(
    http.get(`${root}/source-purges`, () =>
      HttpResponse.json({ items: [], total: 0 }),
    ),
    http.get(`${root}/sources/version`, () =>
      HttpResponse.json({
        id: "version",
        sourceId: "source",
        filename: "evidence.md",
        mime: "text/markdown",
        createdAt: 1792479600000,
        description: "真实文档摘要",
      }),
    ),
    http.get(`${root}/sources/version/raw`, () =>
      HttpResponse.json({
        filename: "evidence.md",
        mime: "text/markdown",
        bytes: btoa("# Original\r\n\r\n[Reference](https://example.com)"),
      }),
    ),
    http.get(`${root}/sources/version/chunks`, () => HttpResponse.json([])),
  );
}
it("点击文件名使用弹窗展示真实摘要与 Markdown，不展示自定义元数据", async () => {
  fixtures();
  show();
  await userEvent.click(screen.getByRole("button", { name: "evidence.md" }));
  const dialog = screen.getByRole("dialog", { name: "evidence.md" });
  expect(await within(dialog).findByText("真实文档摘要")).toBeVisible();
  expect(
    await within(dialog).findByRole("heading", { name: "Original" }),
  ).toBeVisible();
  expect(within(dialog).getByRole("link", { name: "Reference" })).toBeVisible();
  expect(within(dialog).queryByText("自定义元数据")).not.toBeInTheDocument();
  expect(
    within(dialog).queryByRole("heading", { name: "整理状态" }),
  ).not.toBeInTheDocument();
});
it("行尾菜单下载原字节，重建只选择当前文档", async () => {
  fixtures();
  const save = vi.spyOn(desktop, "saveBlob").mockResolvedValue(true);
  const { rebuild } = show();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "操作" }));
  await user.click(screen.getByRole("menuitem", { name: "下载" }));
  await waitFor(() => expect(save).toHaveBeenCalledOnce());
  expect(save.mock.calls[0]?.[1]).toBe("evidence.md");
  expect(await save.mock.calls[0]?.[0].text()).toBe(
    "# Original\r\n\r\n[Reference](https://example.com)",
  );
  await user.click(screen.getByRole("button", { name: "操作" }));
  await user.click(screen.getByRole("menuitem", { name: "重建知识" }));
  expect(rebuild).toHaveBeenCalledWith(source);
});
it("删除展示影响并明确确认，网络未知重试保留原身份且清理完成后才关闭", async () => {
  fixtures();
  const bodies: Record<string, unknown>[] = [];
  let fail = true;
  let completed = false;
  server.use(
    http.get(root, () => HttpResponse.json({ id: "kb", policyVersion: 3 })),
    http.get(`${root}/source-purges/preview`, ({ request }) => {
      expect(new URL(request.url).searchParams.get("sourceId")).toBe("source");
      return HttpResponse.json({
        sourceId: "source",
        digest: "impact",
        policyVersion: 3,
        versions: [{ versionId: "version" }],
        pages: [],
        blockingJobIds: [],
      });
    }),
    http.post(`${root}/source-purges/confirm`, async ({ request }) => {
      bodies.push((await request.json()) as Record<string, unknown>);
      return fail
        ? new HttpResponse(null, { status: 504 })
        : HttpResponse.json({ resourceId: "purge" });
    }),
    http.get(`${root}/source-purges/purge`, () =>
      HttpResponse.json({
        id: "purge",
        state: completed ? "completed" : "pending",
      }),
    ),
    http.post(`${root}/source-purges/purge/recover`, () =>
      HttpResponse.json({ state: "completed" }),
    ),
  );
  const { deleted, hideRows } = show();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "操作" }));
  await user.click(screen.getByRole("menuitem", { name: "删除" }));
  const dialog = screen.getByRole("dialog", { name: "删除文档" });
  const confirm = await within(dialog).findByRole("button", { name: "删除" });
  await waitFor(() => expect(confirm).toBeEnabled());
  expect(bodies).toHaveLength(0);
  await user.click(confirm);
  expect(await within(dialog).findByRole("alert")).toBeVisible();
  await user.click(within(dialog).getByRole("button", { name: "取消" }));
  await user.click(screen.getByRole("button", { name: "操作" }));
  await user.click(screen.getByRole("menuitem", { name: "删除" }));
  const reopened = screen.getByRole("dialog", { name: "删除文档" });
  fail = false;
  // 状态未完成时仍显示核验提示，不能把接纳回执当成删除完成。
  await user.click(
    within(reopened).getByRole("button", { name: "重试并核验结果" }),
  );
  expect(await within(reopened).findByRole("alert")).toBeVisible();
  expect(bodies[0]?.requestId).toBe(bodies[1]?.requestId);
  expect(deleted).not.toHaveBeenCalled();
  hideRows();
  expect(screen.getByText("空文档区域")).toBeInTheDocument();
  expect(reopened).toBeInTheDocument();
  completed = true;
  await user.click(
    within(reopened).getByRole("button", { name: "重试并核验结果" }),
  );
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(deleted).toHaveBeenCalledWith(source);
  expect(bodies).toHaveLength(2);
});

it("已接纳的删除在来源消失及页面重开后仍可继续，恢复请求保持同一身份", async () => {
  const requests: Record<string, unknown>[] = [];
  let completed = false;
  server.use(
    http.get(root, () => HttpResponse.json({ id: "kb", policyVersion: 4 })),
    http.get(`${root}/source-purges`, () =>
      HttpResponse.json({
        items: completed
          ? []
          : [
              {
                id: "persisted",
                state: "pending",
                version: 2,
                versions: [{ versionId: "gone" }],
              },
            ],
        total: completed ? 0 : 1,
      }),
    ),
    http.get(`${root}/source-purges/persisted`, () =>
      HttpResponse.json({
        id: "persisted",
        state: completed ? "completed" : "pending",
        version: 2,
      }),
    ),
    http.post(
      `${root}/source-purges/persisted/recover`,
      async ({ request }) => {
        requests.push((await request.json()) as Record<string, unknown>);
        if (requests.length === 1)
          return new HttpResponse(null, { status: 504 });
        completed = true;
        return HttpResponse.json({ state: "completed" });
      },
    ),
  );
  const first = show();
  first.hideRows();
  const user = userEvent.setup();
  await user.click(
    await screen.findByRole("button", { name: "查询并继续清理" }),
  );
  let dialog = screen.getByRole("dialog", { name: "继续文档清理" });
  await user.click(within(dialog).getByRole("button", { name: "继续清理" }));
  expect(await within(dialog).findByRole("alert")).toBeVisible();
  // 模拟离开并返回，前端临时 Map 已丢失，仍按持久清单恢复。
  const { cleanup } = await import("@testing-library/react");
  cleanup();
  const second = show();
  second.hideRows();
  await user.click(
    await screen.findByRole("button", { name: "查询并继续清理" }),
  );
  dialog = screen.getByRole("dialog", { name: "继续文档清理" });
  await user.click(within(dialog).getByRole("button", { name: "继续清理" }));
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(requests).toHaveLength(2);
  expect(requests[0]?.requestId).toBe(requests[1]?.requestId);
  expect(requests[1]).toMatchObject({ expectedPolicyVersion: 4 });
});

it("远端来源行消失后仍可从持久清理入口恢复", async () => {
  const remote =
    "http://ally.test/api/v1/knowledge/connections/remote/libraries/kb";
  server.use(
    http.get(`${remote}/source-purges`, () =>
      HttpResponse.json({
        items: [{ id: "purge", state: "pending", version: 1, versions: [] }],
        total: 1,
      }),
    ),
  );
  show({ backend: "custom", connectionId: "remote", knowledgeBaseId: "kb" });
  expect(
    await screen.findByRole("button", { name: "查询并继续清理" }),
  ).toBeEnabled();
});

it("文档菜单不再提供整理过程或聊天入口", async () => {
  fixtures();
  show();
  await userEvent.click(screen.getByRole("button", { name: "操作" }));
  expect(
    screen.queryByRole("menuitem", { name: "整理过程" }),
  ).not.toBeInTheDocument();
});

it.each(["processed", "processing", "failed", "queued", "pending"] as const)(
  "来源处于 %s 时菜单只包含文档操作",
  async (state) => {
    fixtures();
    show(undefined, true, { ...source, state });
    await userEvent.click(screen.getByRole("button", { name: "操作" }));
    expect(
      screen.getAllByRole("menuitem").map((item) => item.textContent),
    ).toEqual(["下载", "重建知识", "删除"]);
  },
);

it.each(["processed", "failed"])(
  "原件预览展示远页文档，%s 不再产生整理通知阅读请求",
  async (state) => {
    const id = "a".repeat(32);
    vi.stubGlobal(
      "IntersectionObserver",
      class {
        constructor(private callback: IntersectionObserverCallback) {}
        observe(target: Element) {
          const rectangle = target.getBoundingClientRect();
          queueMicrotask(() =>
            this.callback(
              [
                {
                  target,
                  time: 0,
                  isIntersecting: true,
                  intersectionRatio: 1,
                  boundingClientRect: rectangle,
                  intersectionRect: rectangle,
                  rootBounds: rectangle,
                },
              ],
              {} as IntersectionObserver,
            ),
          );
        }
        unobserve() {}
        disconnect() {}
      },
    );
    let focused = false;
    const reads: unknown[] = [];
    vi.spyOn(document, "hasFocus").mockImplementation(() => focused);
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue(
      new DOMRect(0, 0, 100, 100),
    );
    const reference = {
      knowledgeLibraryId: "kb",
      knowledgeSourceId: "original",
      knowledgeSourceName: "远页文档.md",
      knowledgeSourceState: state,
    };
    server.use(
      http.get(`${root}/sources/${id}`, () =>
        HttpResponse.json({
          id,
          filename: "远页文档.md",
          state,
          description: state === "processed" ? "整理生成的文档摘要" : "",
          rowVersion: 7,
          diagnostic: "无法解析此文件",
        }),
      ),
      http.get(`${root}/sources/${id}/raw`, () =>
        HttpResponse.json({ bytes: btoa("original"), filename: "远页文档.md" }),
      ),
      http.get(
        `http://ally.test/api/v1/work-board/results/knowledge_source/${id}`,
        () =>
          HttpResponse.json({
            result: {
              ownerKind: "knowledge_source",
              ownerId: id,
              latestVersion: 7,
              viewedVersion: 0,
              reference,
            },
          }),
      ),
      http.post(
        `http://ally.test/api/v1/work-board/results/knowledge_source/${id}/view`,
        async ({ request }) => {
          reads.push(await request.json());
          return HttpResponse.json({});
        },
      ),
      http.get("http://ally.test/api/v1/work-board/summary", () =>
        HttpResponse.json({}),
      ),
    );
    render(
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <TooltipProvider>
          <KnowledgeResultDocument
            resource={emptyLibrary("kb", "库", "general").ref}
            sourceId={id}
            onClose={vi.fn()}
          />
        </TooltipProvider>
      </QueryClientProvider>,
    );
    expect(
      await screen.findByRole("heading", { name: "远页文档.md" }),
    ).toBeVisible();
    expect(screen.queryByText("无法解析此文件")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "整理状态" }),
    ).not.toBeInTheDocument();
    expect(reads).toEqual([]);
    focused = true;
    fireEvent(window, new Event("focus"));
    await act(() => new Promise((resolve) => setTimeout(resolve, 400)));
    expect(reads).toEqual([]);
  },
);
