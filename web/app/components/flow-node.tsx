import { Handle, Position, type NodeProps } from "@xyflow/react";
import styles from "./flow-node.module.css";

export interface FlowNodeData {
  label: string;
  cluster: string;
  layer: string;
  showClusterBadge: boolean;
}

const layerClass: Record<string, string> = {
  Foundation: styles.foundation,
  Platform: styles.platform,
  Middleware: styles.middleware,
  Apps: styles.apps,
  Uncategorized: styles.uncategorized,
};

const clusterBorderClass: Record<string, string> = {
  Homelab: styles.clusterHomelab,
  NAS: styles.clusterNAS,
};

const badgeClass: Record<string, string> = {
  Homelab: styles.badgeHomelab,
  NAS: styles.badgeNAS,
};

export function FlowNode({ data }: NodeProps) {
  const d = data as unknown as FlowNodeData;
  const classes = [
    styles.node,
    layerClass[d.layer] || styles.uncategorized,
    clusterBorderClass[d.cluster] || "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={classes}>
      <Handle type="target" position={Position.Top} />
      {d.label}
      {d.showClusterBadge && (
        <span className={`${styles.badge} ${badgeClass[d.cluster] || ""}`}>
          {d.cluster}
        </span>
      )}
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}
