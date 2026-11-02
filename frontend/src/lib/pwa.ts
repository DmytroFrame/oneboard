import { useSyncExternalStore } from "react";
import { registerSW } from "virtual:pwa-register";

/** The non-standard event Chromium fires when the app can be installed. */
interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
}

let deferredPrompt: BeforeInstallPromptEvent | null = null;
const listeners = new Set<() => void>();
const notify = () => listeners.forEach((l) => l());

const standalone = () =>
  matchMedia("(display-mode: standalone)").matches || (navigator as { standalone?: boolean }).standalone === true;

let snapshot = { canInstall: false, installed: standalone() };
const update = () => {
  snapshot = { canInstall: deferredPrompt !== null, installed: standalone() };
  notify();
};

/** Registers the service worker (updates apply on the next load) and listens for install prompts. */
export function initPWA() {
  registerSW({ immediate: true });
  window.addEventListener("beforeinstallprompt", (e) => {
    e.preventDefault(); // keep it for our own "Install" button instead of the browser's mini-infobar
    deferredPrompt = e as BeforeInstallPromptEvent;
    update();
  });
  window.addEventListener("appinstalled", () => {
    deferredPrompt = null;
    update();
  });
  matchMedia("(display-mode: standalone)").addEventListener("change", update);
}

export async function promptInstall() {
  if (!deferredPrompt) return;
  await deferredPrompt.prompt();
  await deferredPrompt.userChoice;
  deferredPrompt = null;
  update();
}

/** iOS Safari has no install prompt: people use Share → Add to Home Screen. */
export const isIOS = /iP(hone|ad|od)/.test(navigator.userAgent) || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);

export function useInstallState() {
  return useSyncExternalStore(
    (l) => (listeners.add(l), () => listeners.delete(l)),
    () => snapshot,
  );
}
