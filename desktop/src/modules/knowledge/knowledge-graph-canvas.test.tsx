import type * as GraphLayout from "./knowledge-graph-layout";
import { Profiler } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { TooltipProvider } from "@/shared/ui/tooltip";
import { KnowledgeGraphCanvas } from "./knowledge-graph-canvas";
import {
  graphClusterOutline,
  visibleGraphLabels,
} from "./knowledge-graph-layout";

vi.mock("./knowledge-graph-layout", async (load) => {
  const actual = await load<typeof GraphLayout>();
  return {
    ...actual,
    graphClusterOutline: vi.fn(actual.graphClusterOutline),
    visibleGraphLabels: vi.fn(actual.visibleGraphLabels),
  };
});

beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(1000);
  vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(600);
});
afterEach(() => {
  vi.restoreAllMocks();
});

it("悬停与平移不重新计算未变化的簇轮廓", () => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  const frame = vi.spyOn(window, "requestAnimationFrame").mockReturnValue(0);
  const nodes = Array.from({ length: 400 }, (_, i) => ({
    slug: `node-${i}`,
    title: `节点 ${i}`,
    page_type: "concept",
    link_count: 2,
  }));
  const view = render(
    <TooltipProvider>
      <KnowledgeGraphCanvas
        graph={{
          nodes,
          edges: nodes.map((node, i) => ({
            source: node.slug,
            target: `node-${(i + 1) % nodes.length}`,
          })),
          meta: {
            mode: "overview",
            total: 400,
            returned: 400,
            truncated: false,
          },
        }}
        selected=""
        onSelect={() => {}}
      />
    </TooltipProvider>,
  );
  try {
    const initialCalls = vi.mocked(graphClusterOutline).mock.calls.length;
    expect(initialCalls).toBeGreaterThan(0);
    fireEvent.mouseEnter(screen.getByRole("button", { name: /^节点 0$/ }));
    fireEvent.keyDown(screen.getByRole("group"), { key: "ArrowLeft" });
    expect(vi.mocked(graphClusterOutline).mock.calls.length).toBe(initialCalls);
  } finally {
    view.unmount();
    frame.mockRestore();
    vi.unstubAllGlobals();
  }
});

it.each([400, 1500])(
  "%i 节点布局帧直接更新 SVG，标签每六帧刷新且不触发 React 提交",
  (count) => {
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        disconnect() {}
      },
    );
    const frames: FrameRequestCallback[] = [];
    const request = vi
      .spyOn(window, "requestAnimationFrame")
      .mockImplementation((callback) => {
        frames.push(callback);
        return frames.length;
      });
    const commit = vi.fn();
    const nodes = Array.from({ length: count }, (_, i) => ({
      slug: `p${i}`,
      title: `页面 ${i}`,
      page_type: "concept",
      link_count: 2,
    }));
    const view = render(
      <Profiler id="graph" onRender={commit}>
        <TooltipProvider>
          <KnowledgeGraphCanvas
            graph={{
              nodes,
              edges: nodes.map((n, i) => ({
                source: n.slug,
                target: `p${(i + 1) % count}`,
              })),
              meta: {
                mode: "overview",
                total: count,
                returned: count,
                truncated: false,
              },
            }}
            selected=""
            onSelect={() => {}}
          />
        </TooltipProvider>
      </Profiler>,
    );
    try {
      const canvas = screen.getByRole("group");
      const node = screen.getByRole("button", { name: "页面 0" });
      const before = node.getAttribute("transform");
      const commits = commit.mock.calls.length;
      const labels = vi.mocked(visibleGraphLabels).mock.calls.length;
      for (let i = 0; i < 5; i++)
        act(() => {
          frames.shift()?.(i * 16);
        });
      expect(node.getAttribute("transform")).not.toBe(before);
      expect(vi.mocked(visibleGraphLabels).mock.calls.length).toBe(labels);
      expect(commit.mock.calls.length).toBe(commits);
      act(() => {
        frames.shift()?.(96);
      });
      expect(vi.mocked(visibleGraphLabels).mock.calls.length).toBe(labels + 1);
      for (let i = 6; i < 60; i++)
        act(() => {
          frames.shift()?.(i * 16);
        });
      expect(vi.mocked(visibleGraphLabels).mock.calls.length).toBe(labels + 10);
      expect(canvas.contains(node)).toBe(true);
      expect(commit.mock.calls.length).toBe(commits);
    } finally {
      view.unmount();
      request.mockRestore();
      vi.unstubAllGlobals();
    }
  },
);

