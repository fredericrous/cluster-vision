import { describe, it, expect } from "vitest";
import {
  fetchConfig,
  fetchApplications,
  fetchApplication,
  fetchLandscape,
  fetchRoadmap,
  fetchDependencyGraph,
  fetchComponents,
  fetchCapabilityTree,
  fetchSyncLogs,
} from "../app/api.server";

describe("fetchConfig", () => {
  it("returns eam and ai flags", async () => {
    const config = await fetchConfig();
    expect(config.eam).toBe(true);
    expect(config.ai).toBe(true);
  });
});

describe("fetchApplications", () => {
  it("returns items and total", async () => {
    const data = await fetchApplications();
    expect(data.items).toHaveLength(2);
    expect(data.total).toBe(2);
    expect(data.items[0].name).toBe("grafana");
    expect(data.items[1].name).toBe("authelia");
  });
});

describe("fetchApplication", () => {
  it("returns enriched detail", async () => {
    const { items } = await fetchApplications();
    const detail = await fetchApplication(items[0].id);

    expect(detail.application).toBeDefined();
    expect(detail.application.name).toBe("grafana");
    expect(detail.dependencies).toBeDefined();
    expect(detail.components).toBeDefined();
    expect(detail.capabilities).toBeDefined();
    expect(detail.k8s_sources).toBeDefined();
    expect(detail.k8s_sources.length).toBeGreaterThan(0);
  });
});

describe("fetchLandscape", () => {
  it("returns capabilities and unmapped", async () => {
    const data = await fetchLandscape();
    expect(data.capabilities).toBeDefined();
    expect(data.capabilities.length).toBeGreaterThan(0);
    expect(data.capabilities[0].name).toBe("Monitoring & Observability");
    expect(data.capabilities[0].children).toHaveLength(1);
    expect(data.unmapped).toBeDefined();
    expect(data.unmapped.length).toBeGreaterThan(0);
  });
});

describe("fetchRoadmap", () => {
  it("returns apps with version history", async () => {
    const data = await fetchRoadmap();
    expect(data.length).toBeGreaterThan(0);
    expect(data[0].version_history).toBeDefined();
  });
});

describe("fetchDependencyGraph", () => {
  it("returns nodes and edges", async () => {
    const data = await fetchDependencyGraph();
    expect(data.nodes).toHaveLength(2);
    expect(data.edges).toHaveLength(1);
    expect(data.edges[0].source).toBe(data.nodes[0].id);
    expect(data.edges[0].target).toBe(data.nodes[1].id);
  });
});

describe("fetchComponents", () => {
  it("returns component list", async () => {
    const data = await fetchComponents();
    expect(data).toHaveLength(2);
    expect(data[0].type).toBe("compute");
    expect(data[1].type).toBe("storage");
  });
});

describe("fetchCapabilityTree", () => {
  it("returns tree with children", async () => {
    const tree = await fetchCapabilityTree();
    expect(tree).toHaveLength(1);
    expect(tree[0].name).toBe("Security");
    expect(tree[0].children).toHaveLength(1);
    expect(tree[0].children[0].name).toBe("Authentication");
    expect(tree[0].children[0].app_count).toBe(2);
  });
});

describe("fetchSyncLogs", () => {
  it("returns sync log entries", async () => {
    const logs = await fetchSyncLogs();
    expect(logs.length).toBeGreaterThan(0);
    expect(logs[0].apps_created).toBeDefined();
    expect(logs[0].started_at).toBeDefined();
  });
});
