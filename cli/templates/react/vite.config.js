// Vite writes the production page into web/. xgoef serves that directory
// as xgoef://app/. A relative base keeps the built asset URLs on that origin.
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: "./",
  plugins: [react()],
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
