import { useCallback, useMemo, useState } from "react";
import { useNavigate } from "react-router";
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
import type { Route } from "./+types/dependency-graph";
import { fetchDependencyGraph } from "../../api.server";
import type { GraphNode, GraphEdge } from "../../api.server";
import styles from "./eam.module.css";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Dependency Graph — Cluster Vision EAM" }];
}

export async function loader() {
  return fetchDependencyGraph();
}

const STATUS_COLORS: Record<string, string> = {
  active: "#22c55e",
  maintenance: "#f59e0b",
  sunset: "#f97316",
  retired: "#ef4444",
};

function buildLayout(rawNodes: GraphNode[], rawEdges: GraphEdge[]) {
  const g = new dagre.graphlib.Graph();
  g.setGraph({ rankdir: "LR", nodesep: 60, ranksep: 120 });
  g.setDefaultEdgeLabel(() => ({}));

  const nodeW = 160;
  const nodeH = 50;

  for (const n of rawNodes) {
    g.setNode(n.id, { width: nodeW, height: nodeH });
  }
  for (const e of rawEdges) {
    g.setEdge(e.source, e.target);
  }

  dagre.layout(g);

  const nodes: Node[] = rawNodes.map((n) => {
    const pos = g.node(n.id);
    return {
      id: n.id,
      position: { x: pos.x - nodeW / 2, y: pos.y - nodeH / 2 },
      data: {
        label: n.display_name || n.name,
        status: n.status,
        risk: n.technical_risk,
      },
      style: {
        background: `${STATUS_COLORS[n.status] || "#64748b"}22`,
        border: `2px solid ${STATUS_COLORS[n.status] || "#64748b"}`,
        borderRadius: 8,
        padding: "8px 12px",
        fontSize: "0.8rem",
        fontWeight: 500,
        color: "var(--text-primary)",
        width: nodeW,
      },
    };
  });

  const edges: Edge[] = rawEdges.map((e, i) => ({
    id: `e-${i}`,
    source: e.source,
    target: e.target,
    animated: true,
    style: { stroke: "#64748b", strokeWidth: 1.5 },
  }));

  return { nodes, edges };
}

