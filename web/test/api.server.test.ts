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
    const err = await fetchDiagrams().catch((e) => e);
    expect(err.init.status).toBe(500);
    expect(err.data.error).toBe("boom");
  });

  it("keeps the 503 the API answers before the first sync", async () => {
    server.use(
      http.get("http://localhost:8080/api/diagrams", () =>
        HttpResponse.json({ error: "no cluster data available yet" }, { status: 503 })
      )
    );
    const err = await fetchDiagrams().catch((e) => e);
    expect(err.init.status).toBe(503);
  });

  it("maps an unknown snapshot to a 404 route response", async () => {
    server.use(
      http.get("http://localhost:8080/api/snapshots/:id/diagrams", () =>
        HttpResponse.json({ error: "snapshot not found" }, { status: 404 })
      )
    );
    const err = await fetchDiagrams(
      new Request("http://x/topology?before=prev&after=bogus")
    ).catch((e) => e);
    expect(err.init.status).toBe(404);
    expect(err.data).toEqual({ error: "snapshot not found", kind: "snapshot" });
  });

  it("maps an unreachable API to a 502 route response", async () => {
    server.use(
      http.get("http://localhost:8080/api/diagrams", () => HttpResponse.error())
    );
    const err = await fetchDiagrams().catch((e) => e);
    expect(err.init.status).toBe(502);
    expect(err.data.error).toContain("Could not reach the Cluster Vision API");
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
    const err = await fetchDiagram("nope").catch((e) => e);
    expect(err.init.status).toBe(404);
    expect(err.data).toEqual({ error: 'Diagram "nope" not found', kind: "diagram" });
  });
});

describe("fetchDiagramsByPrefix", () => {
  it("filters diagrams by id prefix", async () => {
    const { diagrams } = await fetchDiagramsByPrefix("topo");
    expect(diagrams).toHaveLength(1);
    expect(diagrams[0].id).toBe("topology");
  });
});

describe("toRouteError", () => {
  it("leaves the wording to the boundary when the API sent no message", async () => {
    server.use(
      http.get("http://localhost:8080/api/snapshots/:id/diagrams", () =>
        new HttpResponse("404 page not found", { status: 404 })
      )
    );
    const err = await fetchDiagrams(new Request("http://x/?after=bogus")).catch((e) => e);
    expect(err.init.status).toBe(404);
    expect(err.data).toEqual({ kind: "snapshot" });
  });
});
