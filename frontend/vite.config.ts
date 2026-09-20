/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // The API is reached through the same origin in production, where the
    // reverse proxy routes /api. Proxying it in development keeps that true, so
    // no CORS rule exists in one place and not the other.
    proxy: {
      "/api": { target: process.env.API_ORIGIN ?? "http://localhost:8080", changeOrigin: true },
      "/healthz": { target: process.env.API_ORIGIN ?? "http://localhost:8080", changeOrigin: true },
    },
  },
  test: {
    environment: "jsdom",
    alias: {
      // WebGL is not available in jsdom, and the tests are about what the page
      // says, not about what the canvas draws.
      "maplibre-gl": new URL("./src/test/stubs/maplibre.ts", import.meta.url).pathname,
    },
    setupFiles: ["./src/test/setup.ts"],
    globals: true,
    css: true,
  },
});
