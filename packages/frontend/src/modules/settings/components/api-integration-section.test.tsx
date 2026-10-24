import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterAll, afterEach, beforeAll, expect, it } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { client } from "@/shared/api/generated/client.gen";
import { APIIntegrationSection } from "./api-integration-section";
const origin = "http://integration-scene.test";
const server = setupServer(
  http.get(`${origin}/api/v1/integrations/service`, () =>
    HttpResponse.json({
      enabled: true,
      available: true,
      ip: "127.0.0.1",
      port: 8431,
      publicBaseUrl: "https://example.test/proxy/api/v1",
    }),
  ),
  http.get(`${origin}/api/v1/integrations/keys`, () =>
    HttpResponse.json({ resources: [], totalSize: 0 }),
  ),
);
beforeAll(() => {
  client.setConfig({ baseUrl: origin });
  server.listen({ onUnhandledRequest: "error" });
});
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
function show() {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <MemoryRouter>
        <APIIntegrationSection />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}
it("服务状态和客户端地址清晰展示，不再提供技术文档区", async () => {
  show();
  expect(screen.getByRole("heading", { name: "API 集成" })).toBeVisible();
  expect(screen.getByRole("button", { name: "创建 API Key" })).toBeVisible();
  const overview = await screen.findByRole("status", {
    name: "API 服务状态",
  });
  expect(within(overview).queryByText("API 服务可用")).not.toBeInTheDocument();
  expect(within(overview).getByText("运行中")).toBeVisible();
  expect(
    within(overview).getByText("https://example.test/proxy/api/v1"),
  ).toBeVisible();
  expect(
    within(overview).getByRole("button", { name: "复制访问地址" }),
  ).toBeVisible();
  expect(screen.getByRole("button", { name: "连接设置" })).toHaveAttribute(
    "aria-expanded",
    "false",
  );
  expect(screen.queryByLabelText("端口")).not.toBeInTheDocument();
  expect(
    screen.queryByRole("heading", { name: "API 文档" }),
  ).not.toBeInTheDocument();
  expect(screen.queryByText("查看接入信息")).not.toBeInTheDocument();
  expect(await screen.findByText("尚未创建 API Key。")).toBeVisible();
});
it("监听切换失败保留有效地址与草稿，并可就近重试", async () => {
  server.use(
    http.put(`${origin}/api/v1/integrations/service`, () =>
      HttpResponse.json({ message: "occupied" }, { status: 503 }),
    ),
  );
  const user = userEvent.setup();
  show();
  await user.click(await screen.findByRole("button", { name: "连接设置" }));
  const port = await screen.findByLabelText("端口");
  await user.clear(port);
  await user.type(port, "18431");
  await user.click(screen.getByRole("button", { name: "保存" }));
  expect(await screen.findByRole("alert")).toBeVisible();
  expect(port).toHaveValue(18431);
  await user.click(screen.getByRole("button", { name: "连接设置" }));
  await user.click(screen.getByRole("button", { name: "连接设置" }));
  expect(screen.getByLabelText("端口")).toHaveValue(18431);
  const overview = screen.getByRole("status", { name: "API 服务状态" });
  expect(
    within(overview).getByText("https://example.test/proxy/api/v1"),
  ).toBeVisible();
});

it("密钥只提供修改和删除，删除后列表移除该密钥", async () => {
  let deleted = false;
  server.use(
    http.get(`${origin}/api/v1/integrations/keys`, () =>
      HttpResponse.json({
        resources: deleted
          ? []
          : [
              {
                id: "key-a",
                name: "团队检索",
                displayPrefix: "sk-test",
                enabled: true,
                revision: 3,
                grants: [
                  {
                    resourceKind: "knowledge",
                    resourceIds: ["kb-a"],
                    capabilities: ["retrieve"],
                  },
                ],
              },
            ],
        totalSize: deleted ? 0 : 1,
      }),
    ),
    http.get(`${origin}/api/v1/integrations/resources`, () =>
      HttpResponse.json({
        resources: [{ id: "kb-a", name: "团队知识库" }],
        totalSize: 1,
      }),
    ),
    http.delete(
      `${origin}/api/v1/integrations/keys/key-a`,
      async ({ request }) => {
        expect(await request.json()).toEqual({ expectedRevision: 3 });
        deleted = true;
        return HttpResponse.json({});
      },
    ),
  );
  const user = userEvent.setup();
  show();
  expect(await screen.findByText("团队检索")).toBeVisible();
  expect(screen.queryByText("团队知识库")).not.toBeInTheDocument();
  expect(screen.getByRole("table", { name: "访问密钥" })).toBeVisible();
  expect(screen.getByRole("button", { name: "修改" })).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "启用" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "撤销" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "轮换" }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "删除" }));
  await user.click(await screen.findByRole("button", { name: "确认删除" }));
  expect(await screen.findByText("尚未创建 API Key。")).toBeVisible();
});
