#!/usr/bin/env node

/**
 * cluster-vision MCP Server
 *
 * Thin HTTP client that proxies to the cluster-vision REST API.
 * Launched as a stdio MCP server by the client (e.g., Claude Desktop).
 *
 * Tools:
 * - cv_list_applications
 * - cv_get_application
 * - cv_dependency_graph
 * - cv_find_dependents
 * - cv_landscape
 * - cv_get_diagram
 * - cv_list_snapshots
 * - cv_diff
 */

const CV_BASE_URL = process.env.CV_BASE_URL ?? "http://localhost:8080"

async function apiRequest(method: string, path: string) {
  const res = await fetch(`${CV_BASE_URL}${path}`, { method })
  return res.json()
}

const DIAGRAM_TYPES = [
  "topology",
  "dependencies",
  "network",
  "security",
  "images",
  "versions",
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

const tools = {
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
    handler: async (args: any) => {
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
    handler: async (args: any) =>
      apiRequest("GET", `/api/eam/applications/${encodeURIComponent(args.id)}`),
  },

  cv_dependency_graph: {
    description:
      "Get the full application dependency graph as nodes and edges. Each node has: id, name, namespace, cluster, criticality, technical_risk, capabilities[]. Each edge has source and target app IDs. Use this for topology questions: critical paths, dependency counts, blast radius analysis.",
    inputSchema: {
      type: "object",
      properties: {},
    },
    handler: async (_args: any) => apiRequest("GET", "/api/eam/graph"),
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
    handler: async (args: any) => {
      const graph = (await apiRequest("GET", "/api/eam/graph")) as {
        nodes: Array<{ id: string; name: string; namespace: string; cluster: string; criticality: string; technical_risk: string; capabilities: string[] }>
        edges: Array<{ source: string; target: string; description: string | null }>
      }

      const target = graph.nodes.find(
        (n) =>
          n.name === args.name &&
          (!args.namespace || n.namespace === args.namespace),
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
    handler: async (_args: any) => apiRequest("GET", "/api/eam/landscape"),
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
    handler: async (args: any) => {
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
          description:
            "Diagram to diff, e.g. dependencies, images, rbac, certificates. Omit or pass \"all\" for every diagram.",
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
    handler: async (args: any) => {
      const q = new URLSearchParams()
      q.set("from", args?.from || "prev")
      q.set("to", args?.to || "now")
      const diagram = args?.diagram && args.diagram !== "all" ? args.diagram : ""
      const path = diagram
        ? `/api/diagrams/${encodeURIComponent(diagram)}/diff?${q}`
        : `/api/diff?${q}`
      return apiRequest("GET", path)
    },
  },

  cv_get_diagram: {
    description:
      "Get a rendered cluster topology diagram in Mermaid or Markdown format. Useful for human-readable cluster overviews. Specify a type to get one diagram, omit to get all.",
    inputSchema: {
      type: "object",
      properties: {
        type: {
          type: "string",
          enum: DIAGRAM_TYPES as unknown as string[],
          description:
            "Diagram type. Options: topology (namespace layout), dependencies (service flows), network (gateways/policies), security (mTLS/PSA), images (CVEs), versions (helm versions), nodes (k8s nodes), workloads (deployments/statefulsets), storage (PVs/PVCs), namespace-summary (ambient/mTLS labels), service-map (services/ports), rbac (role bindings), certificates (cert-manager), helm-workloads (helm releases). Omit to return all.",
        },
      },
    },
    handler: async (args: any) => {
      const data = (await apiRequest("GET", "/api/diagrams")) as any
      const diagrams: any[] = data?.diagrams ?? (Array.isArray(data) ? data : [])
      if (!args.type) return { diagrams }
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

async function handleMessage(message: any) {
  if (message.method === "initialize") {
    return {
      protocolVersion: "2024-11-05",
      capabilities: { tools: {} },
      serverInfo: { name: "cluster-vision", version: "0.1.0" },
    }
  }

  if (message.method === "tools/list") {
    return {
      tools: Object.entries(tools).map(([name, def]) => ({
        name,
        description: def.description,
        inputSchema: def.inputSchema,
      })),
    }
  }

  if (message.method === "tools/call") {
    const { name, arguments: args } = message.params
    const tool = tools[name as keyof typeof tools]
    if (!tool) {
      return { error: { code: -32601, message: `Unknown tool: ${name}` } }
    }
    try {
      const result = await tool.handler(args)
      return { content: [{ type: "text", text: JSON.stringify(result, null, 2) }] }
    } catch (error: any) {
      return { content: [{ type: "text", text: `Error: ${error.message}` }], isError: true }
    }
  }

  return { error: { code: -32601, message: `Unknown method: ${message.method}` } }
}

let buffer = ""
process.stdin.setEncoding("utf-8")
process.stdin.on("data", async (chunk: string) => {
  buffer += chunk
  const lines = buffer.split("\n")
  buffer = lines.pop() ?? ""

  for (const line of lines) {
    if (!line.trim()) continue
    try {
      const message = JSON.parse(line)
      const response = await handleMessage(message)
      if (message.id !== undefined) {
        process.stdout.write(
          JSON.stringify({ jsonrpc: "2.0", id: message.id, result: response }) + "\n",
        )
      }
    } catch (err: any) {
      process.stderr.write(`MCP error: ${err.message}\n`)
    }
  }
})
