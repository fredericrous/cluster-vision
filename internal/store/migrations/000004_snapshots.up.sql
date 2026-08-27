-- Cluster snapshots: the observed cluster model at a point in time, plus the
-- desired-state revisions (root Flux Kustomizations) it was observed under.
-- Diagrams are regenerated from `data` on read, so a snapshot never encodes
-- how we drew things — only what the cluster looked like.
CREATE TABLE snapshots (
    id            uuid PRIMARY KEY,
    taken_at      timestamptz NOT NULL,
    observed_hash bytea NOT NULL,          -- sha256 over observed fields + revisions
    model_version smallint NOT NULL,       -- bump when model.ClusterData changes shape
    revisions     jsonb NOT NULL,          -- []diff.Revision, sorted; equality = same desired state
    summary       jsonb NOT NULL,          -- per-diagram change counts vs. the previous snapshot
    data          jsonb NOT NULL           -- model.ClusterData
);
CREATE INDEX idx_snapshots_taken_at ON snapshots (taken_at DESC);

-- One row per (cluster, root kustomization) so a git sha (or prefix) can be
-- resolved to a snapshot with a plain btree.
CREATE TABLE snapshot_revisions (
    snapshot_id   uuid NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    cluster       text NOT NULL,
    kustomization text NOT NULL,
    source_kind   text NOT NULL,
    revision      text NOT NULL,
    sha           text NOT NULL,
    PRIMARY KEY (snapshot_id, cluster, kustomization)
);
CREATE INDEX idx_snapshot_revisions_sha ON snapshot_revisions (sha text_pattern_ops);
