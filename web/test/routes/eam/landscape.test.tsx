import { describe, it, expect } from "vitest";
import { screen } from "@testing-library/react";
import Landscape from "../../../app/routes/eam/landscape";
import { mockLandscapeData } from "../../mocks";
import { renderWithRouter } from "../../render-helper";

const landscapeData = mockLandscapeData();

function renderLandscape(overrides = {}) {
  return renderWithRouter(
    <Landscape
      loaderData={{ ...landscapeData, ...overrides }}
      params={{}}
      matches={[] as any}
      actionData={undefined}
    />
  );
}

describe("Landscape", () => {
  it("renders heading", () => {
    renderLandscape();
    expect(screen.getByText("Application Landscape")).toBeInTheDocument();
  });

  it("renders capability section headers", () => {
    const { container } = renderLandscape();
    expect(container.textContent).toContain("Monitoring & Observability");
    expect(container.textContent).toContain("Dashboards");
  });

  it("renders app badges within capabilities", () => {
    const { container } = renderLandscape();
    expect(container.textContent).toContain("Grafana");
  });

  it("renders unmapped apps section", () => {
    const { container } = renderLandscape();
    expect(container.textContent).toContain("Unmapped Applications");
    expect(container.textContent).toContain("authelia");
  });

  it("shows empty state when no data", () => {
    const { container } = renderLandscape({ capabilities: [], unmapped: [] });
    expect(container.textContent).toContain("No applications or capabilities");
  });
});
