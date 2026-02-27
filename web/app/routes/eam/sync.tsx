import { useState, useEffect } from "react";
import type { Route } from "./+types/sync";
import { fetchSyncLogs } from "../../api.server";
import type { SyncLog } from "../../api.server";
import styles from "./eam.module.css";

const API_URL = "/api";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Import / Sync — Cluster Vision EAM" }];
}

export async function loader() {
  return fetchSyncLogs();
}

async function clientTriggerSync() {
  const res = await fetch(`${API_URL}/eam/sync/trigger`, { method: "POST" });
  if (!res.ok) throw new Error("Sync failed");
  return res.json();
}

async function clientFetchSyncLogs(): Promise<SyncLog[]> {
  const res = await fetch(`${API_URL}/eam/sync/logs`);
  if (!res.ok) throw new Error("Failed to fetch logs");
  return res.json();
}

async function clientTriggerEnrich() {
  const res = await fetch(`${API_URL}/eam/enrich`, { method: "POST" });
  if (!res.ok) throw new Error("Enrichment failed");
  return res.json();
}

export default function Sync({ loaderData }: Route.ComponentProps) {
  const [logs, setLogs] = useState<SyncLog[]>(loaderData ?? []);
  const [syncing, setSyncing] = useState(false);
  const [enriching, setEnriching] = useState(false);
  const [aiEnabled, setAiEnabled] = useState(false);
  const [lastResult, setLastResult] = useState<any>(null);

  useEffect(() => {
    fetch(`${API_URL}/config`).then(r => r.json()).then(c => setAiEnabled(c.ai)).catch(() => {});
  }, []);

  async function handleSync() {
    setSyncing(true);
    setLastResult(null);
    try {
      const result = await clientTriggerSync();
      setLastResult(result);
      const refreshed = await clientFetchSyncLogs();
      setLogs(refreshed);
    } catch (err) {
      alert(`Sync failed: ${err}`);
    } finally {
      setSyncing(false);
    }
  }

  async function handleEnrich() {
    setEnriching(true);
    try {
      await clientTriggerEnrich();
      alert("AI enrichment started in background. Refresh pages to see results.");
    } catch (err) {
      alert(`Enrichment failed: ${err}`);
    } finally {
      setEnriching(false);
    }
  }

  async function refreshLogs() {
    const refreshed = await clientFetchSyncLogs();
    setLogs(refreshed);
  }

  return (
    <div className={styles.page}>
      <h1 className={styles.heading}>Import / Sync</h1>
      <p className={styles.subtitle}>
        Auto-discover applications from Kubernetes cluster data. Sync maps HelmReleases, workloads, nodes, and storage into EAM entities.
      </p>

      <div className={styles.syncStatus}>
        <div style={{ display: "flex", gap: "0.75rem", alignItems: "center" }}>
          <button className={styles.btnPrimary} onClick={handleSync} disabled={syncing}>
            {syncing ? "Syncing..." : "Sync Now"}
          </button>
          <button className={styles.btnSecondary} onClick={refreshLogs}>
            Refresh Logs
          </button>
          {aiEnabled && (
            <button className={styles.btnSecondary} onClick={handleEnrich} disabled={enriching}>
              {enriching ? "Analyzing..." : "Re-analyze with AI"}
            </button>
          )}
        </div>

        {lastResult && (
          <div style={{ padding: "0.75rem", background: "var(--bg-secondary)", borderRadius: 8, fontSize: "0.8125rem" }}>
            <strong>Last sync result:</strong>{" "}
            {lastResult.AppsCreated ?? lastResult.apps_created ?? 0} created,{" "}
            {lastResult.AppsUpdated ?? lastResult.apps_updated ?? 0} updated,{" "}
            {lastResult.ComponentsCreated ?? lastResult.components_created ?? 0} components
            {(lastResult.Errors?.length > 0 || lastResult.errors?.length > 0) && (
              <span style={{ color: "#ef4444" }}>
                , {(lastResult.Errors || lastResult.errors).length} errors
              </span>
            )}
          </div>
        )}
      </div>

      <h2 style={{ fontSize: "1rem", fontWeight: 600, margin: "0 0 0.75rem" }}>Sync History</h2>

      {logs.length > 0 ? (
        <div>
          {logs.map((log) => (
            <div key={log.id} className={styles.logEntry}>
              <div className={styles.logStats}>
                <span className={styles.logStat}>+{log.apps_created} created</span>
                <span className={styles.logStat}>{log.apps_updated} updated</span>
                <span className={styles.logStat}>{log.components_created} components</span>
                {log.errors.length > 0 && (
                  <span style={{ color: "#ef4444" }}>{log.errors.length} errors</span>
                )}
              </div>
              <div className={styles.logTime}>
                {new Date(log.started_at).toLocaleString()}
                {log.finished_at && (
                  <> ({Math.round((new Date(log.finished_at).getTime() - new Date(log.started_at).getTime()) / 1000)}s)</>
                )}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <p style={{ color: "var(--text-muted)", fontSize: "0.8125rem" }}>
          No sync logs yet. Click "Sync Now" to auto-discover applications from your clusters.
        </p>
      )}
    </div>
  );
}
