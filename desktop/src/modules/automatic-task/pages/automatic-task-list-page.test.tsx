import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useNavigate } from "react-router-dom";
import { beforeEach, expect, it, vi } from "vitest";

import type * as SDK from "@/shared/api/generated/sdk.gen";
import "@/shared/i18n";
import { AutomaticTaskListPage } from "./automatic-task-list-page";
import { AutomaticTaskDetailPage } from "./automatic-task-detail-page";

const { toggle, runError, taskKind } = vi.hoisted(() => ({
  toggle: vi.fn(),
  runError: { code: "" },
  taskKind: { value: "workflow" },
}));
vi.mock("@/shared/api/generated/sdk.gen", async (original) => ({
  ...(await original<typeof SDK>()),
  listGlobalAutomaticTasks: async () => ({
    data: {
      resources: [
        {
          id: "task-1",
          wakerId: "waker-1",
          executorName: "私人助理",
          name: "每日归档",
          trigger: "api",
          enabled: false,
          revision: 3,
        },
      ],
      total: 1,
      counts: { total: 1, enabled: 0, scheduled: 0, api: 1 },
    },
  }),
  getAutomaticTaskApi: async () => ({
    data: { ip: "127.0.0.1", port: 8431, available: true },
  }),
  getGlobalAutomaticTask: async () => ({
    data:
      taskKind.value === "partner"
        ? {
            id: "workflow-task",
            wakerId: "waker-1",
            executorName: "私人助理",
            name: "每日归档",
            trigger: "schedule",
            enabled: true,
            instruction: "整理项目文件",
            runtimeId: "codex",
            accountId: "system",
            model: "gpt-6.1-sol",
            permissionLevel: "default",
            workingDirectory: "/Users/example/Projects/需要完整展示的工作目录",
            schedule: {
              kind: "daily",
              hour: 9,
              minute: 0,
              timezone: "Asia/Shanghai",
            },
          }
        : {
            id: "workflow-task",
            wakerId: "waker-1",
            name: "流程验收",
            executorName: "纯计算验收",
            workflowId: "flow-1",
            workflowRevisionPolicy: "current",
            trigger: "api",
            enabled: false,
          },
  }),
  getWorkflow: async () => ({
    data: { definition: { id: "flow-1", title: "纯计算验收" } },
  }),
  listAutomaticRuns: async () => ({
    data: {
      totalSize: 1,
      resources: [
        {
          id: "occurrence-1",
          taskId: "workflow-task",
          wakerId: "waker-1",
          workflowRunId: "frozen-run",
          configuration: { workflowId: "flow-1" },
          source: "manual",
          state: "completed",
          errorCode: runError.code,
        },
      ],
    },
  }),
  listWakers: async () => ({
    data: {
      resources: [
        { id: "waker-1", name: "私人助理" },
        { id: "waker-2", name: "资料助手" },
      ],
      totalSize: 2,
    },
  }),
  listChannels: async () => ({ data: { resources: [], totalSize: 0 } }),
  listRuntimes: async () => ({ data: { resources: [] } }),
  listAiAccounts: async () => ({ data: { resources: [] } }),
  listProjects: async () => ({ data: { resources: [] } }),
  previewAutomaticSchedule: async () => ({
    data: { nextRuns: [], runsIn24Hours: 0 },
  }),
  setAutomaticTaskEnabled: toggle,
}));

beforeEach(() => {
  toggle.mockReset().mockResolvedValue({ data: {} });
  runError.code = "";
  taskKind.value = "workflow";
});

function renderWorkflowTask() {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <MemoryRouter initialEntries={["/automatic-tasks/workflow-task"]}>
        <Routes>
          <Route
            path="/automatic-tasks/:taskId"
            element={<AutomaticTaskDetailPage />}
          />
          <Route
            path="/workflows/flow-1/runs/frozen-run"
            element={<h1>已打开固定版本运行</h1>}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

it("工作流详情保留版本和参数，不展示伙伴专用配置或重复启停状态", async () => {
  renderWorkflowTask();
  expect(
    await screen.findByRole("link", { name: "纯计算验收" }),
  ).toHaveAttribute("href", "/workflows/flow-1");
  expect(screen.getAllByText("已停用")).toHaveLength(1);
  expect(screen.getByText("运行版本")).toBeVisible();
  expect(screen.getByText("工作流输入参数")).toBeVisible();
  for (const label of ["智能体", "账号", "模型", "推理强度", "执行权限"]) {
    expect(screen.queryByText(label)).not.toBeInTheDocument();
  }
  expect(screen.getByRole("button", { name: "编辑任务" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "删除任务" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "立即运行" })).toBeDisabled();
});

it("触发配置中的轮换密钥操作保留确认弹窗", async () => {
  const user = userEvent.setup();
  renderWorkflowTask();
  const rotate = await screen.findByRole("button", { name: "重新生成密钥" });
  await user.click(rotate);
  expect(
    await screen.findByRole("dialog", { name: "重新生成任务密钥" }),
  ).toBeVisible();
  await user.click(screen.getByRole("button", { name: "取消" }));
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
});

it("定时伙伴任务保留完整工作范围与实际智能体配置", async () => {
  taskKind.value = "partner";
  renderWorkflowTask();
  await screen.findByRole("heading", { name: "每日归档" });
  expect(screen.getByText("整理项目文件")).toBeVisible();
  expect(
    screen.getByText("/Users/example/Projects/需要完整展示的工作目录"),
  ).toBeVisible();
  expect(screen.getByText("Codex", { selector: "span" })).toBeVisible();
  expect(screen.getByText("gpt-6.1-sol")).toBeVisible();
  expect(screen.getByText("执行权限")).toBeVisible();
  expect(screen.getByText("Asia/Shanghai")).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "重新生成密钥" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "立即运行" })).toBeEnabled();
});

