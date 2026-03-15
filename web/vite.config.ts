import { reactRouter } from "@react-router/dev/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import tsconfigPaths from "vite-tsconfig-paths";

export default defineConfig({
  plugins: [
    react({ babel: { plugins: ["babel-plugin-react-compiler"] } }),
    reactRouter(),
    tsconfigPaths(),
  ],
  css: {
    modules: {
      localsConvention: "camelCase",
    },
  },
});
