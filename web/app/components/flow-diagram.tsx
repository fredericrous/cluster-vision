import { useMemo } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  type Node,
  type Edge,
} from "@xyflow/react";
import dagre from "@dagrejs/dagre";
import "@xyflow/react/dist/style.css";
import { FlowNode, type FlowNodeData } from "./flow-node";
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

const NODE_WIDTH = 150;
const NODE_HEIGHT = 40;

const nodeTypes = { flow: FlowNode };

const layerColors: Record<string, string> = {
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

function layoutNodes(
  rawNodes: FlowNodeRaw[],
  rawEdges: FlowEdgeRaw[]
): { nodes: Node[]; edges: Edge[] } {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: "TB", nodesep: 60, ranksep: 80 });

  const clusters = new Set(rawNodes.map((n) => n.cluster));
  const showClusterBadge = clusters.size > 1;

  for (const n of rawNodes) {
    g.setNode(n.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  }
  for (const e of rawEdges) {
    g.setEdge(e.source, e.target);
  }

  dagre.layout(g);

  const nodes: Node[] = rawNodes.map((n) => {
    const pos = g.node(n.id);
    return {
      id: n.id,
      type: "flow",
      position: { x: pos.x - NODE_WIDTH / 2, y: pos.y - NODE_HEIGHT / 2 },
      data: {
        label: n.label,
        cluster: n.cluster,
        layer: n.layer,
        showClusterBadge,
      } satisfies FlowNodeData,
    };
  });

  const edges: Edge[] = rawEdges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    animated: false,
  }));

  return { nodes, edges };
}

export function FlowDiagram({ content }: { content: string }) {
  const { nodes, edges, clusters } = useMemo(() => {
    const raw: FlowDataRaw = JSON.parse(content);
    const { nodes, edges } = layoutNodes(raw.nodes, raw.edges);
    const clusters = [...new Set(raw.nodes.map((n) => n.cluster))];
    return { nodes, edges, clusters };
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
        minZoom={0.2}
        maxZoom={2}
      >
        <Background gap={20} size={1} />
        <Controls showInteractive={false} />
        <MiniMap
          nodeColor={(n) => {
            const d = n.data as unknown as FlowNodeData;
            return layerColors[d.layer] || layerColors.Uncategorized;
          }}
          maskColor="rgba(0, 0, 0, 0.7)"
          pannable
          zoomable
        />
      </ReactFlow>
      <div className={styles.legend}>
        {Object.entries(layerColors).map(([layer, color]) => (
          <span key={layer} className={styles.legendItem}>
            <span
              className={styles.legendSwatch}
              style={{ background: color }}
            />
            {layer}
          </span>
        ))}
        {showClusterLegend && (
          <>
            <span className={styles.legendDivider} />
            {clusters.map((cluster) => (
              <span key={cluster} className={styles.legendItem}>
                <span
                  className={styles.legendSwatch}
                  style={{
                    background: clusterColors[cluster] || "#64748b",
                    borderRadius: "1px",
                  }}
                />
                {cluster}
              </span>
            ))}
          </>
        )}
      </div>
    </div>
  );
}
