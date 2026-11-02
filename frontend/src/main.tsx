// Global styles first, so component modules (imported through App) come after them in the cascade.
import "@/styles/global.scss";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "@/App";
import { storedSettings } from "@/context/preferences";
import { initI18n } from "@/i18n";
import { initPWA } from "@/lib/pwa";
import { initTouchDragAndDrop } from "@/lib/touch";

// Until the session is restored, use the language this browser used last time.
initI18n(storedSettings().language);
initPWA();
initTouchDragAndDrop();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
