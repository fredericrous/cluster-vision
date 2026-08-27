import { useCallback, useMemo, useState } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  type Node,
  type Edge,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import dagre from "@dagrejs/dagre";
import { SmartStepEdge } from "@jalez/react-flow-smart-edge";
import { FlowNode, type FlowNodeData } from "./flow-node";
import { LayerGroup } from "./flow-group";
import type { Change, DiagramDiff } from "../api.server";
import styles from "./flow-diagram.module.css";

interface FlowNodeRaw {
  id: string;
  label: string;
  cluster: string;
  layer: string; // real Flux layer directory name (e.g. crds, controllers, apps)
}

interface FlowEdgeRaw {
  id: string;
  source: string;
  target: string;
  crossCluster?: boolean;
  // Cross-cluster edges carry a label (Cilium global Service name or
  // Istio host) so multiple distinct paths between the same kustomization
  // pair can be distinguished visually.
  label?: string;
}

interface FlowDataRaw {
  nodes: FlowNodeRaw[];
  edges: FlowEdgeRaw[];
}

const NODE_H = 44;
const PAD_X = 20;
const PAD_TOP = 32;
const PAD_BOTTOM = 16;
const CLUSTER_GAP = 60;
const MIN_NODE_W = 120;
const MAX_NODE_W = 300;
// Horizontal padding (14px * 2) + border (1px * 2) + cluster accent (3px)
const NODE_PAD = 33;

// Cluster display order WITHIN a row.
const CLUSTER_ORDER: Record<string, number> = { NAS: 0, Homelab: 1, Monitor: 2 };

// Cluster row assignment. Clusters in the same row lay out side-by-side;
// rows stack vertically with CLUSTER_ROW_GAP between them. Monitor sits
// on a second row because it consumes services from BOTH NAS (Garage S3)
// and Homelab (Authelia OIDC), so a horizontal-only layout would force
// long crossing edges over the whole diagram. Defaults to row 0.
const CLUSTER_ROW: Record<string, number> = { NAS: 0, Homelab: 0, Monitor: 1 };
const CLUSTER_ROW_GAP = 80;

const nodeTypes = { flow: FlowNode, layerGroup: LayerGroup };
const edgeTypes = { smartStep: SmartStepEdge };

/** Measure the widest label and return a uniform node width. */
function computeNodeWidth(labels: string[]): number {
  if (typeof document === "undefined") return 160; // SSR fallback
  const canvas = document.createElement("canvas");
  const ctx = canvas.getContext("2d");
  if (!ctx) return 160;
  ctx.font = "500 0.8rem Inter, sans-serif";
  let maxW = 0;
  for (const label of labels) {
    maxW = Math.max(maxW, ctx.measureText(label).width);
  }
  return Math.min(MAX_NODE_W, Math.max(MIN_NODE_W, Math.ceil(maxW) + NODE_PAD));
}

// Dynamic color palette for arbitrary layer names.
// Colors are assigned in discovery order; this palette has enough entries
// for the typical Flux layer count (crds, controllers, platform-foundation,
// security, monitoring, apps, etc.).
const LAYER_PALETTE = [
  "#3b82f6", // blue
  "#8b5cf6", // violet
  "#f59e0b", // amber
  "#22c55e", // green
  "#ef4444", // red
  "#06b6d4", // cyan
  "#ec4899", // pink
  "#f97316", // orange
  "#a855f7", // purple
  "#14b8a6", // teal
  "#eab308", // yellow
  "#64748b", // slate (fallback)
];

function assignLayerColors(layers: string[]): Record<string, string> {
  const map: Record<string, string> = {};
  const sorted = [...layers].sort();
  for (let i = 0; i < sorted.length; i++) {
    map[sorted[i]] = LAYER_PALETTE[i % LAYER_PALETTE.length];
  }
  return map;
}

const REMOVED_LAYER = "(removed)";

