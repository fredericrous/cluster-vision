import { useMemo } from "react";
import type { Route } from "./+types/circle-map";
import { fetchDiagram } from "../../api.server";
import { pack, hierarchy } from "d3-hierarchy";
import styles from "./eam.module.css";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Circle Map — Cluster Vision EAM" }];
}

export async function loader() {
  return fetchDiagram("dependencies");
}

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

interface FlowData {
  nodes: FlowNodeRaw[];
  edges: FlowEdgeRaw[];
}

const LAYER_COLORS: Record<string, string> = {
  crds: "#3b82f6",
  controllers: "#8b5cf6",
  "platform-foundation": "#f59e0b",
  security: "#ef4444",
  monitoring: "#06b6d4",
  apps: "#22c55e",
  "data-storage": "#ec4899",
};

function getLayerColor(layer: string): string {
  return LAYER_COLORS[layer] || "#64748b";
}

export default function CircleMap({ loaderData }: Route.ComponentProps) {
  const { diagram } = loaderData;

  const circleData = useMemo(() => {
    if (diagram.type !== "flow") return null;

    const raw: FlowData = JSON.parse(diagram.content);

    // Group nodes by layer
    const layers = new Map<string, FlowNodeRaw[]>();
    for (const node of raw.nodes) {
      if (!layers.has(node.layer)) layers.set(node.layer, []);
      layers.get(node.layer)!.push(node);
    }

    // Count edges per node for sizing
    const edgeCount = new Map<string, number>();
    for (const edge of raw.edges) {
      edgeCount.set(edge.source, (edgeCount.get(edge.source) || 0) + 1);
      edgeCount.set(edge.target, (edgeCount.get(edge.target) || 0) + 1);
    }

    // Build hierarchy for d3 pack layout
    const root = {
      name: "root",
      children: Array.from(layers.entries()).map(([layer, nodes]) => ({
        name: layer,
        children: nodes.map((n) => ({
          name: n.label,
          value: Math.max(1, edgeCount.get(n.id) || 1),
          layer,
        })),
      })),
    };

    const WIDTH = 800;
    const HEIGHT = 800;

    const h = hierarchy(root).sum((d: any) => d.value || 0);
    const packed = pack<any>().size([WIDTH, HEIGHT]).padding(8)(h);

    return { packed, width: WIDTH, height: HEIGHT };
  }, [diagram]);

  if (!circleData) {
    return <div className={styles.page}><h1 className={styles.heading}>Circle Map</h1><p>No flow data available.</p></div>;
  }

  const { packed, width, height } = circleData;

  return (
    <div className={styles.page}>
      <h1 className={styles.heading}>Circle Map</h1>
      <p className={styles.subtitle}>
        Flux kustomization layers as nested circles. Size proportional to dependency count.
      </p>

      <div className={styles.circleMapContainer}>
        <svg className={styles.circleMapSvg} viewBox={`0 0 ${width} ${height}`}>
          {packed.descendants().map((node, i) => {
            const depth = node.depth;
            const layer = node.data.layer || node.data.name;
            const color = getLayerColor(layer);

            if (depth === 0) return null; // skip root

            return (
              <g key={i}>
                <circle
                  cx={node.x}
                  cy={node.y}
                  r={node.r}
                  fill={depth === 1 ? `${color}20` : `${color}40`}
                  stroke={color}
                  strokeWidth={depth === 1 ? 2 : 1}
                  strokeOpacity={depth === 1 ? 0.6 : 0.3}
                />
                {/* Show label for layers (depth 1) and leaf nodes with enough space */}
                {(depth === 1 || (depth === 2 && node.r > 20)) && (
                  <text
                    x={node.x}
                    y={depth === 1 ? node.y - node.r + 16 : node.y}
                    textAnchor="middle"
                    dominantBaseline="central"
                    fill={depth === 1 ? color : "var(--text-primary)"}
                    fontSize={depth === 1 ? 13 : 9}
                    fontWeight={depth === 1 ? 600 : 400}
                    pointerEvents="none"
                  >
                    {node.data.name}
                  </text>
                )}
              </g>
            );
          })}
        </svg>
      </div>

      {/* Legend */}
      <div style={{ display: "flex", gap: "1rem", marginTop: "1rem", flexWrap: "wrap" }}>
        {Object.entries(LAYER_COLORS).map(([layer, color]) => (
          <div key={layer} style={{ display: "flex", alignItems: "center", gap: "0.25rem" }}>
            <span style={{ width: 12, height: 12, borderRadius: "50%", background: color, display: "inline-block" }} />
            <span style={{ fontSize: "0.75rem", color: "var(--text-secondary)" }}>{layer}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
