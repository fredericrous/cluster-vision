---
status: active
branch: chore/greenrack
repos: [cluster-vision, homelab, decisions]
adrs: [ADR-0075, ADR-0076]
plan-id: p-01M7T6CENX4FA5BMW93V2GFFJ7
---

# Sweep cluster-vision onto greenrack

## Review panel

👉 **Decide:** none. Approve if a no-UI-change sweep, browser-checked over a frozen API (compare mode included), is enough before v0.26.0 ships.
📍 cluster-vision, homelab, decisions · 5 phases · 1 release · 1 step for you · next: Baseline in `cluster-vision-wt-greenrack`. Panel: backend, lang:typescript, platform, react, tui, ui-design, unix (opus-5-5), codex.
**Changed by review:** Flux reconcile goes OCI source → kustomization → helmrelease, with a zot mirror check; compare mode (the DS `Select`) is frozen and checked, served by a handler script; 390 runs emulate touch, and the flow-node height stays 44.
**Verdicts:** 4 approve-with-changes, 3 approve; codex approve-with-changes; 1 blocker fixed in the delta; 17 carried; words 2286 → 2911 (×1.27, the excess is the blocker and high fixes); remove 0, merge 0.

📄 Full reviews: [2026-11-19-sweep-cluster-vision-onto-greenrack.reviews.md](2026-11-19-sweep-cluster-vision-onto-greenrack.reviews.md)

## Picture

```
 v0.25.6 (live since 14:42Z) = origin/main 642b28f
   |
   |  chore/greenrack, one PR (web/, CLAUDE.md, greengate.conf, .claude/)
   |  commit 1          commit 2              commit 3
   |  Duro 5.1 -------> Duro 6.3 -----------> greenrack 7.0.1
   |  (all at 5.45)     + @aval-adr config    new names, --gr-* in
   |                    as a direct dep       7 CSS files, hooks
   v
 v0.26.0 = v0.25.6 + the sweep -> charts PR -> homelab pin -> pod rolls
```

The sweep is three bisectable commits on one PR, released as v0.26.0 with
nothing else in it, so a fault in 0.26.0 points at the sweep alone.

## Context

Programme greenrack-rename, ticket t08 (decisions
`docs/programmes/greenrack-rename/t08-fleet-sweep.md`). cluster-vision is the
third t08 app. The plan copies customer-vision's
(`customer-vision:docs/plans/2026-11-19-sweep-customer-vision-onto-greenrack.md`)
and differs where this app does.

**Release scope.** Main was released first today. v0.25.6 went live at 14:42Z
through cluster-vision#65, charts#535 and homelab#1243. Since then main holds
only `642b28f chore: bump chart to 0.25.6 [skip ci]`. So there is no Phase 0:
the sweep starts from main and v0.26.0 carries only the sweep.

cluster-vision `web/` today (read 2026-11-19):

- **npm, not pnpm.** `web/package-lock.json` is lockfileVersion 3.
  `web/.npmrc` sets `legacy-peer-deps=true`, so peers are never installed. CI
  and the Dockerfile run `npm ci` on Node 24.
- **Versions.** `@duro-app/ui`, `@duro-app/eslint-config` and
  `@duro-app/eslint-plugin` are ^5.1.0 (lock 5.1.0). `@duro-app/tokens` comes
  only through ui, never imported directly.
- **The app consumes the precompiled bundle.** There is no StyleX/babel
  tokens pipeline and no postcss. `root.tsx` imports `@duro-app/ui/reset.css`
  and `@duro-app/ui/dist/index.css`, and `data-table.tsx` imports
  `@duro-app/ui/table`. All three subpaths are exported by 5.45, 6.3 and
  greenrack-ui 7.0.1 (read from the tarballs).
