import type { CSSProperties } from "react";
import type { Change } from "../api.server";

export interface EdgeLook {
  animated?: boolean;
  style?: CSSProperties;
}

/** How a cross-cluster edge looks outside compare mode. */
const CROSS_CLUSTER: CSSProperties = { strokeDasharray: "8 4", stroke: "#f59e0b", strokeWidth: 2 };

/** Compare-mode styling by change op; `undefined` = unchanged edge. */
function diffStyle(change: Change | undefined): CSSProperties {
  if (!change) return { opacity: 0.35 };
  if (change.op === "added") return { stroke: "#22c55e", strokeWidth: 2.5 };
  if (change.op === "removed") return { stroke: "#ef4444", strokeWidth: 2, strokeDasharray: "2 4" };
  return { stroke: "#f59e0b", strokeWidth: 2.5, strokeDasharray: "8 4" };
}

/** Style of one flow edge. In compare mode the diff colour, width and dash
 *  win — an added, removed and unchanged cross-cluster edge must not look
 *  the same — while the animation stays as the cross-cluster cue (and the
 *  dash too, where the diff style sets none). */
export function edgeLook(
  crossCluster: boolean,
  change: Change | undefined,
  compareActive: boolean
): EdgeLook {
  const base = crossCluster ? CROSS_CLUSTER : {};
  const style = compareActive ? { ...base, ...diffStyle(change) } : base;
  const look: EdgeLook = {};
  if (crossCluster) look.animated = true;
  if (Object.keys(style).length > 0) look.style = style;
  return look;
}
