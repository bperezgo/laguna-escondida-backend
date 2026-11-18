# Plan 4: Network Segmentation

## Context

The restaurant has a single flat network: customers and staff devices share the same
WiFi and the same uplink. On busy nights, customer traffic saturates the link and
internal tools (cloud API calls, invoice prints to networked printers, kitchen-display
updates) crawl. We want:

- A **staff VLAN** for the local server, register, KDS, printer, payment terminal,
  managers' phones.
- A **guest VLAN** for customers, isolated from staff devices, rate-limited so
  customers can't starve the staff link.
- A **failover internet** path (4G/LTE) so that even if the primary WAN drops, the
  staff network keeps reaching the cloud sync endpoint.
- Reliable local DNS / mDNS so the staff devices can find the local server
  consistently.

This is mostly a hardware + config plan, not a code plan.

## Goals

1. Two SSIDs, two VLANs, two firewall zones.
2. Guest cannot reach any staff device or the local server. Period.
3. Staff devices have **reserved IPs** (DHCP static leases) so app config can hard-code
   `http://laguna-server.local` and `http://kds-1.local`.
4. Guest WiFi bandwidth capped (and de-prioritized) such that staff traffic always
   has headroom.
5. Single WAN outage does not stop the kitchen.

## Hardware

Three viable hardware lanes. Pick one; do not mix.

| Lane | Devices | Approx cost (USD) | Skill required |
|------|---------|-------------------|----------------|
| **UniFi (recommended)** | UniFi Dream Router or UDM-SE + 1–2 UniFi U6 APs | $300–600 | Low (UniFi UI) |
| **TP-Link Omada** | ER605/ER7206 + EAP620/EAP660 + Omada controller | $200–400 | Low–Medium |
| **MikroTik** | RB5009 + cAP ax | $250–400 | High (RouterOS CLI/Winbox) |

**Recommendation: UniFi Dream Router (UDR)** as a single device that handles routing,
firewall, VLANs, controller, and 4×4 WiFi 6 in one box. Add a wall- or
ceiling-mounted U6+ if WiFi coverage to the kitchen is weak.

Pair with a small **managed switch** (UniFi USW-Lite-8-PoE or USW-Flex-Mini) if more
than one wired drop is needed. The KDS tablet, register, printer, and payment
terminal should be wired where possible — WiFi is the variable to eliminate.

## Logical design

```
        ┌──────────────────────────────────────────────────────────────┐
        │                   UniFi Dream Router                         │
        │                                                              │
        │  WAN1 (fiber/cable)         WAN2 (4G LTE failover modem)     │
        │  ────────────┬────────────────────────┬───────────────────   │
        │              │                        │                      │
        │  ┌───────────▼────────────────────────▼─────────────┐        │
        │  │  Router + Firewall + Controller (one box)        │        │
        │  └───┬──────────────────────────────┬───────────────┘        │
        │      │ VLAN 10 (Staff)              │ VLAN 20 (Guest)        │
        │      │ 10.10.0.0/24                 │ 10.20.0.0/24           │
        │      │ SSID: LE-Staff               │ SSID: LE-Guest         │
        │      │                              │                        │
        └──────┼──────────────────────────────┼────────────────────────┘
               │                              │
        ┌──────▼──────┐                ┌──────▼──────┐
        │ Local svr   │                │  Customer   │
        │ 10.10.0.10  │                │  phones,    │
        │ Register    │                │  laptops    │
        │ 10.10.0.20  │                └─────────────┘
        │ KDS-1       │
        │ 10.10.0.30  │
        │ Printer     │
        │ 10.10.0.40  │
        │ Datafono    │
        │ 10.10.0.50  │
        └─────────────┘
```

## Configuration

### VLANs

| ID | Name | Subnet | DHCP range | Purpose |
|----|------|--------|------------|---------|
| 10 | Staff | 10.10.0.0/24 | 10.10.0.100–200 | Internal devices |
| 20 | Guest | 10.20.0.0/24 | 10.20.0.100–250 | Customers |
| 30 | IoT (optional, future) | 10.30.0.0/24 | 10.30.0.100–200 | Smart plugs, cameras |

### DHCP reservations (Staff)

| Hostname | MAC | IP | Notes |
|----------|-----|----|----|
| laguna-server | (register MAC) | 10.10.0.10 | Backend host |
| register-1 | (register MAC) | 10.10.0.20 | Same machine as server initially |
| kds-cocina | (tablet MAC) | 10.10.0.30 | Kitchen display |
| printer-bar | (printer MAC) | 10.10.0.40 | Networked thermal |
| datafono | (terminal MAC) | 10.10.0.50 | Payment terminal |

(Update with real MACs during install.)

### Firewall rules

In order, top to bottom:

1. **Block Guest → Staff**: `src=VLAN20 dst=VLAN10 action=drop` (also block IoT
   later).
2. **Block Guest → Router admin**: `src=VLAN20 dst=router-mgmt-port action=drop`.
3. **Allow Staff → Internet**: `src=VLAN10 dst=any action=allow`.
4. **Allow Guest → Internet (except RFC1918)**: `src=VLAN20 dst=!RFC1918 action=allow`.
5. **Allow Staff ↔ Staff**: implicit within VLAN.
6. **Allow established/related**: default.

