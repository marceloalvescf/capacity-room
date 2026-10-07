import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e",
  retries: 0,
  workers: 1,
  reporter: [["list"], ["html", { outputFolder: "/results/report", open: "never" }]],
  outputDir: "/results/artifacts",
  use: { baseURL: "http://localhost:8080" },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
