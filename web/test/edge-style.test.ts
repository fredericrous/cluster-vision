import { describe, it, expect } from "vitest";
import { edgeLook } from "../app/lib/edge-style";
import type { Change } from "../app/api.server";

const change = (op: Change["op"]): Change => ({ kind: "edge", op, id: "e", label: "e" });

describe("edgeLook", () => {
  it("keeps the cross-cluster look outside compare mode", () => {
    expect(edgeLook(true, undefined, false)).toEqual({
      animated: true,
      style: { strokeDasharray: "8 4", stroke: "#f59e0b", strokeWidth: 2 },
    });
    expect(edgeLook(false, undefined, false)).toEqual({});
  });

  it("lets the diff style win on cross-cluster edges in compare mode", () => {
    const added = edgeLook(true, change("added"), true);
    const removed = edgeLook(true, change("removed"), true);
    const same = edgeLook(true, undefined, true);
    expect(added.style?.stroke).toBe("#22c55e");
    expect(removed.style?.stroke).toBe("#ef4444");
    expect(removed.style?.strokeDasharray).toBe("2 4");
    expect(same.style?.opacity).toBe(0.35);
    // Distinct from one another…
    expect(new Set([added, removed, same].map((l) => JSON.stringify(l.style))).size).toBe(3);
    // …and still animated as the cross-cluster cue.
    for (const l of [added, removed, same]) expect(l.animated).toBe(true);
  });

  it("styles ordinary edges by diff only", () => {
    expect(edgeLook(false, change("added"), true)).toEqual({
      style: { stroke: "#22c55e", strokeWidth: 2.5 },
    });
    expect(edgeLook(false, undefined, true)).toEqual({ style: { opacity: 0.35 } });
  });
});
