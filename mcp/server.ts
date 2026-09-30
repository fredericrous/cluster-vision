/**
 * cluster-vision MCP server logic: tool definitions and the JSON-RPC
 * dispatcher. The stdio transport lives in index.ts so this module can be
 * imported by tests without reading stdin.
 */

export const CV_BASE_URL = process.env.CV_BASE_URL ?? "http://localhost:8080"

/**
 * Diagram IDs the Go server emits with a fixed name (every `ID: "…"`
 * DiagramResult literal in internal/diagram). server.test.ts checks this list
 * against the Go source, so a renamed or added diagram fails the build here
 * instead of silently offering an ID the API never returns.
 */
export const DIAGRAM_IDS = [
  "topology",
  "topology-mesh",
  "topology-other",
  "dependencies",
  "network",
  "security",
  "security-chart",
  "images",
  "charts",
  "nodes",
  "workloads",
  "storage",
  "crds",
  "quotas",
  "certificates",
  "network-policies",
  "configs",
  "helm-workloads",
  "service-map",
  "namespace-summary",
  "rbac",
  "labels",
  "velero",
] as const

/**
 * One physical-topology diagram is generated per tfstate / docker-compose
 * data source, as `topology-<source name>`, so those IDs cannot be listed
 * statically.
 */
export const DYNAMIC_DIAGRAM_ID_PATTERN = "^topology-[a-z0-9_-]+$"

/** A non-2xx or non-JSON answer from the cluster-vision API. */
export class ApiError extends Error {
  readonly status: number
  constructor(status: number, message: string) {
    super(message)
    this.name = "ApiError"
    this.status = status
  }
}

export async function apiRequest(method: string, path: string): Promise<unknown> {
  let res: Response
  try {
    res = await fetch(`${CV_BASE_URL}${path}`, { method })
  } catch (err) {
    const cause = err instanceof Error && err.cause instanceof Error ? `: ${err.cause.message}` : ""
    throw new Error(`cannot reach cluster-vision at ${CV_BASE_URL} (set CV_BASE_URL)${cause}`)
  }
  const text = await res.text()
  let body: unknown
  let isJSON = true
  try {
    body = text === "" ? null : JSON.parse(text)
  } catch {
    isJSON = false
  }

  if (!res.ok) {
    const detail =
      isJSON && body && typeof body === "object" && "error" in body
        ? String((body as { error: unknown }).error)
        : text.trim().slice(0, 200)
    // Go's mux answers an unregistered route with a plain-text 404. The EAM
    // and snapshot routes are only registered when the server has a
    // DATABASE_URL, so say so rather than leaving a bare "404 page not found".
    const hint =
      res.status === 404 && !isJSON
        ? " (route not available: EAM and snapshot endpoints need DATABASE_URL on the cluster-vision server)"
        : ""
    throw new ApiError(res.status, `${method} ${path} failed with HTTP ${res.status}: ${detail}${hint}`)
  }
  if (!isJSON) {
    throw new ApiError(res.status, `${method} ${path} returned a non-JSON body: ${text.trim().slice(0, 200)}`)
  }
  return body
}

type Args = Record<string, any>

interface Tool {
  description: string
  inputSchema: Record<string, unknown>
  handler: (args: Args) => Promise<unknown>
}

