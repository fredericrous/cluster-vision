import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import {
  fetchDiagrams,
  fetchDiagram,
  fetchDiagramsByPrefix,
} from "../app/api.server";
import { server, mockDiagrams } from "./msw-handlers";

describe("fetchDiagrams", () => {
  it("returns all diagrams with generated_at", async () => {
    const data = await fetchDiagrams();
    expect(data.diagrams).toHaveLength(mockDiagrams.length);
    expect(data.generated_at).toBe("2026-07-27T10:00:00Z");
  });

  it("throws on API error", async () => {
    server.use(
      http.get("http://localhost:8080/api/diagrams", () =>
        HttpResponse.json({ error: "boom" }, { status: 500 })
      )
    );
    await expect(fetchDiagrams()).rejects.toThrow("API error: 500");
  });
});

describe("fetchDiagram", () => {
  it("returns the diagram matching the id", async () => {
    const { diagram, generatedAt } = await fetchDiagram("dependencies");
    expect(diagram.id).toBe("dependencies");
    expect(diagram.type).toBe("flow");
    expect(generatedAt).toBe("2026-07-27T10:00:00Z");
  });

  it("throws when the diagram does not exist", async () => {
    await expect(fetchDiagram("nope")).rejects.toThrow(
      'Diagram "nope" not found'
    );
  });
});

describe("fetchDiagramsByPrefix", () => {
  it("filters diagrams by id prefix", async () => {
    const { diagrams } = await fetchDiagramsByPrefix("topo");
    expect(diagrams).toHaveLength(1);
    expect(diagrams[0].id).toBe("topology");
  });
});
