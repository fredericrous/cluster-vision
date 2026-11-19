# Sweep cluster-vision onto greenrack: reviews

All seats ran on opus-5-5. Rounds: 1 = full panel, 2 = delta, then Codex, then a delta on Codex's high finding.

- **backend**: approve-with-changes (r1 37k, 45 s; r2 52k, 23 s; deltas 65k, 68 s).
  - r1: coarse-pointer change is recorded, not a stop (accepted). Exact doctor pin (rejected: the fleet runs `@^7`; follow-up). Merge-method check (accepted). Screenshot count (accepted).
  - r2, carried: the Phase 5 Heading note; hover and focus-visible at 1280 only; the count is 24 × 2 = 48 per build, 192 in total.
  - Delta: the snapshots file/directory clash (high, accepted).
- **lang:typescript**: approve (r1 46k, 86 s; r2 55k, 33 s).
  - r1: print-config on `web/test/compare-bar.test.tsx`, because lint never reads `web/test/` (accepted). Phase 2 fallout bounds (accepted).
- **platform**: approve (r1 57k, 86 s; r2 65k, 19 s).
  - r1, all accepted: the Flux OCI → kustomization → helmrelease chain, gated on the spec version (high); the chart and image checked through the zot mirror, not the index (high); a named rollback chain and a no-migration diff.
- **react**: approve-with-changes (r1 33k, 29 s; r2 40k, 9 s).
  - r1, all accepted: touch emulation with `NODE_H`; the a11y tree and Tab order; the Heading non-goal; vite `optimizeDeps`.
  - r2, carried: the a11y capture belongs in the Baseline list too.
- **tui**: approve-with-changes (r1 36k, 35 s; r2 47k, 28 s; deltas 48k, 15 s).
  - r1, all accepted: no `@duro-app` in the new hooks; `git status` clean after a hook run; read "old names left".
  - r2, carried: confirm before `checkout -- .`; the Heading variants under typecheck (the design lookup says they still compile); name the homelab run; the squash wording.
  - Delta: the snapshots clash (high, accepted, same as unix). Carried: the handler's stderr goes to a log; `<p>` naming; "query removed" wording.
- **ui-design**: approve (r1 39k, 53 s; r2 45k, 12 s).
  - r1: the API-down `/topology` error state (accepted); hover and focus-visible (accepted); the Heading line (rejected as a duplicate of react).
  - r2, carried: the screenshot count.
- **unix**: approve-with-changes (r1 39k, 47 s; r2 44k, 12 s; deltas 49k, 14 s).
  - r1, all accepted: bind 127.0.0.1; reset before a sweep rerun; audit compared by advisory ID.
  - r2, carried: a bounded wait for the OCI push; `--plain-http` for the mirror.
  - Delta: blocking, `snapshots` cannot be both a file and a directory (accepted; handler script). Carried: name the script `<scratch>/serve-api.py`, and keep `<p>` for the port.
- **codex**: approve-with-changes.
  - Finding 1, high, accepted: compare mode holds the only DS `Select` popups.
  - Finding 2, medium, carried: check the charts PR content against the tag, and render the 0.25.6 and 0.26.0 manifests.
- **design-system-lookup** (before the panel): every component's props are unchanged in 7.0.1. Heading `headingSm/Md/Lg` is deprecated but works, and no rule reports it. `greenrack/no-raw-html-element` keeps `allow`.