export const tools: Record<string, Tool> = {
  cv_list_applications: {
    description:
      "List EAM applications from cluster-vision with optional filters. Returns name, business_criticality, technical_risk, lifecycle_phase, time_category, status, and tags for each app.",
    inputSchema: {
      type: "object",
      properties: {
        status: {
          type: "string",
          enum: ["active", "retired", "inactive"],
          description: "Filter by application lifecycle status",
        },
        risk: {
          type: "string",
          enum: ["low", "medium", "high"],
          description: "Filter by technical risk level",
        },
        search: {
          type: "string",
          description: "Search by application name",
        },
        limit: {
          type: "number",
          description: "Max results (default 50)",
        },
        offset: {
          type: "number",
          description: "Pagination offset (default 0)",
        },
      },
    },
    handler: async (args) => {
      const params = new URLSearchParams()
      if (args.status) params.set("status", args.status)
      if (args.risk) params.set("risk", args.risk)
      if (args.search) params.set("search", args.search)
      params.set("limit", String(args.limit ?? 50))
      params.set("offset", String(args.offset ?? 0))
      return apiRequest("GET", `/api/eam/applications?${params}`)
    },
  },

  cv_get_application: {
    description:
      "Get full details for a single application by its UUID, including upstream dependencies, IT components, business capabilities, and Kubernetes sources (namespace, helm release, workload type, images).",
    inputSchema: {
      type: "object",
      required: ["id"],
      properties: {
        id: {
          type: "string",
          description: "Application UUID (from cv_list_applications or cv_dependency_graph)",
        },
      },
    },
    handler: async (args) => apiRequest("GET", `/api/eam/applications/${encodeURIComponent(args.id)}`),
  },

  cv_dependency_graph: {
    description:
      "Get the full application dependency graph as nodes and edges. Each node has: id, name, namespace, cluster, criticality, technical_risk, capabilities[]. Each edge has source and target app IDs. Use this for topology questions: critical paths, dependency counts, blast radius analysis.",
    inputSchema: {
      type: "object",
      properties: {},
    },
    handler: async () => apiRequest("GET", "/api/eam/graph"),
  },

  cv_find_dependents: {
    description:
      "Find all applications that depend on a given application (reverse dependency lookup). Returns the app's node info plus a list of dependents with their names, namespaces, and criticality.",
    inputSchema: {
      type: "object",
      required: ["name"],
      properties: {
        name: {
          type: "string",
          description: "Application name to find dependents of",
        },
        namespace: {
          type: "string",
          description: "Optional: filter by Kubernetes namespace to disambiguate apps with the same name",
        },
      },
    },
    handler: async (args) => {
      const graph = (await apiRequest("GET", "/api/eam/graph")) as {
        nodes: Array<{
          id: string
          name: string
          namespace: string
          cluster: string
          criticality: string
          technical_risk: string
          capabilities: string[]
        }>
        edges: Array<{ source: string; target: string; description: string | null }>
      }

      const target = graph.nodes.find(
        (n) => n.name === args.name && (!args.namespace || n.namespace === args.namespace),
      )

      if (!target) {
        return {
          error: `No application found with name "${args.name}"${args.namespace ? ` in namespace "${args.namespace}"` : ""}`,
          available_names: graph.nodes.map((n) => `${n.name} (${n.namespace})`).slice(0, 20),
        }
      }

      const dependentEdges = graph.edges.filter((e) => e.target === target.id)
      const dependents = dependentEdges.map((e) => {
        const node = graph.nodes.find((n) => n.id === e.source)
        return {
          id: e.source,
          name: node?.name ?? e.source,
          namespace: node?.namespace ?? "",
          criticality: node?.criticality ?? "unknown",
          technical_risk: node?.technical_risk ?? "unknown",
          description: e.description,
        }
      })

      return {
        target: { id: target.id, name: target.name, namespace: target.namespace, criticality: target.criticality },
        dependent_count: dependents.length,
        dependents,
      }
    },
  },

  cv_landscape: {
    description:
      "Get the business capability landscape: a tree of business capabilities with mapped applications and vulnerability counts (critical/high CVEs). Use this for business impact analysis and to understand which capabilities are at risk.",
    inputSchema: {
      type: "object",
      properties: {},
    },
    handler: async () => apiRequest("GET", "/api/eam/landscape"),
  },

  cv_list_snapshots: {
    description:
      "List cluster snapshots, newest first. A snapshot is written whenever the observed cluster state (or the Flux revision it runs) changes. Each entry has: id, taken_at, revisions[] (cluster, kustomization, sha), summary (per-diagram added/removed/changed counts vs the previous snapshot, drift flag = changed with no new commit), new_revision (first snapshot after a deploy). Use the ids or shas as selectors for cv_diff.",
    inputSchema: {
      type: "object",
      properties: {
        since: {
          type: "string",
          description: "Optional RFC3339 lower bound on taken_at",
        },
        limit: {
          type: "number",
          description: "Max entries (default 20, max 500)",
        },
      },
    },
    handler: async (args) => {
      const q = new URLSearchParams()
      if (args?.since) q.set("from", args.since)
      q.set("limit", String(args?.limit ?? 20))
      return apiRequest("GET", `/api/snapshots?${q}`)
    },
  },

  cv_diff: {
    description:
      "What changed in the cluster between two points in time. Returns, per diagram, the added/removed/changed nodes, edges or rows with field-level old→new values, plus a cluster-wide total, a drift flag (true when the Flux revision did not change but the observed state did — something moved outside GitOps) and forge compare links for the revision range. Selectors for `from`/`to`: a snapshot id, a git sha or sha prefix (first snapshot observed at that revision), an RFC3339 time (nearest snapshot at or before it), or `now`. `from` also accepts `prev` (the change before `to`, default) and `deploy` (the last state before the current revision — 'since last deploy'). Advisory changes (registry latest tags, vulnerability counts) are listed separately under `advisory` and never counted.",
    inputSchema: {
      type: "object",
      properties: {
        diagram: {
          type: "string",
          description: 'Diagram to diff, e.g. dependencies, images, rbac, certificates. Omit or pass "all" for every diagram.',
        },
        from: {
          type: "string",
          description: "Before selector: snapshot id | git sha | RFC3339 | prev | deploy. Default prev.",
        },
        to: {
          type: "string",
          description: "After selector: snapshot id | git sha | RFC3339 | now. Default now.",
        },
      },
    },
    handler: async (args) => {
      const q = new URLSearchParams()
      q.set("from", args?.from || "prev")
      q.set("to", args?.to || "now")
      const diagram = args?.diagram && args.diagram !== "all" ? args.diagram : ""
      const path = diagram ? `/api/diagrams/${encodeURIComponent(diagram)}/diff?${q}` : `/api/diff?${q}`
      return apiRequest("GET", path)
    },
  },

  cv_get_diagram: {
    description:
      "Get a rendered cluster diagram (Mermaid, flow JSON or Markdown table). Specify a type to get one diagram, omit to get all.",
    inputSchema: {
      type: "object",
      properties: {
        type: {
          anyOf: [
            { type: "string", enum: [...DIAGRAM_IDS] },
            { type: "string", pattern: DYNAMIC_DIAGRAM_ID_PATTERN },
          ],
          description:
            "Diagram ID. topology-<source> (one physical topology per tfstate/docker-compose data source), topology-mesh (east-west gateways), topology-other (nodes outside any source), topology (Kubernetes-only fallback), dependencies (Flux kustomization flows), network (gateways/routes), security (mTLS/PSA), security-chart, images (CVEs), charts (Helm chart versions), nodes, workloads, storage (PVs/PVCs), crds, quotas, certificates (cert-manager), network-policies, configs (ConfigMaps/Secrets), helm-workloads, service-map, namespace-summary, rbac, labels, velero. Omit to return all; an unknown ID returns the list of available ones.",
        },
      },
    },
    handler: async (args) => {
      const data = (await apiRequest("GET", "/api/diagrams")) as any
      const diagrams: any[] = data?.diagrams ?? (Array.isArray(data) ? data : [])
      if (!args?.type) return { diagrams }
      const match = diagrams.find((d: any) => d.id === args.type)
      if (!match) {
        return {
          error: `Diagram type "${args.type}" not found`,
          available: diagrams.map((d: any) => d.id),
        }
      }
      return match
    },
  },
}

