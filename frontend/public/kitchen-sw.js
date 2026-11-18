// Kitchen alerts only: no fetch handler and no cache, so it never changes how the app loads.
// Registered with scope /kitchen, so the clients it sees are kitchen board tabs.

self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => event.waitUntil(self.clients.claim()));

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
      const board = windows.find((client) => new URL(client.url).pathname.startsWith("/kitchen"));
      if (board) return board.focus();
      return self.clients.openWindow("/kitchen");
    })()
  );
});
