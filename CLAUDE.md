# Cluster Vision

Go backend (`cmd/`, `internal/`, `mcp/`) serving Kubernetes-derived diagrams over
`/api/diagrams`, plus a React Router 7 SSR frontend in `web/`.

## Duro design system (@duro-app/ui v1)

Machine-queryable docs — run once per session:
`npx @duro-app/cli manifest --json`
Then: `npx @duro-app/cli <Component>` (props+usage) ·
`npx @duro-app/cli <recipe> --source-only` · `npx @duro-app/cli spacing|icons|rules` ·
free text (e.g. `npx @duro-app/cli "tags that wrap"`) searches usage metadata.
MCP server: `claude mcp add duro -- npx -y -p @duro-app/cli -p @modelcontextprotocol/sdk duro mcp`

Lint: `@duro-app/eslint-plugin` (`duro.configs.recommended`) enforces the
critical rules (html.* elements, deep token imports, no deprecated Table
parts; warns on raw px/hex with token equivalents). Run `npm run lint` in `web/`.

v1 notes: Icon/StatusIcon `size` is a token (`sm|md|lg|xl|xxl` = 16/18/24/36/48px);
Dialog/Drawer/DetailPanel `closeAnimationDuration` is a motion token
(`instant|fast|base|slow`); `Table.HeaderCell` no longer takes `isActions`;
`Table.Root` no longer takes `columns` (it derives its grid from the header row,
and column widths come from `Table.HeaderCell`'s `width`/`compactWidth`).

### How this app consumes Duro — read before writing UI code

**This app does NOT use `react-strict-dom` directly, and cannot today.**
Importing it in app code throws at module scope:

```
Unexpected 'stylex.create' call at runtime. Styles must be compiled by
'@stylexjs/babel-plugin'.
```

RSD's own dist declares its default element styles with `stylex.create` and
expects the consuming app's Babel to compile them. `web/` has no StyleX/Babel
pipeline — it consumes `@duro-app/ui`'s **precompiled** bundle, which contains
zero react-strict-dom references and works as-is.

The same reason rules out `css.create()` in app code: StyleX hashes token
variable names against the compiling package's rootDir, so tokens compiled here
would produce `var(--bgCard-<other-hash>)` names that
`@duro-app/ui/dist/index.css` never defines. Do not add
`@duro-app/tokens/tokens/*.css` deep imports for that purpose.

So, in `web/app`:

1. **Reach for a Duro component first.** `Stack`/`Inline`/`Cluster`/`Grid` for
   layout, `Heading`/`Text` for typography, `Badge`/`Callout`/`Card`/`Table` for
   the rest. This is what the lint rule is steering you toward here.
2. **Need custom CSS?** Use a CSS module and style it with the published
   `--duro-*` custom properties (`--duro-color-bg-card`, `--duro-spacing-md`,
   `--duro-radius-sm`, …). They ship with `@duro-app/ui`'s stylesheet and follow
   the active `ThemeProvider` theme. `app/app.css` aliases this app's legacy
   `--bg-secondary`/`--border`/… names onto them, so older CSS modules stay on
   the Duro palette too.
   Never hand-write StyleX variable names (`var(--bgCard-xj2l5r)`) — the hash is
   a build artefact.
3. **Raw elements are allow-listed per file** in `web/eslint.config.js`, only
   for: the react-router document shell, sized/surface containers that need real
   CSS, DOM that `@xyflow/react` or mermaid owns, and elements Duro has no
   component for (`<pre>`, `<details>`). Adding a raw element anywhere else is a
   lint error — convert it to a Duro component instead of widening the list.

To lift those exemptions and use `html.*`, wire `react-strict-dom/babel-preset`
plus its PostCSS plugin into `web/vite.config.ts` first.

### Tables

`app/components/data-table.tsx` wraps TanStack + `Table` from
`@duro-app/ui/table` (the TanStack-aware subpath). Per-column knobs go through
`columnDef.meta`:

- `meta.width` — a `grid-template-columns` track, e.g. `'minmax(200px, 400px)'`
- `meta.truncate` — clip the cell to one line with an ellipsis

Column headers are sortable via a link-style `Button` (Duro's `Table.HeaderCell`
has no `onClick`), which also makes sorting keyboard-operable.

`app/components/markdown-table.tsx` renders API-provided markdown tables through
the same Duro `Table` primitives.

## Frontend commands (run in `web/`)

| Command             | What it does                          |
| ------------------- | ------------------------------------- |
| `npm run dev`       | React Router dev server               |
| `npm run build`     | Production SSR build                  |
| `npm run typecheck` | `react-router typegen && tsc`         |
| `npm run test:run`  | Vitest, single run                    |
| `npm run lint`      | ESLint with the Duro rules            |

The dev server expects the Go API at `http://localhost:8080` (override with
`API_URL`).

## Known pre-existing issue

`@jalez/react-flow-smart-edge` is CJS-shaped but published as `"type":
"module"`, so it throws `ReferenceError: module is not defined` during SSR of
`app/components/flow-diagram.tsx`. The component is lazy-loaded, so flow routes
still render and hydrate on the client; the error is logged server-side only.
