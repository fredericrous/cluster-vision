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

const NODE_W = 160;
const NODE_H = 44;
const GAP_X = 30;
const GAP_Y = 20;
const PAD_X = 20;
const PAD_TOP = 32;
const PAD_BOTTOM = 16;
const LAYER_GAP = 40;
const CLUSTER_GAP = 60;
const MAX_COLS = 8;

// Cluster display order: NAS is deployed before Homelab.
const CLUSTER_ORDER: Record<string, number> = { NAS: 0, Homelab: 1 };

const nodeTypes = { flow: FlowNode, layerGroup: LayerGroup };

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


// Compute topological depth: 0 = no deps, N = longest chain of deps.
function computeDepths(
  nodes: FlowNodeRaw[],
  edges: FlowEdgeRaw[]
): Map<string, number> {
  const deps = new Map<string, Set<string>>();
  for (const n of nodes) deps.set(n.id, new Set());
  for (const e of edges) deps.get(e.target)?.add(e.source);

  const depth = new Map<string, number>();

  function resolve(id: string): number {
    if (depth.has(id)) return depth.get(id)!;
    const d = deps.get(id);
    if (!d || d.size === 0) {
      depth.set(id, 0);
      return 0;
    }
    // Temporarily mark to detect cycles
    depth.set(id, 0);
    let max = 0;
    for (const dep of d) max = Math.max(max, resolve(dep) + 1);
    depth.set(id, max);
    return max;
  }

  for (const n of nodes) resolve(n.id);
  return depth;
}

// Barycenter heuristic: order nodes within each rank so edges go
// as straight down as possible, minimizing crossings.
function minimizeCrossings(
  byRank: Map<number, FlowNodeRaw[]>,
  edges: FlowEdgeRaw[]
): void {
  const upNeighbors = new Map<string, string[]>();
  const downNeighbors = new Map<string, string[]>();
  for (const e of edges) {
    if (!upNeighbors.has(e.target)) upNeighbors.set(e.target, []);
    upNeighbors.get(e.target)!.push(e.source);
    if (!downNeighbors.has(e.source)) downNeighbors.set(e.source, []);
    downNeighbors.get(e.source)!.push(e.target);
  }

  const nodeIndex = new Map<string, number>();
  const ranks = [...byRank.keys()].sort((a, b) => a - b);

  // Initialize alphabetical
  for (const rank of ranks) {
    const nodes = byRank.get(rank)!;
    nodes.sort((a, b) => a.label.localeCompare(b.label));
    for (let i = 0; i < nodes.length; i++) nodeIndex.set(nodes[i].id, i);
  }

  function sortByBarycenter(
    nodes: FlowNodeRaw[],
    getNeighbors: (id: string) => string[]
  ) {
    const bary = new Map<string, number>();
    for (const n of nodes) {
      const nbrs = getNeighbors(n.id);
      if (nbrs.length === 0) continue;
      let sum = 0, count = 0;
      for (const nbr of nbrs) {
        if (nodeIndex.has(nbr)) { sum += nodeIndex.get(nbr)!; count++; }
      }
      if (count > 0) bary.set(n.id, sum / count);
    }
    nodes.sort((a, b) => {
      const ba = bary.get(a.id);
      const bb = bary.get(b.id);
      if (ba !== undefined && bb !== undefined) return ba - bb;
      if (ba !== undefined) return -1;
      if (bb !== undefined) return 1;
      return a.label.localeCompare(b.label);
    });
    for (let i = 0; i < nodes.length; i++) nodeIndex.set(nodes[i].id, i);
  }

  // Down pass then up pass
  for (const rank of ranks) {
    sortByBarycenter(byRank.get(rank)!, (id) => upNeighbors.get(id) || []);
  }
  for (let i = ranks.length - 1; i >= 0; i--) {
    sortByBarycenter(byRank.get(ranks[i])!, (id) => downNeighbors.get(id) || []);
  }
}

