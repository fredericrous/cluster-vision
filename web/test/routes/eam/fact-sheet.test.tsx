import { describe, it, expect } from "vitest";
import { screen } from "@testing-library/react";
import FactSheet from "../../../app/routes/eam/fact-sheet";
import {
  mockApplication,
  mockDependency,
  mockComponent,
  mockCapability,
  mockK8sSource,
  mockVersionHistory,
} from "../../mocks";
import { renderWithRouter } from "../../render-helper";

const app = mockApplication({
  name: "grafana",
  display_name: "Grafana",
  description: "Open-source monitoring dashboards",
  status: "active",
  business_criticality: "high",
  technical_risk: "low",
  lifecycle_phase: "active",
  time_category: "invest",
  tags: ["monitoring", "dashboards"],
});

const k8sSrc = mockK8sSource({
  app_id: app.id,
  cluster: "homelab",
  namespace: "monitoring",
  helm_release: "grafana",
  chart_name: "grafana",
  chart_version: "7.0.0",
  images: ["grafana/grafana:10.0.0"],
});

const dep = mockDependency({
  source_app_id: app.id,
  description: "queries",
});

const comp = mockComponent({ name: "node-1", type: "compute", version: "v1.30.0" });
const cap = mockCapability({ name: "Monitoring" });
const history = [
  mockVersionHistory({ chart_version: "7.0.0", outdated: false }),
  mockVersionHistory({ chart_version: "6.0.0", outdated: true }),
];

function renderFactSheet() {
  return renderWithRouter(
    <FactSheet
      loaderData={{
        application: app,
        dependencies: [dep],
        components: [comp],
        capabilities: [cap],
        k8s_sources: [k8sSrc],
        version_history: history,
      }}
      params={{ id: app.id }}
      matches={[] as any}
      actionData={undefined}
    />
  );
}

describe("FactSheet", () => {
  it("renders app name", () => {
    renderFactSheet();
    expect(screen.getByText("Grafana")).toBeInTheDocument();
  });

  it("renders status and risk info", () => {
    const { container } = renderFactSheet();
    expect(container.textContent).toContain("active");
    expect(container.textContent).toContain("risk: low");
  });

  it("renders time category", () => {
    const { container } = renderFactSheet();
    expect(container.textContent).toContain("TIME: invest");
  });

  it("renders description", () => {
    const { container } = renderFactSheet();
    expect(container.textContent).toContain("Open-source monitoring dashboards");
  });

  it("renders K8s source info", () => {
    const { container } = renderFactSheet();
    expect(container.textContent).toContain("homelab");
    expect(container.textContent).toContain("monitoring");
    expect(container.textContent).toContain("grafana/grafana:10.0.0");
  });

  it("renders dependencies section", () => {
    const { container } = renderFactSheet();
    expect(container.textContent).toContain("Dependencies");
    expect(container.textContent).toContain("Depends on");
  });

  it("renders components section", () => {
    const { container } = renderFactSheet();
    expect(container.textContent).toContain("IT Components");
    expect(container.textContent).toContain("node-1");
  });

  it("renders capabilities section", () => {
    const { container } = renderFactSheet();
    expect(container.textContent).toContain("Business Capabilities");
    expect(container.textContent).toContain("Monitoring");
  });

  it("renders version history", () => {
    const { container } = renderFactSheet();
    expect(container.textContent).toContain("Version History");
    expect(container.textContent).toContain("7.0.0");
  });

  it("renders tags", () => {
    const { container } = renderFactSheet();
    expect(container.textContent).toContain("monitoring");
    expect(container.textContent).toContain("dashboards");
  });

  it("has edit button", () => {
    const { container } = renderFactSheet();
    expect(container.textContent).toContain("Edit");
  });
});