export default function DependencyGraph({ loaderData }: Route.ComponentProps) {
  const { nodes: rawNodes, edges: rawEdges } = loaderData;
  const navigate = useNavigate();

  // Extract unique filter values
  const allCapabilities = useMemo(() => {
    const s = new Set<string>();
    for (const n of rawNodes) n.capabilities?.forEach((c) => s.add(c));
    return Array.from(s).sort();
  }, [rawNodes]);

  const allStatuses = useMemo(() => [...new Set(rawNodes.map((n) => n.status))].sort(), [rawNodes]);
  const allClusters = useMemo(() => [...new Set(rawNodes.map((n) => n.cluster).filter(Boolean))].sort(), [rawNodes]);

  const [filterStatus, setFilterStatus] = useState<Set<string>>(new Set());
  const [filterCluster, setFilterCluster] = useState<Set<string>>(new Set());
  const [filterCap, setFilterCap] = useState<Set<string>>(new Set());
  const [highlightId, setHighlightId] = useState<string | null>(null);

  const filteredNodes = useMemo(() => {
    return rawNodes.filter((n) => {
      if (filterStatus.size > 0 && !filterStatus.has(n.status)) return false;
      if (filterCluster.size > 0 && !filterCluster.has(n.cluster)) return false;
      if (filterCap.size > 0 && !n.capabilities?.some((c) => filterCap.has(c))) return false;
      return true;
    });
  }, [rawNodes, filterStatus, filterCluster, filterCap]);

  const filteredNodeIds = useMemo(() => new Set(filteredNodes.map((n) => n.id)), [filteredNodes]);

  const filteredEdges = useMemo(() => {
    return rawEdges.filter((e) => filteredNodeIds.has(e.source) && filteredNodeIds.has(e.target));
  }, [rawEdges, filteredNodeIds]);

  const { nodes, edges } = useMemo(() => buildLayout(filteredNodes, filteredEdges), [filteredNodes, filteredEdges]);

  // Highlighting
  const highlightedEdges = useMemo(() => {
    if (!highlightId) return edges;
    const connected = new Set<string>();
    for (const e of filteredEdges) {
      if (e.source === highlightId || e.target === highlightId) {
        connected.add(e.source);
        connected.add(e.target);
      }
    }
    return edges.map((e) => {
      const isHighlighted =
        filteredEdges.some(
          (fe) =>
            (fe.source === highlightId || fe.target === highlightId) &&
            e.source === fe.source &&
            e.target === fe.target
        );
      return {
        ...e,
        style: isHighlighted
          ? { stroke: "#f59e0b", strokeWidth: 3 }
          : { ...e.style, opacity: highlightId ? 0.2 : 1 },
      };
    });
  }, [edges, filteredEdges, highlightId]);

  const highlightedNodes = useMemo(() => {
    if (!highlightId) return nodes;
    const connected = new Set<string>([highlightId]);
    for (const e of filteredEdges) {
      if (e.source === highlightId) connected.add(e.target);
      if (e.target === highlightId) connected.add(e.source);
    }
    return nodes.map((n) => ({
      ...n,
      style: {
        ...n.style,
        opacity: connected.has(n.id) ? 1 : 0.2,
      },
    }));
  }, [nodes, filteredEdges, highlightId]);

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setHighlightId((prev) => (prev === node.id ? null : node.id));
  }, []);

  const onNodeDoubleClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      navigate(`/eam/applications/${node.id}`);
    },
    [navigate]
  );

  const onPaneClick = useCallback(() => setHighlightId(null), []);

  function toggleFilter(set: Set<string>, val: string, setter: (s: Set<string>) => void) {
    const next = new Set(set);
    if (next.has(val)) next.delete(val);
    else next.add(val);
    setter(next);
  }

  return (
    <div className={styles.page} style={{ maxWidth: "none" }}>
      <h1 className={styles.heading}>Dependency Graph</h1>
      <p className={styles.subtitle}>
        Application-to-application dependencies. Click to highlight, double-click to open fact sheet.
      </p>

      <div className={styles.graphContainer}>
        <div className={styles.graphPanel}>
          {allStatuses.length > 0 && (
            <div className={styles.filterSection}>
              <div className={styles.filterTitle}>Status</div>
              {allStatuses.map((s) => (
                <label key={s} className={styles.filterItem}>
                  <input type="checkbox" checked={filterStatus.has(s)} onChange={() => toggleFilter(filterStatus, s, setFilterStatus)} />
                  {s}
                </label>
              ))}
            </div>
          )}
          {allClusters.length > 0 && (
            <div className={styles.filterSection}>
              <div className={styles.filterTitle}>Cluster</div>
              {allClusters.map((c) => (
                <label key={c} className={styles.filterItem}>
                  <input type="checkbox" checked={filterCluster.has(c)} onChange={() => toggleFilter(filterCluster, c, setFilterCluster)} />
                  {c}
                </label>
              ))}
            </div>
          )}
          {allCapabilities.length > 0 && (
            <div className={styles.filterSection}>
              <div className={styles.filterTitle}>Capability</div>
              {allCapabilities.map((c) => (
                <label key={c} className={styles.filterItem}>
                  <input type="checkbox" checked={filterCap.has(c)} onChange={() => toggleFilter(filterCap, c, setFilterCap)} />
                  {c}
                </label>
              ))}
            </div>
          )}
        </div>

        <div className={styles.graphCanvas}>
          {highlightedNodes.length > 0 ? (
            <ReactFlow
              nodes={highlightedNodes}
              edges={highlightedEdges}
              colorMode="dark"
              fitView
              nodesConnectable={false}
              deleteKeyCode={null}
              minZoom={0.1}
              maxZoom={2}
              onNodeClick={onNodeClick}
              onNodeDoubleClick={onNodeDoubleClick}
              onPaneClick={onPaneClick}
            >
              <Background gap={20} size={1} />
              <Controls showInteractive={false} />
              <MiniMap pannable zoomable maskColor="rgba(0, 0, 0, 0.7)" />
            </ReactFlow>
          ) : (
            <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100%", color: "var(--text-muted)" }}>
              No applications to display. Trigger a sync first.
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
