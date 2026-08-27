import { Handle, Position, type NodeProps } from "@xyflow/react";
import styles from "./flow-node.module.css";

export interface FlowNodeData {
  label: string;
  cluster: string;
  layer: string;
  layerColor: string; // assigned dynamically from palette
  width: number; // computed from label measurement
  // Compare mode. `same` dims the node so changes stand out; each changed
  // state also carries a glyph and a border style, never colour alone.
  diff?: "added" | "removed" | "changed" | "same";
}

const diffClass: Record<NonNullable<FlowNodeData["diff"]>, string> = {
  added: styles.diffAdded,
  removed: styles.diffRemoved,
  changed: styles.diffChanged,
  same: styles.diffSame,
};

const diffGlyph: Record<NonNullable<FlowNodeData["diff"]>, string> = {
  added: "+",
  removed: "−",
  changed: "~",
  same: "",
};

const clusterBorderClass: Record<string, string> = {
  Homelab: styles.clusterHomelab,
  NAS: styles.clusterNAS,
};

export function FlowNode({ data }: NodeProps) {
  const d = data as unknown as FlowNodeData;
  const classes = [
    styles.node,
    clusterBorderClass[d.cluster] || "",
    d.diff ? diffClass[d.diff] : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div
      className={classes}
      style={{
        width: d.width,
        background: `${d.layerColor}33`, // 20% opacity
        borderColor: `${d.layerColor}66`, // 40% opacity
      }}
    >
      <Handle type="target" position={Position.Top} />
      {d.diff && d.diff !== "same" && (
        <span className={styles.glyph} aria-label={d.diff}>
          {diffGlyph[d.diff]}
        </span>
      )}
      {d.label}
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}
