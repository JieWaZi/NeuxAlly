import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { TooltipProvider } from "@/shared/ui/tooltip";
import { toSource } from "./knowledge-queries";
import { KnowledgeSourceTable } from "./knowledge-source-table";

it("keeps failed diagnostics out of the row and reveals them on status hover", async () => {
  const user = userEvent.setup();
  const diagnostic = "model_generation_failed: upstream service unavailable";
  render(
    <TooltipProvider delayDuration={0}>
      <KnowledgeSourceTable
        rows={[
          {
            id: "source",
            name: "evidence.md",
            size: "1 KB",
            date: "2026/10/20",
            state: "failed",
            diagnostic,
          },
        ]}
        selection={[]}
        select={vi.fn()}
        selectPage={vi.fn()}
        disabled={false}
        pagination={{
          page: 1,
          pageSize: 50,
          totalItems: 1,
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
    </TooltipProvider>,
  );
  expect(screen.getByRole("columnheader", { name: "文件大小" })).toBeVisible();
  expect(screen.getByText("1 KB").closest("td")).not.toBe(
    screen.getByText("evidence.md").closest("td"),
  );
  expect(screen.queryByText(diagnostic)).not.toBeInTheDocument();
  await user.hover(screen.getByText("整理失败"));
  expect(await screen.findByRole("tooltip")).toHaveTextContent(diagnostic);
});

it("权威排队和运行状态分别展示且不可再次选择，未入队和失败仍可整理", () => {
  const rows = ["pending", "queued", "processing", "failed", "cancelled"].map(
    (state) => toSource({ id: state, filename: `${state}.md`, state }),
  );
  expect(rows.map((row) => row.state)).toEqual([
    "pending",
    "queued",
    "processing",
    "failed",
    "cancelled",
  ]);
  render(
    <TooltipProvider>
      <KnowledgeSourceTable
        rows={rows}
        selection={[]}
        select={vi.fn()}
        selectPage={vi.fn()}
        disabled={false}
        pagination={{
          page: 1,
          pageSize: 50,
          totalItems: 5,
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
    </TooltipProvider>,
  );
  expect(screen.getByText("排队中")).toBeVisible();
  expect(screen.getByText("整理中")).toBeVisible();
  expect(
    screen.getByRole("checkbox", { name: "选择 queued.md" }),
  ).toBeDisabled();
  expect(
    screen.getByRole("checkbox", { name: "选择 processing.md" }),
  ).toBeDisabled();
  expect(
    screen.getByRole("checkbox", { name: "选择 pending.md" }),
  ).toBeEnabled();
  expect(
    screen.getByRole("checkbox", { name: "选择 failed.md" }),
  ).toBeEnabled();
});
