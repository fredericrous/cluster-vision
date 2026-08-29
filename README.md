# Cluster Vision

Auto-generated infrastructure diagrams and an application landscape, derived
from live Kubernetes state.

Point it at a cluster and it renders what is actually running — topology, nodes
and storage, networking and network policies, GitOps dependencies, Helm charts
and workloads, images, RBAC, certificates, CRDs, quotas and backup schedules —
plus cluster snapshots you can diff to see what changed between two points in
time.

No diagram to maintain by hand, and no drift: the picture is read from the
cluster every refresh.

## Running it

```sh
helm install cluster-vision oci://ghcr.io/fredericrous/charts/cluster-vision
```

The container exposes `web:3000` (the UI) and `api:8080` (the Go API).
Snapshots and the application-landscape features need Postgres — set
`DATABASE_URL` to enable them; without it the rest of the app runs unchanged.

## Licensing

Cluster Vision is **source-available**, not open source. You can read, audit,
fork and modify the code, and self-host it.

| | |
| --- | --- |
| **Free**, including in production | Personal use · registered non-profits · educational institutions · evaluation, anywhere |
| **Needs a commercial licence** | Running it in or for a business · offering it to third parties as a hosted or managed service |

Every version converts automatically to **Apache License 2.0** four years after
its release — the current one on **2030-08-28**. See [`LICENSE`](LICENSE) for the
Business Source License 1.1 terms, [`NOTICE`](NOTICE) for how the licence is
split across this repository, and [`THIRD-PARTY-NOTICES.md`](THIRD-PARTY-NOTICES.md)
for the dependency inventory.

The MCP server (`mcp/`) and the Helm chart (`chart/cluster-vision/`) are
**Apache-2.0** and stay freely usable, forkable and redistributable.

## Commercial licence

**From €2,400 per production cluster per year.** Non-production clusters —
development, staging, CI — are **free**. Volume terms above three production
clusters.

A licence covers production use in your organisation, and the source you already
have. Support terms, air-gapped deployment and multi-cluster or site licences are
negotiable.

**Contact: [licensing@daddyshome.fr](mailto:licensing@daddyshome.fr)**

Tell us roughly how many clusters and we'll come back with a quote — there is no
sales process to sit through.

---

Copyright © 2026 Frédéric Rous.
