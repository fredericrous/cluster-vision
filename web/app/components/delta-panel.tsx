import { useState } from "react";
import { Badge, Button, Callout, Inline, Stack, Text } from "@duro-app/ui";
import type { Change, DiagramDiff } from "../api.server";

function glyph(op: Change["op"]) {
  if (op === "added") return <Badge variant="success" size="sm">+</Badge>;
  if (op === "removed") return <Badge variant="error" size="sm">−</Badge>;
  return <Badge variant="warning" size="sm">~</Badge>;
}

function ChangeRow({ change, onFocus }: { change: Change; onFocus?: (c: Change) => void }) {
  const body = (
    <Stack gap="xs">
      <Text variant="bodySm">{change.label || change.id}</Text>
      {change.fields?.map((f) => (
        <Text key={f.name} variant="caption" color="muted">
          {f.name}: {f.from || "∅"} → {f.to || "∅"}
        </Text>
      ))}
    </Stack>
  );
  return (
    <Inline gap="sm" align="start">
      {glyph(change.op)}
      {onFocus ? (
        <Button variant="link" size="small" onClick={() => onFocus(change)}>
          {body}
        </Button>
      ) : (
        body
      )}
    </Inline>
  );
}

/** The flat list an on-call engineer reads: every change in this view,
 *  then advisory (external) changes collapsed underneath. */
export function DeltaPanel({
  diff,
  onFocus,
}: {
  diff: DiagramDiff;
  onFocus?: (c: Change) => void;
}) {
  const [showAdvisory, setShowAdvisory] = useState(false);
  const total = diff.summary.added + diff.summary.removed + diff.summary.changed;

  return (
    <Stack gap="md">
      <Inline gap="sm" align="baseline">
        <Text variant="label">
          {diff.title} · {total} change{total === 1 ? "" : "s"}
        </Text>
      </Inline>

      {diff.drift && (
        <Callout variant="warning" align="start">
          Changed with no new commit — something moved outside GitOps: an
          operator upgrade, a manual edit, or an image rollout.
        </Callout>
      )}

      {total === 0 && (
        <Text variant="bodySm" color="muted">
          No changes in this view.
        </Text>
      )}

      <Stack gap="sm">
        {diff.changes.map((c) => (
          <ChangeRow key={`${c.kind}:${c.id}`} change={c} onFocus={onFocus} />
        ))}
      </Stack>

      {diff.advisory.length > 0 && (
        <Stack gap="sm">
          <Button
            variant="link"
            size="small"
            onClick={() => setShowAdvisory((v) => !v)}
            aria-label={showAdvisory ? "Hide external changes" : "Show external changes"}
          >
            <Text variant="caption" color="muted">
              {showAdvisory ? "▾" : "▸"} Also changed (external) · {diff.advisory.length}
            </Text>
          </Button>
          {showAdvisory &&
            diff.advisory.map((c) => (
              <ChangeRow key={`adv:${c.id}`} change={c} />
            ))}
        </Stack>
      )}
    </Stack>
  );
}
