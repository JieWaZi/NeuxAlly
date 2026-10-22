import {
  useEffect,
  useId,
  useImperativeHandle,
  useRef,
  useState,
  type Ref,
} from "react";
import { Button } from "@/shared/ui/button";
import { CanvasViewportControls } from "@/shared/ui/canvas-viewport-controls";
import { useRouteSceneActive } from "@/shared/workspace/app-scene-workspace";
import type { KnowledgeGraph } from "./knowledge-graph-model";
import {
  createGraphRenderer,
  type GraphRenderer,
  type GraphRendererState,
} from "./knowledge-graph-renderer";
import { useGraphCopy } from "./use-graph-copy";

export type KnowledgeGraphCanvasControls = {
  fit: () => void;
  focus: (slug: string) => void;
};

/** React 管理宿主和共享控件；逐帧 SVG 更新由迁自 WeKnora 的 Renderer 独立管理。 */
export function KnowledgeGraphCanvas({
  graph,
  selected,
  onSelect,
  onFocus,
  showArrows = true,
  controlsRef,
  center,
  onBloom,
}: {
  graph: KnowledgeGraph;
  center?: string;
  onBloom?: (slug: string) => void;
  showArrows?: boolean;
  onFocus?: (slug: string) => void;
  controlsRef?: Ref<KnowledgeGraphCanvasControls>;
  selected: string;
  onSelect: (slug: string) => void;
}) {
  const t = useGraphCopy();
  const active = useRouteSceneActive();
  const marker = useId();
  const container = useRef<HTMLDivElement>(null),
    svg = useRef<SVGSVGElement>(null);
  const renderer = useRef<GraphRenderer | null>(null),
    prior = useRef<GraphRendererState | undefined>(undefined);
  const previousGraph = useRef<KnowledgeGraph | undefined>(undefined);
  const [scale, setScale] = useState(1),
    [groupFocus, setGroupFocus] = useState(false);
  const [size, setSize] = useState({ width: 0, height: 0 });
  // 回调与选择变化只更新 Renderer 的输入，不重建节点或重启布局。
  const inputs = useRef({ selected, showArrows, onSelect, onFocus, onBloom });
  useEffect(() => {
    inputs.current = { selected, showArrows, onSelect, onFocus, onBloom };
    renderer.current?.update(inputs.current);
  });
  useEffect(() => {
    const host = container.current;
    if (!host) return;
    const resize = () => {
      const { clientWidth: width, clientHeight: height } = host;
      setSize((old) =>
        old.width === width && old.height === height ? old : { width, height },
      );
    };
    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(host);
    return () => observer.disconnect();
  }, []);
  const language = t("canvas");
  useEffect(() => {
    const host = container.current,
      canvas = svg.current;
    if (!host || !canvas || !size.width || !size.height) return;
    const engine = createGraphRenderer({
      graph,
      svg: canvas,
      container: host,
      ...size,
      marker,
      center,
      prior: prior.current,
      preserveLayout:
        previousGraph.current === graph || graph.meta.mode === "ego",
      copy: t,
      onSelect: (slug) => inputs.current.onSelect(slug),
      onFocus: (slug) => inputs.current.onFocus?.(slug),
      onBloom: (slug) => inputs.current.onBloom?.(slug),
      onScale: setScale,
      onGroupFocus: setGroupFocus,
    });
    renderer.current = engine;
    previousGraph.current = graph;
    engine.update(inputs.current);
    return () => {
      prior.current = engine.snapshot();
      engine.destroy();
      renderer.current = null;
    };
  }, [graph, size, marker, center, t]);
  useEffect(() => {
    renderer.current?.setActive(active);
  }, [active, graph, size, marker, center, t]);
  useImperativeHandle(
    controlsRef,
    () => ({
      fit: () => renderer.current?.fit(),
      focus: (slug) => renderer.current?.focus(slug),
    }),
    [],
  );
  return (
    <div
      ref={container}
      className="bg-surface-canvas surface-pattern-dots surface-pattern-dots-canvas relative min-h-0 min-w-0 flex-1 overflow-hidden"
    >
      <svg
        ref={svg}
        role="group"
        aria-label={language}
        tabIndex={0}
        className="focus-visible:ring-ring h-full w-full cursor-grab touch-none outline-none select-none focus-visible:ring-2 focus-visible:ring-inset active:cursor-grabbing"
      />
      <CanvasViewportControls
        scale={scale}
        onZoomIn={() => renderer.current?.zoom(1.1)}
        onZoomOut={() => renderer.current?.zoom(1 / 1.1)}
        onFit={() => renderer.current?.fit()}
        labels={{ zoomIn: t("zoomIn"), zoomOut: t("zoomOut"), fit: t("fit") }}
      />
      {groupFocus && (
        <div className="bottom-scene-bottom absolute left-1/2 -translate-x-1/2">
          <Button
            variant="secondary"
            onClick={() => renderer.current?.exitGroup()}
          >
            {t("leaveGroup")} · Esc
          </Button>
        </div>
      )}
    </div>
  );
}
