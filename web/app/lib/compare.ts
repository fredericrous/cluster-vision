import { createContext, useContext } from "react";
import type {
  Change,
  DiagramDiff,
  DiffResponse,
  Snapshot,
  SnapshotRef,
} from "../api.server";

/** What the layout loader hands every page in compare mode. */
export interface CompareState {
  enabled: boolean; // API has a database → snapshots exist as a feature
  active: boolean; // `before` is in the URL
  before: string | null;
  after: string;
  snapshots: Snapshot[];
  diff: DiffResponse | null;
  error: string | null;
}

export const inactiveCompare: CompareState = {
  enabled: false,
  active: false,
  before: null,
  after: "now",
  snapshots: [],
  diff: null,
  error: null,
};

export const CompareContext = createContext<CompareState>(inactiveCompare);
export const useCompare = () => useContext(CompareContext);

/** The diff of the diagram a page is rendering, or null outside compare mode. */
export const DiagramDiffContext = createContext<DiagramDiff | null>(null);
export const useDiagramDiff = () => useContext(DiagramDiffContext);

export type DiffState = "added" | "removed" | "changed" | "same";

/** Format a cell the way the Go differ does, so keys line up. */
export function fieldString(v: unknown): string {
  if (v === null || v === undefined) return "";
  if (typeof v === "string") return v;
  if (typeof v === "boolean") return v ? "true" : "false";
  if (typeof v === "number") return Number.isInteger(v) ? String(v) : String(v);
  return JSON.stringify(v);
}

export function rowKey(row: Record<string, unknown>, keyFields: string[]): string {
  return keyFields.map((f) => fieldString(row[f])).join("/");
}

/** Index a diagram diff by element id. */
export function indexChanges(diff: DiagramDiff | null) {
  const ops = new Map<string, Change>();
  if (!diff) return ops;
  for (const c of diff.changes) ops.set(c.id, c);
  return ops;
}

export function shortSha(r: SnapshotRef | undefined): string {
  if (!r || r.id === "now") return "now";
  const sha = r.revisions[0]?.sha;
  return sha ? sha.slice(0, 7) : r.id.slice(0, 8);
}

export function formatWhen(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString(undefined, {
    weekday: "short",
    hour: "2-digit",
    minute: "2-digit",
    month: "short",
    day: "numeric",
  });
}

export function snapshotLabel(s: SnapshotRef): string {
  return `${formatWhen(s.taken_at)} · ${shortSha(s)}`;
}

/** Plain-text/Markdown export of a diff for incident channels. */
export function diffToMarkdown(diff: DiffResponse, pageUrl: string): string {
  const lines: string[] = [];
  const total = diff.total.added + diff.total.removed + diff.total.changed;
  lines.push(
    `**Cluster changes** ${shortSha(diff.from)} → ${shortSha(diff.to)} · ${total} change${total === 1 ? "" : "s"}${diff.drift ? " · changed with no new commit" : ""}`
  );
  lines.push(`${formatWhen(diff.from.taken_at)} → ${diff.to.id === "now" ? "now" : formatWhen(diff.to.taken_at)}`);
  for (const l of diff.compare_links) {
    lines.push(`- ${l.cluster}: ${l.url}`);
  }
  lines.push("");
  for (const d of diff.diagrams) {
    if (d.changes.length === 0) continue;
    lines.push(`### ${d.title} (${d.changes.length})`);
    for (const c of d.changes) {
      const glyph = c.op === "added" ? "+" : c.op === "removed" ? "−" : "~";
      let line = `- ${glyph} ${c.label || c.id}`;
      if (c.fields?.length) {
        line += ": " + c.fields.map((f) => `${f.name} ${f.from || "∅"} → ${f.to || "∅"}`).join(", ");
      }
      lines.push(line);
    }
    lines.push("");
  }
  lines.push(pageUrl);
  return lines.join("\n");
}
