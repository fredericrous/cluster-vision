const API_URL = process.env.API_URL || "http://localhost:8080";

export interface DiagramResult {
  id: string;
  title: string;
  type: "mermaid" | "markdown";
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
