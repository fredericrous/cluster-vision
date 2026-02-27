import { describe, it, expect } from "vitest";
import { screen } from "@testing-library/react";
import Applications from "../../../app/routes/eam/applications";
import { mockApplication } from "../../mocks";
import { renderWithRouter } from "../../render-helper";

const app1 = mockApplication({
  name: "grafana",
  display_name: "Grafana",
  status: "active",
  business_criticality: "high",
});
const app2 = mockApplication({
  name: "authelia",
  status: "maintenance",
  business_criticality: "medium",
});

function renderApplications() {
  return renderWithRouter(
    <Applications
      loaderData={{ items: [app1, app2], total: 2 }}
      params={{}}
      matches={[]}
      actionData={undefined}
    />
  );
}

describe("Applications", () => {
  it("renders heading", () => {
    renderApplications();
    expect(screen.getByText("Applications")).toBeInTheDocument();
  });

  it("renders total count", () => {
    const { container } = renderApplications();
    expect(container.textContent).toContain("2 applications discovered");
  });

  it("renders app names", () => {
    const { container } = renderApplications();
    expect(container.textContent).toContain("Grafana");
    expect(container.textContent).toContain("authelia");
  });

  it("renders status values", () => {
    const { container } = renderApplications();
    expect(container.textContent).toContain("active");
    expect(container.textContent).toContain("maintenance");
  });

  it("renders criticality values", () => {
    const { container } = renderApplications();
    expect(container.textContent).toContain("high");
    expect(container.textContent).toContain("medium");
  });

  it("has New Application button", () => {
    const { container } = renderApplications();
    expect(container.textContent).toContain("New Application");
  });
});
