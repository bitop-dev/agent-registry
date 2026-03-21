import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";
import path from "path";

export default defineConfig({
  plugins: [svelte(), tailwindcss()],
  resolve: { alias: { $lib: path.resolve("./src/lib") } },
  build: { outDir: "../cmd/registry-server/web", emptyOutDir: true },
  server: { proxy: { "/v1": "http://localhost:9080", "/artifacts": "http://localhost:9080", "/healthz": "http://localhost:9080" } },
});
