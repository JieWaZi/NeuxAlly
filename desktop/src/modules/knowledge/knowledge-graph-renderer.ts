// 来源：Tencent/WeKnora，提交 2c0d8384398ca756ee414eef01b71caf91e2caa6 的 WikiBrowser.vue。
// Copyright (C) 2026 Tencent. All rights reserved. MIT 许可与迁入边界见根目录 THIRD_PARTY_NOTICES.md。
import {
  buildGraphLayout,
  graphClusterOutline,
  graphLandmarkTitle,
  graphNodeRadius,
  graphPointBounds,
  initialGraphViewport,
  stepGraphLayout,
  visibleGraphLabels,
  type ForceNode,
  type GraphPoint,
} from "./knowledge-graph-layout";
import { graphAdjacency, type KnowledgeGraph } from "./knowledge-graph-model";
import { graphNodeColor, type useGraphCopy } from "./use-graph-copy";

type Viewport = GraphPoint & { scale: number };
type Copy = ReturnType<typeof useGraphCopy>;
export type GraphRendererState = {
  positions: ReadonlyMap<string, ForceNode>;
  viewport: Viewport;
};
export type GraphRendererOptions = {
  graph: KnowledgeGraph;
  svg: SVGSVGElement;
  container: HTMLDivElement;
  width: number;
  height: number;
  marker: string;
  center?: string;
  prior?: GraphRendererState;
  preserveLayout?: boolean;
  copy: Copy;
  onSelect: (slug: string) => void;
  onFocus?: (slug: string) => void;
  onBloom?: (slug: string) => void;
  onScale: (scale: number) => void;
  onGroupFocus: (focused: boolean) => void;
};
export type GraphRenderer = ReturnType<typeof createGraphRenderer>;

