import { render } from "@testing-library/react";
import { expect, it } from "vitest";

import { RuntimeCliIcon } from "./runtime-cli-icon";

it("为 Claude 与 Codex 渲染品牌图标", () => {
  const { container: claudeContainer } = render(
    <RuntimeCliIcon runtimeId="claude" />,
  );
  const { container: codexContainer } = render(
    <RuntimeCliIcon runtimeId="codex" />,
  );

  expect(claudeContainer.querySelector("svg")).toBeTruthy();
  expect(codexContainer.querySelector("svg")).toBeTruthy();
});

it("未知 runtime 回退到 Bot 图标", () => {
  const { container } = render(<RuntimeCliIcon runtimeId="unknown" />);

  expect(container.querySelector("svg")).toHaveClass("size-icon-md");
});

it.each(["gemini", "opencode", "grok"])("为 %s 渲染品牌图标", (runtimeId) => {
  const { container } = render(<RuntimeCliIcon runtimeId={runtimeId} />);

  expect(container.querySelector("svg")).toBeTruthy();
  expect(container.querySelector("svg")).not.toHaveClass("size-icon-md");
});
