import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";

import { SegmentedControl } from "./segmented-control";

it("SegmentedControl 使用统一轨道样式并支持键盘切换", async () => {
  const user = userEvent.setup();
  const onValueChange = vi.fn();
  render(
    <SegmentedControl
      ariaLabel="实体模式"
      distribution="fill"
      onValueChange={onValueChange}
      options={[
        { label: "伙伴", value: "waker" },
        { label: "团队", value: "team" },
      ]}
      size="sm"
      value="waker"
    />,
  );

  const group = screen.getByRole("radiogroup", { name: "实体模式" });
  expect(group).toHaveClass("bg-button-fill", "rounded-control");
  expect(group).not.toHaveClass("border");

  const waker = screen.getByRole("radio", { name: "伙伴" });
  const team = screen.getByRole("radio", { name: "团队" });
  expect(waker).toHaveAttribute("aria-checked", "true");
  expect(waker).toHaveClass(
    "bg-surface-raised",
    "rounded-control-inner",
    "font-medium",
  );
  expect(waker).not.toHaveClass("shadow-xs");
  expect(team).toHaveAttribute("aria-checked", "false");
  expect(team).toHaveClass("text-button-content");

  waker.focus();
  await user.keyboard("{ArrowRight}");
  expect(onValueChange).toHaveBeenCalledWith("team");
});

it("卡片选项保留简短名称与说明，键盘切换跳过不可用项", async () => {
  const user = userEvent.setup();
  const onValueChange = vi.fn();
  render(
    <SegmentedControl
      appearance="cards"
      ariaLabel="触发方式"
      onValueChange={onValueChange}
      options={[
        { label: "定时", value: "schedule", description: "按计划自动运行" },
        { label: "事件", value: "event", disabled: true },
        { label: "本机 API", value: "api", description: "收到本机请求时运行" },
      ]}
      value="schedule"
    />,
  );
  const scheduled = screen.getByRole("radio", { name: "定时" });
  expect(scheduled).toHaveAccessibleDescription("按计划自动运行");
  scheduled.focus();
  await user.keyboard("{ArrowRight}");
  expect(onValueChange).toHaveBeenCalledWith("api");
  expect(screen.getByRole("radio", { name: "本机 API" })).toHaveFocus();
});