/** React 只装配宿主；帧循环、坐标和交互沿用上游直接操作已有 SVG 的机制。 */
export function createGraphRenderer(options: GraphRendererOptions) {
  const { graph, svg, container, width, height, marker, center, prior } =
    options;
  const abort = new window.AbortController();
  const signal = abort.signal;
  let callbacks = options;
  const incremental = options.preserveLayout ?? graph.meta.mode === "ego";
  const layout = buildGraphLayout(
    graph.nodes,
    graph.edges,
    width,
    height,
    incremental ? prior?.positions : undefined,
  );
  const retained = new Set(incremental ? prior?.positions.keys() : []);
  const nodes = graph.nodes.map((page): ForceNode => {
    const previous = prior?.positions.get(page.slug);
    const position = previous?.pinned
      ? previous
      : layout.placements.get(page.slug);
    return {
      slug: page.slug,
      linkCount: page.link_count,
      x: position?.x ?? 0,
      y: position?.y ?? 0,
      vx: 0,
      vy: 0,
      pinned: previous?.pinned ?? false,
    };
  });
  const nodeMap = new Map(nodes.map((node) => [node.slug, node]));
  const adjacency = graphAdjacency(graph);
  let viewport =
    incremental && prior
      ? { ...prior.viewport }
      : initialGraphViewport(
          nodes,
          Math.max(160, width - Math.min(240, width * 0.25)),
          height,
          nodeMap.get(center ?? "") ?? nodes[0],
        );
  let selected = "",
    hovered = "",
    arrows = true,
    active = false,
    destroyed = false;
  let groupFocus: {
    key: string;
    viewport: Viewport;
    slugs: Set<string>;
  } | null = null;
  let markersVisible: boolean | undefined;
  let simulation = 0,
    flight = 0,
    alpha = 1,
    frame = 0;
  let clickTimer: ReturnType<typeof setTimeout> | undefined;
  let hoverTimer: ReturnType<typeof setTimeout> | undefined;
  let suppressClick = false;
  let drag: {
    id: number;
    capture: SVGElement;
    start: GraphPoint;
    origin: GraphPoint;
    slug?: string;
    moved: boolean;
  } | null = null;
  const media = window.matchMedia?.("(prefers-reduced-motion: reduce)");

  function element<K extends keyof SVGElementTagNameMap>(
    tag: K,
    attributes: Record<string, string | number> = {},
  ) {
    const node = document.createElementNS("http://www.w3.org/2000/svg", tag);
    for (const [key, value] of Object.entries(attributes))
      node.setAttribute(key, String(value));
    return node;
  }
  const root = element("g", { "data-graph-root": "true" });
  const defs = element("defs");
  const arrow = element("marker", {
    id: marker,
    viewBox: "0 0 8 8",
    refX: 7,
    refY: 4,
    markerWidth: 6,
    markerHeight: 6,
    orient: "auto-start-reverse",
  });
  arrow.append(
    element("path", { d: "M0,0 L8,4 L0,8", class: "fill-border-strong" }),
  );
  defs.append(arrow);
  svg.replaceChildren(defs, root);
  svg.setAttribute("viewBox", `0 0 ${width} ${height}`);
  const contours =
    graph.meta.mode === "overview"
      ? layout.clusters.map((cluster) => {
          const path = element("path", {
            class: "fill-border-default stroke-border-default",
            "stroke-width": 1.5,
            "vector-effect": "non-scaling-stroke",
            opacity: 0.5,
            "pointer-events": "none",
          });
          root.append(path);
          return { cluster, path, scale: NaN, positions: [] as number[] };
        })
      : [];
  const edgeGroup = element("g", { "pointer-events": "none" });
  const nodeGroup = element("g");
  root.append(edgeGroup, nodeGroup);
  const pairs = new Map<
    string,
    { source: string; target: string; bidirectional: boolean }
  >();
  const directed = new Set(
    graph.edges.map((edge) => JSON.stringify([edge.source, edge.target])),
  );
  for (const edge of graph.edges) {
    const key = JSON.stringify([edge.source, edge.target].sort());
    if (!pairs.has(key))
      pairs.set(key, {
        ...edge,
        bidirectional: directed.has(JSON.stringify([edge.target, edge.source])),
      });
  }
  const edges = [...pairs.values()].flatMap((edge) => {
    const source = nodeMap.get(edge.source),
      target = nodeMap.get(edge.target);
    if (!source || !target) return [];
    const line = element("line", {
      "data-edge": "true",
      class: "stroke-border-strong",
      "stroke-width": 0.8,
      opacity: 0.42,
    });
    edgeGroup.append(line);
    return [{ ...edge, source, target, line }];
  });
  const edgesBySlug = new Map(
    nodes.map((node) => [
      node.slug,
      edges.filter((edge) => edge.source === node || edge.target === node),
    ]),
  );
  const important = new Set(
    [...nodes]
      .sort((a, b) => b.linkCount - a.linkCount)
      .slice(0, Math.max(1, Math.min(12, Math.ceil(nodes.length / 30))))
      .map((node) => node.slug),
  );

  const nodeElements = graph.nodes.flatMap((page) => {
    const node = nodeMap.get(page.slug);
    if (!node) return [];
    const radius = graphNodeRadius(node.linkCount);
    const group = element("g", {
      role: "button",
      "aria-label": page.title,
      "aria-pressed": "false",
      tabindex: 0,
      "data-graph-node": page.slug,
      class: `group cursor-pointer outline-none ${graphNodeColor(page.page_type)}`,
    });
    const title = element("title");
    title.textContent = `${page.title} · ${options.copy("links", { count: node.linkCount })}`;
    const hit = element("circle", { fill: "transparent" });
    const ring = element("circle", {
      fill: "none",
      stroke: "currentColor",
      "stroke-width": 1.5,
      "vector-effect": "non-scaling-stroke",
      class: "opacity-0 group-focus-visible:opacity-100",
    });
    const circle = element("circle", {
      r: radius,
      fill: "currentColor",
      class: "stroke-surface-panel",
      "stroke-width": 1.5,
    });
    const text = element("text", {
      "text-anchor": "middle",
      class: "type-ui fill-content-secondary stroke-surface-panel",
      "stroke-width": 1.5,
      "paint-order": "stroke",
      "pointer-events": "none",
    });
    text.textContent =
      page.title.length > 14 ? page.title.slice(0, 14) + "…" : page.title;
    group.append(title, hit, ring);
    const frontier = Boolean(
      center &&
      node.slug !== center &&
      node.linkCount > (adjacency.get(node.slug)?.size ?? 0),
    );
    if (frontier)
      group.append(
        element("circle", {
          r: radius + 3,
          fill: "none",
          stroke: "currentColor",
          "stroke-width": 1.5,
          "stroke-dasharray": "3 3",
          opacity: 0.55,
        }),
      );
    group.append(circle, text);
    const bloom = frontier
      ? element("g", {
          role: "button",
          tabindex: -1,
          "aria-label": `${options.copy("bloom")}：${page.title}`,
          visibility: "hidden",
        })
      : undefined;
    const bloomText = element("text", {
      "text-anchor": "middle",
      "dominant-baseline": "central",
      class: "type-title fill-current",
    });
    bloomText.textContent = "+";
    const bloomHit = element("circle", { fill: "transparent" });
    const bloomCircle = element("circle", {
      class: "fill-surface-panel",
      stroke: "currentColor",
      "stroke-width": 1.5,
      "vector-effect": "non-scaling-stroke",
    });
    if (bloom) {
      bloom.append(bloomHit, bloomCircle, bloomText);
      group.append(bloom);
      bloom.addEventListener(
        "pointerdown",
        (event) => event.stopPropagation(),
        { signal },
      );
      bloom.addEventListener(
        "pointermove",
        (event) => event.stopPropagation(),
        { signal },
      );
      bloom.addEventListener(
        "click",
        (event) => {
          event.stopPropagation();
          clearTimeout(clickTimer);
          callbacks.onBloom?.(node.slug);
        },
        { signal },
      );
      bloom.addEventListener(
        "keydown",
        (event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            event.stopPropagation();
            callbacks.onBloom?.(node.slug);
          }
        },
        { signal },
      );
    }
    group.addEventListener(
      "mouseenter",
      () => {
        clearTimeout(hoverTimer);
        highlight(node.slug);
      },
      { signal },
    );
    group.addEventListener(
      "mouseleave",
      () => {
        clearTimeout(hoverTimer);
        hoverTimer = setTimeout(() => highlight(""), 60);
      },
      { signal },
    );
    group.addEventListener("focus", () => highlight(node.slug), { signal });
    group.addEventListener(
      "blur",
      (event) => {
        if (
          !(event.relatedTarget instanceof Node) ||
          !group.contains(event.relatedTarget)
        )
          highlight("");
      },
      { signal },
    );
    group.addEventListener("pointerdown", (event) => begin(event, node.slug), {
      signal,
    });
    group.addEventListener(
      "click",
      (event) => {
        event.stopPropagation();
        clearTimeout(clickTimer);
        if (!suppressClick) {
          const slug = nearest(event, node.slug) ?? node.slug;
          if (event.shiftKey) callbacks.onBloom?.(slug);
          else clickTimer = setTimeout(() => callbacks.onSelect(slug), 220);
        }
        suppressClick = false;
      },
      { signal },
    );
    group.addEventListener(
      "dblclick",
      (event) => {
        event.stopPropagation();
        clearTimeout(clickTimer);
        callbacks.onFocus?.(nearest(event, node.slug) ?? node.slug);
      },
      { signal },
    );
    group.addEventListener(
      "keydown",
      (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          event.stopPropagation();
          if (event.shiftKey) callbacks.onBloom?.(node.slug);
          else callbacks.onSelect(node.slug);
        }
      },
      { signal },
    );
    nodeGroup.append(group);
    return [
      {
        node,
        group,
        circle,
        ring,
        hit,
        text,
        bloom,
        bloomHit,
        bloomCircle,
        bloomText,
        labelWidth: [...(text.textContent ?? "")].reduce(
          (sum, char) => sum + (char.charCodeAt(0) > 255 ? 12 : 7),
          0,
        ),
      },
    ];
  });
  const anchors = contours.flatMap(({ cluster }) =>
    cluster.slugs.length >= 5
      ? cluster.anchors.flatMap((slug) => {
          const node = nodeMap.get(slug),
            page = graph.nodes.find((entry) => entry.slug === slug);
          if (!node || !page) return [];
          const title = graphLandmarkTitle(page.title).slice(0, 20);
          const group = element("g", {
            role: "button",
            tabindex: 0,
            "aria-label": `${title} · ${options.copy("focusGroup")}`,
            class: "cursor-pointer outline-none",
          });
          const text = element("text", {
            "text-anchor": "middle",
            class:
              "type-body fill-content-primary stroke-surface-panel font-semibold",
            "stroke-width": 1.5,
            "paint-order": "stroke",
          });
          text.textContent = title;
          group.append(text);
          root.append(group);
          group.addEventListener(
            "pointerdown",
            (event) => event.stopPropagation(),
            { signal },
          );
          group.addEventListener(
            "click",
            (event) => {
              event.stopPropagation();
              focusGroup(cluster.representative);
            },
            { signal },
          );
          group.addEventListener(
            "keydown",
            (event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                event.stopPropagation();
                focusGroup(cluster.representative);
              }
            },
            { signal },
          );
          return [
            {
              node,
              group,
              text,
              cluster,
              labelWidth: [...title].reduce(
                (sum, char) => sum + (char.charCodeAt(0) > 255 ? 13 : 7),
                0,
              ),
            },
          ];
        })
      : [],
  );
  // 沿用上游批量写入后再读文本宽度，避免逐节点读写导致强制布局。
  for (const entry of [...nodeElements, ...anchors])
    entry.labelWidth = entry.text.getComputedTextLength?.() || entry.labelWidth;

  function setEdgePositions(edge: (typeof edges)[number]) {
    const { source: s, target: t, line } = edge;
    const dx = t.x - s.x,
      dy = t.y - s.y,
      distance = Math.hypot(dx, dy) || 1;
    const margin = arrows && viewport.scale >= 0.8 ? 4 : 0;
    const rs = graphNodeRadius(s.linkCount) + margin,
      rt = graphNodeRadius(t.linkCount) + margin;
    line.setAttribute("x1", String(s.x + (dx / distance) * rs));
    line.setAttribute("y1", String(s.y + (dy / distance) * rs));
    line.setAttribute("x2", String(t.x - (dx / distance) * rt));
    line.setAttribute("y2", String(t.y - (dy / distance) * rt));
  }
  function updatePositions() {
    for (const { group, node } of nodeElements)
      group.setAttribute("transform", `translate(${node.x},${node.y})`);
    for (const edge of edges) setEdgePositions(edge);
  }
  function refreshLabels() {
    for (const contour of contours) {
      const members = contour.cluster.slugs.flatMap((slug) => {
        const node = nodeMap.get(slug);
        return node ? [node] : [];
      });
      if (
        contour.scale !== viewport.scale ||
        contour.positions.length !== members.length * 2 ||
        members.some(
          (node, i) =>
            contour.positions[i * 2] !== node.x ||
            contour.positions[i * 2 + 1] !== node.y,
        )
      ) {
        contour.path.setAttribute(
          "d",
          graphClusterOutline(
            members.map((node) => ({
              ...node,
              radius: graphNodeRadius(node.linkCount),
            })),
            viewport.scale,
          ),
        );
        contour.scale = viewport.scale;
        contour.positions = members.flatMap((node) => [node.x, node.y]);
      }
    }
    const groupLabels = anchors.flatMap((entry) => {
      const outside = Boolean(
        groupFocus && !groupFocus.slugs.has(entry.node.slug),
      );
      const shown = outside
        ? entry.node.slug === entry.cluster.anchors[0]
        : viewport.scale < 1.2;
      entry.group.setAttribute("display", shown ? "inline" : "none");
      if (!shown) return [];
      const y =
        entry.node.y +
        graphNodeRadius(entry.node.linkCount) +
        18 / viewport.scale;
      entry.group.setAttribute(
        "transform",
        `translate(${entry.node.x},${y}) scale(${1 / viewport.scale})`,
      );
      return [
        {
          slug: `anchor:${entry.node.slug}`,
          priority: 100000 + entry.node.linkCount,
          required:
            outside ||
            entry.node.slug === selected ||
            entry.node.slug === hovered,
          left: entry.node.x * viewport.scale - entry.labelWidth / 2 - 8,
          right: entry.node.x * viewport.scale + entry.labelWidth / 2 + 8,
          top: y * viewport.scale - 16,
          bottom: y * viewport.scale + 5,
        },
      ];
    });
    const captioned = new Set(groupLabels.map((label) => label.slug.slice(7)));
    const candidates = nodeElements.flatMap((entry) => {
      const { node, text, hit, ring, bloom, bloomHit, bloomCircle, bloomText } =
        entry;
      const radius = graphNodeRadius(node.linkCount),
        scale = viewport.scale;
      hit.setAttribute("r", String(Math.max(radius + 6, 18 / scale)));
      ring.setAttribute("r", String(radius + 4 / scale));
      text.setAttribute(
        "transform",
        `translate(0,${radius + 16 / scale}) scale(${1 / scale})`,
      );
      if (bloom) {
        bloom.setAttribute(
          "transform",
          `translate(${radius + 10 / scale},${-radius - 10 / scale})`,
        );
        bloomHit.setAttribute("r", String(18 / scale));
        bloomCircle.setAttribute("r", String(9 / scale));
        bloomText.setAttribute("transform", `scale(${1 / scale})`);
      }
      const required = node.slug === selected || node.slug === hovered;
      if (
        captioned.has(node.slug) ||
        (!required &&
          scale < 0.5 &&
          !(groupFocus?.slugs.has(node.slug) && scale >= 0.4) &&
          !important.has(node.slug))
      )
        return [];
      const baseline = (node.y + radius) * scale + 16,
        half = entry.labelWidth / 2 + 5;
      return [
        {
          slug: node.slug,
          priority: node.linkCount,
          required,
          left: node.x * scale - half,
          right: node.x * scale + half,
          top: baseline - 15,
          bottom: baseline + 4,
        },
      ];
    });
    const visible = visibleGraphLabels([...groupLabels, ...candidates]);
    for (const entry of anchors) {
      const shown = visible.has(`anchor:${entry.node.slug}`);
      entry.group.setAttribute("visibility", shown ? "visible" : "hidden");
      entry.group.setAttribute("tabindex", shown ? "0" : "-1");
    }
    for (const entry of nodeElements)
      entry.text.setAttribute(
        "visibility",
        visible.has(entry.node.slug) ? "visible" : "hidden",
      );
  }
  function applyHighlight() {
    const focus = selected || hovered;
    const connected = new Set([
      focus,
      hovered,
      ...(adjacency.get(focus) ?? []),
      ...(adjacency.get(hovered) ?? []),
    ]);
    for (const entry of nodeElements) {
      const { node, group, ring, bloom, circle } = entry;
      const focused = node.slug === focus || node.slug === hovered;
      group.setAttribute(
        "opacity",
        focus
          ? connected.has(node.slug)
            ? "1"
            : "0.2"
          : groupFocus && !groupFocus.slugs.has(node.slug)
            ? "0.12"
            : "1",
      );
      circle.setAttribute(
        "r",
        String(graphNodeRadius(node.linkCount) + (focused ? 3 : 0)),
      );
      ring.classList.toggle("opacity-0", node.slug !== selected);
      ring.classList.toggle("opacity-100", node.slug === selected);
      group.setAttribute("aria-pressed", String(node.slug === selected));
      if (bloom) {
        const shown = node.slug === hovered || node.slug === selected;
        bloom.setAttribute("visibility", shown ? "visible" : "hidden");
        bloom.setAttribute("tabindex", shown ? "0" : "-1");
      }
    }
    for (const edge of edges) {
      const highlighted =
        edge.source.slug === focus ||
        edge.target.slug === focus ||
        Boolean(
          hovered &&
          (edge.source.slug === hovered || edge.target.slug === hovered),
        );
      edge.line.setAttribute(
        "class",
        highlighted ? "stroke-primary" : "stroke-border-strong",
      );
      edge.line.setAttribute(
        "opacity",
        focus
          ? highlighted
            ? "0.85"
            : "0.08"
          : groupFocus &&
              (!groupFocus.slugs.has(edge.source.slug) ||
                !groupFocus.slugs.has(edge.target.slug))
            ? "0.05"
            : "0.42",
      );
    }
  }
  function highlight(slug: string) {
    if (hovered === slug || drag) return;
    hovered = slug;
    applyHighlight();
    refreshLabels();
  }
  function updateMarkers() {
    const markers = arrows && viewport.scale >= 0.8;
    if (markersVisible === markers) return;
    markersVisible = markers;
    for (const edge of edges) {
      if (markers) edge.line.setAttribute("marker-end", `url(#${marker})`);
      else edge.line.removeAttribute("marker-end");
      if (markers && edge.bidirectional)
        edge.line.setAttribute("marker-start", `url(#${marker})`);
      else edge.line.removeAttribute("marker-start");
      setEdgePositions(edge);
    }
  }
  function applyViewport(previousScale = viewport.scale, publishScale = true) {
    root.setAttribute(
      "transform",
      `translate(${viewport.x},${viewport.y}) scale(${viewport.scale})`,
    );
    container.style.backgroundPosition = `${viewport.x}px ${viewport.y}px`;
    container.style.backgroundSize = `${20 * viewport.scale}px ${20 * viewport.scale}px`;
    if (previousScale !== viewport.scale) {
      updateMarkers();
      if (publishScale) callbacks.onScale(viewport.scale);
    }
    refreshLabels();
  }
  function cancelFlight() {
    cancelAnimationFrame(flight);
    if (flight) callbacks.onScale(viewport.scale);
    flight = 0;
  }
  function zoom(
    factor: number,
    anchor: GraphPoint = { x: width / 2, y: height / 2 },
  ) {
    cancelFlight();
    const scale = Math.max(0.005, Math.min(5, viewport.scale * factor)),
      ratio = scale / viewport.scale;
    const previousScale = viewport.scale;
    viewport = {
      scale,
      x: anchor.x - (anchor.x - viewport.x) * ratio,
      y: anchor.y - (anchor.y - viewport.y) * ratio,
    };
    applyViewport(previousScale);
  }
  function flyTo(target: Viewport) {
    cancelFlight();
    if (!running() || media?.matches) {
      const scale = viewport.scale;
      viewport = target;
      applyViewport(scale);
      return;
    }
    const start = { ...viewport },
      time = performance.now();
    function animate(now: number) {
      if (!running()) return;
      const progress = Math.max(0, Math.min(1, (now - time) / 400)),
        ease = 1 - Math.pow(1 - progress, 3),
        oldScale = viewport.scale;
      viewport = {
        x: start.x + (target.x - start.x) * ease,
        y: start.y + (target.y - start.y) * ease,
        scale: start.scale + (target.scale - start.scale) * ease,
      };
      applyViewport(oldScale, progress === 1);
      flight = progress < 1 ? requestAnimationFrame(animate) : 0;
    }
    flight = requestAnimationFrame(animate);
  }
  function focus(slug: string) {
    const node = nodeMap.get(slug);
    if (node)
      flyTo({
        scale: viewport.scale,
        x:
          Math.max(160, width - Math.min(480, width * 0.7)) / 2 -
          node.x * viewport.scale,
        y: height / 2 - node.y * viewport.scale,
      });
  }
  function fit() {
    cancelFlight();
    const bounds = graphPointBounds(nodes),
      oldScale = viewport.scale;
    const scale = Math.max(
      0.005,
      Math.min(
        1.5,
        (width - 100) / Math.max(100, bounds.right - bounds.left),
        (height - 120) / Math.max(100, bounds.bottom - bounds.top),
      ),
    );
    viewport = {
      scale,
      x: width / 2 - ((bounds.left + bounds.right) * scale) / 2,
      y: height / 2 - ((bounds.top + bounds.bottom) * scale) / 2,
    };
    applyViewport(oldScale);
  }
  function exitGroup() {
    if (!groupFocus) return;
    const target = groupFocus.viewport;
    groupFocus = null;
    callbacks.onGroupFocus(false);
    applyHighlight();
    flyTo(target);
  }
  function focusGroup(key: string) {
    if (groupFocus?.key === key) {
      exitGroup();
      return;
    }
    const cluster = layout.clusters.find(
      (entry) => entry.representative === key,
    );
    if (!cluster) return;
    const bounds = graphPointBounds(
      cluster.slugs.flatMap((slug) => {
        const node = nodeMap.get(slug);
        return node ? [node] : [];
      }),
    );
    const scale = Math.min(
      1.75,
      Math.max(
        0.4,
        Math.min(
          (Math.max(160, width - Math.min(240, width * 0.25)) - 160) /
            Math.max(100, bounds.right - bounds.left),
          (height - 180) / Math.max(100, bounds.bottom - bounds.top),
        ),
      ),
    );
    groupFocus = {
      key,
      viewport: groupFocus?.viewport ?? { ...viewport },
      slugs: new Set(cluster.slugs),
    };
    callbacks.onGroupFocus(true);
    applyHighlight();
    flyTo({
      scale,
      x: width / 2 - ((bounds.left + bounds.right) * scale) / 2,
      y: height / 2 - ((bounds.top + bounds.bottom) * scale) / 2,
    });
  }
  function point(event: Pick<MouseEvent, "clientX" | "clientY">) {
    const rect = svg.getBoundingClientRect();
    return {
      x: ((event.clientX - rect.left) * width) / (rect.width || width),
      y: ((event.clientY - rect.top) * height) / (rect.height || height),
    };
  }
  function nearest(
    event: Pick<MouseEvent, "clientX" | "clientY">,
    fallback?: string,
  ) {
    if (!event.clientX && !event.clientY) return fallback;
    const cursor = point(event);
    let slug = fallback,
      distance = Infinity;
    for (const node of nodes) {
      const delta = Math.hypot(
        node.x * viewport.scale + viewport.x - cursor.x,
        node.y * viewport.scale + viewport.y - cursor.y,
      );
      if (
        delta <=
          Math.max(18, graphNodeRadius(node.linkCount) * viewport.scale + 6) &&
        delta < distance
      ) {
        slug = node.slug;
        distance = delta;
      }
    }
    return slug;
  }
  function begin(event: PointerEvent, slug?: string) {
    if (event.button !== 0) return;
    event.stopPropagation();
    clearTimeout(clickTimer);
    cancelFlight();
    slug = nearest(event, slug);
    const origin = slug ? (nodeMap.get(slug) ?? viewport) : viewport;
    drag = {
      id: event.pointerId,
      capture:
        event.currentTarget instanceof SVGElement ? event.currentTarget : svg,
      start: point(event),
      origin: { x: origin.x, y: origin.y },
      slug,
      moved: false,
    };
    const node = slug ? nodeMap.get(slug) : undefined;
    if (node) node.pinned = true;
    suppressClick = false;
    drag.capture.setPointerCapture?.(event.pointerId);
  }
  function move(event: PointerEvent) {
    if (!drag) {
      highlight(nearest(event) ?? "");
      return;
    }
    if (event.pointerId !== drag.id) return;
    const cursor = point(event),
      dx = cursor.x - drag.start.x,
      dy = cursor.y - drag.start.y;
    if (Math.hypot(dx, dy) > 3) drag.moved = true;
    if (!drag.moved) return;
    const node = drag.slug ? nodeMap.get(drag.slug) : undefined;
    if (node) {
      node.x = drag.origin.x + dx / viewport.scale;
      node.y = drag.origin.y + dy / viewport.scale;
      node.vx = 0;
      node.vy = 0;
      const entry = nodeElements.find((entry) => entry.node === node);
      entry?.group.setAttribute("transform", `translate(${node.x},${node.y})`);
      for (const edge of edgesBySlug.get(node.slug) ?? [])
        setEdgePositions(edge);
      refreshLabels();
    } else {
      viewport = { ...viewport, x: drag.origin.x + dx, y: drag.origin.y + dy };
      applyViewport();
    }
  }
  function releaseDrag() {
    if (drag?.capture.hasPointerCapture?.(drag.id))
      drag.capture.releasePointerCapture(drag.id);
    drag = null;
  }
  function end(event: PointerEvent) {
    if (!drag || event.pointerId !== drag.id) return;
    suppressClick = drag.moved;
    releaseDrag();
  }
  svg.addEventListener("pointerdown", (event) => begin(event), { signal });
  svg.addEventListener("pointermove", move, { signal });
  svg.addEventListener("pointerup", end, { signal });
  svg.addEventListener("pointercancel", end, { signal });
  svg.addEventListener(
    "pointerleave",
    () => {
      if (!drag) highlight("");
    },
    { signal },
  );
  svg.addEventListener(
    "click",
    () => {
      if (!suppressClick) {
        callbacks.onSelect("");
        if (!selected) exitGroup();
      }
      suppressClick = false;
    },
    { signal },
  );
  svg.addEventListener(
    "wheel",
    (event) => {
      event.preventDefault();
      zoom(event.deltaY > 0 ? 0.92 : 1.08, point(event));
    },
    { signal, passive: false },
  );
  svg.addEventListener(
    "keydown",
    (event) => {
      if (event.key === "Escape") {
        event.preventDefault();
        callbacks.onSelect("");
        exitGroup();
        return;
      }
      if (event.target !== svg) return;
      const directions: Record<string, GraphPoint> = {
        ArrowLeft: { x: 40, y: 0 },
        ArrowRight: { x: -40, y: 0 },
        ArrowUp: { x: 0, y: 40 },
        ArrowDown: { x: 0, y: -40 },
      };
      const direction = directions[event.key];
      if (direction) {
        event.preventDefault();
        viewport = {
          ...viewport,
          x: viewport.x + direction.x,
          y: viewport.y + direction.y,
        };
        applyViewport();
      }
      if (["+", "=", "-", "Home"].includes(event.key)) {
        event.preventDefault();
        if (event.key === "Home") fit();
        else zoom(event.key === "-" ? 0.8 : 1.25);
      }
    },
    { signal },
  );
  function running() {
    return active && !destroyed && !document.hidden;
  }
  function tick() {
    simulation = 0;
    if (!running()) return;
    alpha *= 0.985;
    if (alpha < 0.02) {
      refreshLabels();
      return;
    }
    stepGraphLayout(nodes, graph.edges, layout.placements, alpha, retained);
    updatePositions();
    if (++frame % 6 === 0) refreshLabels();
    simulation = requestAnimationFrame(tick);
  }
  function resume() {
    cancelAnimationFrame(simulation);
    simulation = 0;
    if (!running() || alpha < 0.02) return;
    if (media?.matches) {
      for (; alpha >= 0.02; alpha *= 0.985)
        stepGraphLayout(nodes, graph.edges, layout.placements, alpha, retained);
      updatePositions();
      refreshLabels();
    } else simulation = requestAnimationFrame(tick);
  }
  function visibility() {
    cancelFlight();
    if (document.hidden) {
      clearTimeout(clickTimer);
      clearTimeout(hoverTimer);
      releaseDrag();
    }
    resume();
  }
  document.addEventListener("visibilitychange", visibility, { signal });
  media?.addEventListener("change", visibility);
  updatePositions();
  updateMarkers();
  applyViewport();
  callbacks.onScale(viewport.scale);
  callbacks.onGroupFocus(false);
  return {
    fit,
    focus,
    zoom,
    exitGroup,
    snapshot: (): GraphRendererState => ({
      positions: nodeMap,
      viewport: { ...viewport },
    }),
    update(
      next: Pick<GraphRendererOptions, "onSelect" | "onFocus" | "onBloom"> & {
        selected: string;
        showArrows: boolean;
      },
    ) {
      callbacks = { ...callbacks, ...next };
      const changed = selected !== next.selected;
      selected = next.selected;
      if (arrows !== next.showArrows) {
        arrows = next.showArrows;
        updateMarkers();
      }
      if (changed) {
        applyHighlight();
        refreshLabels();
      }
    },
    setActive(value: boolean) {
      if (active === value) return;
      active = value;
      if (!value) {
        cancelFlight();
        clearTimeout(clickTimer);
        clearTimeout(hoverTimer);
        releaseDrag();
      }
      resume();
    },
    destroy() {
      destroyed = true;
      cancelAnimationFrame(simulation);
      cancelAnimationFrame(flight);
      clearTimeout(clickTimer);
      clearTimeout(hoverTimer);
      releaseDrag();
      abort.abort();
      media?.removeEventListener("change", visibility);
      svg.replaceChildren();
    },
  };
}
