## Why

Restaurants with several devices open the POS by a local name instead of an IP. The old manual setup served `pos.laguna.lan`. The new installer always writes `EDGE_LAN_HOST=` (blank), so a fresh box answers only on `https://<ip>`. The installer also picks the IP itself, from the default route, with no way to confirm or change it. Where the gateway (UniFi) pins the box's IP and holds the DNS record, Caddy has to serve exactly that IP and that name. The owner's own restaurant needs a reinstall that works this way.

## What Changes

- A fresh install serves the fixed local name **`pos.kuenta.lan`** alongside the box's IP, and the internal-CA certificate covers both.
- The wizard gets an optional **advanced network step**, shown only when the owner ticks "Configurar la red manualmente" on the welcome page. The step shows:
  - the box's IP, as a list of the IPv4 addresses this PC holds right now, with the detected one preselected
  - the allowed network (CIDR), derived from the chosen address and editable
  - the local name, prefilled with `pos.kuenta.lan` and editable
- The values are checked before enrollment, so a bad value never uses up the single-use code. The checks: the PC holds the IP, the IP is inside the network, and the name is a valid DNS name.
- The silent install accepts the same three values from `KUENTA_SETUP_LAN_IP`, `KUENTA_SETUP_LAN_CIDR` and `KUENTA_SETUP_LAN_HOST`.
- The finish page names the address to open, and the DNS record the gateway must hold: `pos.kuenta.lan -> <ip>`.
- `set-lan-ip.ps1` also rebuilds `EDGE_SITE_ADDR`. Today it leaves Caddy on the old address after an IP change.
- **BREAKING (docs/dev tooling):** the Technitium DNS fallback is dropped. `scripts/setup-technitium-dns.ps1`, `scripts/dev-test-here.ps1` and `docs/DNS_SETUP_GUIDE.md` / `docs/DNS_RESTORE_GUIDE.md` are deleted. The remaining docs move from `pos.laguna.lan` to `pos.kuenta.lan` and say that the gateway, not the box, answers DNS.

## Non-goals

- The box does not serve DNS or mDNS. The gateway holds the record.
- No change to the name or IP of an already installed box through the installer. A repair keeps `env\box.env` as it is.
- No per-restaurant names (`pos.<slug>.lan`).

## Capabilities

### New Capabilities
- `edge-lan-identity`: the local name and address a box serves the POS on, how they reach Caddy and its certificate, and how they are changed on an installed box.

### Modified Capabilities
- `edge-installer`: "The installer writes the box's configuration" gains the default name and the advanced network step. "The installer can run unattended" gains the network variables. "The wizard finishes when the box is usable" names the address and the gateway record.

## Impact

- Touches:
  - `edge/installer/kuenta.iss`
  - `edge/installer/exit-codes.json` and `.inc` (new code 37)
  - `edge/installer/test/run-installer-tests.ps1`
  - `edge/scripts/write-box-config.ps1`
  - new `edge/scripts/lan-config.ps1`
  - `edge/scripts/set-lan-ip.ps1`
  - `edge/scripts/verify-install.ps1` (only if needed for the name check)
  - `edge/env/box.env.example`, `edge/Caddyfile` (comment)
  - deleted: `edge/scripts/setup-technitium-dns.ps1`, `edge/scripts/dev-test-here.ps1`, `edge/docs/DNS_SETUP_GUIDE.md`, `edge/docs/DNS_RESTORE_GUIDE.md`
  - docs: `edge/README.md`, `edge/provisioning/README.md`, `edge/certs/README.md`, `edge/docs/LAN_SERVING_WALKTHROUGH.md`
  - CI: `.github/workflows/edge-ci.yml` (if the installer test needs new env)
- No backend, frontend or API change.
- Depends on: none.
- Trust boundary: the LAN exposure of the box. The CIDR feeds the Windows Firewall rule and Caddy's `@notlan` guard, and the owner can now widen it. The input comes from an elevated admin running the installer. No change to auth, tenancy, sync, secrets or any cloud endpoint.
