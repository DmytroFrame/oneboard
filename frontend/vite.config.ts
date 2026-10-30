import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  // The lazily loaded card modal (editor + CRDT) is ~670 kB on its own; it is fetched on first use.
  build: { chunkSizeWarningLimit: 800 },
  server: {
    port: 5173,
    proxy: {
      // ws: true also proxies the real-time WebSocket at /api/ws
      "/api": { target: "http://localhost:8080", ws: true },
    },
  },
});
