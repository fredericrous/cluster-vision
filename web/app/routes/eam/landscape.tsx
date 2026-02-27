import { Link } from "react-router";
import type { Route } from "./+types/landscape";
import { fetchLandscape } from "../../api.server";
import type { LandscapeCapability, LandscapeApp } from "../../api.server";
import styles from "./eam.module.css";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Application Landscape — Cluster Vision EAM" }];
}

export async function loader() {
  return fetchLandscape();
}

function riskClass(app: LandscapeApp): string {
  if (app.vuln_critical > 0 || app.technical_risk === "high") return styles.riskHigh;
  if (app.vuln_high > 0 || app.technical_risk === "medium") return styles.riskMedium;
  return styles.riskLow;
}

function CapabilitySection({ cap, depth = 0 }: { cap: LandscapeCapability; depth?: number }) {
  if (cap.level === 1) {
    return (
      <div className={styles.capabilitySection}>
        <div className={styles.capabilityHeader}>{cap.name}</div>
        {cap.apps.length > 0 && (
          <div className={styles.capabilityRow}>
            <span className={styles.capabilityLabel}>(direct)</span>
            <div className={styles.appBadges}>
              {cap.apps.map((app) => (
                <Link key={app.id} to={`/eam/applications/${app.id}`} className={`${styles.appBadge} ${riskClass(app)}`}>
                  {app.display_name || app.name}
                </Link>
              ))}
            </div>
          </div>
        )}
        {cap.children.map((child) => (
          <CapabilitySection key={child.id} cap={child} depth={depth + 1} />
        ))}
      </div>
    );
  }

  return (
    <div className={styles.capabilityRow}>
      <span className={styles.capabilityLabel} style={{ paddingLeft: `${(depth - 1) * 1}rem` }}>
        {cap.name}
      </span>
      <div className={styles.appBadges}>
        {cap.apps.map((app) => (
          <Link key={app.id} to={`/eam/applications/${app.id}`} className={`${styles.appBadge} ${riskClass(app)}`}>
            {app.display_name || app.name}
          </Link>
        ))}
      </div>
    </div>
  );
}

export default function Landscape({ loaderData }: Route.ComponentProps) {
  const { capabilities, unmapped } = loaderData;

  return (
    <div className={styles.page}>
      <h1 className={styles.heading}>Application Landscape</h1>
      <p className={styles.subtitle}>
        Business capabilities mapped to applications. Color indicates risk level.
      </p>

      <div className={styles.landscapeGrid}>
        {capabilities?.map((cap) => (
          <CapabilitySection key={cap.id} cap={cap} />
        ))}

        {unmapped && unmapped.length > 0 && (
          <div className={styles.capabilitySection}>
            <div className={styles.capabilityHeader}>Unmapped Applications</div>
            <div className={styles.capabilityRow}>
              <span className={styles.capabilityLabel}>No capability assigned</span>
              <div className={styles.appBadges}>
                {unmapped.map((app) => (
                  <Link key={app.id} to={`/eam/applications/${app.id}`} className={`${styles.appBadge} ${riskClass(app)}`}>
                    {app.display_name || app.name}
                  </Link>
                ))}
              </div>
            </div>
          </div>
        )}

        {(!capabilities || capabilities.length === 0) && (!unmapped || unmapped.length === 0) && (
          <p style={{ color: "var(--text-muted)" }}>
            No applications or capabilities yet. Trigger a sync from the Import/Sync page, then map capabilities.
          </p>
        )}
      </div>
    </div>
  );
}
