import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import {
  mockApplication,
  mockComponent,
  mockCapability,
  mockSyncLog,
  mockK8sSource,
  mockDependency,
  mockVersionHistory,
  mockLandscapeData,
  mockGraphData,
} from "./mocks";

const app1 = mockApplication({ name: "grafana", display_name: "Grafana", business_criticality: "high" });
const app2 = mockApplication({ name: "authelia", status: "active", technical_risk: "medium" });

export const handlers = [
  http.get("http://localhost:8080/api/config", () => {
    return HttpResponse.json({ eam: true, ai: true });
  }),

  http.get("http://localhost:8080/api/eam/applications", () => {
    return HttpResponse.json({ items: [app1, app2], total: 2 });
  }),

  http.get("http://localhost:8080/api/eam/applications/:id", ({ params }) => {
    const app = [app1, app2].find((a) => a.id === params.id);
    if (!app) return HttpResponse.json({ error: "not found" }, { status: 404 });

    return HttpResponse.json({
      application: app,
      dependencies: [mockDependency({ source_app_id: app.id })],
      components: [mockComponent()],
      capabilities: [mockCapability({ name: "Monitoring" })],
      k8s_sources: [mockK8sSource({ app_id: app.id })],
    });
  }),

  http.post("http://localhost:8080/api/eam/applications", async ({ request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    const created = mockApplication({ name: body.name as string });
    return HttpResponse.json(created, { status: 201 });
  }),

  http.put("http://localhost:8080/api/eam/applications/:id", async ({ params, request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    const updated = mockApplication({ id: params.id as string, ...body });
    return HttpResponse.json(updated);
  }),

  http.get("http://localhost:8080/api/eam/applications/:id/versions", () => {
    return HttpResponse.json([
      mockVersionHistory({ chart_version: "7.0.0", outdated: false }),
      mockVersionHistory({ chart_version: "6.0.0", outdated: true }),
    ]);
  }),

  http.get("http://localhost:8080/api/eam/landscape", () => {
    return HttpResponse.json(mockLandscapeData());
  }),

  http.get("http://localhost:8080/api/eam/roadmap", () => {
    return HttpResponse.json([
      { ...app1, version_history: [mockVersionHistory()] },
    ]);
  }),

  http.get("http://localhost:8080/api/eam/graph", () => {
    return HttpResponse.json(mockGraphData());
  }),

  http.get("http://localhost:8080/api/eam/capabilities/tree", () => {
    const parent = mockCapability({ name: "Security", level: 1 });
    const child = mockCapability({
      name: "Authentication",
      level: 2,
      parent_id: parent.id,
      app_count: 2,
    });
    parent.children = [child];
    return HttpResponse.json([parent]);
  }),

  http.get("http://localhost:8080/api/eam/components", () => {
    return HttpResponse.json([
      mockComponent({ name: "node-1", type: "compute" }),
      mockComponent({ name: "ceph-block", type: "storage" }),
    ]);
  }),

  http.post("http://localhost:8080/api/eam/sync/trigger", () => {
    return HttpResponse.json({
      apps_created: 3,
      apps_updated: 2,
      components_created: 1,
      errors: [],
    });
  }),

  http.get("http://localhost:8080/api/eam/sync/logs", () => {
    return HttpResponse.json([mockSyncLog(), mockSyncLog()]);
  }),

  http.post("http://localhost:8080/api/eam/enrich", () => {
    return HttpResponse.json({ status: "started" });
  }),
];

export const server = setupServer(...handlers);
