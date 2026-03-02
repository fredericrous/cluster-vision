import { Link } from "react-router";
import type { Route } from "./+types/fact-sheet";
import { fetchApplication, fetchAppVersionHistory } from "../../api.server";
import { Card, Badge } from "@fredericrous/duro-design-system";
import styles from "./eam.module.css";

export function meta({ data }: Route.MetaArgs) {
  const name = data?.application?.display_name || data?.application?.name || "Application";
  return [{ title: `${name} — Cluster Vision EAM` }];
}

export async function loader({ params }: Route.LoaderArgs) {
  const [detail, history] = await Promise.all([
    fetchApplication(params.id!),
    fetchAppVersionHistory(params.id!),
  ]);
  return { ...detail, version_history: history };
}

function SourceTag({ source }: { source: string }) {
  const colors: Record<string, string> = {
    "auto-discovered": "var(--text-muted)",
    "ai-inferred": "#a855f7",
    "human-verified": "#22c55e",
  };
  return (
    <span className={styles.sourceIndicator} style={{ color: colors[source] }}>
      [{source}]
    </span>
  );
}

function FieldRow({ label, value, source }: { label: string; value: string | null; source?: string }) {
  return (
    <div className={styles.fieldRow}>
      <span className={styles.fieldLabel}>{label}</span>
      <span className={styles.fieldValue}>
        {value || "—"}
        {source && <SourceTag source={source} />}
      </span>
    </div>
  );
}

