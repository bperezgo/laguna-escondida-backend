# Tablet & LAN provisioning runbook (Android)

Do these once per box and once per tablet. Order matters: gateway first, then each tablet.

## A. Gateway (once)

En el gateway (p. ej. UniFi): IP fija para la caja y registro DNS local `pos.kuenta.lan -> <ip>`.
The box itself never answers DNS.

1. **Fix the edge box's IP** by its MAC (UniFi: Client → Fixed IP, e.g. `192.168.10.10`).
2. **Add the local DNS record:** `pos.kuenta.lan` → the box IP.
3. **DHCP-reserve a fixed IP for each tablet** (needed for the firewall allowlist).
4. **Isolate the POS devices:** put them on a dedicated **VLAN** if the router supports it,
   otherwise a dedicated **POS SSID** separate from guest Wi-Fi.

> If the gateway can't hold a local DNS record, tablets open `https://<box IP>` instead; the
> certificate covers the IP too.

## B. Edge box (once)

1. Deploy this repo to `C:\kuentapos`.
2. `scripts\fetch-artifacts.ps1`
3. `scripts\install.ps1 -AllowedTabletIPs '<tablet IPs>'`  (elevated)
4. Export the Caddy root cert — see `../certs/README.md`.

## C. Each Android tablet

1. **Install the Caddy root cert** (`../certs/README.md`). Without this, HTTPS warns and login
   silently fails (the `Secure` cookie won't be sent).
2. **DNS:** ensure the tablet uses the **gateway** as its DNS (the DHCP default).
3. **Turn OFF Private DNS / DNS-over-HTTPS:** Settings → Network & internet → Private DNS → **Off**.
   Otherwise the tablet bypasses the gateway and can't resolve `pos.kuenta.lan`.
4. **Open** `https://pos.kuenta.lan` and confirm: no cert warning, login works.
5. **Kiosk-lock** the browser to `https://pos.kuenta.lan` (kiosk app / managed device / screen pin).

## Quick verification

- [ ] `nslookup pos.kuenta.lan` on the tablet's network resolves to the box IP.
- [ ] `https://pos.kuenta.lan` loads with a valid (trusted) cert — no warning.
- [ ] Login succeeds and stays logged in (cookie round-trips → TLS + trust are correct).
- [ ] A non-allowlisted device on the LAN is refused (firewall / Caddy `remote_ip`).
- [ ] Kitchen SSE updates stream live (no buffering delay).
