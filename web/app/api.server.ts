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

export interface Revision {
  cluster: string;
  kustomization: string;
  source_kind: string;
  revision: string;
  sha: string;
}

export interface SnapshotRef {
  id: string; // uuid, or "now" for the live state
  taken_at: string;
  revisions: Revision[];
}

export interface Summary {
  added: number;
  removed: number;
  changed: number;
}

export interface Snapshot extends SnapshotRef {
  model_version: number;
  summary: {
    previous_id: string | null;
    total: Summary;
    diagrams: Record<string, Summary>;
    drift: boolean;
  };
  new_revision: boolean;
}

export interface FieldChange {
  name: string;
  from: string;
  to: string;
}

export interface Change {
  kind: "node" | "edge" | "row" | "content";
  op: "added" | "removed" | "changed";
  id: string;
  label: string;
  fields?: FieldChange[];
}

export interface DiagramDiff {
  diagram_id: string;
  title: string;
  type: string;
  key_fields?: string[];
  from: SnapshotRef;
  to: SnapshotRef;
  changes: Change[];
  advisory: Change[];
  summary: Summary;
  drift: boolean;
}

export interface CompareLink {
  cluster: string;
  from_sha: string;
  to_sha: string;
  url: string;
}

export interface DiffResponse {
  from: SnapshotRef;
  to: SnapshotRef;
  diagrams: DiagramDiff[];
  total: Summary;
  drift: boolean;
  compare_links: CompareLink[];
}

export interface ApiConfig {
  eam: boolean;
  ai: boolean;
  snapshots: boolean;
}

/** Compare selectors carried in the URL. `before` present = compare mode on. */
export interface CompareParams {
  before: string | null;
  after: string; // "now" when absent
}

export function compareParams(request?: Request): CompareParams {
  if (!request) return { before: null, after: "now" };
  const url = new URL(request.url);
  return {
    before: url.searchParams.get("before"),
    after: url.searchParams.get("after") || "now",
  };
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${API_URL}${path}`);
  if (!res.ok) {
    let detail: string | undefined;
    try {
      const body = (await res.json()) as { error?: string };
      detail = body.error;
    } catch {
      // not JSON — no detail
    }
    throw new ApiError(res.status, `API error: ${res.status} ${res.statusText}`, detail);
  }
  return res.json() as Promise<T>;
}

export class ApiError extends Error {
  status: number;
  /** The API's own message (`{"error": "..."}`), when it sent one. */
  detail?: string;
  constructor(status: number, message: string, detail?: string) {
    super(message);
    this.status = status;
    this.detail = detail;
  }
}

/** Live diagrams, or the diagrams regenerated from a stored snapshot when
 *  the request carries `after=<snapshot>`. Every page reads through here
 *  so compare mode shows the "after" state, not a mix of live and past. */
export async function fetchDiagrams(request?: Request): Promise<DiagramsResponse> {
  const { after } = compareParams(request);
  if (after !== "now") {
    return getJSON<DiagramsResponse>(
      `/api/snapshots/${encodeURIComponent(after)}/diagrams`
    );
  }
  return getJSON<DiagramsResponse>("/api/diagrams");
}

export async function fetchDiagram(
  id: string,
  request?: Request
): Promise<{ diagram: DiagramResult; generatedAt: string }> {
  const data = await fetchDiagrams(request);
  const diagram = data.diagrams.find((d) => d.id === id);
  if (!diagram) {
    throw new Error(`Diagram "${id}" not found`);
  }
  return { diagram, generatedAt: data.generated_at };
}

export async function fetchDiagramsByPrefix(
  prefix: string,
  request?: Request
): Promise<{ diagrams: DiagramResult[]; generatedAt: string }> {
  const data = await fetchDiagrams(request);
  const diagrams = data.diagrams.filter((d) => d.id.startsWith(prefix));
  return { diagrams, generatedAt: data.generated_at };
}

export async function fetchConfig(): Promise<ApiConfig> {
  try {
    return await getJSON<ApiConfig>("/api/config");
  } catch {
    return { eam: false, ai: false, snapshots: false };
  }
}

export async function fetchSnapshots(limit = 50): Promise<Snapshot[]> {
  const data = await getJSON<{ snapshots: Snapshot[] }>(
    `/api/snapshots?limit=${limit}`
  );
  return data.snapshots;
}

export async function fetchDiff(
  before: string,
  after: string
): Promise<DiffResponse> {
  const q = new URLSearchParams({ from: before, to: after });
  return getJSON<DiffResponse>(`/api/diff?${q}`);
}
