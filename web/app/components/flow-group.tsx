import type { NodeProps } from "@xyflow/react";
import styles from "./flow-group.module.css";

export function LayerGroup({ data }: NodeProps) {
  const label = (data as Record<string, unknown>).label as string;
  return (
    <div className={styles.group}>
      <span className={styles.label}>{label}</span>
    </div>
  );
}
