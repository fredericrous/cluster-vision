import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { createMemoryRouter, data, Outlet, RouterProvider, useRouteError } from "react-router";
import { describeRouteError } from "../app/lib/route-error";
import { liveSearch } from "../app/components/route-error-boundary";
import ViewBoundary, { ErrorBoundary } from "../app/routes/view-boundary";

const response = (status: number, body: unknown) => ({
  status,
  statusText: "",
  internal: false,
  data: body,
});

describe("describeRouteError", () => {
  it("explains a 503 as the first sync still running", () => {
    const v = describeRouteError(response(503, { error: "no cluster data available yet" }));
    expect(v.status).toBe(503);
    expect(v.variant).toBe("info");
    expect(v.title).toBe("Data is still loading");
    expect(v.detail).toContain("first cluster sync");
  });

  it("names a missing snapshot on 404", () => {
    const v = describeRouteError(response(404, { error: "snapshot not found", kind: "snapshot" }));
    expect(v.title).toBe("Snapshot not found");
    // A bare "snapshot not found" only repeats the title: explain instead.
    expect(v.detail).toContain("does not exist, or it has expired");
    const d = describeRouteError(response(404, { error: 'selector "x": no such snapshot' }));
    expect(d.detail).toBe('selector "x": no such snapshot');
  });

  it("names a missing diagram on 404", () => {
    const v = describeRouteError(response(404, { error: 'Diagram "x" not found', kind: "diagram" }));
    expect(v.title).toBe("Diagram not found");
  });

  it("falls back to a generic message", () => {
    expect(describeRouteError(response(500, { error: "boom" })).detail).toBe("boom");
    const v = describeRouteError(new Error("secret internals"));
    expect(v.status).toBeNull();
    expect(v.detail).toBe("An unexpected error occurred.");
  });
});

describe("liveSearch", () => {
  it("drops the compare selectors and keeps the rest", () => {
    expect(liveSearch("?before=prev&after=abc&q=x")).toBe("?q=x");
    expect(liveSearch("?after=abc")).toBe("");
  });
});

describe("view error boundary", () => {
  it("renders the error inside the layout, keeping the sidebar", async () => {
    const router = createMemoryRouter(
      [
        {
          path: "/",
          element: (
            <>
              <nav>Sidebar</nav>
              <Outlet />
            </>
          ),
          children: [
            {
              Component: ViewBoundary,
              // Framework mode passes `error` as a prop; a plain data router
              // does not, so hand it over the way the framework would.
              ErrorBoundary: function Boundary() {
                const error = useRouteError();
                return <ErrorBoundary {...({ error } as Parameters<typeof ErrorBoundary>[0])} />;
              },
              children: [
                {
                  index: true,
                  loader: () => {
                    throw data({ error: "no cluster data available yet" }, { status: 503 });
                  },
                  Component: () => <p>view</p>,
                },
              ],
            },
          ],
        },
      ],
      { initialEntries: ["/"] }
    );
    render(<RouterProvider router={router} />);
    expect(await screen.findByText("Data is still loading")).toBeInTheDocument();
    expect(screen.getByText("Sidebar")).toBeInTheDocument();
    expect(screen.queryByText("view")).not.toBeInTheDocument();
  });
});
