import type { NewOrderAlert } from "./newOrderTracker";

const SOUND_SRC = "/sounds/kitchen-alert.wav";
const SW_SRC = "/kitchen-sw.js";
const SW_SCOPE = "/kitchen";

let audioContext: AudioContext | null = null;
let alertBuffer: AudioBuffer | null = null;

export type NotificationState = NotificationPermission | "unsupported";

export function notificationState(): NotificationState {
  return typeof Notification === "undefined" ? "unsupported" : Notification.permission;
}

export function isAudioUnlocked(): boolean {
  return audioContext?.state === "running" && alertBuffer !== null;
}

function play(volume: number): void {
  if (!audioContext || !alertBuffer) return;
  const source = audioContext.createBufferSource();
  const gain = audioContext.createGain();
  gain.gain.value = volume;
  source.buffer = alertBuffer;
  source.connect(gain).connect(audioContext.destination);
  source.start();
}

/** Must run inside a user gesture: browsers block page audio until then, per page load. */
export async function unlockAudio(): Promise<void> {
  audioContext ??= new AudioContext();
  // resume() first, while still inside the gesture; the fetch below would lose it.
  const resumed = audioContext.resume();
  if (!alertBuffer) {
    const response = await fetch(SOUND_SRC);
    alertBuffer = await audioContext.decodeAudioData(await response.arrayBuffer());
  }
  await resumed;
  play(0);
}

export function playAlertSound(): void {
  if (isAudioUnlocked()) play(1);
}

export async function requestNotificationPermission(): Promise<NotificationState> {
  if (notificationState() !== "default") return notificationState();
  return Notification.requestPermission();
}

export function registerKitchenServiceWorker(): void {
  if (!("serviceWorker" in navigator)) return;
  navigator.serviceWorker
    .register(SW_SRC, { scope: SW_SCOPE })
    .catch((error) => console.error("Error registering kitchen service worker:", error));
}

const ORDER_TYPE_LABEL: Record<NewOrderAlert["order_type"], string> = {
  delivery: "Domicilio",
  takeaway: "Para llevar",
};

// Through the service worker: Android Chromium throws on `new Notification()`.
export async function showOrderNotification(alert: NewOrderAlert): Promise<void> {
  if (notificationState() !== "granted" || !("serviceWorker" in navigator)) return;
  try {
    const registration = await navigator.serviceWorker.getRegistration(SW_SCOPE);
    await registration?.showNotification(
      `${ORDER_TYPE_LABEL[alert.order_type]} · ${alert.temporal_identifier}`,
      { body: "Nuevo pedido pagado en cocina", tag: alert.open_bill_id }
    );
  } catch (error) {
    console.error("Error showing kitchen notification:", error);
  }
}
