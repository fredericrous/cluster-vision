const API_URL = process.env.API_URL || "http://localhost:8080";

export interface DiagramResult {
  id: string;
  title: string;
  type: "mermaid" | "markdown" | "table" | "flow";
  content: string;
}

interface DiagramsResponse {
  diagrams: DiagramResult[];
  generated_at: string;
}

export async function fetchDiagrams(): Promise<DiagramsResponse> {
  const res = await fetch(`${API_URL}/api/diagrams`);
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`);
  }
  return res.json();
}

export async function fetchDiagram(
  id: string
): Promise<{ diagram: DiagramResult; generatedAt: string }> {
  const data = await fetchDiagrams();
  const diagram = data.diagrams.find((d) => d.id === id);
  if (!diagram) {
    throw new Error(`Diagram "${id}" not found`);
  }
  return { diagram, generatedAt: data.generated_at };
}

export async function fetchDiagramsByPrefix(
  prefix: string
): Promise<{ diagrams: DiagramResult[]; generatedAt: string }> {
  const data = await fetchDiagrams();
  const diagrams = data.diagrams.filter((d) => d.id.startsWith(prefix));
  return { diagrams, generatedAt: data.generated_at };
}

// EAM API

export interface AppConfig {
  eam: boolean;
  ai: boolean;
}

export async function fetchConfig(): Promise<AppConfig> {
  try {
    const res = await fetch(`${API_URL}/api/config`);
    if (!res.ok) return { eam: false, ai: false };
    return res.json();
  } catch {
    return { eam: false, ai: false };
  }
}

export interface Application {
  id: string;
  name: string;
  display_name: string | null;
  description: string | null;
  description_source: string;
  status: string;
  business_criticality: string;
  business_criticality_source: string;
  technical_risk: string;
  technical_risk_source: string;
  technical_risk_reasoning: string | null;
  lifecycle_phase: string;
  time_category: string | null;
  time_category_source: string;
  time_category_reasoning: string | null;
  end_of_life_date: string | null;
  tags: string[];
  ai_confidence: number;
  manual_override: boolean;
  created_at: string;
  updated_at: string;
}

export interface ITComponent {
  id: string;
  name: string;
  type: string;
  version: string | null;
  provider: string | null;
  description: string | null;
  status: string;
  tags: string[];
  created_at: string;
  updated_at: string;
}

export interface BusinessCapability {
  id: string;
  name: string;
  description: string | null;
  parent_id: string | null;
  level: number;
  sort_order: number;
  children: BusinessCapability[];
  app_count: number;
}

export interface K8sSource {
  id: string;
  app_id: string;
  cluster: string;
  namespace: string;
  helm_release: string | null;
  workload_name: string | null;
  workload_kind: string | null;
  chart_name: string | null;
  chart_version: string | null;
  images: string[];
  last_sync_at: string;
}

export interface VersionHistoryEntry {
  id: string;
  app_id: string;
  chart_version: string | null;
  image_tag: string | null;
  latest_version: string | null;
  outdated: boolean;
  vuln_critical: number;
  vuln_high: number;
  recorded_at: string;
}

export interface AppDependency {
  source_app_id: string;
  target_app_id: string;
  description: string | null;
}

export interface SyncLog {
  id: string;
  apps_created: number;
  apps_updated: number;
  components_created: number;
  errors: string[];
  started_at: string;
  finished_at: string | null;
}

export interface LandscapeCapability {
  id: string;
  name: string;
  level: number;
  children: LandscapeCapability[];
  apps: LandscapeApp[];
}

export interface LandscapeApp {
  id: string;
  name: string;
  display_name: string | null;
  status: string;
  technical_risk: string;
  vuln_critical: number;
  vuln_high: number;
}

export interface GraphNode {
  id: string;
  name: string;
  display_name: string | null;
  status: string;
  technical_risk: string;
  criticality: string;
  namespace: string;
  cluster: string;
  capabilities: string[];
}

export interface GraphEdge {
  source: string;
  target: string;
  description: string | null;
}

async function eamFetch(path: string, init?: RequestInit) {
  const res = await fetch(`${API_URL}${path}`, init);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API error ${res.status}: ${text}`);
  }
  if (res.status === 204) return null;
  return res.json();
}

export async function fetchApplications(params?: string) {
  const qs = params ? `?${params}` : "";
  return eamFetch(`/api/eam/applications${qs}`) as Promise<{
    items: Application[];
    total: number;
  }>;
}

export async function fetchApplication(id: string) {
  return eamFetch(`/api/eam/applications/${id}`) as Promise<{
    application: Application;
    dependencies: AppDependency[];
    components: ITComponent[];
    capabilities: BusinessCapability[];
    k8s_sources: K8sSource[];
  }>;
}

export async function createApplication(app: Partial<Application>) {
  return eamFetch("/api/eam/applications", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(app),
  }) as Promise<Application>;
}

export async function updateApplication(id: string, app: Partial<Application>) {
  return eamFetch(`/api/eam/applications/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(app),
  }) as Promise<Application>;
}

export async function deleteApplication(id: string) {
  return eamFetch(`/api/eam/applications/${id}`, { method: "DELETE" });
}

export async function fetchAppVersionHistory(id: string) {
  return eamFetch(`/api/eam/applications/${id}/versions`) as Promise<
    VersionHistoryEntry[]
  >;
}

export async function fetchComponents(type?: string) {
  const qs = type ? `?type=${type}` : "";
  return eamFetch(`/api/eam/components${qs}`) as Promise<ITComponent[]>;
}

export async function fetchCapabilityTree() {
  return eamFetch("/api/eam/capabilities/tree") as Promise<
    BusinessCapability[]
  >;
}

export async function fetchLandscape() {
  return eamFetch("/api/eam/landscape") as Promise<{
    capabilities: LandscapeCapability[];
    unmapped: LandscapeApp[];
  }>;
}

export async function fetchRoadmap() {
  return eamFetch("/api/eam/roadmap") as Promise<
    (Application & { version_history: VersionHistoryEntry[] })[]
  >;
}

export async function fetchDependencyGraph() {
  return eamFetch("/api/eam/graph") as Promise<{
    nodes: GraphNode[];
    edges: GraphEdge[];
  }>;
}

export async function triggerSync() {
  return eamFetch("/api/eam/sync/trigger", { method: "POST" });
}

export async function fetchSyncLogs() {
  return eamFetch("/api/eam/sync/logs") as Promise<SyncLog[]>;
}

export async function createCapability(cap: Partial<BusinessCapability>) {
  return eamFetch("/api/eam/capabilities", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(cap),
  }) as Promise<BusinessCapability>;
}

export async function updateCapability(
  id: string,
  cap: Partial<BusinessCapability>
) {
  return eamFetch(`/api/eam/capabilities/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(cap),
  }) as Promise<BusinessCapability>;
}

export async function deleteCapability(id: string) {
  return eamFetch(`/api/eam/capabilities/${id}`, { method: "DELETE" });
}