/** Rebuild removed nodes/edges from the diff so the layout runs on the
 *  union of both sides and nothing moves between "before" and "after".
 *  Removed elements only carry their id/label; cluster is recovered from
 *  the id (`<cluster>/<name>`) and the layer is a placeholder. */
function withRemoved(
  rawNodes: FlowNodeRaw[],
  rawEdges: FlowEdgeRaw[],
  diff: DiagramDiff | null
): { nodes: FlowNodeRaw[]; edges: FlowEdgeRaw[]; ops: Map<string, Change> } {
  const ops = new Map<string, Change>();
  if (!diff) return { nodes: rawNodes, edges: rawEdges, ops };
  for (const c of diff.changes) ops.set(c.id, c);

  const nodes = [...rawNodes];
  const known = new Set(rawNodes.map((n) => n.id));
  const edges = [...rawEdges];
  for (const c of diff.changes) {
    if (c.op !== "removed") continue;
    if (c.kind === "node" && !known.has(c.id)) {
      const slash = c.id.indexOf("/");
      nodes.push({
        id: c.id,
        label: c.label || c.id,
        cluster: slash > 0 ? c.id.slice(0, slash) : "",
        layer: REMOVED_LAYER,
      });
      known.add(c.id);
    }
    if (c.kind === "edge") {
      // "<src>-><dst>" or "xc-cm:<src>-><dst>:<label>"
      const cross = c.id.startsWith("xc-");
      let body = cross ? c.id.slice(c.id.indexOf(":") + 1) : c.id;
      let label: string | undefined;
      if (cross) {
        const last = body.lastIndexOf(":");
        if (last > 0) {
          label = body.slice(last + 1);
          body = body.slice(0, last);
        }
      }
      const [source, target] = body.split("->");
      if (source && target) {
        edges.push({ id: c.id, source, target, crossCluster: cross, label });
      }
    }
  }
  return { nodes, edges, ops };
}

function diffStateOf(id: string, ops: Map<string, Change>, active: boolean): FlowNodeData["diff"] {
  if (!active) return undefined;
  const c = ops.get(id);
  if (!c) return "same";
  return c.op;
}