- **The app reads 31 `--duro-*` custom properties directly**, in `app.css` (16
  aliases) and 6 CSS modules: `diagram-page`, `flow-diagram`, `flow-group`,
  `flow-node`, `markdown-table` and `layout`. Each of the 31 is defined in
  `dist/index.css` of ui 5.1.0, 5.45.0, 6.3.0 and greenrack-ui 7.0.1 (as
  `--gr-*`), and each resolves to the same values in all four versions (both
  themes' colours, every px). That includes `--duro-size-touch-target` at 44px,
  which `flow-diagram.tsx`'s hardcoded `NODE_H = 44` and `NODE_PAD = 29`
  mirror for the dagre layout. So no token change moves the diagrams.
- **Peers.** 5.45 adds react-native peers and a `qrcode` runtime dependency.
  With `legacy-peer-deps`, the peers are not installed. The other peers
  (react 19, react-strict-dom ^0.0.55, effect 3, react-hook-form,
  @tanstack/react-table) are unchanged at 5.45, 6.3 and 7.0.1.
- **ESLint.** `web/eslint.config.js` takes `base`, `react` and `tests` from
  `@duro-app/eslint-config`, with `react` mapped to `app/**/*.{ts,tsx}`. It has
  no `effect`, no `adrLint` and no `stack`. Nine per-file
  `duro/no-raw-html-element` allow lists apply last.
  - At 6.3, `base` and `tests` move to `@aval-adr/eslint-config`, which is a
    peer `>=0.16.0 <1`. Under `legacy-peer-deps` it must be a direct
    devDependency.
  - greenrack-eslint-config 7.0.1 exports only `react` and keeps that peer.
- **Release path.** A `v*` tag runs `release.yaml`, which builds the image
  and pushes `chore: bump chart to X [skip ci]` to main. It opens the
  `fredericrous/charts` PR "update cluster-vision to X", which has to merge
  green before homelab can pin
  `kubernetes/homelab/apps/cluster-vision/helm-release.yaml`. The live host is
  `infra.daddyshome.fr`.
- **CI builds the image on the PR.** The `image` job in `ci.yaml` builds the
  Dockerfile and smoke-runs it, so no Docker-less builder workaround is
  needed.
- **Local serve.** Run `npm run build`, then `API_URL=http://127.0.0.1:<p>
  npm start`.
  - `<p>` is the frozen-API handler script described in the next bullet,
    bound to 127.0.0.1 and serving one frozen copy of the live
    `/api/diagrams`, `/api/config` and `/api/snapshots` (layout and home fetch
    the last one). The copies are taken once, through `kubectl port-forward`,
    and every build compares against the same files.
  - Compare mode is frozen too, for one before/after snapshot pair with
    populated changes: `/api/diff` (the query string is dropped, so the one
    capture answers it) and `/api/snapshots/<after>/diagrams`. A plain
    `http.server` cannot hold `snapshots` as both a file and a directory, so
    `<p>` is instead a small handler script kept outside both trees, run with
    `python3 -I <script> --bind 127.0.0.1 <p>`. It maps each exact path to its
    capture file (`<scratch>/api/snapshots.json`,
    `<scratch>/api/snapshot-<after>-diagrams.json`, and so on), sends
    `application/json`, and returns 404 with the path on stderr for anything
    else; a 404 line after a run means a fetch was not frozen. At the
    baseline and each commit, `/dependencies` in compare mode is checked at
    1280 and touch-emulated 390: both `Select` popups open, the captured
    snapshots are selected by keyboard with focus returned, and the delta
    panel and diagram highlights match.
- **Sweep dry run** (`--check`, main 642b28f): npm-name 52, npm-name-escaped 0,
  css-var 99, data-attr 0, eslint-rule 3, lexical-pkg 0, editor-theme 0.
- **Hooks.** `.claude/hooks/duro-catalog.sh` and `duro-plan.sh` sit beside
  `aval-heads.sh`. `settings.local.json` holds no duro reference. `.gitignore`
  keeps `.claude/.duro-session.cache*` until t13.
- **Board** (2026-11-19 15:2xZ): cluster-vision has no other worktree. homelab
  has no worktree on `apps/cluster-vision`.

## Goal

cluster-vision `web/` builds, lints, tests and renders on
`@greenharness/greenrack-ui`, `-eslint-config` and `-eslint-plugin` ^7.0.1.
No `@duro-app/`, `--duro-`, `duro/` rule id or `duro-*` hook name is left
outside the allowlist named in the PR. v0.26.0 runs in the cluster.

## Non-goals

- The mermaid pins in `mermaid-diagram.tsx` (`layout: "dagre"`,
  `look: "classic"`, `wrappingWidth: 200`) and mermaid 12.3.0 stay as they
  are.
- Go, `mcp/` and the chart: nothing outside the DS wiring changes.
- The `duro-stack` scope name (t12), and the kept `.duro-session.cache*` glob
  (t13).
- Lifting the raw-element exemptions, or adopting react-strict-dom.
- Capital `Duro` in prose, except in the files the sweep already edits.
- Any UI change. Deprecated `Heading variant="headingSm|Md|Lg"` (layout.tsx:148, home.tsx:223/238, diagram-page.tsx:87) stays; Phase 5's record notes it as a follow-up.

## Code review

- cluster-vision: backend, lang:typescript, react, ui-design (unix and tui left out: the Go command line and its output are untouched, only web/ and its config change; platform left out: the only CI/ops edit is the greengate.conf doctor row's CLI swap, and the workflows and Dockerfile are unchanged)
- homelab: none — a two-line version pin
- decisions: none — markdown only

## Behaviour

- Every route renders as before, at 1280 and 390, on the frozen data. The
  app is dark only (`color-scheme: dark`); the Baseline confirms whether
  `prefers-color-scheme` changes anything.
  - All 22 view routes: `/`, `/topology`, `/dependencies`, `/circle-map`,
    `/network`, `/security`, `/nodes`, `/charts`, `/images`, `/workloads`,
    `/storage`, `/crds`, `/quotas`, `/certificates`, `/network-policies`,
    `/configs`, `/helm-workloads`, `/service-map`, `/namespace-summary`,
    `/rbac`, `/labels` and `/velero`.
  - An unknown path, which renders the error page.
  - One view route (`/topology`) served with `API_URL` at a closed port: the in-layout RouteErrorBoundary Callout (502) with its small Retry button, whose height is recorded.
- The mermaid diagrams (topology, network, the security pie) and the
  `@xyflow` flow diagram (dependencies) keep their node counts. Their SVG and
  canvas sizes stay within 2px.
- The sidebar (SideNav), the data tables' tooltips and the ScrollArea look and
  behave as before, and their accessibility-tree roles and names and the Tab
  order on `/` and `/dependencies` are identical at baseline, c1, c2 and c3.
- The Claude hooks are `greenrack-catalog.sh` and `greenrack-plan.sh`.
  `.claude/settings.json` holds one greenrack entry per event, and
  `aval-heads.sh` is untouched.

## Phases

The lockfile recipe is the same in every phase: `npm install
--package-lock-only --prefer-online` in `web/`, then `npm ci`. The lock stays
lockfileVersion 3, `! grep -qF 'git.daddyshome.fr' web/package-lock.json`
exits 0, and every `resolved` under `node_modules/@duro-app|@greenharness|@aval-adr`
is on `registry.npmjs.org`. The lockfile is never edited with sed. `npm audit
--omit=dev` runs at every commit; a new finding stops the phase and goes to
the person (the v0.25.6 tag was refused on an audit).

**Baseline**, before Phase 1's first commit, in the worktree
`cluster-vision-wt-greenrack` (branch `chore/greenrack`) on origin/main's tree.
Freeze the API: `kubectl port-forward` to the cluster-vision Service, and save
`/api/diagrams`, `/api/config` and `/api/snapshots` to a scratch dir outside
the tree. Serve the production build by the Context recipe. Through
`browser-check` (haiku), take screenshots of the Behaviour routes at 1280 and
390 (23 routes × 2 widths = 46 per build, 184 over the baseline and three commits), and record the mermaid and flow diagrams' node counts and SVG/canvas
sizes; the computed `background`, `border` and `padding` of the sidebar, a
diagram surface and a flow node, plus the hover background and `:focus-visible`
outline of one SideNav item and one sortable table-header button; and every `var(--…)` name referenced in
`web/build/client/assets/*.css` (StyleX `--x*` hashes left out, fallback forms
included) with the count left undefined. Run `eslint --print-config`, saved as
`jq -S '.rules'`, on six files: `web/app/root.tsx`, `web/app/routes/home.tsx`
(no exemption), `web/app/api.server.ts`, `web/app/components/flow-diagram.tsx`
and `web/app/components/mermaid-diagram.tsx` (both exempted), and the test file
`web/test/compare-bar.test.tsx` (`npm run lint` is `eslint app` and never lints
`web/test/`, so print-config is the only gate on the `tests` preset). Record `npm ls @duro-app/eslint-plugin @aval-adr/eslint-config
eslint-plugin-react-hooks`, and the exit codes of `npm run typecheck`, `npm run
lint` and `npm run test:run`, with the lint warning count and the test count. Save `npm audit --omit=dev --json` to the scratch dir; it exits 1 on any finding, so each commit compares advisory IDs against it, not the exit code.

- [ ] Phase 1: DS 5.1 → 5.45, under the old names.
  - Files: `web/package.json` and `web/package-lock.json`. Typecheck or lint
    fallout under `web/app/**` is fixed only where a DS 5.x change requires it,
    and each fix cites the DS change. The 390 runs emulate a touch device
    (`pointer: coarse`, `hover: none`). The 5.45 touch-target size on a coarse
    pointer is expected and not a stop: it is recorded in the PR, as
    customer-vision's was. The flow-node rendered height must still equal
    `NODE_H` (44) with no overlapping node boxes. Any other rendered
    difference stops the phase and goes to the person.
  - ui, eslint-config and eslint-plugin go to ^5.45.0 together, so `npm ls`
    shows one eslint-plugin copy. eslint-config 5.45 pulls
    `@aval-adr/eslint-config` ^0.6.0 as a dependency, so print-config runs at
    this commit too.
  - Commit `chore(deps): Duro 5.45`.
- [ ] Phase 2: DS 5.45 → 6.3.0, under `@duro-app`.
  - Files: `web/package.json`, `web/package-lock.json`,
    `web/eslint.config.js`, `greengate.conf`, and lint fallout under
    `web/app/**`. Each fallout fix names the rule and the preset that brought
    it (`@aval-adr/eslint-config` ^0.17 `base`/`tests` or DS 6.x), and a change
    to rendered output stops the phase and goes to the person, as in Phase 1.
  - ui, eslint-config and eslint-plugin go to ^6.3.0.
  - Add `@aval-adr/eslint-config` as a direct devDependency at ^0.17.0 (inside
    the peer range `>=0.16.0 <1`). `npm ls` shows one copy of it and one copy
    of `eslint-plugin-react-hooks`: two copies fail flat config with `Cannot
    redefine plugin`.
  - `eslint.config.js` takes `react` from `@duro-app/eslint-config`, and
    `base` and `tests` from `@aval-adr/eslint-config`. The raw-element
    exemptions keep their nine allow lists. Any ban that print-config shows
    lost is re-stated by name.
  - `greengate.conf`'s doctor row becomes `npx -y @duro-app/cli@^6 doctor`,
    then `greengate trust`.
  - Commit `chore(deps): Duro 6.3`.
- [ ] Phase 3: the rename to greenrack 7.0.1.
  - (a) Write the allowlist to a `mktemp` file outside the tree, one pathspec
    per line: `docs/plans/**`, `.adr/packs/**`, `.claude/skills/**` and
    `.gitignore`.
    - From decisions' `docs/programmes/greenrack-rename/`, run `./sweep-app.sh
      --check --allow <file> <wt>` first.
    - Then run `./sweep-app.sh --allow <file> --expect npm-name --expect
      css-var --expect eslint-rule <wt> > <scratch>/cv-sweep.txt 2>&1; rc=$?`.
      Pass only the rules that the check counts as non-zero. Never pipe it
      through `tee` or `tail`. Read the `old names left` section of `cv-sweep.txt` before judging the exit code.
    - Exit 1 is expected only when "old names left" lists (b)'s hand-edit
      files, `web/package.json` (pins at major 6) or (d)'s files. Exit 2, 3 or
      4 stops the phase. After an interrupt or an exit 3, run `git -C <wt> checkout -- .` before any rerun: a rerun exits 3 for every rule the first run already applied, and 2 on a dirty tree.
  - (b) Hand edits:
    - Versions go to ^7.0.1. `@aval-adr/eslint-config` stays direct (ADR-0076,
      until greenspotter publishes its config).
    - `greengate.conf`: the row becomes `greenrack-doctor`, running `npx -y
      @greenharness/greenrack-cli@^7 doctor`, then `greengate trust`.
    - `CLAUDE.md`'s "Duro design system" section: the `greenrack` CLI and MCP,
      `greenrack/*` rules, `--gr-*`, `@greenharness/greenrack-ui/table`, and
      greenrack in the section's prose.
    - The comments that name Duro in the files the sweep edits:
      - `app.css`'s header;
      - `eslint.config.js`'s header and its exemption comments;
      - the `holds-until:` notes in `diagram-page.module.css`,
        `circle-map.module.css`, `layout.module.css`, `images.tsx` and
        `flow-diagram.tsx`, which now name `@greenharness/greenrack-tokens`;
      - `circle-map.tsx:119`'s rule id.
  - (c) Run `npm` by the recipe, then `rm -rf web/node_modules/.vite`
    (`optimizeDeps.include` names the package). The production build serves
    `/` with status 200.
  - (d) Run `npx -y @greenharness/greenrack-cli@^7 hook install`, then `skill
    install`. Then count the hook entries per event, for the PR.
    - `git ls-files .claude/hooks` lists only `aval-heads.sh`,
      `greenrack-catalog.sh` and `greenrack-plan.sh`. Delete any `duro-*.sh`
      the installer leaves.
    - `grep -c '@duro-app' .claude/hooks/greenrack-*.sh` is 0, and the hook's stderr refresh hint names `npx -y @greenharness/greenrack-cli@^7 hook install`.
    - If `.claude/skills/` is new, keep it out of prettier, as in
      customer-vision.
    - Run the SessionStart hook once; `git status --porcelain .claude` must be empty. If the new hook writes another cache name, add its glob to `.gitignore`.
  - (e) In the production build, every `var(--…)` name the CSS references is
    defined, the 31 `--gr-*` names and their fallback forms included. The
    undefined count is no higher than the Baseline's (expected 0).
  - (f) Run `greengate agents-md`, then `sweep-app.sh --check --allow <file>
    <wt>`, which must exit 0. `web/vite.config.ts`'s `optimizeDeps.include`
    names `@greenharness/greenrack-ui` and its `/table` subpath, and `npm run
    dev` serves `/` once without a `504 Outdated Optimize Dep`.
  - Commit `feat!: move onto greenrack 7`.
  - Before opening the PR, read the repository's allowed merge methods (`gh repo view --json mergeCommitAllowed,rebaseMergeAllowed`); if only squash is allowed, drop the bisect claim and say so in the PR. The PR merges with a merge commit or a rebase, never a squash, so commits
    1–3 stay bisectable. CI's `image` smoke job must conclude `success` (not
    skipped).
- [ ] Phase 4: release v0.26.0, after the merge.
  - Before tagging, `git log --oneline v0.25.6..origin/main` lists only the
    sweep PR's commits and `chore: bump chart to … [skip ci]` commits. Anything
    else is named in the PR, and the single-change claim is dropped.
  - Tag `v0.26.0` with `tag-release`. Read the Release run's `conclusion`,
    merge the `fredericrous/charts` "update cluster-vision to 0.26.0" PR on
    green Lint and Test Charts, and check what Flux reads: `helm show chart oci://192.168.1.43:5000/ghcr.io/fredericrous/charts/cluster-vision --version 0.26.0` succeeds through the zot mirror (the HelmRepository is `type: oci`, homelab `controllers/fredericrous.yaml`), and `192.168.1.43:5000/ghcr.io/fredericrous/cluster-vision:0.26.0` resolves there too.
  - Then open a homelab PR (`homelab-wt-cv-0.26.0`) that puts chart and image
    at `0.26.0` in `kubernetes/homelab/apps/cluster-vision/helm-release.yaml`.
    Merge it with `merge-when-green`, then wait for homelab main's OCI artifact push and run, in order, `flux reconcile source oci flux-system -n flux-system`, `flux reconcile kustomization cluster-vision -n flux-system` and `flux reconcile helmrelease cluster-vision -n cluster-vision --with-source`. Proceed only when `kubectl get hr cluster-vision -n cluster-vision -o jsonpath='{.spec.chart.spec.version}'` reads 0.26.0.
  - Done when:
    - the HelmRelease is Ready at chart 0.26.0, with no remediation rollback;
    - the pod runs 0.26.0 and is Ready with 0 restarts after 30 minutes;
    - the 10-minute refresh is complete, not partial, and the EAM sync reports
      0 errors;
    - the served JS and CSS, fetched through port-forward, carry no
      `@duro-app` and no `--duro-`;
    - the `--gr-` vars resolve, with 0 undefined `var()`.
  - Rollback: revert the homelab PR with `merge-when-green`, then run the same source, kustomization and helmrelease reconcile chain, back to 0.25.6. Upgrade remediation (retries 3) already rolls a failed upgrade back by itself. v0.26.0 adds no migration: before tagging, `git diff --stat v0.25.6..origin/main -- internal cmd mcp chart` shows nothing but `chart/cluster-vision/Chart.yaml`, and Postgres data written under 0.26.0 is read unchanged by 0.25.6.
- [ ] Phase 5: the programme record, in the decisions worktree
      `decisions-wt-greenrack-t08-clv`, with one PR.
  - `t08-fleet-sweep.md`'s cluster-vision entry becomes "merged, released and
    live". It records the hook count and what differed: npm with legacy peers,
    and the frozen-API browser check.
  - `map.md`'s Handoff names the next app, activity-vision.
  - t08 stays open.

### 🧑 Your steps

- After Phase 4, run `greenbelay open 'https://infra.daddyshome.fr/'`, then
  `/topology` and `/dependencies`. Check that the sidebar, the diagrams and the
  tables look as before, and that the console shows no errors. The agent's
  browser has no Authelia session for the live site.

## Verification

| Check | Expected | Observed |
| --- | --- | --- |
| `npm run typecheck && npm run lint && npm run test:run && npm run build` in `web/` at the baseline and after commits 1–3 | exit 0; lint errors 0, warnings no higher than the baseline; same test count | |
| `npm ls` greenrack-ui, -eslint-config, -eslint-plugin, `@aval-adr/eslint-config`, `eslint-plugin-react-hooks` | one copy each; no `@duro-app` package; no `git.daddyshome.fr`; `npm audit --omit=dev` no new finding | |
| `print-config` `jq -S '.rules'` on the six files at baseline, c1, c2, c3 | the same bans per file (a moved rule id is fine); the nine raw-element allow lists unchanged | |
| `browser-check` (haiku) on the production build over the frozen API, after each commit, against the previous one: 23 routes plus the API-down `/topology` error state at 1280 and 390, diagram node counts and sizes, sidebar/surface/flow-node computed styles, SideNav item and sort-button hover and focus-visible | no measured difference except the 5.45 coarse-pointer change at c1, recorded in the PR; flow-node height equals `NODE_H` (44) under touch emulation, no overlapping node boxes; no new console error; 0 undefined `var()` | |
| `sweep-app.sh --check --allow <file> <wt>` | exit 0 | |
| hook entries in `.claude/settings.json` | one greenrack entry per event, plus `aval-heads` | |
| PR CI: `image` smoke job `conclusion` | success | |
| Phase 4 cluster: HelmRelease; pod after 30 min; refresh and EAM sync; `@duro-app` / `--duro-` count in served assets | Ready at 0.26.0, 0 restarts; complete refresh, 0 EAM errors; both counts 0 | |

## Decision log

<!-- panel: repos=cluster-vision,homelab,decisions adds= reviewers=backend,lang:typescript,platform,react,tui,ui-design,unix,codex body-sha=b3691aabaaf4 -->
