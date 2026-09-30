-- One k8s_sources row per place an application runs. FindK8sSource used to
-- report every lookup error as "not found", so a sync that hit a transient
-- error inserted a second row next to the existing one; nothing stopped it.
--
-- Keep one row per identity: a manual override wins (it is hand-maintained
-- and sync never refreshes its last_sync_at), then the most recently synced,
-- then the highest id as a deterministic tie-break.
DELETE FROM k8s_sources k
USING (
    SELECT id,
           row_number() OVER (
               PARTITION BY app_id, cluster, namespace,
                            coalesce(helm_release, ''), coalesce(workload_name, '')
               ORDER BY manual_override DESC, last_sync_at DESC, id DESC
           ) AS rn
    FROM k8s_sources
) ranked
WHERE ranked.id = k.id AND ranked.rn > 1;

CREATE UNIQUE INDEX idx_k8s_sources_identity ON k8s_sources (
    app_id, cluster, namespace,
    (coalesce(helm_release, '')), (coalesce(workload_name, ''))
);
