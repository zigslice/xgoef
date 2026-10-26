// Vite writes the production page into web/. xgoef serves that directory
// as xgoef://app/. A relative base keeps the built asset URLs on that origin.
import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig({
  base: "./",
  plugins: [svelte()],
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
  },
  build: {
    outDir: "web",
    emptyOutDir: true,
  },
});