it("工作流自动任务没有 Chat 时仍能打开实际 Run", async () => {
  const user = userEvent.setup();
  renderWorkflowTask();
  const link = await screen.findByRole("link", { name: "查看运行" });
  expect(link).toHaveAttribute("href", "/workflows/flow-1/runs/frozen-run");
  await user.click(link);
  expect(
    await screen.findByRole("heading", { name: "已打开固定版本运行" }),
  ).toBeVisible();
});

it("工作流运行已缺失时不显示失效入口", async () => {
  runError.code = "WorkflowNotFound";
  renderWorkflowTask();
  await screen.findByRole("row", { name: /立即运行.*已完成/ });
  expect(
    screen.queryByRole("link", { name: "查看运行" }),
  ).not.toBeInTheDocument();
});

function renderList() {
  function CreateForPartner() {
    const navigate = useNavigate();
    return (
      <>
        <button
          onClick={() => navigate("/automatic-tasks?create=1&wakerId=waker-1")}
        >
          为私人助理创建
        </button>
        <button
          onClick={() => navigate("/automatic-tasks?create=1&wakerId=waker-2")}
        >
          为资料助手创建
        </button>
      </>
    );
  }
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <MemoryRouter initialEntries={["/automatic-tasks"]}>
        <CreateForPartner />
        <Routes>
          <Route path="/automatic-tasks" element={<AutomaticTaskListPage />} />
          <Route path="/automatic-tasks/:taskId" element={<h1>任务详情</h1>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

it("名称普通展示；点击行内其他单元格进入详情", async () => {
  const user = userEvent.setup();
  renderList();
  const row = await screen.findByRole("row", { name: /每日归档/ });
  expect(
    screen.queryByRole("link", { name: "每日归档" }),
  ).not.toBeInTheDocument();
  await user.click(within(row).getByRole("cell", { name: "API" }));
  expect(
    await screen.findByRole("heading", { name: "任务详情" }),
  ).toBeVisible();
});

it("已打开列表接收伙伴创建链接后打开表单，关闭后可以为另一伙伴重新打开", async () => {
  const user = userEvent.setup();
  renderList();
  await screen.findByRole("row", { name: /每日归档/ });
  await user.click(screen.getByRole("button", { name: "为私人助理创建" }));
  const first = await screen.findByRole("dialog", { name: "新建自动任务" });
  await waitFor(() =>
    expect(
      within(first).getByRole("combobox", { name: "执行者" }),
    ).toHaveTextContent("私人助理"),
  );
  await user.click(within(first).getByRole("button", { name: "取消" }));
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  await user.click(screen.getByRole("button", { name: "为资料助手创建" }));
  const second = await screen.findByRole("dialog", { name: "新建自动任务" });
  await waitFor(() =>
    expect(
      within(second).getByRole("combobox", { name: "执行者" }),
    ).toHaveTextContent("资料助手"),
  );
});

it("启停开关操作留在列表；聚焦行后用 Enter 进入详情", async () => {
  const user = userEvent.setup();
  renderList();
  const row = await screen.findByRole("row", { name: /每日归档/ });
  await user.click(screen.getByRole("switch", { name: "启停 每日归档" }));
  await waitFor(() => expect(toggle).toHaveBeenCalledTimes(1));
  expect(toggle.mock.calls[0]?.[0]).toMatchObject({
    path: { wakerId: "waker-1", taskId: "task-1" },
    body: { enabled: true, revision: 3 },
  });
  expect(
    screen.queryByRole("heading", { name: "任务详情" }),
  ).not.toBeInTheDocument();
  row.focus();
  await user.keyboard("{Enter}");
  expect(
    await screen.findByRole("heading", { name: "任务详情" }),
  ).toBeVisible();
});

it("主页右上角设置打开 API 地址弹窗", async () => {
  const user = userEvent.setup();
  renderList();
  await user.click(screen.getByRole("button", { name: "设置" }));
  expect(
    await screen.findByRole("dialog", { name: "自动任务 API 设置" }),
  ).toBeVisible();
  await waitFor(() =>
    expect(screen.getByLabelText(/IP 地址/)).toHaveValue("127.0.0.1"),
  );
  expect(screen.getByLabelText(/端口/)).toHaveValue(8431);
});

vi.mock("@/shared/api/global-events", () => ({
  registerCoreConnectionRecovery: () => () => {},
  subscribeGlobalEvents: () => () => {},
}));
