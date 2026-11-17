import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  workers: 1,
  timeout: 30000,
  outputDir: "../build/e2e/results",
  use: {
    baseURL: process.env.WEB_URL ?? "http://127.0.0.1:8765",
    browserName: "chromium",
    viewport: { width: 1440, height: 1000 },
    trace: "off",
  },
  reporter: [["list"], ["json", { outputFile: "../build/e2e/report.json" }]],
});