// JSON-RPC 2.0 error codes.
export const PARSE_ERROR = -32700
export const INVALID_REQUEST = -32600
export const METHOD_NOT_FOUND = -32601
export const INVALID_PARAMS = -32602
export const INTERNAL_ERROR = -32603

type Id = string | number | null

export type RpcResponse =
  | { jsonrpc: "2.0"; id: Id; result: unknown }
  | { jsonrpc: "2.0"; id: Id; error: { code: number; message: string } }

class RpcError extends Error {
  readonly code: number
  constructor(code: number, message: string) {
    super(message)
    this.code = code
  }
}

async function dispatch(message: any): Promise<unknown> {
  switch (message.method) {
    case "initialize":
      return {
        protocolVersion: "2024-11-05",
        capabilities: { tools: {} },
        serverInfo: { name: "cluster-vision", version: "0.1.0" },
      }
    case "ping":
      return {}
    case "tools/list":
      return {
        tools: Object.entries(tools).map(([name, def]) => ({
          name,
          description: def.description,
          inputSchema: def.inputSchema,
        })),
      }
    case "tools/call": {
      const name = message.params?.name
      const tool = typeof name === "string" && Object.hasOwn(tools, name) ? tools[name] : undefined
      if (!tool) throw new RpcError(INVALID_PARAMS, `Unknown tool: ${name}`)
      // A failing tool is a tool result with isError, not a protocol error:
      // the model gets to read what went wrong.
      try {
        const result = await tool.handler(message.params?.arguments ?? {})
        return { content: [{ type: "text", text: JSON.stringify(result, null, 2) }] }
      } catch (error) {
        const msg = error instanceof Error ? error.message : String(error)
        return { content: [{ type: "text", text: `Error: ${msg}` }], isError: true }
      }
    }
    default:
      throw new RpcError(METHOD_NOT_FOUND, `Unknown method: ${message.method}`)
  }
}

function hasId(message: any): boolean {
  return message !== null && typeof message === "object" && "id" in message && message.id !== undefined
}

/**
 * Handle one line of the stdio stream. Returns the JSON-RPC response to
 * write, or null for a notification (no id), which never gets one.
 */
export async function handleLine(line: string): Promise<RpcResponse | null> {
  let message: any
  try {
    message = JSON.parse(line)
  } catch (err) {
    return {
      jsonrpc: "2.0",
      id: null,
      error: { code: PARSE_ERROR, message: `Parse error: ${err instanceof Error ? err.message : String(err)}` },
    }
  }

  if (message === null || typeof message !== "object" || Array.isArray(message) || typeof message.method !== "string") {
    return { jsonrpc: "2.0", id: hasId(message) ? message.id : null, error: { code: INVALID_REQUEST, message: "Invalid Request" } }
  }

  // Client notifications (notifications/initialized, …/cancelled) need no
  // handling from a stateless proxy.
  if (!hasId(message) && message.method.startsWith("notifications/")) return null

  const id: Id = hasId(message) ? message.id : null
  try {
    const result = await dispatch(message)
    return hasId(message) ? { jsonrpc: "2.0", id, result } : null
  } catch (err) {
    if (!hasId(message)) {
      process.stderr.write(`MCP notification ${message.method} failed: ${err instanceof Error ? err.message : String(err)}\n`)
      return null
    }
    const code = err instanceof RpcError ? err.code : INTERNAL_ERROR
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: "2.0", id, error: { code, message: msg } }
  }
}
