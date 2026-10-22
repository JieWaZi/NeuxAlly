import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { TooltipProvider } from "@/shared/ui/tooltip";
import { LibraryList } from "./knowledge-library";
import { emptyLibrary } from "./knowledge-model";

it("keeps cached libraries and search available after refresh fails with a retry", async () => {
  const retry = vi.fn();
  render(
    <TooltipProvider>
      <LibraryList
        libraries={[emptyLibrary("cached", "已缓存知识库", "general")]}
        open={vi.fn()}
        create={vi.fn()}
        state="error"
        error={new Error("refresh failed")}
        onRetry={retry}
      />
    </TooltipProvider>,
  );
  expect(screen.getByRole("heading", { name: "已缓存知识库" })).toBeVisible();
  expect(screen.getByRole("alert")).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "重试" }));
  expect(retry).toHaveBeenCalledOnce();
  expect(screen.getByRole("searchbox")).toBeEnabled();
  await userEvent.type(screen.getByRole("searchbox"), "没有匹配");
  expect(screen.getAllByRole("alert")).toHaveLength(1);
  expect(screen.getAllByRole("button", { name: "重试" })).toHaveLength(1);
});

it("shows retry inside the data region while keeping the library shell mounted", async () => {
  const retry = vi.fn();
  render(
    <TooltipProvider>
      <LibraryList
        libraries={[]}
        open={vi.fn()}
        create={vi.fn()}
        state="error"
        error={new Error("offline")}
        onRetry={retry}
      />
    </TooltipProvider>,
  );
  expect(screen.getByRole("heading", { name: "知识库" })).toBeVisible();
  expect(screen.getByRole("searchbox")).toBeEnabled();
  expect(screen.getByRole("alert")).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "重试" }));
  expect(retry).toHaveBeenCalledOnce();
});

it("后台刷新不插入提示或改变内容区域结构", async () => {
  const { KnowledgeReadFeedback } = await import("./knowledge-read-feedback");
  const view = render(
    <KnowledgeReadFeedback
      hasData
      state={{
        pending: false,
        error: undefined,
        refreshing: true,
        retry: vi.fn(),
      }}
    />,
  );
  expect(view.container).toBeEmptyDOMElement();
});
