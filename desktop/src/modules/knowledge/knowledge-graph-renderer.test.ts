import { fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  createGraphRenderer,
  type GraphRendererState,
} from "./knowledge-graph-renderer";
import type { KnowledgeGraph } from "./knowledge-graph-model";

const graph: KnowledgeGraph = {
  nodes: ["a", "b", "c", "d"].map((slug) => ({
    slug,
    title: slug,
    page_type: "concept",
    link_count: 1,
  })),
  edges: [
    { source: "a", target: "b" },
    { source: "c", target: "d" },
  ],
  meta: { mode: "ego", total: 4, returned: 4, truncated: false },
};
let nextFrame = 0;
const frames = new Map<number, FrameRequestCallback>();
beforeEach(() => {
  frames.clear();
  nextFrame = 0;
  vi.stubGlobal("PointerEvent", MouseEvent);
  vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => {
    frames.set(++nextFrame, callback);
    return nextFrame;
  });
  vi.spyOn(window, "cancelAnimationFrame").mockImplementation((id) => {
    frames.delete(id);
  });
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  document.body.replaceChildren();
});
function show(data = graph, prior?: GraphRendererState) {
  const container = document.createElement("div"),
    svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  container.append(svg);
  document.body.append(container);
  const select = vi.fn(),
    focus = vi.fn(),
    bloom = vi.fn();
  const scale = vi.fn();
  const renderer = createGraphRenderer({
    graph: data,
    container,
    svg,
    width: 1000,
    height: 600,
    marker: "arrow",
    center: "a",
    prior,
    copy: (key) => key,
    onSelect: select,
    onFocus: focus,
    onBloom: bloom,
    onScale: scale,
    onGroupFocus: () => {},
  });
  return { container, svg, select, focus, bloom, scale, renderer };
}
function advance() {
  const first = frames.entries().next().value;
  if (first) {
    frames.delete(first[0]);
    first[1](16);
  }
}

it("拖动只改当前节点与相连边，增量展开保留固定坐标和视口", () => {
  const { svg, renderer } = show();
  try {
    const node = svg.querySelector('[data-graph-node="a"]');
    expect(node).not.toBeNull();
    if (!node) throw new Error("缺少测试节点");
    const lines = svg.querySelectorAll("line"),
      connected = lines[0],
      unrelated = lines[1];
    if (!connected || !unrelated) throw new Error("缺少测试关系");
    const changed = vi.spyOn(connected, "setAttribute"),
      unchanged = vi.spyOn(unrelated, "setAttribute");
    const initial = renderer.snapshot(),
      position = initial.positions.get("a");
    if (!position) throw new Error("缺少坐标");
    const originalX = position.x;
    const x = position.x * initial.viewport.scale + initial.viewport.x,
      y = position.y * initial.viewport.scale + initial.viewport.y;
    fireEvent.pointerDown(node, { button: 0, clientX: x, clientY: y });
    fireEvent.pointerMove(svg, { clientX: x + 40, clientY: y });
    fireEvent.pointerUp(svg);
    expect(changed).toHaveBeenCalledWith("x1", expect.any(String));
    expect(unchanged).not.toHaveBeenCalled();
    const saved = renderer.snapshot();
    expect(saved.positions.get("a")?.pinned).toBe(true);
    expect(saved.positions.get("a")?.x).toBeCloseTo(
      originalX + 40 / initial.viewport.scale,
    );
    renderer.zoom(1.1);
    const expanded = show(
      {
        ...graph,
        nodes: [
          ...graph.nodes,
          { slug: "e", title: "e", page_type: "entity", link_count: 1 },
        ],
        edges: [...graph.edges, { source: "a", target: "e" }],
      },
      renderer.snapshot(),
    );
    try {
      expect(expanded.renderer.snapshot().positions.get("a")).toMatchObject({
        x: position.x,
        y: position.y,
        pinned: true,
      });
      expect(expanded.renderer.snapshot().viewport).toEqual(
        renderer.snapshot().viewport,
      );
    } finally {
      expanded.renderer.destroy();
    }
  } finally {
    renderer.destroy();
  }
});

