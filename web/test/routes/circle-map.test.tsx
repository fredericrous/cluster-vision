import { describe, it, expect } from "vitest";
import { screen } from "@testing-library/react";
import CircleMap from "../../app/routes/circle-map";
import { mockDiagrams } from "../msw-handlers";
import { renderWithRouter } from "../render-helper";

function renderCircleMap(diagram = mockDiagrams[0]) {
  return renderWithRouter(
    <CircleMap
      loaderData={{ diagram, generatedAt: "2026-07-27T10:00:00Z" }}
      params={{}}
      matches={[] as any}
      actionData={undefined}
    />
  );
}

describe("CircleMap", () => {
  it("renders heading and subtitle", () => {
    renderCircleMap();
    expect(
      screen.getByRole("heading", { name: "Circle Map" })
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Flux kustomization layers as nested circles/)
    ).toBeInTheDocument();
  });

  it("renders a circle per layer and per node", () => {
    const { container } = renderCircleMap();
    // Scoped to the pack layout's own svg — the legend below it draws one
    // swatch circle per layer colour, which is not part of the map.
    const map = container.querySelector("svg[viewBox]");
    // 3 nodes + 3 layer groups (root circle is skipped)
    expect(map?.querySelectorAll("circle").length).toBe(6);
  });

  it("labels the layer groups", () => {
    const { container } = renderCircleMap();
    expect(container.textContent).toContain("platform-foundation");
    expect(container.textContent).toContain("monitoring");
  });

  it("shows empty state for non-flow diagrams", () => {
    renderCircleMap(mockDiagrams[1]);
    expect(screen.getByText("No flow data available.")).toBeInTheDocument();
  });
});
