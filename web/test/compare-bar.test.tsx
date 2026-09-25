import { afterEach, describe, it, expect, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { CompareBar } from "../app/components/compare-bar";
import { CompareContext, type CompareState } from "../app/lib/compare";
import type { DiffResponse } from "../app/api.server";

const ref = (id: string) => ({
  id,
  taken_at: "2026-08-26T13:00:00Z",
  revisions: [{ cluster: "Homelab", kustomization: "flux-system", source_kind: "GitRepository", revision: "main@sha1:aaaaaaa111", sha: "aaaaaaa111" }],
});

const diff: DiffResponse = {
  from: ref("a"),
  to: { ...ref("now"), id: "now" },
  diagrams: [],
  total: { added: 0, removed: 0, changed: 1 },
  drift: false,
  // One link per changed root kustomization: same cluster twice.
  compare_links: [
    { cluster: "Homelab", from_sha: "aaaaaaa111", to_sha: "bbbbbbb222", url: "https://git.example/o/infra/compare/aaaaaaa111...bbbbbbb222" },
    { cluster: "Homelab", from_sha: "ccccccc333", to_sha: "ddddddd444", url: "https://git.example/o/apps/compare/ccccccc333...ddddddd444" },
  ],
};

const state: CompareState = {
  enabled: true,
  active: true,
  before: "a",
  after: "now",
  snapshots: [],
  diff,
  error: null,
};

function renderBar(route: string) {
  return render(
    <MemoryRouter initialEntries={[route]}>
      <CompareContext value={state}>
        <CompareBar />
      </CompareContext>
    </MemoryRouter>
  );
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("CompareBar", () => {
  it("copies the URL of the current route, with its compare selectors", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    renderBar("/network?before=a&after=now");
    await user.click(screen.getByRole("button", { name: "Copy as Markdown" }));
    expect(writeText).toHaveBeenCalledOnce();
    const md = writeText.mock.calls[0][0] as string;
    expect(md.trim().endsWith("/network?before=a&after=now")).toBe(true);
  });

  it("renders one distinctly labelled link per changed kustomization", () => {
    const errors = vi.spyOn(console, "error").mockImplementation(() => {});
    renderBar("/network?before=a");
    expect(screen.getByRole("link", { name: /Homelab · o\/infra/ })).toHaveAttribute(
      "href",
      diff.compare_links[0].url
    );
    expect(screen.getByRole("link", { name: /Homelab · o\/apps/ })).toBeInTheDocument();
    expect(errors.mock.calls.flat().join(" ")).not.toMatch(/same key/);
  });
});