export default function FactSheet({ loaderData }: Route.ComponentProps) {
  const { application: app, dependencies, components, capabilities, k8s_sources, version_history } = loaderData;

  const riskVariant = app.technical_risk === "high" ? "error" : app.technical_risk === "medium" ? "warning" : "success";
  const statusVariant = app.status === "active" ? "success" : app.status === "retired" ? "error" : "warning";

  return (
    <div className={styles.factSheet}>
      <div className={styles.factHeader}>
        <h1 className={styles.factTitle}>{app.display_name || app.name}</h1>
        <div className={styles.factBadges}>
          <Badge variant={statusVariant} size="sm">{app.status}</Badge>
          <Badge variant={riskVariant} size="sm">risk: {app.technical_risk}</Badge>
          <Badge variant="default" size="sm">{app.lifecycle_phase}</Badge>
          {app.time_category && <Badge variant="default" size="sm">TIME: {app.time_category}</Badge>}
        </div>
        <div className={styles.factActions}>
          <Link to={`/eam/applications/${app.id}/edit`}>
            <button className={styles.btnSecondary}>Edit</button>
          </Link>
        </div>
      </div>

      <div className={styles.cardGrid}>
        <Card header="Overview">
          <div className={styles.cardSection}>
            <FieldRow label="Description" value={app.description} source={app.description_source} />
            <FieldRow label="Criticality" value={app.business_criticality} source={app.business_criticality_source} />
            <FieldRow label="Technical Risk" value={app.technical_risk} source={app.technical_risk_source} />
            {app.technical_risk_reasoning && (
              <FieldRow label="Risk Reasoning" value={app.technical_risk_reasoning} />
            )}
            <FieldRow label="TIME Category" value={app.time_category} source={app.time_category_source} />
            {app.time_category_reasoning && (
              <FieldRow label="TIME Reasoning" value={app.time_category_reasoning} />
            )}
            {app.ai_confidence > 0 && (
              <FieldRow label="AI Confidence" value={`${Math.round(app.ai_confidence * 100)}%`} />
            )}
            {app.tags.length > 0 && (
              <div className={styles.fieldRow}>
                <span className={styles.fieldLabel}>Tags</span>
                <div className={styles.tagList}>
                  {app.tags.map((tag) => (
                    <Badge key={tag} variant="default" size="sm">{tag}</Badge>
                  ))}
                </div>
              </div>
            )}
          </div>
        </Card>

        <Card header="K8s Runtime">
          <div className={styles.cardSection}>
            {k8s_sources && k8s_sources.length > 0 ? (
              k8s_sources.map((src) => (
                <div key={src.id}>
                  <FieldRow label="Cluster" value={src.cluster} />
                  <FieldRow label="Namespace" value={src.namespace} />
                  {src.helm_release && <FieldRow label="Helm Release" value={src.helm_release} />}
                  {src.chart_name && <FieldRow label="Chart" value={`${src.chart_name} ${src.chart_version || ""}`} />}
                  {src.images && src.images.length > 0 && (
                    <div className={styles.fieldRow}>
                      <span className={styles.fieldLabel}>Images</span>
                      <div className={styles.imageList}>
                        {src.images.map((img) => (
                          <span key={img}>{img}</span>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              ))
            ) : (
              <p style={{ color: "var(--text-muted)", fontSize: "0.8125rem" }}>No K8s sources discovered</p>
            )}
          </div>
        </Card>

        <Card header="Dependencies">
          <div className={styles.cardSection}>
            {dependencies && dependencies.length > 0 ? (
              dependencies.map((dep) => (
                <div key={`${dep.source_app_id}-${dep.target_app_id}`} className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>
                    {dep.source_app_id === app.id ? "Depends on" : "Depended by"}
                  </span>
                  <Link
                    to={`/eam/applications/${dep.source_app_id === app.id ? dep.target_app_id : dep.source_app_id}`}
                    style={{ color: "var(--accent)", textDecoration: "none", fontSize: "0.8125rem" }}
                  >
                    {dep.description || (dep.source_app_id === app.id ? dep.target_app_id : dep.source_app_id)}
                  </Link>
                </div>
              ))
            ) : (
              <p style={{ color: "var(--text-muted)", fontSize: "0.8125rem" }}>No dependencies</p>
            )}
          </div>
        </Card>

        <Card header="IT Components">
          <div className={styles.cardSection}>
            {components && components.length > 0 ? (
              components.map((comp) => (
                <div key={comp.id} className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>{comp.type}</span>
                  <span className={styles.fieldValue}>{comp.name} {comp.version || ""}</span>
                </div>
              ))
            ) : (
              <p style={{ color: "var(--text-muted)", fontSize: "0.8125rem" }}>No linked components</p>
            )}
          </div>
        </Card>

        <Card header="Business Capabilities">
          <div className={styles.cardSection}>
            {capabilities && capabilities.length > 0 ? (
              capabilities.map((cap) => (
                <div key={cap.id} className={styles.fieldRow}>
                  <span className={styles.fieldValue}>{cap.name}</span>
                </div>
              ))
            ) : (
              <p style={{ color: "var(--text-muted)", fontSize: "0.8125rem" }}>No capabilities mapped</p>
            )}
          </div>
        </Card>

        <Card header="Version History">
          <div className={styles.cardSection}>
            {version_history && version_history.length > 0 ? (
              version_history.slice(0, 10).map((entry) => (
                <div key={entry.id} className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>
                    {new Date(entry.recorded_at).toLocaleDateString()}
                  </span>
                  <span className={styles.fieldValue}>
                    {entry.chart_version || entry.image_tag || "—"}
                    {entry.outdated && <span style={{ marginLeft: "0.375rem" }}><Badge variant="error" size="sm">outdated</Badge></span>}
                    {entry.vuln_critical > 0 && (
                      <span style={{ marginLeft: "0.375rem" }}>
                        <Badge variant="error" size="sm">{entry.vuln_critical} crit</Badge>
                      </span>
                    )}
                  </span>
                </div>
              ))
            ) : (
              <p style={{ color: "var(--text-muted)", fontSize: "0.8125rem" }}>No version history</p>
            )}
          </div>
        </Card>

        <Card header="Lifecycle">
          <div className={styles.cardSection}>
            <FieldRow label="Phase" value={app.lifecycle_phase} />
            <FieldRow label="End of Life" value={app.end_of_life_date} />
            <FieldRow label="Created" value={new Date(app.created_at).toLocaleDateString()} />
            <FieldRow label="Last Updated" value={new Date(app.updated_at).toLocaleDateString()} />
          </div>
        </Card>
      </div>
    </div>
  );
}