function buildLayout(
  rawNodes: FlowNodeRaw[],
  rawEdges: FlowEdgeRaw[],
  ops: Map<string, Change>,
  compareActive: boolean
): { nodes: Node[]; edges: Edge[]; layerColorMap: Record<string, string> } {
  const nodeW = computeNodeWidth(rawNodes.map((n) => n.label));

  const clusters = [...new Set(rawNodes.map((n) => n.cluster))].sort(
    (a, b) => (CLUSTER_ORDER[a] ?? 99) - (CLUSTER_ORDER[b] ?? 99)
  );
  const layers = [...new Set(rawNodes.map((n) => n.layer))];
  const layerColorMap = assignLayerColors(layers);

  // Split nodes and edges per cluster
  const nodesByCluster = new Map<string, FlowNodeRaw[]>();
  const nodeIdToCluster = new Map<string, string>();
  for (const n of rawNodes) {
    if (!nodesByCluster.has(n.cluster)) nodesByCluster.set(n.cluster, []);
    nodesByCluster.get(n.cluster)!.push(n);
    nodeIdToCluster.set(n.id, n.cluster);
  }

  const edgesByCluster = new Map<string, FlowEdgeRaw[]>();
  for (const e of rawEdges) {
    if (e.crossCluster) continue;
    const cluster = nodeIdToCluster.get(e.source) || nodeIdToCluster.get(e.target);
    if (cluster) {
      if (!edgesByCluster.has(cluster)) edgesByCluster.set(cluster, []);
      edgesByCluster.get(cluster)!.push(e);
    }
  }

  // Run Dagre layout per cluster, collect positioned nodes grouped by rank
  type PositionedNode = { raw: FlowNodeRaw; x: number; y: number; rank: number };
  const clusterResults = new Map<string, PositionedNode[]>();

  for (const cluster of clusters) {
    const cNodes = nodesByCluster.get(cluster) || [];
    const cEdges = edgesByCluster.get(cluster) || [];
    if (cNodes.length === 0) continue;

    const g = new dagre.graphlib.Graph();
    g.setGraph({
      rankdir: "TB",
      nodesep: 60,
      ranksep: 80,
      marginx: 0,
      marginy: 0,
      ranker: "network-simplex",
    });
    g.setDefaultEdgeLabel(() => ({}));

    for (const n of cNodes) {
      g.setNode(n.id, { width: nodeW, height: NODE_H });
    }
    for (const e of cEdges) {
      g.setEdge(e.source, e.target);
    }

    dagre.layout(g);

    const positioned: PositionedNode[] = [];
    for (const n of cNodes) {
      const pos = g.node(n.id);
      positioned.push({
        raw: n,
        // Dagre returns center coords; convert to top-left
        x: pos.x - nodeW / 2,
        y: pos.y - NODE_H / 2,
        rank: pos.rank ?? 0,
      });
    }
    clusterResults.set(cluster, positioned);
  }

  // For each rank in each cluster, compute bounding box
  type RankBBox = { minX: number; maxX: number; minY: number; maxY: number; nodes: PositionedNode[] };
  const clusterRankBoxes = new Map<string, Map<number, RankBBox>>();

  for (const cluster of clusters) {
    const positioned = clusterResults.get(cluster);
    if (!positioned) continue;

    const byRank = new Map<number, PositionedNode[]>();
    for (const p of positioned) {
      if (!byRank.has(p.rank)) byRank.set(p.rank, []);
      byRank.get(p.rank)!.push(p);
    }

    const boxes = new Map<number, RankBBox>();
    for (const [rank, nodes] of byRank) {
      let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
      for (const n of nodes) {
        minX = Math.min(minX, n.x);
        maxX = Math.max(maxX, n.x + nodeW);
        minY = Math.min(minY, n.y);
        maxY = Math.max(maxY, n.y + NODE_H);
      }
      boxes.set(rank, { minX, maxX, minY, maxY, nodes });
    }
    clusterRankBoxes.set(cluster, boxes);
  }

  // Group clusters by row. Each row is laid out independently — its own
  // rank heights and cluster X cursor — so a single tall cluster on
  // another row doesn't push the rest down.
  const clustersByRow = new Map<number, string[]>();
  for (const cluster of clusters) {
    const row = CLUSTER_ROW[cluster] ?? 99;
    if (!clustersByRow.has(row)) clustersByRow.set(row, []);
    clustersByRow.get(row)!.push(cluster);
  }
  const sortedRows = [...clustersByRow.keys()].sort((a, b) => a - b);

  // Compute cluster widths (max rank width per cluster) — same regardless of row
  const clusterWidths = new Map<string, number>();
  for (const cluster of clusters) {
    const boxes = clusterRankBoxes.get(cluster);
    if (!boxes) continue;
    let maxW = 0;
    for (const box of boxes.values()) {
      maxW = Math.max(maxW, box.maxX - box.minX + 2 * PAD_X);
    }
    clusterWidths.set(cluster, maxW);
  }

  // Per-row rank heights (uniform across that row's clusters only)
  const rowRankHeights = new Map<number, Map<number, number>>();
  for (const [row, rowClusters] of clustersByRow) {
    const ranks = new Set<number>();
    for (const cluster of rowClusters) {
      const boxes = clusterRankBoxes.get(cluster);
      if (boxes) for (const r of boxes.keys()) ranks.add(r);
    }
    const heights = new Map<number, number>();
    for (const rank of ranks) {
      let maxH = 0;
      for (const cluster of rowClusters) {
        const box = clusterRankBoxes.get(cluster)?.get(rank);
        if (box) maxH = Math.max(maxH, box.maxY - box.minY + PAD_TOP + PAD_BOTTOM);
      }
      heights.set(rank, maxH);
    }
    rowRankHeights.set(row, heights);
  }

  // Per-row rank Y offsets (relative to row top)
  const rowRankY = new Map<number, Map<number, number>>();
  const rowHeight = new Map<number, number>();
  for (const row of sortedRows) {
    const heights = rowRankHeights.get(row)!;
    const ranksInRow = [...heights.keys()].sort((a, b) => a - b);
    const yMap = new Map<number, number>();
    let yCursor = 0;
    for (const rank of ranksInRow) {
      yMap.set(rank, yCursor);
      yCursor += heights.get(rank)! + 20;
    }
    rowRankY.set(row, yMap);
    rowHeight.set(row, Math.max(0, yCursor - 20)); // remove last trailing gap
  }

  // Cumulative Y offset per row
  const rowYOffset = new Map<number, number>();
  let yAcc = 0;
  for (const row of sortedRows) {
    rowYOffset.set(row, yAcc);
    yAcc += rowHeight.get(row)! + CLUSTER_ROW_GAP;
  }

  // Per-row cluster X offsets (each row starts at x=0)
  const clusterXMap = new Map<string, number>();
  for (const row of sortedRows) {
    const rowClusters = clustersByRow.get(row)!.slice().sort(
      (a, b) => (CLUSTER_ORDER[a] ?? 99) - (CLUSTER_ORDER[b] ?? 99)
    );
    let xCursor = 0;
    for (const cluster of rowClusters) {
      const w = clusterWidths.get(cluster);
      if (w === undefined) continue;
      clusterXMap.set(cluster, xCursor);
      xCursor += w + CLUSTER_GAP;
    }
  }

  // Resolve a cluster's rank Y in the global coordinate space
  const rankYFor = (cluster: string, rank: number): number => {
    const row = CLUSTER_ROW[cluster] ?? 99;
    return (rowYOffset.get(row) ?? 0) + (rowRankY.get(row)?.get(rank) ?? 0);
  };
  const rankHeightFor = (cluster: string, rank: number): number => {
    const row = CLUSTER_ROW[cluster] ?? 99;
    return rowRankHeights.get(row)?.get(rank) ?? 0;
  };

  // Build ReactFlow nodes: group containers + child nodes
  const allNodes: Node[] = [];

  for (const cluster of clusters) {
    const boxes = clusterRankBoxes.get(cluster);
    if (!boxes) continue;
    const cX = clusterXMap.get(cluster)!;
    const cW = clusterWidths.get(cluster)!;

    for (const [rank, box] of boxes) {
      const rY = rankYFor(cluster, rank);
      const rH = rankHeightFor(cluster, rank);
      const groupId = `${cluster}-rank-${rank}`;

      allNodes.push({
        id: groupId,
        type: "layerGroup",
        position: { x: cX, y: rY },
        style: { width: cW, height: rH },
        data: { label: `${cluster} — Rank ${rank}` },
        draggable: true,
        selectable: false,
      });

      // Place child nodes relative to group, centering the Dagre layout within the group
      const contentW = box.maxX - box.minX;
      const contentH = box.maxY - box.minY;
      const offsetX = PAD_X + (cW - 2 * PAD_X - contentW) / 2;
      const offsetY = PAD_TOP + (rH - PAD_TOP - PAD_BOTTOM - contentH) / 2;

      for (const p of box.nodes) {
        allNodes.push({
          id: p.raw.id,
          type: "flow",
          position: {
            x: offsetX + (p.x - box.minX),
            y: offsetY + (p.y - box.minY),
          },
          width: nodeW,
          height: NODE_H,
          parentId: groupId,
          extent: "parent" as const,
          data: {
            label: p.raw.label,
            cluster: p.raw.cluster,
            layer: p.raw.layer,
            layerColor: layerColorMap[p.raw.layer] || LAYER_PALETTE[LAYER_PALETTE.length - 1],
            width: nodeW,
            diff: diffStateOf(p.raw.id, ops, compareActive),
          } satisfies FlowNodeData,
        });
      }
    }
  }

  const edgeDiffStyle = (id: string): Record<string, unknown> => {
    if (!compareActive) return {};
    const c = ops.get(id);
    if (!c) return { style: { opacity: 0.35 } };
    if (c.op === "added") return { style: { stroke: "#22c55e", strokeWidth: 2.5 } };
    if (c.op === "removed") return { style: { stroke: "#ef4444", strokeWidth: 2, strokeDasharray: "2 4" } };
    return { style: { stroke: "#f59e0b", strokeWidth: 2.5, strokeDasharray: "8 4" } };
  };

  const edges: Edge[] = rawEdges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    type: "smartStep",
    ...edgeDiffStyle(e.id),
    ...(e.label
      ? {
          label: e.label,
          labelStyle: { fontSize: 10, fill: "#fbbf24", fontWeight: 600 },
          labelBgStyle: { fill: "rgba(15, 23, 42, 0.85)" },
          labelBgPadding: [4, 2] as [number, number],
          labelBgBorderRadius: 3,
        }
      : {}),
    ...(e.crossCluster
      ? {
          animated: true,
          style: { strokeDasharray: "8 4", stroke: "#f59e0b", strokeWidth: 2 },
        }
      : {}),
  }));

  return { nodes: allNodes, edges, layerColorMap };
}

