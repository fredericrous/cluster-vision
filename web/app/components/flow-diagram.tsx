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
  layer: string;
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

const LAYER_ORDER = ["Foundation", "Platform", "Middleware", "Apps", "Uncategorized"];

const NODE_W = 150;
const NODE_H = 40;
const GAP_X = 30;
const GAP_Y = 16;
const PAD_X = 20;
const PAD_TOP = 32; // space for layer label
const PAD_BOTTOM = 16;
const LAYER_GAP = 40;
const MAX_COLS = 8;

const nodeTypes = { flow: FlowNode, layerGroup: LayerGroup };

const layerMiniMapColors: Record<string, string> = {
  Foundation: "rgba(59, 130, 246, 0.5)",
  Platform: "rgba(139, 92, 246, 0.5)",
  Middleware: "rgba(245, 158, 11, 0.5)",
  Apps: "rgba(34, 197, 94, 0.5)",
  Uncategorized: "rgba(100, 116, 139, 0.5)",
};

const clusterColors: Record<string, string> = {
  Homelab: "#6366f1",
  NAS: "#14b8a6",
};

function buildLayout(
  rawNodes: FlowNodeRaw[],
  rawEdges: FlowEdgeRaw[]
): { nodes: Node[]; edges: Edge[]; clusters: string[] } {
  const clusters = [...new Set(rawNodes.map((n) => n.cluster))];
  const showClusterBadge = clusters.length > 1;

  // Group nodes by layer
  const byLayer = new Map<string, FlowNodeRaw[]>();
  for (const layer of LAYER_ORDER) byLayer.set(layer, []);
  for (const n of rawNodes) {
    const bucket = byLayer.get(n.layer) || byLayer.get("Uncategorized")!;
    bucket.push(n);
  }
  // Sort within each layer alphabetically
  for (const nodes of byLayer.values()) {
    nodes.sort((a, b) => a.label.localeCompare(b.label));
  }

  const allNodes: Node[] = [];
  let currentY = 0;

  for (const layer of LAYER_ORDER) {
    const layerNodes = byLayer.get(layer)!;
    if (layerNodes.length === 0) continue;

    const cols = Math.min(layerNodes.length, MAX_COLS);
    const rows = Math.ceil(layerNodes.length / cols);
    const groupW = cols * (NODE_W + GAP_X) - GAP_X + 2 * PAD_X;
    const groupH = PAD_TOP + rows * (NODE_H + GAP_Y) - GAP_Y + PAD_BOTTOM;

    const groupId = `group-${layer}`;
    allNodes.push({
      id: groupId,
      type: "layerGroup",
      position: { x: 0, y: currentY },
      style: { width: groupW, height: groupH },
      data: { label: layer },
      draggable: true,
      selectable: false,
    });

    for (let i = 0; i < layerNodes.length; i++) {
      const n = layerNodes[i];
      const col = i % cols;
      const row = Math.floor(i / cols);
      allNodes.push({
        id: n.id,
        type: "flow",
        position: {
          x: PAD_X + col * (NODE_W + GAP_X),
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
            if (n.type === "layerGroup") {
              const layer = (n.data as Record<string, unknown>).label as string;
              return layerMiniMapColors[layer] || layerMiniMapColors.Uncategorized;
            }
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
