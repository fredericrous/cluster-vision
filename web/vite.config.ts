import babel from "@rolldown/plugin-babel";
import { reactRouter } from "@react-router/dev/vite";
import { reactCompilerPreset } from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import tsconfigPaths from "vite-tsconfig-paths";

export default defineConfig({
  plugins: [
    // No react() from @vitejs/plugin-react: reactRouter() already injects
    // React Refresh in dev, and a second copy made every page throw
    // "Identifier 'RefreshRuntime' has already been declared", so the dev
    // server never hydrated. Only the compiler preset is taken from it:
    // @vitejs/plugin-react v6 dropped the inline `babel` option in favor of
    // running babel as a separate rolldown plugin, and reactCompilerPreset
    // comes pre-wired with the right include filter for the React Compiler.
    babel({ presets: [reactCompilerPreset()] }),
    reactRouter(),
    tsconfigPaths(),
  ],
  optimizeDeps: {
    // Dependencies Vite otherwise only discovers when a page first imports
    // them (the design system, and the lazily loaded diagram renderers). Each
    // late discovery re-optimizes and reloads the page mid-session, and a
    // request caught in between fails with "504 Outdated Optimize Dep".
    include: [
      "@duro-app/ui",
      "@duro-app/ui/table",
      "@base-ui/react/separator",
      "@tanstack/react-table",
      "mermaid",
      "d3-hierarchy",
      "@xyflow/react",
      "@dagrejs/dagre",
      "@jalez/react-flow-smart-edge",
    ],
  },
  ssr: {
    // Published as "type": "module" with a CommonJS `main`, so Node's own
    // loader throws "module is not defined" when the server renders a flow
    // diagram (every /dependencies-style route answered 500 and fell back to
    // client rendering). Bundling it lets Vite resolve its real ESM build
    // through the `module` field instead. That build imports named exports
    // from `pathfinding`, which is CommonJS and whose names Node cannot see,
    // so it is bundled too and, in dev, pre-bundled to ESM.
    noExternal: ["@jalez/react-flow-smart-edge", "pathfinding"],
    optimizeDeps: {
      include: ["pathfinding"],
    },
  },
  css: {
    modules: {
      localsConvention: "camelCase",
    },
  },
});
