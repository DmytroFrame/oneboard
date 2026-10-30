import { defineConfig, minimal2023Preset } from "@vite-pwa/assets-generator/config";

// PWA icons: `npx pwa-assets-generator` (run twice by `npm run icons`).
// logo.svg (rounded) → transparent icons and favicon; logo-maskable.svg (full bleed) → Android
// maskable and iOS home-screen icons, which the OS crops to its own shape.
const maskableOnly = process.env.ICONS === "maskable";

export default defineConfig({
  preset: maskableOnly
    ? {
        transparent: { sizes: [], favicons: [] },
        maskable: { ...minimal2023Preset.maskable, padding: 0 },
        apple: { ...minimal2023Preset.apple, padding: 0 },
      }
    : { ...minimal2023Preset, maskable: { sizes: [] }, apple: { sizes: [] } },
  images: [maskableOnly ? "public/logo-maskable.svg" : "public/logo.svg"],
});
