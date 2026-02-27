import type {
  Application,
  ITComponent,
  BusinessCapability,
  SyncLog,
  LandscapeCapability,
  LandscapeApp,
  GraphNode,
  GraphEdge,
  K8sSource,
  AppDependency,
  VersionHistoryEntry,
} from "../app/api.server";

let counter = 0;
function mockId(): string {
  counter++;
  return `00000000-0000-0000-0000-${String(counter).padStart(12, "0")}`;
}

export function mockApplication(
  overrides: Partial<Application> = {}
): Application {
  const id = mockId();
  return {
    id,
    name: `app-${id.slice(-4)}`,
    display_name: null,
    description: "A test application",
    description_source: "auto-discovered",
    status: "active",
    business_criticality: "medium",
    business_criticality_source: "ai-inferred",
    technical_risk: "low",
    technical_risk_source: "ai-inferred",
    technical_risk_reasoning: null,
    lifecycle_phase: "active",
    time_category: "tolerate",
    time_category_source: "ai-inferred",
    time_category_reasoning: null,
    end_of_life_date: null,
    tags: [],
    ai_confidence: 0.85,
    manual_override: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

export function mockComponent(
  overrides: Partial<ITComponent> = {}
): ITComponent {
  const id = mockId();
  return {
    id,
    name: `component-${id.slice(-4)}`,
    type: "compute",
    version: "v1.30.0",
    provider: "proxmox",
    description: null,
    status: "active",
    tags: [],
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

export function mockCapability(
  overrides: Partial<BusinessCapability> = {}
): BusinessCapability {
  const id = mockId();
  return {
    id,
    name: `Capability ${id.slice(-4)}`,
    description: null,
    parent_id: null,
    level: 1,
    sort_order: 0,
    children: [],
    app_count: 0,
    ...overrides,
  };
}

export function mockSyncLog(overrides: Partial<SyncLog> = {}): SyncLog {
  const id = mockId();
  return {
    id,
    apps_created: 5,
    apps_updated: 3,
    components_created: 2,
    errors: [],
    started_at: "2026-01-15T10:00:00Z",
    finished_at: "2026-01-15T10:00:05Z",
    ...overrides,
  };
}

export function mockK8sSource(overrides: Partial<K8sSource> = {}): K8sSource {
  const id = mockId();
  return {
    id,
    app_id: mockId(),
    cluster: "homelab",
    namespace: "monitoring",
    helm_release: "grafana",
    workload_name: null,
    workload_kind: null,
    chart_name: "grafana",
    chart_version: "7.0.0",
    images: ["grafana/grafana:10.0.0"],
    last_sync_at: "2026-01-15T10:00:00Z",
    ...overrides,
  };
}

export function mockDependency(
  overrides: Partial<AppDependency> = {}
): AppDependency {
  return {
    source_app_id: mockId(),
    target_app_id: mockId(),
    description: "depends on",
    ...overrides,
  };
}

export function mockVersionHistory(
  overrides: Partial<VersionHistoryEntry> = {}
): VersionHistoryEntry {
  const id = mockId();
  return {
    id,
    app_id: mockId(),
    chart_version: "7.0.0",
    image_tag: "v10.0.0",
    latest_version: "v10.1.0",
    outdated: false,
    vuln_critical: 0,
    vuln_high: 0,
    recorded_at: "2026-01-15T10:00:00Z",
    ...overrides,
  };
}

export function mockLandscapeData() {
  const app1: LandscapeApp = {
    id: mockId(),
    name: "grafana",
    display_name: "Grafana",
    status: "active",
    technical_risk: "low",
    vuln_critical: 0,
    vuln_high: 0,
  };
  const app2: LandscapeApp = {
    id: mockId(),
    name: "authelia",
    display_name: null,
    status: "active",
    technical_risk: "medium",
    vuln_critical: 0,
    vuln_high: 2,
  };

  const cap: LandscapeCapability = {
    id: mockId(),
    name: "Monitoring & Observability",
    level: 1,
    children: [
      {
        id: mockId(),
        name: "Dashboards",
        level: 2,
        children: [],
        apps: [app1],
      },
    ],
    apps: [],
  };

  return {
    capabilities: [cap],
    unmapped: [app2],
  };
}

export function mockGraphData() {
  const node1: GraphNode = {
    id: mockId(),
    name: "grafana",
    display_name: "Grafana",
    status: "active",
    technical_risk: "low",
    criticality: "medium",
    namespace: "monitoring",
    cluster: "homelab",
    capabilities: ["Dashboards"],
  };
  const node2: GraphNode = {
    id: mockId(),
    name: "loki",
    display_name: null,
    status: "active",
    technical_risk: "low",
    criticality: "medium",
    namespace: "monitoring",
    cluster: "homelab",
    capabilities: ["Logging"],
  };

  const edge: GraphEdge = {
    source: node1.id,
    target: node2.id,
    description: "queries logs from",
  };

  return { nodes: [node1, node2], edges: [edge] };
}
