import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "./msw-handlers";
import {
  ApiError,
  compareParams,
  fetchDiagrams,
  fetchDiff,
  fetchSnapshots,
  type DiffResponse,
} from "../app/api.server";
import {
  diffToMarkdown,
  fieldString,
  indexChanges,
  rowKey,
  shortSha,
} from "../app/lib/compare";

const snap = (id: string, sha: string, takenAt: string) => ({
  id,
  taken_at: takenAt,
  revisions: [{ cluster: "Homelab", kustomization: "flux-system", source_kind: "GitRepository", revision: `main@sha1:${sha}`, sha }],
});

const diffResponse: DiffResponse = {
  from: snap("a", "aaaaaaa111", "2026-08-26T13:00:00Z"),
  to: { id: "now", taken_at: "2026-08-27T14:30:00Z", revisions: snap("x", "bbbbbbb222", "").revisions },
  diagrams: [
    {
      diagram_id: "certificates",
      title: "Certificates",
      type: "table",
      key_fields: ["cluster", "namespace", "name"],
      from: snap("a", "aaaaaaa111", "2026-08-26T13:00:00Z"),
      to: { id: "now", taken_at: "2026-08-27T14:30:00Z", revisions: [] },
      changes: [
        { kind: "row", op: "changed", id: "Homelab/b/web-tls", label: "b / web-tls", fields: [{ name: "issuer", from: "x", to: "y" }] },
        { kind: "row", op: "removed", id: "Homelab/a/old", label: "a / old" },
      ],
      advisory: [],
      summary: { added: 0, removed: 1, changed: 1 },
      drift: false,
    },
    {
      diagram_id: "topology",
      title: "Topology",
      type: "mermaid",
      from: snap("a", "aaaaaaa111", "2026-08-26T13:00:00Z"),
      to: { id: "now", taken_at: "2026-08-27T14:30:00Z", revisions: [] },
      changes: [],
      advisory: [],
      summary: { added: 0, removed: 0, changed: 0 },
      drift: false,
    },
  ],
  total: { added: 0, removed: 1, changed: 1 },
  drift: false,
  compare_links: [{ cluster: "Homelab", from_sha: "aaaaaaa111", to_sha: "bbbbbbb222", url: "https://github.com/o/r/compare/aaaaaaa111...bbbbbbb222" }],
};

describe("compareParams", () => {
  it("is off without a before param", () => {
    expect(compareParams(new Request("http://x/certificates"))).toEqual({ before: null, after: "now" });
  });
  it("reads before/after", () => {
    expect(compareParams(new Request("http://x/certificates?before=deploy&after=abc"))).toEqual({ before: "deploy", after: "abc" });
  });
});

describe("fetchDiagrams in compare mode", () => {
  it("reads the after snapshot instead of the live state", async () => {
    server.use(
      http.get("http://localhost:8080/api/snapshots/abc/diagrams", () =>
        HttpResponse.json({ diagrams: [{ id: "topology", title: "T", type: "mermaid", content: "old" }], generated_at: "2026-08-26T13:00:00Z" })
      )
    );
    const data = await fetchDiagrams(new Request("http://x/topology?before=prev&after=abc"));
    expect(data.diagrams[0].content).toBe("old");
    expect(data.generated_at).toBe("2026-08-26T13:00:00Z");
  });
});

describe("fetchDiff / fetchSnapshots", () => {
  it("passes selectors through and returns the diff", async () => {
    let seen = "";
    server.use(
      http.get("http://localhost:8080/api/diff", ({ request }) => {
        seen = new URL(request.url).search;
        return HttpResponse.json(diffResponse);
      })
    );
    const d = await fetchDiff("deploy", "now");
    expect(seen).toBe("?from=deploy&to=now");
    expect(d.total.changed).toBe(1);
  });

  it("surfaces the API's selector error as ApiError.detail", async () => {
    server.use(
      http.get("http://localhost:8080/api/diff", () =>
        HttpResponse.json({ error: 'selector "prev": no earlier snapshot' }, { status: 404 })
      )
    );
    const err = await fetchDiff("prev", "now").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(404);
    expect(err.detail).toContain("no earlier snapshot");
    expect(err.message).toContain("API error: 404");
  });

  it("lists snapshots", async () => {
    server.use(
      http.get("http://localhost:8080/api/snapshots", () =>
        HttpResponse.json({ snapshots: [{ ...snap("a", "aaaaaaa111", "2026-08-26T13:00:00Z"), model_version: 1, summary: { previous_id: null, total: { added: 0, removed: 0, changed: 0 }, diagrams: {}, drift: false }, new_revision: false }] })
      )
    );
    const list = await fetchSnapshots(5);
    expect(list).toHaveLength(1);
    expect(list[0].id).toBe("a");
  });
});

describe("compare helpers", () => {
  it("formats fields like the Go differ", () => {
    expect(fieldString("x")).toBe("x");
    expect(fieldString(3)).toBe("3");
    expect(fieldString(true)).toBe("true");
    expect(fieldString(null)).toBe("");
  });

  it("keys rows in key-field order", () => {
    expect(rowKey({ name: "web-tls", cluster: "Homelab", namespace: "b", ready: "yes" }, ["cluster", "namespace", "name"])).toBe("Homelab/b/web-tls");
  });

  it("indexes changes by id", () => {
    const ops = indexChanges(diffResponse.diagrams[0]);
    expect(ops.get("Homelab/b/web-tls")?.op).toBe("changed");
    expect(ops.size).toBe(2);
  });

  it("shortens shas and handles now", () => {
    expect(shortSha(diffResponse.from)).toBe("aaaaaaa");
    expect(shortSha(diffResponse.to)).toBe("now");
  });

  it("renders markdown with only the changed views", () => {
    const md = diffToMarkdown(diffResponse, "https://cv/certificates?before=a");
    expect(md).toContain("**Cluster changes** aaaaaaa → now · 2 changes");
    expect(md).toContain("### Certificates (2)");
    expect(md).toContain("- ~ b / web-tls: issuer x → y");
    expect(md).toContain("- − a / old");
    expect(md).not.toContain("Topology");
    expect(md).toContain("https://github.com/o/r/compare/aaaaaaa111...bbbbbbb222");
    expect(md.trim().endsWith("https://cv/certificates?before=a")).toBe(true);
  });
});
