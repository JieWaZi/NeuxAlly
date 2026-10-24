import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import "@/shared/i18n";

import { SkillCard } from "./skill-card";

const skill = {
  id: "code-review",
  name: "代码审查",
  description: "检查当前改动。",
  version: "1.0.0",
  scope: "built_in" as const,
  source: "ally" as const,
  enabled: true,
  affectedWakerCount: 1,
  updatedAt: "2026-08-05T08:00:00Z",
};

describe("SkillCard", () => {
  it("separates detail navigation from the partner enable switch", async () => {
    const user = userEvent.setup();
    const onEnabledChange = vi.fn();

    render(
      <MemoryRouter>
        <SkillCard
          skill={skill}
          wakerId="waker-1"
          onEnabledChange={onEnabledChange}
        />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole("link", { name: "查看技能 代码审查" }),
    ).toHaveAttribute("href", "/wakers/waker-1/skills/code-review");

    await user.click(screen.getByRole("switch", { name: "停用 代码审查" }));
    expect(onEnabledChange).toHaveBeenCalledWith(false);
    expect(
      screen.queryByRole("button", { name: "卸载技能 代码审查" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: "选择技能 代码审查" }),
    ).not.toBeInTheDocument();
  });

  it("exposes selection and uninstall actions outside the detail link", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    const onSelectedChange = vi.fn();

    render(
      <MemoryRouter>
        <SkillCard
          skill={{ ...skill, scope: "public", source: "git" }}
          wakerId="waker-1"
          onDelete={onDelete}
          onEnabledChange={vi.fn()}
          onSelectedChange={onSelectedChange}
        />
      </MemoryRouter>,
    );

    await user.click(
      screen.getByRole("checkbox", { name: "选择技能 代码审查" }),
    );
    expect(onSelectedChange).toHaveBeenCalledWith(true);

    await user.click(screen.getByRole("button", { name: "卸载技能 代码审查" }));
    expect(onDelete).toHaveBeenCalledTimes(1);
  });
});
