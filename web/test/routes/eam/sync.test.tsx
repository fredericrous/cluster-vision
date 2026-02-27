import { describe, it, expect, vi, afterEach } from "vitest";
import { screen, act } from "@testing-library/react";
import Sync from "../../../app/routes/eam/sync";
import { mockSyncLog } from "../../mocks";
import { renderWithRouter } from "../../render-helper";

const logs = [
  mockSyncLog({ apps_created: 5, apps_updated: 3, components_created: 2 }),
  mockSyncLog({ apps_created: 1, apps_updated: 0, components_created: 0 }),
];

let fetchSpy: ReturnType<typeof vi.fn> | null = null;

afterEach(() => {
  if (fetchSpy) {
    vi.restoreAllMocks();
    fetchSpy = null;
  }
});

function mockFetchConfig(aiEnabled: boolean) {
  fetchSpy = vi.fn().mockImplementation((url: string | URL | Request) => {
    const urlStr = typeof url === "string" ? url : url instanceof URL ? url.toString() : url.url;
    if (urlStr.endsWith("/config")) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ eam: true, ai: aiEnabled }),
      });
    }
    // Fallback for sync/logs requests
    return Promise.resolve({
      ok: true,
      json: () => Promise.resolve([]),
    });
  });
  vi.stubGlobal("fetch", fetchSpy);
}

function renderSync() {
  return renderWithRouter(
    <Sync
      loaderData={logs}
      params={{}}
      matches={[]}
      actionData={undefined}
    />
  );
}

describe("Sync", () => {
  it("renders heading", () => {
    mockFetchConfig(false);
    renderSync();
    expect(screen.getByText("Import / Sync")).toBeInTheDocument();
  });

  it("renders sync history entries", () => {
    mockFetchConfig(false);
    const { container } = renderSync();
    expect(container.textContent).toContain("created");
    expect(container.textContent).toContain("updated");
  });

  it("has Sync Now button", () => {
    mockFetchConfig(false);
    const { container } = renderSync();
    expect(container.textContent).toContain("Sync Now");
  });

  it("shows Re-analyze with AI when ai is enabled", async () => {
    mockFetchConfig(true);
    const { container } = renderSync();

    await act(async () => {
      await new Promise((r) => setTimeout(r, 50));
    });

    expect(container.textContent).toContain("Re-analyze with AI");
  });

  it("hides Re-analyze with AI when ai is disabled", async () => {
    mockFetchConfig(false);
    const { container } = renderSync();

    await act(async () => {
      await new Promise((r) => setTimeout(r, 50));
    });

    expect(container.textContent).not.toContain("Re-analyze with AI");
  });
});