it("Scene 与窗口隐藏暂停布局，恢复后续算；卸载取消帧和延迟单击", () => {
  vi.useFakeTimers();
  const { svg, renderer, select } = show();
  try {
    renderer.setActive(true);
    expect(frames.size).toBe(1);
    advance();
    expect(frames.size).toBe(1);
    renderer.setActive(false);
    expect(frames.size).toBe(0);
    renderer.setActive(true);
    expect(frames.size).toBe(1);
    const hidden = vi.spyOn(document, "hidden", "get").mockReturnValue(true);
    document.dispatchEvent(new Event("visibilitychange"));
    expect(frames.size).toBe(0);
    hidden.mockReturnValue(false);
    document.dispatchEvent(new Event("visibilitychange"));
    expect(frames.size).toBe(1);
    const node = svg.querySelector('[data-graph-node="a"]');
    if (!node) throw new Error("缺少测试节点");
    fireEvent.click(node);
    renderer.destroy();
    expect(frames.size).toBe(0);
    vi.advanceTimersByTime(300);
    expect(select).not.toHaveBeenCalled();
    document.dispatchEvent(new Event("visibilitychange"));
    expect(frames.size).toBe(0);
  } finally {
    renderer.destroy();
    vi.useRealTimers();
  }
});

it("选择与箭头更新不重建 SVG，双击和 Shift 点击保持展开语义", () => {
  const { svg, renderer, select, focus, bloom } = show();
  try {
    const node = svg.querySelector('[data-graph-node="b"]');
    if (!node) throw new Error("缺少测试节点");
    renderer.update({
      selected: "b",
      showArrows: false,
      onSelect: select,
      onFocus: focus,
      onBloom: bloom,
    });
    expect(svg.querySelector('[data-graph-node="b"]')).toBe(node);
    expect(node.getAttribute("aria-pressed")).toBe("true");
    expect(svg.querySelector("line")?.hasAttribute("marker-end")).toBe(false);
    fireEvent.click(node, { shiftKey: true });
    expect(bloom).toHaveBeenCalledWith("b");
    fireEvent.doubleClick(node);
    expect(focus).toHaveBeenCalledWith("b");
    fireEvent.keyDown(node, { key: "Enter" });
    expect(select).toHaveBeenCalledWith("b");
  } finally {
    renderer.destroy();
  }
});

it("节点按下由节点自身捕获指针，点击不能重定向到背景，结束后释放同一元素", () => {
  const { svg, renderer } = show();
  try {
    const node = svg.querySelector('[data-graph-node="a"]');
    if (!(node instanceof SVGElement)) throw new Error("缺少节点");
    const capture = vi.fn(),
      release = vi.fn(),
      background = vi.fn();
    node.setPointerCapture = capture;
    node.hasPointerCapture = () => true;
    node.releasePointerCapture = release;
    svg.setPointerCapture = background;
    fireEvent.pointerDown(node, { button: 0, clientX: 0, clientY: 0 });
    expect(capture).toHaveBeenCalledTimes(1);
    expect(background).not.toHaveBeenCalled();
    fireEvent.pointerUp(svg);
    expect(release).toHaveBeenCalledTimes(1);
  } finally {
    renderer.destroy();
  }
});

it("分组缩放飞行只在结束时发布比例，动画帧不进入 React 状态", () => {
  vi.spyOn(performance, "now").mockReturnValue(0);
  const nodes = Array.from({ length: 5 }, (_, i) => ({
    slug: `p${i}`,
    title: `p${i}`,
    page_type: "concept",
    link_count: i === 0 ? 4 : 1,
  }));
  const { svg, renderer, scale } = show({
    nodes,
    edges: nodes.slice(1).map((n) => ({ source: "p0", target: n.slug })),
    meta: { mode: "overview", total: 5, returned: 5, truncated: false },
  });
  try {
    renderer.setActive(true);
    const caption = svg.querySelector('[aria-label="p0 · focusGroup"]');
    if (!caption) throw new Error("缺少分组标签");
    const calls = scale.mock.calls.length;
    fireEvent.click(caption);
    for (const time of [16, 32, 64])
      for (const [id, callback] of [...frames]) {
        frames.delete(id);
        callback(time);
      }
    expect(scale.mock.calls.length).toBe(calls);
    for (const [id, callback] of [...frames]) {
      frames.delete(id);
      callback(401);
    }
    expect(scale.mock.calls.length).toBe(calls + 1);
  } finally {
    renderer.destroy();
  }
});