export function FlowDiagram({
  content,
  diff = null,
}: {
  content: string;
  diff?: DiagramDiff | null;
}) {
  const { nodes, edges: baseEdges, layerColorMap } = useMemo(() => {
    const raw: FlowDataRaw = JSON.parse(content);
    const union = withRemoved(raw.nodes, raw.edges, diff);
    return buildLayout(union.nodes, union.edges, union.ops, diff !== null);
  }, [content, diff]);

  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null);

  const edges = useMemo(
    () =>
      baseEdges.map((e) => {
        const isSelected = e.id === selectedEdgeId;
        return {
          ...e,
          zIndex: isSelected ? 1000 : 0,
          selected: isSelected,
          style: isSelected
            ? { ...e.style, stroke: "#ff8c00", strokeWidth: 3.5 }
            : e.style,
        };
      }),
    [baseEdges, selectedEdgeId]
  );

  const onEdgeClick = useCallback((_: React.MouseEvent, edge: Edge) => {
    setSelectedEdgeId((prev) => (prev === edge.id ? null : edge.id));
  }, []);

  const onPaneClick = useCallback(() => {
    setSelectedEdgeId(null);
  }, []);

  return (
    <div className={styles.container}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        colorMode="dark"
        fitView
        nodesConnectable={false}
        deleteKeyCode={null}
        minZoom={0.1}
        maxZoom={2}
        onEdgeClick={onEdgeClick}
        onPaneClick={onPaneClick}
      >
        <Background gap={20} size={1} />
        <Controls showInteractive={false} />
        <MiniMap
          nodeColor={(n) => {
            if (n.type === "layerGroup") return "rgba(100, 116, 139, 0.15)";
            const layer = (n.data as Record<string, unknown>).layer as string;
            return layerColorMap[layer] || "#64748b";
          }}
          maskColor="rgba(0, 0, 0, 0.7)"
          pannable
          zoomable
        />
      </ReactFlow>
      <div className={styles.legend}>
        {diff && (
          <>
            <span className={styles.legendItem}>
              <span className={styles.legendGlyph}>+</span> added
            </span>
            <span className={styles.legendItem}>
              <span className={styles.legendGlyph}>−</span> removed
            </span>
            <span className={styles.legendItem}>
              <span className={styles.legendGlyph}>~</span> changed
            </span>
          </>
        )}
        {Object.entries(layerColorMap).map(([layer, color]) => (
          <span key={layer} className={styles.legendItem}>
            <span
              className={styles.legendSwatch}
              style={{ background: color }}
            />
            {layer}
          </span>
        ))}
      </div>
    </div>
  );
}
