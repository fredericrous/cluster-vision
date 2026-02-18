import { Handle, Position, type NodeProps } from "@xyflow/react";
import styles from "./flow-node.module.css";

export interface FlowNodeData {
  label: string;
  cluster: string;
  layer: string;
  layerColor: string; // assigned dynamically from palette
  width: number; // computed from label measurement
}

const clusterBorderClass: Record<string, string> = {
  Homelab: styles.clusterHomelab,
  NAS: styles.clusterNAS,
};

export function FlowNode({ data }: NodeProps) {
  const d = data as unknown as FlowNodeData;
  const classes = [styles.node, clusterBorderClass[d.cluster] || ""]
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
      {d.label}
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}