function buildLayout(
  rawNodes: FlowNodeRaw[],
  rawEdges: FlowEdgeRaw[]
): { nodes: Node[]; edges: Edge[]; layerColorMap: Record<string, string> } {
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

  // Phase 1: compute ranks per cluster
  type RankInfo = {
    nodes: FlowNodeRaw[];
    cols: number;
    rows: number;
    naturalW: number; // width this rank needs (with padding)
    naturalH: number; // height this rank needs (with padding)
  };

  const clusterRanks = new Map<string, Map<number, RankInfo>>();
  let maxRank = 0;

  for (const cluster of clusters) {
    const cNodes = nodesByCluster.get(cluster) || [];
    const cEdges = edgesByCluster.get(cluster) || [];
    if (cNodes.length === 0) continue;

    const depths = computeDepths(cNodes, cEdges);
    const byRank = new Map<number, FlowNodeRaw[]>();
    for (const n of cNodes) {
      const d = depths.get(n.id) ?? 0;
      if (!byRank.has(d)) byRank.set(d, []);
      byRank.get(d)!.push(n);
    }
    minimizeCrossings(byRank, cEdges);

    const rankInfos = new Map<number, RankInfo>();
    for (const [rank, nodes] of byRank) {
      maxRank = Math.max(maxRank, rank);
      const cols = Math.min(nodes.length, MAX_COLS);
      const rows = Math.ceil(nodes.length / cols);
      const naturalW = cols * (NODE_W + GAP_X) - GAP_X + 2 * PAD_X;
      const naturalH = PAD_TOP + rows * (NODE_H + GAP_Y) - GAP_Y + PAD_BOTTOM;
      rankInfos.set(rank, { nodes, cols, rows, naturalW, naturalH });
    }
    clusterRanks.set(cluster, rankInfos);
  }

  // Phase 2: uniform dimensions across clusters
  // clusterWidth = max naturalW across all ranks in that cluster
  const clusterWidths = new Map<string, number>();
  for (const cluster of clusters) {
    const rankInfos = clusterRanks.get(cluster);
    if (!rankInfos) continue;
    let maxW = 0;
    for (const info of rankInfos.values()) maxW = Math.max(maxW, info.naturalW);
    clusterWidths.set(cluster, maxW);
  }

  // rankHeight = max naturalH across all clusters at that rank
  const rankHeights = new Map<number, number>();
  for (let r = 0; r <= maxRank; r++) {
    let maxH = 0;
    for (const cluster of clusters) {
      const info = clusterRanks.get(cluster)?.get(r);
      if (info) maxH = Math.max(maxH, info.naturalH);
    }
    if (maxH > 0) rankHeights.set(r, maxH);
  }

  // Phase 3: compute offsets (columns for clusters, rows for ranks)
  const clusterX = new Map<string, number>();
  let xCursor = 0;
  for (const cluster of clusters) {
    const w = clusterWidths.get(cluster);
    if (w === undefined) continue;
    clusterX.set(cluster, xCursor);
    xCursor += w + CLUSTER_GAP;
  }

  const rankYMap = new Map<number, number>();
  let yCursor = 0;
  const sortedRanks = [...rankHeights.keys()].sort((a, b) => a - b);
  for (const r of sortedRanks) {
    rankYMap.set(r, yCursor);
    yCursor += rankHeights.get(r)! + LAYER_GAP;
  }

  // Phase 4: create group + child nodes
  const allNodes: Node[] = [];

  for (const cluster of clusters) {
    const rankInfos = clusterRanks.get(cluster);
    if (!rankInfos) continue;
    const cX = clusterX.get(cluster)!;
    const cW = clusterWidths.get(cluster)!;

    for (const [rank, info] of rankInfos) {
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

      // Center the grid of child nodes within the (possibly larger) group
      const gridW = info.cols * (NODE_W + GAP_X) - GAP_X;
      const gridH = info.rows * (NODE_H + GAP_Y) - GAP_Y;
      const offsetX = PAD_X + (cW - 2 * PAD_X - gridW) / 2;
      const offsetY = PAD_TOP + (rH - PAD_TOP - PAD_BOTTOM - gridH) / 2;

      for (let i = 0; i < info.nodes.length; i++) {
        const n = info.nodes[i];
        const col = i % info.cols;
        const row = Math.floor(i / info.cols);
        allNodes.push({
          id: n.id,
          type: "flow",
          position: {
            x: offsetX + col * (NODE_W + GAP_X),
            y: offsetY + row * (NODE_H + GAP_Y),
          },
          parentId: groupId,
          extent: "parent" as const,
          data: {
            label: n.label,
            cluster: n.cluster,
            layer: n.layer,
            layerColor: layerColorMap[n.layer] || LAYER_PALETTE[LAYER_PALETTE.length - 1],
          } satisfies FlowNodeData,
        });
      }
    }
  }

  const edges: Edge[] = rawEdges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    type: "smoothstep",
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
      baseEdges.map((e) => ({
        ...e,
        zIndex: e.id === selectedEdgeId ? 1000 : 0,
      })),
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
