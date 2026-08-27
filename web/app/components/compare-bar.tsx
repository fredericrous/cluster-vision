import { useCallback, useMemo, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import {
  Alert,
  Badge,
  Button,
  Cluster,
  Inline,
  Select,
  Stack,
  Text,
} from "@duro-app/ui";
import {
  diffToMarkdown,
  formatWhen,
  shortSha,
  snapshotLabel,
  useCompare,
} from "../lib/compare";

/** The route to deep-link a diagram's changes to. Diagram ids map 1:1 to
 *  routes except the `topology-*` sections and `security-chart`. */
export function routeForDiagram(id: string): string {
  if (id.startsWith("topology")) return "/topology";
  if (id === "security-chart") return "/security";
  return `/${id}`;
}

const BEFORE_PRESETS: { value: string; label: string }[] = [
  { value: "prev", label: "Previous change" },
  { value: "deploy", label: "Last deploy" },
];

/** Compare control + summary strip. Rendered by the layout on every page.
 *  Off: a single "Changes" button. On: Before / After pickers, the
 *  cluster-wide summary with per-view links, Copy as Markdown, Exit. */
export function CompareBar() {
  const compare = useCompare();
  const location = useLocation();
  const navigate = useNavigate();
  const [copied, setCopied] = useState(false);

  const setParams = useCallback(
    (before: string | null, after: string) => {
      const params = new URLSearchParams(location.search);
      if (before) params.set("before", before);
      else params.delete("before");
      if (after && after !== "now") params.set("after", after);
      else params.delete("after");
      const qs = params.toString();
      navigate(`${location.pathname}${qs ? `?${qs}` : ""}`);
    },
    [location.pathname, location.search, navigate]
  );

  const pageUrl = useMemo(() => {
    if (typeof window === "undefined") return "";
    return window.location.href;
  }, []);

  const copyMarkdown = useCallback(async () => {
    if (!compare.diff) return;
    try {
      await navigator.clipboard.writeText(diffToMarkdown(compare.diff, pageUrl));
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  }, [compare.diff, pageUrl]);

  if (!compare.enabled) return null;

  if (!compare.active) {
    if (compare.snapshots.length === 0) {
      return (
        <Text variant="caption" color="muted">
          Change history starts now — come back after the next cluster change.
        </Text>
      );
    }
    return (
      <Inline gap="sm" align="center">
        <Button variant="secondary" size="small" onClick={() => setParams("prev", "now")}>
          Changes
        </Button>
        <Text variant="caption" color="muted">
          Compare this view with an earlier cluster state
        </Text>
      </Inline>
    );
  }

  const { diff } = compare;
  const total = diff ? diff.total.added + diff.total.removed + diff.total.changed : 0;
  const changedViews = diff
    ? diff.diagrams.filter((d) => d.summary.added + d.summary.removed + d.summary.changed > 0)
    : [];

  return (
    <Stack gap="sm">
      <Cluster gap="md" align="center">
        <Inline gap="xs" align="center">
          <Text variant="caption" color="muted" weight="medium">
            Before
          </Text>
          <Select.Root
            value={compare.before ?? "prev"}
            onValueChange={(v) => setParams(v || "prev", compare.after)}
          >
            <Select.Trigger aria-label="Before">
              <Select.Value placeholder="Before" />
              <Select.Icon />
            </Select.Trigger>
            <Select.Popup>
              {BEFORE_PRESETS.map((p) => (
                <Select.Item key={p.value} value={p.value}>
                  <Select.ItemText>{p.label}</Select.ItemText>
                </Select.Item>
              ))}
              {compare.snapshots.map((s) => (
                <Select.Item key={s.id} value={s.id}>
                  <Select.ItemText>
                    {snapshotLabel(s)}
                    {s.new_revision ? " · deploy" : ""}
                  </Select.ItemText>
                </Select.Item>
              ))}
            </Select.Popup>
          </Select.Root>
        </Inline>

        <Inline gap="xs" align="center">
          <Text variant="caption" color="muted" weight="medium">
            After
          </Text>
          <Select.Root
            value={compare.after}
            onValueChange={(v) => setParams(compare.before ?? "prev", v || "now")}
          >
            <Select.Trigger aria-label="After">
              <Select.Value placeholder="Now" />
              <Select.Icon />
            </Select.Trigger>
            <Select.Popup>
              <Select.Item value="now">
                <Select.ItemText>Now</Select.ItemText>
              </Select.Item>
              {compare.snapshots.map((s) => (
                <Select.Item key={s.id} value={s.id}>
                  <Select.ItemText>
                    {snapshotLabel(s)}
                    {s.new_revision ? " · deploy" : ""}
                  </Select.ItemText>
                </Select.Item>
              ))}
            </Select.Popup>
          </Select.Root>
        </Inline>

        <Button variant="link" size="small" onClick={() => setParams(null, "now")}>
          Exit compare
        </Button>
      </Cluster>

      {compare.error && <Alert variant="warning">{compare.error}</Alert>}

      {diff && (
        <Cluster gap="md" align="center">
          <Text variant="bodySm" weight="semibold">
            {shortSha(diff.from)} → {shortSha(diff.to)} · {total} change{total === 1 ? "" : "s"}
          </Text>
          <Text variant="caption" color="muted">
            {formatWhen(diff.from.taken_at)} →{" "}
            {diff.to.id === "now" ? "now" : formatWhen(diff.to.taken_at)}
          </Text>
          {diff.drift && (
            <Badge variant="warning" size="sm">
              changed with no new commit
            </Badge>
          )}
          {changedViews.map((d) => (
            <Link
              key={d.diagram_id}
              to={`${routeForDiagram(d.diagram_id)}${location.search}`}
              style={{ textDecoration: "none" }}
            >
              <Badge variant={location.pathname === routeForDiagram(d.diagram_id) ? "info" : "default"} size="sm">
                {d.title} {d.summary.added + d.summary.removed + d.summary.changed}
              </Badge>
            </Link>
          ))}
          {diff.compare_links.map((l) => (
            <a key={l.cluster} href={l.url} target="_blank" rel="noreferrer">
              <Text variant="caption" color="accent">
                {l.cluster}: {l.from_sha.slice(0, 7)}…{l.to_sha.slice(0, 7)} ↗
              </Text>
            </a>
          ))}
          <Button variant="link" size="small" onClick={copyMarkdown}>
            {copied ? "Copied" : "Copy as Markdown"}
          </Button>
        </Cluster>
      )}
    </Stack>
  );
}