it("零尺寸先挂载容器，尺寸可用后创建图谱，折叠后释放动画", () => {
  let width = 0,
    height = 0,
    resize = () => {};
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockImplementation(
    () => width,
  );
  vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockImplementation(
    () => height,
  );
  vi.stubGlobal(
    "ResizeObserver",
    class {
      constructor(callback: () => void) {
        resize = callback;
      }
      observe() {}
      disconnect() {}
    },
  );
  const request = vi.spyOn(window, "requestAnimationFrame").mockReturnValue(17);
  const cancel = vi.spyOn(window, "cancelAnimationFrame");
  const view = render(
    <TooltipProvider>
      <KnowledgeGraphCanvas
        graph={{
          nodes: [
            {
              slug: "a",
              title: "等待尺寸",
              page_type: "concept",
              link_count: 0,
            },
          ],
          edges: [],
          meta: { mode: "overview", total: 1, returned: 1, truncated: false },
        }}
        selected=""
        onSelect={() => {}}
      />
    </TooltipProvider>,
  );
  try {
    expect(screen.getByRole("group")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "等待尺寸" }),
    ).not.toBeInTheDocument();
    expect(request).not.toHaveBeenCalled();
    width = 1000;
    height = 600;
    act(() => resize());
    expect(
      screen.getByRole("button", { name: "等待尺寸" }),
    ).toBeInTheDocument();
    expect(request).toHaveBeenCalled();
    width = 0;
    height = 0;
    act(() => resize());
    expect(
      screen.queryByRole("button", { name: "等待尺寸" }),
    ).not.toBeInTheDocument();
    expect(cancel).toHaveBeenCalledWith(17);
  } finally {
    view.unmount();
    vi.unstubAllGlobals();
  }
});

it("概览因容器折叠重建时保留布局和用户视口", () => {
  let width = 1000,
    height = 600,
    resize = () => {};
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockImplementation(
    () => width,
  );
  vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockImplementation(
    () => height,
  );
  vi.stubGlobal(
    "ResizeObserver",
    class {
      constructor(callback: () => void) {
        resize = callback;
      }
      observe() {}
      disconnect() {}
    },
  );
  vi.spyOn(window, "requestAnimationFrame").mockReturnValue(0);
  const view = render(
    <TooltipProvider>
      <KnowledgeGraphCanvas
        graph={{
          nodes: [
            {
              slug: "a",
              title: "保留视口",
              page_type: "concept",
              link_count: 0,
            },
          ],
          edges: [],
          meta: { mode: "overview", total: 1, returned: 1, truncated: false },
        }}
        selected=""
        onSelect={() => {}}
      />
    </TooltipProvider>,
  );
  try {
    fireEvent.keyDown(screen.getByRole("group"), { key: "ArrowLeft" });
    const root = screen.getByRole("group").querySelector("[data-graph-root]");
    const transform = root?.getAttribute("transform");
    width = 0;
    height = 0;
    act(() => resize());
    width = 1000;
    height = 600;
    act(() => resize());
    expect(
      screen
        .getByRole("group")
        .querySelector("[data-graph-root]")
        ?.getAttribute("transform"),
    ).toBe(transform);
  } finally {
    view.unmount();
    vi.unstubAllGlobals();
  }
});
