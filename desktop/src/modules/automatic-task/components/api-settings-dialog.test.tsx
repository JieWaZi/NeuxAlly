import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, expect, it, vi } from "vitest";
import type * as SDK from "@/shared/api/generated/sdk.gen";
import "@/shared/i18n";
import { AutomaticTaskAPISettingsDialog } from "./api-settings-dialog";
import { apiExamples } from "./task-token-dialog";

const { getAPI, setAPI } = vi.hoisted(() => ({
  getAPI: vi.fn(),
  setAPI: vi.fn(),
}));
vi.mock("@/shared/api/generated/sdk.gen", async (original) => ({
  ...(await original<typeof SDK>()),
  getAutomaticTaskApi: getAPI,
  setAutomaticTaskApi: setAPI,
}));

beforeEach(() => {
  getAPI.mockReset().mockResolvedValue({
    data: { ip: "127.0.0.1", port: 8431, available: true },
  });
  setAPI.mockReset().mockResolvedValue({ data: {} });
});

function renderSettings() {
  function Harness() {
    const [open, setOpen] = useState(true);
    return (
      <>
        <button onClick={() => setOpen(true)}>重新打开</button>
        <AutomaticTaskAPISettingsDialog open={open} onOpenChange={setOpen} />
      </>
    );
  }
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <Harness />
    </QueryClientProvider>,
  );
}

it("确认才提交 IP 和端口，刷新服务器设置并关闭弹窗", async () => {
  const user = userEvent.setup();
  renderSettings();
  const ip = screen.getByLabelText(/IP 地址/);
  await waitFor(() => expect(ip).toHaveValue("127.0.0.1"));
  await user.clear(ip);
  await user.type(ip, "192.168.1.20");
  const port = screen.getByLabelText(/端口/);
  await user.clear(port);
  await user.type(port, "18431");
  expect(setAPI).not.toHaveBeenCalled();
  getAPI.mockResolvedValue({
    data: { ip: "192.168.1.20", port: 18431, available: true },
  });
  await user.click(screen.getByRole("button", { name: "确认" }));
  await waitFor(() => expect(setAPI).toHaveBeenCalledOnce());
  expect(setAPI.mock.calls[0]?.[0]).toMatchObject({
    body: { ip: "192.168.1.20", port: 18431 },
  });
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  await user.click(screen.getByRole("button", { name: "重新打开" }));
  await waitFor(() =>
    expect(screen.getByLabelText(/IP 地址/)).toHaveValue("192.168.1.20"),
  );
  expect(screen.getByLabelText(/端口/)).toHaveValue(18431);
});

it("取消不提交，重新打开恢复已保存设置；端口越界不能确认", async () => {
  const user = userEvent.setup();
  renderSettings();
  await waitFor(() =>
    expect(screen.getByLabelText(/IP 地址/)).toHaveValue("127.0.0.1"),
  );
  await user.clear(screen.getByLabelText(/IP 地址/));
  await user.type(screen.getByLabelText(/IP 地址/), "::1");
  await user.clear(screen.getByLabelText(/端口/));
  await user.type(screen.getByLabelText(/端口/), "65536");
  expect(screen.getByRole("button", { name: "确认" })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "取消" }));
  expect(setAPI).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "重新打开" }));
  await waitFor(() =>
    expect(screen.getByLabelText(/IP 地址/)).toHaveValue("127.0.0.1"),
  );
  expect(screen.getByLabelText(/端口/)).toHaveValue(8431);
});

it("确认失败保留草稿和弹窗，允许修正后重试", async () => {
  const user = userEvent.setup();
  renderSettings();
  await waitFor(() =>
    expect(screen.getByLabelText(/IP 地址/)).toHaveValue("127.0.0.1"),
  );
  await user.clear(screen.getByLabelText(/端口/));
  await user.type(screen.getByLabelText(/端口/), "18431");
  setAPI.mockRejectedValueOnce(new Error("occupied"));
  await user.click(screen.getByRole("button", { name: "确认" }));
  expect(await screen.findByRole("alert")).toBeVisible();
  expect(screen.getByRole("dialog")).toBeVisible();
  expect(screen.getByLabelText(/端口/)).toHaveValue(18431);
  await user.click(screen.getByRole("button", { name: "确认" }));
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(setAPI).toHaveBeenCalledTimes(2);
});

it("调用示例使用配置地址，IPv6 有方括号，通配地址使用本机回环示例", () => {
  expect(apiExamples("task", 8431, undefined, "192.168.1.20").endpoint).toBe(
    "http://192.168.1.20:8431/v1/automatic-tasks/task/runs",
  );
  expect(apiExamples("task", 8431, undefined, "::1").endpoint).toBe(
    "http://[::1]:8431/v1/automatic-tasks/task/runs",
  );
  expect(apiExamples("task", 8431, undefined, "0.0.0.0").endpoint).toContain(
    "http://127.0.0.1:8431/",
  );
  expect(apiExamples("task", 8431, undefined, "::").endpoint).toContain(
    "http://[::1]:8431/",
  );
});
