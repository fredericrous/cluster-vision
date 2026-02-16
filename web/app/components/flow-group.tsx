import type { NodeProps } from "@xyflow/react";
import styles from "./flow-group.module.css";

const layerClass: Record<string, string> = {
  Foundation: styles.foundation,
  Platform: styles.platform,
  Middleware: styles.middleware,
  Apps: styles.apps,
  Uncategorized: styles.uncategorized,
};

export function LayerGroup({ data }: NodeProps) {
  const layer = (data as Record<string, unknown>).label as string;
  return (
    <div className={`${styles.group} ${layerClass[layer] || styles.uncategorized}`}>
      <span className={styles.label}>{layer}</span>
    </div>
  );
}
