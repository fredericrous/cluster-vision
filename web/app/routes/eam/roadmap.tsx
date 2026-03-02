import { useMemo } from "react";
import type { Route } from "./+types/roadmap";
import { fetchRoadmap } from "../../api.server";
import { Badge } from "@fredericrous/duro-design-system";
import styles from "./eam.module.css";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Roadmap — Cluster Vision EAM" }];
}

export async function loader() {
  return fetchRoadmap();
}

const phaseOrder = ["plan", "phase_in", "active", "phase_out", "end_of_life"];
const phaseColors: Record<string, string> = {
  plan: "#3b82f6",
  phase_in: "#22c55e",
  active: "#10b981",
  phase_out: "#f59e0b",
  end_of_life: "#ef4444",
};

const ROW_HEIGHT = 32;
const LABEL_WIDTH = 180;
const BAR_START = 200;
const BAR_WIDTH = 600;
const PADDING = 16;

export default function Roadmap({ loaderData }: Route.ComponentProps) {
  const apps = loaderData ?? [];

  const sorted = useMemo(() => {
    return [...apps].sort((a, b) => {
      const ai = phaseOrder.indexOf(a.lifecycle_phase);
      const bi = phaseOrder.indexOf(b.lifecycle_phase);
      if (ai !== bi) return ai - bi;
      return a.name.localeCompare(b.name);
    });
  }, [apps]);

  const svgHeight = sorted.length * ROW_HEIGHT + PADDING * 2;
  const svgWidth = BAR_START + BAR_WIDTH + PADDING;

  return (
    <div className={styles.page}>
      <h1 className={styles.heading}>Roadmap Report</h1>
      <p className={styles.subtitle}>
        Application lifecycle phases. Bars show current phase positioning.
      </p>

      <div className={styles.roadmapContainer}>
        <svg className={styles.roadmapSvg} width={svgWidth} height={svgHeight} viewBox={`0 0 ${svgWidth} ${svgHeight}`}>
          {/* Phase headers */}
          {phaseOrder.map((phase, i) => {
            const x = BAR_START + (i / phaseOrder.length) * BAR_WIDTH;
            const w = BAR_WIDTH / phaseOrder.length;
            return (
              <g key={phase}>
                <rect x={x} y={0} width={w} height={PADDING} fill={phaseColors[phase]} opacity={0.2} />
                <text x={x + w / 2} y={PADDING - 3} textAnchor="middle" fill="var(--text-secondary)" fontSize="10" fontWeight="500">
                  {phase.replace("_", " ")}
                </text>
              </g>
            );
          })}

          {/* App rows */}
          {sorted.map((app, i) => {
            const y = PADDING + i * ROW_HEIGHT;
            const phaseIdx = phaseOrder.indexOf(app.lifecycle_phase);
            const barX = BAR_START + (phaseIdx / phaseOrder.length) * BAR_WIDTH;
            const barW = BAR_WIDTH / phaseOrder.length;
            const color = phaseColors[app.lifecycle_phase] || "#64748b";

            return (
              <g key={app.id}>
                {/* Alternating row background */}
                {i % 2 === 0 && (
                  <rect x={0} y={y} width={svgWidth} height={ROW_HEIGHT} fill="var(--bg-secondary)" opacity={0.3} />
                )}
                {/* App name */}
                <text x={8} y={y + ROW_HEIGHT / 2 + 4} fill="var(--text-primary)" fontSize="12" fontWeight="500">
                  {app.display_name || app.name}
                </text>
                {/* Phase bar */}
                <rect x={barX + 2} y={y + 6} width={barW - 4} height={ROW_HEIGHT - 12} rx={4} fill={color} opacity={0.7} />
                {/* EOL marker */}
                {app.end_of_life_date && (
                  <circle cx={BAR_START + BAR_WIDTH - 8} cy={y + ROW_HEIGHT / 2} r={4} fill="#ef4444" />
                )}
              </g>
            );
          })}
        </svg>
      </div>

      {/* Legend */}
      <div style={{ display: "flex", gap: "1rem", marginTop: "1rem", flexWrap: "wrap" }}>
        {phaseOrder.map((phase) => (
          <div key={phase} style={{ display: "flex", alignItems: "center", gap: "0.25rem" }}>
            <span style={{ width: 12, height: 12, borderRadius: 2, background: phaseColors[phase], display: "inline-block" }} />
            <span style={{ fontSize: "0.75rem", color: "var(--text-secondary)" }}>{phase.replace("_", " ")}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
