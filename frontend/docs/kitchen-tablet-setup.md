# Kitchen tablet setup: cloud board alerts

Takeaway and delivery orders only reach the **cloud** kitchen board. The kitchen tablet keeps the edge board in front and the cloud board in a second tab, which alerts when a paid takeaway or delivery order arrives.

1. **Second tab.** Keep the edge board (`http://<box>/kitchen`) in tab 1. Open the cloud kitchen board (`https://<cloud>/kitchen`) in tab 2.
2. **Sign in** to the cloud in tab 2. Each tab has its own session.
3. **Tap "Activar alertas"** on the cloud board. This turns on the alert sound. Tap it again after every reload, because the browser blocks sound until someone taps the page.
4. **Allow notifications** when the browser asks. If you see "Notificaciones bloqueadas", turn them back on in the site settings for the cloud address. The sound and the tab count still work without them.
5. **Screen timeout: never.** Set it in Android's display settings. When the screen is off, the browser runs nothing and no alerts arrive.
6. **Keep it charging** for the whole service.

While the cloud tab is in the background, each new order adds `(n)` to its title and a badge to its icon, plays the sound and shows a notification ("Domicilio" / "Para llevar" plus the order). Tap the notification or the tab to open the board; the count goes back to zero.

**Rollback:** redeploy the previous frontend, then in the browser's site settings for the cloud address, unregister the `kitchen-sw.js` service worker. It keeps no cache, so there is nothing else to clean up.
