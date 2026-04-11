import babel from "@rolldown/plugin-babel";
import { reactRouter } from "@react-router/dev/vite";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import tsconfigPaths from "vite-tsconfig-paths";

export default defineConfig({
  plugins: [
    react(),
    // @vitejs/plugin-react v6 dropped the inline `babel` option in favor of
    // running babel as a separate rolldown plugin. reactCompilerPreset comes
    // pre-wired with the right include filter for the React Compiler.
    babel({ presets: [reactCompilerPreset()] }),
    reactRouter(),
    tsconfigPaths(),
  ],
  css: {
    modules: {
      localsConvention: "camelCase",
    },
  },
});
