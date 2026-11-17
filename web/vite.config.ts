import { defineConfig } from "vitest/config";
export default defineConfig({
  server: {
    proxy: {
      "/api": { target: "http://127.0.0.1:8765", ws: true },
      "/legacy": "http://127.0.0.1:8765",
      "/sounds": "http://127.0.0.1:8765",
    },
  },
  build: { target: "es2022" },
  test: {
    environment: "jsdom",
    setupFiles: "./src/test-setup.ts",
    include: ["src/**/*.test.{ts,tsx}"],
  },
});
