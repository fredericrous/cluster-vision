# Third-Party Notices

This file inventories the third-party components distributed with or used to
build `cluster-vision`, and the licenses they are used under.

Generated 2026-08-28. Regenerate when dependencies change.

## Summary

**No component is licensed under the GPL, LGPL, AGPL or SSPL.** Every
dependency is under a permissive license, or (for MPL-2.0) under a
file-level copyleft license that does not extend to the Licensed Work.

## Go modules (backend)

51 modules are linked into the `cluster-vision` binary.

| License | Modules |
| --- | ---: |
| Apache-2.0 | 20 |
| MIT | 17 |
| BSD-2-Clause or BSD-3-Clause | 13 |
| ISC | 1 |

<details>
<summary>Full module list</summary>

| Module | Version | License |
| --- | --- | --- |
| `github.com/beorn7/perks` | v1.0.1 | MIT |
| `github.com/cespare/xxhash/v2` | v2.3.0 | MIT |
| `github.com/davecgh/go-spew` | v1.1.2-0.20180830191138-d8f796af33cc | ISC |
| `github.com/emicklei/go-restful/v3` | v3.12.2 | MIT |
| `github.com/fxamacker/cbor/v2` | v2.9.0 | MIT |
| `github.com/go-logr/logr` | v1.4.3 | Apache-2.0 |
| `github.com/go-openapi/jsonpointer` | v0.21.0 | Apache-2.0 |
| `github.com/go-openapi/jsonreference` | v0.20.2 | Apache-2.0 |
| `github.com/go-openapi/swag` | v0.23.0 | Apache-2.0 |
| `github.com/golang-migrate/migrate/v4` | v4.19.1 | MIT |
| `github.com/google/gnostic-models` | v0.7.0 | Apache-2.0 |
| `github.com/google/uuid` | v1.6.0 | BSD-2-Clause or BSD-3-Clause |
| `github.com/jackc/pgpassfile` | v1.0.0 | MIT |
| `github.com/jackc/pgservicefile` | v0.0.0-20240606120523-5a60cdf6a761 | MIT |
| `github.com/jackc/pgx/v5` | v5.8.0 | MIT |
| `github.com/jackc/puddle/v2` | v2.2.2 | MIT |
| `github.com/josharian/intern` | v1.0.0 | MIT |
| `github.com/json-iterator/go` | v1.1.12 | MIT |
| `github.com/lib/pq` | v1.10.9 | MIT |
| `github.com/mailru/easyjson` | v0.7.7 | MIT |
| `github.com/modern-go/concurrent` | v0.0.0-20180306012644-bacd9c7ef1dd | Apache-2.0 |
| `github.com/modern-go/reflect2` | v1.0.3-0.20250322232337-35a7c28c31ee | Apache-2.0 |
| `github.com/munnerz/goautoneg` | v0.0.0-20191010083416-a7dc8b61c822 | BSD-2-Clause or BSD-3-Clause |
| `github.com/prometheus/client_golang` | v1.23.2 | Apache-2.0 |
| `github.com/prometheus/client_model` | v0.6.2 | Apache-2.0 |
| `github.com/prometheus/common` | v0.66.1 | Apache-2.0 |
| `github.com/spf13/pflag` | v1.0.9 | BSD-2-Clause or BSD-3-Clause |
| `github.com/x448/float16` | v0.8.4 | MIT |
| `go.yaml.in/yaml/v2` | v2.4.3 | Apache-2.0 |
| `go.yaml.in/yaml/v3` | v3.0.4 | MIT |
| `golang.org/x/net` | v0.47.0 | BSD-2-Clause or BSD-3-Clause |
| `golang.org/x/oauth2` | v0.30.0 | BSD-2-Clause or BSD-3-Clause |
| `golang.org/x/sync` | v0.19.0 | BSD-2-Clause or BSD-3-Clause |
| `golang.org/x/sys` | v0.38.0 | BSD-2-Clause or BSD-3-Clause |
| `golang.org/x/term` | v0.37.0 | BSD-2-Clause or BSD-3-Clause |
| `golang.org/x/text` | v0.34.0 | BSD-2-Clause or BSD-3-Clause |
| `golang.org/x/time` | v0.12.0 | BSD-2-Clause or BSD-3-Clause |
| `google.golang.org/protobuf` | v1.36.8 | BSD-2-Clause or BSD-3-Clause |
| `gopkg.in/evanphx/json-patch.v4` | v4.13.0 | BSD-2-Clause or BSD-3-Clause |
| `gopkg.in/inf.v0` | v0.9.1 | BSD-2-Clause or BSD-3-Clause |
| `gopkg.in/yaml.v3` | v3.0.1 | MIT |
| `k8s.io/api` | v0.35.1 | Apache-2.0 |
| `k8s.io/apimachinery` | v0.35.1 | Apache-2.0 |
| `k8s.io/client-go` | v0.35.1 | Apache-2.0 |
| `k8s.io/klog/v2` | v2.130.1 | Apache-2.0 |
| `k8s.io/kube-openapi` | v0.0.0-20250910181357-589584f1c912 | Apache-2.0 |
| `k8s.io/utils` | v0.0.0-20251002143259-bc988d571ff4 | Apache-2.0 |
| `sigs.k8s.io/json` | v0.0.0-20250730193827-2d320260d730 | Apache-2.0 |
| `sigs.k8s.io/randfill` | v1.0.0 | Apache-2.0 |
| `sigs.k8s.io/structured-merge-diff/v6` | v6.3.0 | Apache-2.0 |
| `sigs.k8s.io/yaml` | v1.6.0 | MIT |

</details>

## npm packages (`web/`, SSR frontend)

676 packages in the dependency tree (including build-time only).

| License | Packages |
| --- | ---: |
| MIT | 551 |
| ISC | 60 |
| Apache-2.0 | 26 |
| MPL-2.0 | 12 |
| BSD-3-Clause | 11 |
| BSD-2-Clause | 7 |
| Unlicense | 2 |
| Python-2.0 | 1 |
| CC-BY-4.0 | 1 |
| BSD | 1 |
| (MPL-2.0 OR Apache-2.0) | 1 |
| BlueOak-1.0.0 | 1 |
| 0BSD | 1 |
| (MIT OR CC0-1.0) | 1 |

## npm packages (`mcp/`, MCP server)

3 packages in the dependency tree (including build-time only).

| License | Packages |
| --- | ---: |
| MIT | 2 |
| Apache-2.0 | 1 |

## Notes on non-permissive-by-default entries

- **MPL-2.0** — all MPL-2.0 packages are `lightningcss` and its platform
  binaries, which are **build-time only** and not redistributed in the
  container image. MPL-2.0 is file-level copyleft: it obliges publication of
  modifications to the MPL-covered files themselves, which we do not make.
- **`dompurify` (MPL-2.0 OR Apache-2.0)** — dual-licensed; the Apache-2.0
  option is elected.
- **`caniuse-lite` (CC-BY-4.0)** — a browser-support **data** set consumed by
  `browserslist` at build time. Attribution is provided by this notice.
- **`argparse` (Python-2.0)** — build-time only; the Python Software
  Foundation License is permissive and GPL-compatible.
- **`heap`, `khroma`, `pathfinding`** — no `license` field in their published
  package metadata. Verified MIT from their upstream repositories and, for
  `pathfinding`, from the legacy `licenses` array in its manifest.