Add **DNS hijack** for the guest VLAN: force `10.20.0.1` (router) as the only DNS
resolver so customers can't bypass content filters if you add them later.

### QoS / traffic shaping

UniFi: enable "Smart Queues" with WAN download/upload speed entered honestly (from a
speedtest). This gives FQ-CoDel + fair queuing automatically across VLANs.

Then add a hard cap on the guest VLAN:

- Download cap: 30–50% of WAN download.
- Upload cap: 30–50% of WAN upload.
- DSCP priority for staff VLAN: AF31 (assured forwarding).

This guarantees staff bandwidth headroom even when 60 customers all stream video.

### WiFi

Two SSIDs, separate radios where possible:

- **LE-Staff** — WPA3, hidden? (debate), strong password, no client-to-client
  isolation (devices must talk to each other on the staff VLAN).
- **LE-Guest** — WPA2/WPA3 personal (simple password posted on a menu card), client
  isolation **ON** (customer phones can't see each other), bandwidth-capped (see QoS).

Optional: a captive-portal landing page on LE-Guest with the restaurant branding.
UniFi supports this natively.

### mDNS / hostname resolution

After VLAN split, mDNS does **not cross VLANs by default**. The KDS tablet on the
staff VLAN reaches `laguna-server.local` only if either:

- Both are on the same VLAN (which they are — staff), so mDNS works.
- Or you enable mDNS reflector / Avahi bridge for cross-VLAN (not needed here).

For desktop-app installs, prefer **static IP + hosts entry** over mDNS:
`10.10.0.10 laguna-server` in `C:\Windows\System32\drivers\etc\hosts` on every staff
device. Robust, no flakiness on tablet wakeups.

## Failover internet

A 4G/LTE USB modem in the router's WAN2 port (Huawei E3372, Quectel EC25, Cradlepoint
small units, or a TP-Link AC1200 4G dongle). UniFi UDR supports WAN failover natively.

Configuration:

- WAN1 = primary (fiber/cable).
- WAN2 = LTE, **standby**. Switches over automatically when WAN1 health check fails
  (router pings 1.1.1.1 every 5 s, fails 3 in a row → failover).
- SIM card: prepaid data plan from Claro/Movistar/Tigo, $15–25/mo for ~50–100 GB.
- Caveats:
  - LTE upload is slower; do not rely on it for streaming high-quality video to
    cloud dashboards during failover. Rely on it for sync only.
  - LTE latency higher: SSE heartbeats and retries will work but slower.

The combination of **plan 1 + LTE failover** means: WAN1 down → service unaffected
(LAN keeps working) + cloud sync stays alive via LTE → catastrophic-only scenario
moves from "WAN drops for 10 min" to "both WANs drop AND server crashes" which is
very rare.

## Power

The router, switch, local server, KDS displays, and printer should all be on a small
UPS (~600–1000 VA online UPS, $100–250 USD). The single point of failure if the
restaurant loses power is the LAN itself; UPS keeps it alive long enough to either
ride out the blip or shut down gracefully.

Wire the credit-card payment terminal into the same UPS line if it's a fixed
counter-top model.

## Install sequence

1. **Buy hardware.** UDR + a small managed switch + 1 spare AP + LTE modem + UPS.
2. **Pre-stage off-hours.** Plug everything in at home or a quiet morning. Configure
   VLANs, SSIDs, firewall rules, DHCP reservations using current MAC addresses.
3. **Cut over during the slowest window** (mid-morning Tuesday). Move the modem
   uplink, swap routers, reboot devices one by one.
4. **Verify staff devices** can ping each other and the local server, **cannot** ping
   from guest VLAN to staff, and bandwidth caps hold under load.
5. **Failover test**: pull WAN1, confirm WAN2 takes over within ~10 s, services keep
   working.
6. **Document IPs and credentials** in a sealed envelope on-site and in 1Password.

## Risks and mitigations

| Risk | Mitigation |
|------|------------|
| One device hardcodes `192.168.1.x` from old network | Hosts file or DHCP reservation; test all devices post-cutover |
| LTE SIM throttled or out of data | Monitor usage; set alerts at 80% |
| Customers complain about guest WiFi feeling slow | This is by design; size the cap above "good enough for normal browsing" but below "starves staff" |
| Staff VLAN compromised via weak WiFi password | WPA3 + strong password; rotate quarterly; consider WPA2-Enterprise with RADIUS later |
| mDNS / hostnames flaky on tablets after sleep | Use static IPs in hosts file, not mDNS |
| Inter-VLAN bug accidentally lets guest reach staff | Test with a real phone on guest VLAN trying to reach `10.10.0.10`; should fail |

## Open questions for the next session

- Current WAN provider, download/upload speed, and latency baseline?
- Floor plan + AP placement: kitchen often has poor coverage behind appliances.
- Are there existing devices on the network that cannot move VLANs (e.g., a smart
  thermostat) and need a third VLAN now rather than later?
- Does the restaurant accept a 15-minute downtime window for the cutover?
