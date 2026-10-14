import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { configDefaults, defineConfig } from "vitest/config";

// Vite reads its environment when the config is loaded, so the version is set
// before that happens.
process.env["VITE_CAPACITYROOM_VERSION"] = readFileSync(
  fileURLToPath(new URL("../VERSION", import.meta.url)),
  "utf8",
).trim();

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: [
      { find: "@", replacement: fileURLToPath(new URL("./src", import.meta.url)) },
      // `react-remove-scroll`, which every Radix modal layer uses, imports
      // `react-remove-scroll-bar` under both specifiers. The replacement keeps
      // the body-attribute scroll lock and drops the `<style>` injection that a
      // `style-src 'self'` policy refuses.
      {
        find: /^react-remove-scroll-bar(\/constants)?$/,
        replacement: fileURLToPath(new URL("./src/lib/remove-scroll-bar.ts", import.meta.url)),
      },
    ],
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    assetsInlineLimit: 0,
    chunkSizeWarningLimit: 1000,
  },
  server: {
    proxy: {
      "/api": "http://localhost:8080",
      "/health": "http://localhost:8080",
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./tests/setup.ts"],
    exclude: [...configDefaults.exclude, "e2e/**"],
    // Vitest otherwise loads the CommonJS build of react-remove-scroll, whose
    // require of react-remove-scroll-bar no alias reaches. The ES module build,
    // loaded through the transform, takes the aliases above.
    alias: {
      "react-remove-scroll": fileURLToPath(
        new URL("./node_modules/react-remove-scroll/dist/es2015/index.js", import.meta.url),
      ),
    },
    server: { deps: { inline: [/radix-ui/, /react-remove-scroll/] } },
  },
});
