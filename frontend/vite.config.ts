import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { VitePWA } from "vite-plugin-pwa";

export default defineConfig({
  plugins: [
    react(),
    // Installable PWA: manifest + a Workbox service worker that precaches the app shell, so OneBoard
    // opens instantly and offline. The API and the real-time socket are never cached.
    VitePWA({
      registerType: "autoUpdate",
      includeAssets: ["favicon.ico", "logo.svg", "apple-touch-icon-180x180.png"],
      manifest: {
        id: "/",
        name: "OneBoard",
        short_name: "OneBoard",
        description: "Real-time kanban boards for teams",
        lang: "en",
        start_url: "/home",
        scope: "/",
        display: "standalone",
        orientation: "any",
        theme_color: "#0a84ff",
        background_color: "#0a0c14",
        categories: ["productivity", "business"],
        icons: [
          { src: "pwa-64x64.png", sizes: "64x64", type: "image/png" },
          { src: "pwa-192x192.png", sizes: "192x192", type: "image/png" },
          { src: "pwa-512x512.png", sizes: "512x512", type: "image/png", purpose: "any" },
          { src: "maskable-icon-512x512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
        ],
        shortcuts: [
          { name: "Home", url: "/home", icons: [{ src: "pwa-192x192.png", sizes: "192x192" }] },
          { name: "Settings", url: "/settings", icons: [{ src: "pwa-192x192.png", sizes: "192x192" }] },
        ],
      },
      workbox: {
        globPatterns: ["**/*.{js,css,html,svg,png,ico,woff2}"],
        // The lazily loaded card editor chunk is ~670 kB; precache it too so tasks open offline.
        maximumFileSizeToCacheInBytes: 3 * 1024 * 1024,
        navigateFallback: "/index.html",
        navigateFallbackDenylist: [/^\/api\//, /^\/healthz$/, /^\/readyz$/],
        cleanupOutdatedCaches: true,
      },
    }),
  ],
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
