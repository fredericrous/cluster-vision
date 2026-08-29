import duro from "@duro-app/eslint-plugin";
import tsParser from "@typescript-eslint/parser";

const languageOptions = {
  parser: tsParser,
  parserOptions: {
    ecmaVersion: "latest",
    sourceType: "module",
    ecmaFeatures: { jsx: true },
  },
};

/**
 * `duro/no-raw-html-element` assumes an app that has adopted react-strict-dom
 * end to end. This one has not, and cannot today: importing `react-strict-dom`
 * in app code throws at module scope —
 *
 *   Unexpected 'stylex.create' call at runtime. Styles must be compiled by
 *   '@stylexjs/babel-plugin'.
 *
 * — because RSD's own dist declares its default element styles with
 * `stylex.create` and expects the consumer's Babel to compile them. This app
 * has no StyleX/Babel pipeline; it consumes @duro-app/ui's *precompiled* bundle
 * (which contains zero react-strict-dom references) and styles its own
 * containers with CSS modules over the published `--duro-*` custom properties.
 * Adopting `css.create()` here would not work either: token variable names are
 * hashed against the compiling package's rootDir, so app-compiled tokens would
 * never match the names in `@duro-app/ui/dist/index.css`.
 *
 * So the migration target here is the Duro component set, not `html.*`.
 * Everything that maps onto a Duro component has been migrated. What stays raw
 * is scoped per file below: document shell, sized/surface containers that need
 * real CSS, DOM a third-party library owns, and the few elements Duro has no
 * component for. The rule still gates every other file, which is where new
 * markup gets written.
 *
 * To lift these exemptions, wire `react-strict-dom/babel-preset` (plus its
 * PostCSS plugin) into vite.config.ts first.
 */
const rawElementExceptions = [
  {
    // react-router renders the document itself. <main>/<pre> have no Duro
    // equivalent (Text variant="code" is monospace but not whitespace-preserving).
    files: ["app/root.tsx"],
    allow: ["html", "head", "body", "meta", "main", "pre"],
  },
  {
    // App chrome: sticky 220px rail with media queries (CSS module), plus the
    // mailto anchor in the licence footer -- Duro has no anchor component, same
    // reason compare-bar.tsx is exempted below.
    files: ["app/routes/layout.tsx"],
    allow: ["div", "main", "a"],
  },
  {
    // Sized/surface containers: `height: calc(100vh - 4rem)` flow page and the
    // bordered diagram surface.
    files: ["app/components/diagram-page.tsx"],
    allow: ["div"],
  },
  {
    // Mermaid renders an SVG string into a ref'd node via innerHTML; <details>
    // is a native disclosure and <pre> a whitespace-preserving block, neither
    // of which Duro covers.
    files: ["app/components/mermaid-diagram.tsx"],
    allow: ["div", "details", "summary", "pre"],
  },
  {
    // Same innerHTML-hosted mermaid SVG, inline variant, plus the raw-source
    // <pre> fallback when the content is not a markdown table.
    files: ["app/components/markdown-table.tsx"],
    allow: ["div", "pre"],
  },
  {
    // @xyflow/react owns this DOM: custom node/group renderers and the canvas
    // wrapper are targeted by `.react-flow__*` selectors and measured by the
    // library, so they must stay class-styled elements.
    files: [
      "app/components/flow-diagram.tsx",
      "app/components/flow-group.tsx",
      "app/components/flow-node.tsx",
    ],
    allow: ["div", "span"],
  },
  {
    // Sized host for the d3-hierarchy SVG scene.
    files: ["app/routes/circle-map.tsx"],
    allow: ["div"],
  },
  {
    // Compare-mode layout: diagram + changes panel grid, and the
    // <aside> landmark for the panel.
    files: ["app/components/diagram-page.tsx"],
    allow: ["div", "aside"],
  },
  {
    // External compare links (forge "compare A...B" pages) — Duro has no
    // anchor component.
    files: ["app/components/compare-bar.tsx"],
    allow: ["a"],
  },
];

export default [
  { ignores: ["build/**", "node_modules/**", ".react-router/**"] },
  {
    ...duro.configs.recommended,
    files: ["app/**/*.{ts,tsx}"],
    languageOptions,
  },
  ...rawElementExceptions.map(({ files, allow }) => ({
    files,
    languageOptions,
    rules: { "duro/no-raw-html-element": ["error", { allow }] },
  })),
];
