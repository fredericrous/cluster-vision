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

// Cluster display order: NAS is deployed before Homelab.
const CLUSTER_ORDER: Record<string, number> = { NAS: 0, Homelab: 1 };

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

function buildLayout(
  rawNodes: FlowNodeRaw[],
  rawEdges: FlowEdgeRaw[]
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

  // Compute uniform rank heights across clusters (so ranks align horizontally)
  const allRanks = new Set<number>();
  for (const positioned of clusterResults.values()) {
    for (const p of positioned) allRanks.add(p.rank);
  }
  const sortedRanks = [...allRanks].sort((a, b) => a - b);

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

  // Compute uniform rank heights across all clusters at each rank
  const rankHeights = new Map<number, number>();
  for (const rank of sortedRanks) {
    let maxH = 0;
    for (const cluster of clusters) {
      const box = clusterRankBoxes.get(cluster)?.get(rank);
      if (box) maxH = Math.max(maxH, box.maxY - box.minY + PAD_TOP + PAD_BOTTOM);
    }
    rankHeights.set(rank, maxH);
  }

  // Compute cluster widths (max rank width per cluster)
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

  // Compute cluster X offsets (side by side)
  const clusterXMap = new Map<string, number>();
  let xCursor = 0;
  for (const cluster of clusters) {
    const w = clusterWidths.get(cluster);
    if (w === undefined) continue;
    clusterXMap.set(cluster, xCursor);
    xCursor += w + CLUSTER_GAP;
  }

  // Compute rank Y offsets (stacked vertically)
  const rankYMap = new Map<number, number>();
  let yCursor = 0;
  for (const rank of sortedRanks) {
    rankYMap.set(rank, yCursor);
    yCursor += rankHeights.get(rank)! + 20; // gap between rank groups
  }

  // Build ReactFlow nodes: group containers + child nodes
  const allNodes: Node[] = [];

  for (const cluster of clusters) {
    const boxes = clusterRankBoxes.get(cluster);
    if (!boxes) continue;
    const cX = clusterXMap.get(cluster)!;
    const cW = clusterWidths.get(cluster)!;

    for (const [rank, box] of boxes) {
      const rY = rankYMap.get(rank)!;
      const rH = rankHeights.get(rank)!;
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
          } satisfies FlowNodeData,
        });
      }
    }
  }

  const edges: Edge[] = rawEdges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    type: "smartStep",
    ...(e.crossCluster
      ? {
          animated: true,
          style: { strokeDasharray: "8 4", stroke: "#f59e0b", strokeWidth: 2 },
        }
      : {}),
  }));

  return { nodes: allNodes, edges, layerColorMap };
}

export function FlowDiagram({ content }: { content: string }) {
  const { nodes, edges: baseEdges, layerColorMap } = useMemo(() => {
    const raw: FlowDataRaw = JSON.parse(content);
    return buildLayout(raw.nodes, raw.edges);
  }, [content]);

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
