import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";
import { client } from "@/shared/api/generated/client.gen";
import { TooltipProvider } from "@/shared/ui/tooltip";
import { remarkWikiLinks } from "./knowledge-wiki-markdown";
import { KnowledgeWikiView } from "./knowledge-wiki-view";

const server = setupServer();
const root = "http://ally.test/api/v1/knowledge/libraries";
beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
  client.setConfig({ baseUrl: "http://ally.test" });
});
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

it.each(["ready", "switch", "failure"] as const)(
  "索引链接等待同库完整metadata，结果=%s",
  async (outcome) => {
    let release!: () => void;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    let failed = outcome === "failure";
    let completed = false;
    server.use(
      http.get(`${root}/first/pages`, async () => {
        await gate;
        completed = true;
        return failed
          ? new HttpResponse(null, { status: 500 })
          : HttpResponse.json({
              items: [
                {
                  id: "one",
                  slug: "concept/one",
                  title: "旧知识",
                  pageType: "concept",
                },
              ],
              total: 1,
            });
      }),
      http.get(`${root}/:libraryId/themes`, () =>
        HttpResponse.json({ items: [] }),
      ),
      http.get(`${root}/:libraryId/wiki/index`, ({ params }) =>
        HttpResponse.json({
          intro: params.libraryId === "first" ? "# 原库" : "# 新库",
          groups:
            params.libraryId === "first"
              ? [
                  {
                    type: "concept",
                    total: 1,
                    items: [{ slug: "concept/one", title: "旧知识" }],
                  },
                ]
              : [],
        }),
      ),
      http.get(`${root}/second/pages`, () =>
        HttpResponse.json({ items: [], total: 0 }),
      ),
    );
    const onPage = vi.fn();
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const ui = (id: string) => (
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>
          <KnowledgeWikiView
            onMaintenance={() => {}}
            resource={{
              backend: "local",
              connectionId: "local",
              knowledgeBaseId: id,
            }}
            pageId=""
            onPage={onPage}
            writable={false}
            structureVersion={1}
          />
        </TooltipProvider>
      </QueryClientProvider>
    );
    const view = render(ui("first"));
    await userEvent.click(
      await screen.findByRole("link", { name: "旧知识" }, { timeout: 5000 }),
    );
    expect(
      screen.queryByText("未找到链接指向的 Wiki 页面"),
    ).not.toBeInTheDocument();
    if (outcome === "switch") view.rerender(ui("second"));
    release();
    await waitFor(() => expect(completed).toBe(true));
    if (outcome === "ready")
      await waitFor(() =>
        expect(onPage).toHaveBeenCalledExactlyOnceWith("one"),
      );
    if (outcome === "switch") {
      await screen.findByRole("heading", { level: 1, name: "Wiki Index" });
      await waitFor(() => expect(queryClient.isFetching()).toBe(0));
      expect(onPage).not.toHaveBeenCalled();
    }
    if (outcome === "failure") {
      const sidebar = screen.getByRole("complementary", { name: "Wiki 目录" });
      const retry = await within(sidebar).findByRole("button", {
        name: "重试",
      });
      expect(onPage).not.toHaveBeenCalled();
      expect(
        screen.queryByText("未找到链接指向的 Wiki 页面"),
      ).not.toBeInTheDocument();
      failed = false;
      await userEvent.click(retry);
      await screen.findByRole("treeitem", { name: "旧知识" });
      await userEvent.click(screen.getByRole("link", { name: "旧知识" }));
      expect(onPage).toHaveBeenCalledExactlyOnceWith("one");
    }
  },
);

it("索引生成的Markdown继续复用链接净化，正文HTML不执行", async () => {
  server.use(
    http.get(`${root}/first/pages`, () =>
      HttpResponse.json({ items: [], total: 0 }),
    ),
    http.get(`${root}/first/themes`, () => HttpResponse.json({ items: [] })),
    http.get(`${root}/first/wiki/index`, () =>
      HttpResponse.json({
        intro: "# 安全索引",
        groups: [
          {
            type: "concept",
            total: 1,
            items: [
              {
                slug: "concept/safe",
                title: "正常知识",
                summary:
                  '<img src="https://example.com/raw.png" onerror="alert(1)"><script>alert(1)</script> [危险链接](javascript:alert(1))',
              },
            ],
          },
        ],
      }),
    ),
  );
  const view = render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <TooltipProvider>
        <KnowledgeWikiView
          onMaintenance={() => {}}
          resource={{
            backend: "local",
            connectionId: "local",
            knowledgeBaseId: "first",
          }}
          pageId=""
          onPage={vi.fn()}
          writable={false}
          structureVersion={1}
        />
      </TooltipProvider>
    </QueryClientProvider>,
  );
  expect(
    await screen.findByRole("link", { name: "正常知识" }, { timeout: 5000 }),
  ).toHaveAttribute("href", "./concept/safe.md");
  expect(
    view.container.querySelector('img,script,iframe,a[href^="javascript:"]'),
  ).toBeNull();
});

it.each([
  "# 旧模型标题\n\n原有范围。",
  "旧模型标题\n=====\n\n原有范围。",
  "原有范围。",
  "",
])(
  "旧索引和无标题导言固定一级标题，刷新继续更新概述与目录：%s",
  async (intro) => {
    let updated = false;
    server.use(
      http.get(`${root}/first/wiki/index`, () =>
        HttpResponse.json({
          intro: updated ? "# 新模型标题\n\n新增文档范围。" : intro,
          groups: [
            {
              type: "concept",
              total: 1,
              items: [
                {
                  slug: updated ? "concept/new" : "concept/old",
                  title: updated ? "新增知识" : "原有知识",
                },
              ],
            },
          ],
        }),
      ),
    );
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const { KnowledgeWikiStructuredIndex } =
      await import("./knowledge-wiki-index");
    render(
      <QueryClientProvider client={queryClient}>
        <KnowledgeWikiStructuredIndex
          resource={{
            backend: "local",
            connectionId: "local",
            knowledgeBaseId: "first",
          }}
          active
          onLink={vi.fn()}
          remarkPlugins={[remarkWikiLinks]}
        />
      </QueryClientProvider>,
    );
    expect(
      await screen.findByRole("heading", { level: 1, name: "Wiki Index" }),
    ).toBeVisible();
    expect(
      screen.queryByRole("heading", { name: "旧模型标题" }),
    ).not.toBeInTheDocument();
    if (intro) expect(screen.getByText("原有范围。")).toBeVisible();
    expect(screen.getByRole("link", { name: "原有知识" })).toBeVisible();
    updated = true;
    await queryClient.invalidateQueries();
    expect(await screen.findByText("新增文档范围。")).toBeVisible();
    expect(
      screen.getAllByRole("heading", { level: 1, name: "Wiki Index" }),
    ).toHaveLength(1);
    expect(
      screen.queryByRole("heading", { name: "新模型标题" }),
    ).not.toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "新增知识" })).toBeVisible();
    expect(
      screen.queryByRole("link", { name: "原有知识" }),
    ).not.toBeInTheDocument();
  },
);
