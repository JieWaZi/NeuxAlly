import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { TooltipProvider } from "@/shared/ui/tooltip";
import { LibraryList } from "./knowledge-library";
import { emptyLibrary, type Library } from "./knowledge-model";

const library = {
  ...emptyLibrary("library-1", "团队知识", "general"),
  pageTotal: 7,
};

function setup(
  onDelete = vi.fn<() => Promise<void>>().mockResolvedValue(undefined),
) {
  const open = vi.fn();
  render(
    <TooltipProvider>
      <LibraryList
        libraries={[library]}
        open={open}
        create={vi.fn()}
        onDelete={onDelete}
      />
    </TooltipProvider>,
  );
  return { open, onDelete };
}

describe("knowledge library cards", () => {
  it("keeps title tags and Wiki total while opening only from the card body", async () => {
    const user = userEvent.setup();
    const { open } = setup();
    const card = screen.getByRole("article");
    const title = within(card).getByRole("heading", { name: "团队知识" });
    expect(title.parentElement).not.toHaveTextContent("通用");
    expect(title.parentElement).toHaveTextContent("本地");
    expect(within(card).getByText("7 篇 Wiki")).toBeInTheDocument();
    await user.click(within(card).getByRole("button", { name: "团队知识" }));
    expect(open).toHaveBeenCalledWith(library);
  });

  it("requires confirmation before permanently deleting a library and its originals", async () => {
    const user = userEvent.setup();
    const { open, onDelete } = setup();
    await user.click(
      screen.getByRole("button", { name: "团队知识的更多操作" }),
    );
    await user.click(screen.getByRole("menuitem", { name: "永久删除知识库" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("原始资料");
    expect(dialog).toHaveTextContent("无法恢复");
    expect(onDelete).not.toHaveBeenCalled();
    expect(open).not.toHaveBeenCalled();
    await user.click(
      within(dialog).getByRole("button", { name: "永久删除知识库" }),
    );
    expect(onDelete).toHaveBeenCalledWith(library, expect.any(String));
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });

  it("keeps the confirmation open when deletion fails", async () => {
    const user = userEvent.setup();
    setup(vi.fn<() => Promise<void>>().mockRejectedValue(new Error("offline")));
    await user.click(
      screen.getByRole("button", { name: "团队知识的更多操作" }),
    );
    await user.click(screen.getByRole("menuitem", { name: "永久删除知识库" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "永久删除知识库",
      }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "知识库删除失败",
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("omits the official type badge and disables unsupported deletion", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <LibraryList
          libraries={[{ ...library, kind: "official", storage: "remote" }]}
          open={vi.fn()}
          create={vi.fn()}
          onDelete={vi.fn()}
          canDelete={() => false}
        />
      </TooltipProvider>,
    );
    expect(screen.queryByText("官方 Wiki")).not.toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "团队知识的更多操作" }),
    );
    expect(
      screen.getByRole("menuitem", { name: "永久删除知识库" }),
    ).toHaveAttribute("data-disabled");
  });
  it("retains cleanup confirmation after the row disappears and reuses its request on retry", async () => {
    const user = userEvent.setup();
    const onDelete = vi
      .fn<(library: Library, requestId: string) => Promise<void>>()
      .mockRejectedValueOnce(new Error("cleanup pending"))
      .mockResolvedValue(undefined);
    const props = {
      open: vi.fn(),
      create: vi.fn(),
      onDelete,
    };
    const view = render(
      <TooltipProvider>
        <LibraryList {...props} libraries={[library]} />
      </TooltipProvider>,
    );
    await user.click(
      screen.getByRole("button", { name: "团队知识的更多操作" }),
    );
    await user.click(screen.getByRole("menuitem", { name: "永久删除知识库" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "永久删除知识库",
      }),
    );
    await screen.findByRole("alert");
    view.rerender(
      <TooltipProvider>
        <LibraryList {...props} libraries={[]} />
      </TooltipProvider>,
    );
    expect(screen.getByRole("dialog")).toHaveTextContent("原始资料");
    const requestId = onDelete.mock.calls[0]?.[1];
    expect(requestId).toEqual(expect.any(String));
    await user.click(screen.getByRole("button", { name: "查询并继续清理" }));
    expect(onDelete).toHaveBeenLastCalledWith(library, requestId);
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });
  it("identifies a remote Wiki count as the latest observed snapshot", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider delayDuration={0}>
        <LibraryList
          libraries={[
            {
              ...library,
              storage: "remote",
              pageTotalObservedAt: 1792411200000,
            },
          ]}
          open={vi.fn()}
          create={vi.fn()}
        />
      </TooltipProvider>,
    );
    await user.hover(screen.getByText("7 篇 Wiki"));
    expect(await screen.findByRole("tooltip")).toHaveTextContent("最近读取");
    expect(screen.getByRole("tooltip")).toHaveTextContent("2026");
  });
});

it("加载时保留操作和搜索，并在卡片区域显示骨架", () => {
  render(
    <TooltipProvider>
      <LibraryList
        libraries={[]}
        state="loading"
        open={vi.fn()}
        create={vi.fn()}
      />
    </TooltipProvider>,
  );
  expect(screen.getByRole("button", { name: "新建知识库" })).toBeEnabled();
  expect(screen.getByRole("status")).toHaveAttribute("aria-busy", "true");
  expect(screen.queryByText("没有找到知识库")).not.toBeInTheDocument();
});
