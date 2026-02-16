import { useMemo } from "react";
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
  layer: string; // unused — depth is computed from edges
}

interface FlowEdgeRaw {
  id: string;
  source: string;
  target: string;
}

interface FlowDataRaw {
  nodes: FlowNodeRaw[];
  edges: FlowEdgeRaw[];
}

const NODE_W = 150;
const NODE_H = 40;
const GAP_X = 30;
const GAP_Y = 16;
const PAD_X = 20;
const PAD_TOP = 32;
const PAD_BOTTOM = 16;
const LAYER_GAP = 40;
const MAX_COLS = 8;

const nodeTypes = { flow: FlowNode, layerGroup: LayerGroup };

const clusterColors: Record<string, string> = {
  Homelab: "#6366f1",
  NAS: "#14b8a6",
};

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
): { nodes: Node[]; edges: Edge[]; clusters: string[] } {
  const clusters = [...new Set(rawNodes.map((n) => n.cluster))];
  const showClusterBadge = clusters.length > 1;

  // Compute real deployment depth from dependency graph
  const depths = computeDepths(rawNodes, rawEdges);

  // Group by depth
  const byRank = new Map<number, FlowNodeRaw[]>();
  for (const n of rawNodes) {
    const d = depths.get(n.id) ?? 0;
    if (!byRank.has(d)) byRank.set(d, []);
    byRank.get(d)!.push(n);
  }

  // Order within ranks to minimize crossings
  minimizeCrossings(byRank, rawEdges);

  const ranks = [...byRank.keys()].sort((a, b) => a - b);

  // Uniform width across all ranks
  let maxCols = 0;
  for (const rank of ranks) {
    maxCols = Math.max(maxCols, Math.min(byRank.get(rank)!.length, MAX_COLS));
  }
  const uniformW = maxCols * (NODE_W + GAP_X) - GAP_X + 2 * PAD_X;

  const allNodes: Node[] = [];
  let currentY = 0;

  for (const rank of ranks) {
    const rankNodes = byRank.get(rank)!;
    const cols = Math.min(rankNodes.length, MAX_COLS);
    const rows = Math.ceil(rankNodes.length / cols);
    const groupW = Math.max(uniformW, cols * (NODE_W + GAP_X) - GAP_X + 2 * PAD_X);
    const groupH = PAD_TOP + rows * (NODE_H + GAP_Y) - GAP_Y + PAD_BOTTOM;

    const contentW = cols * (NODE_W + GAP_X) - GAP_X;
    const offsetX = PAD_X + (groupW - 2 * PAD_X - contentW) / 2;

    const groupId = `rank-${rank}`;
    allNodes.push({
      id: groupId,
      type: "layerGroup",
      position: { x: 0, y: currentY },
      style: { width: groupW, height: groupH },
      data: { label: `Rank ${rank}` },
      draggable: true,
      selectable: false,
    });

    for (let i = 0; i < rankNodes.length; i++) {
      const n = rankNodes[i];
      const col = i % cols;
      const row = Math.floor(i / cols);
      allNodes.push({
        id: n.id,
        type: "flow",
        position: {
          x: offsetX + col * (NODE_W + GAP_X),
          y: PAD_TOP + row * (NODE_H + GAP_Y),
        },
        parentId: groupId,
        extent: "parent" as const,
        data: {
          label: n.label,
          cluster: n.cluster,
          showClusterBadge,
        } satisfies FlowNodeData,
      });
    }

    currentY += groupH + LAYER_GAP;
  }

  const edges: Edge[] = rawEdges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    type: "smoothstep",
  }));

  return { nodes: allNodes, edges, clusters };
}

export function FlowDiagram({ content }: { content: string }) {
  const { nodes, edges, clusters } = useMemo(() => {
    const raw: FlowDataRaw = JSON.parse(content);
    return buildLayout(raw.nodes, raw.edges);
  }, [content]);

  const showClusterLegend = clusters.length > 1;

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
      >
        <Background gap={20} size={1} />
        <Controls showInteractive={false} />
        <MiniMap
          nodeColor={(n) => {
            if (n.type === "layerGroup") return "rgba(100, 116, 139, 0.3)";
            return "rgba(30, 41, 59, 0.9)";
          }}
          maskColor="rgba(0, 0, 0, 0.7)"
          pannable
          zoomable
        />
      </ReactFlow>
      {showClusterLegend && (
        <div className={styles.legend}>
          {clusters.map((cluster) => (
            <span key={cluster} className={styles.legendItem}>
              <span
                className={styles.legendSwatch}
                style={{ background: clusterColors[cluster] || "#64748b" }}
              />
              {cluster}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
