import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";

export const mockDiagrams = [
  {
    id: "dependencies",
    title: "Flux Dependencies",
    type: "flow" as const,
    content: JSON.stringify({
      nodes: [
        { id: "cilium", label: "cilium", cluster: "Homelab", layer: "platform-foundation" },
        { id: "monitoring", label: "monitoring", cluster: "Homelab", layer: "monitoring" },
        { id: "grafana", label: "grafana", cluster: "Homelab", layer: "apps" },
      ],
      edges: [
        { id: "e1", source: "cilium", target: "monitoring" },
        { id: "e2", source: "monitoring", target: "grafana" },
      ],
    }),
  },
  {
    id: "topology",
    title: "Cluster Topology",
    type: "mermaid" as const,
    content: "graph TD; a-->b",
  },
];

export const handlers = [
  http.get("http://localhost:8080/api/diagrams", () => {
    return HttpResponse.json({
      diagrams: mockDiagrams,
      generated_at: "2026-07-27T10:00:00Z",
    });
  }),
];

export const server = setupServer(...handlers);
