# Deployment runbook — new Windows box

How to put the edge appliance on a **Windows 10 or 11 PC** and have it serving the POS. The normal
way is the installer, `KuentaSetup-<version>.exe`: no terminal, no files to edit, no Go or pnpm.
The script path further down is the developer fallback and what CI's pieces are made of.

> The install root is **`C:\kuentapos`** and the installer does not ask for another: the `Caddyfile`
> hard-codes it. Don't move the folder after installing.
> Before copying a file, the installer makes Administrators its owner and lets only Administrators
> and SYSTEM write under it (`scripts\lock-root.ps1`). Don't loosen its permissions: SYSTEM runs
> what is in there. A box at the old `C:\laguna-edge` has to be uninstalled first.

---

## 1. What gets installed

Five things run as Windows services, started in this order. Only Caddy is reachable from the
network; everything else binds `127.0.0.1`.

```
tablets ──HTTPS:443──> Caddy ──:3000──> Next.js ──:8080──> edge-node (Go) ──:5432──> Postgres
                       (the only LAN-exposed service)
```

| Service id        | Listens on        | Comes from                              |
|-------------------|-------------------|-----------------------------------------|
| `kuentapos-postgres` | 127.0.0.1:5432    | the installer (pinned Postgres)         |
| `kuentapos-edge-node`| 127.0.0.1:8080    | the installer (the release's edge-node) |
| `kuentapos-next`     | 127.0.0.1:3000    | the installer (the release's POS)       |
| `kuentapos-caddy`    | 0.0.0.0:443, :80  | the installer (pinned Caddy)            |

---

---

## 2. Install with the installer (the normal way)

You need: the PC (64-bit Windows 10 or 11, 3 GB free on `C:`, internet) and an **installation code**:
an administrator of the restaurant gets it in the cloud app under **Instalar caja**. It is valid for
30 minutes and works once; getting a new one cancels the previous one.

1. Download `KuentaSetup-<version>.exe` and its `.sha256` from the newest `edge/v*` release of the
   monorepo (`manifest.json` also lists it under `components.installer`). Check the hash:
   `Get-FileHash .\KuentaSetup-<version>.exe` must equal the `.sha256`.
2. Double-click it and accept the Windows administrator prompt. An **unsigned** build (the release
   notes say when) makes SmartScreen show "Windows protected your PC": *More info → Run anyway*.
3. The wizard checks the PC first and stops, changing nothing, if something is wrong: not 64-bit
   Windows 10/11, not elevated, under 3 GB free, a program already using port 80, 443, 3000, 5432
   or 8080 (the message names the port and the program), scripts blocked by Group Policy, or no
   connection to the cloud.
4. Type the code (case, spaces and the hyphen don't matter) and the box's name, which defaults to the
   PC's name. An invalid or expired code lets you retry with a new one. If the organization already has a box, the wizard shows its name and last
   sync and asks before replacing it with the same code — whatever that box had not synced is lost. Declining ends the
   wizard with nothing installed.
5. Wait. It writes the box's configuration, prepares the database, registers the services, checks
   them and waits up to 5 minutes for the first sync, so the restaurant's users can sign in. If the
   cloud is unreachable it finishes with a warning instead of failing; the box syncs by itself later.
6. **Abrir Kuenta** opens the POS in its own window; the **Kuenta** shortcut on the desktop and in
   the Start menu does the same.

What the installer wrote, in case support needs it: `env\box.env` (the PC's LAN address and subnet),
`env\edge-node.env` (the enrollment values and a generated database password; readable only by
Administrators and SYSTEM), `logs\setup.log`. Ticket printing is off (`PRINTER_TRANSPORT=none`): the
POS prints through the browser. The installation code is never written anywhere.

**Running it again** on a PC that already has a box is a *repair*: no code, no new enrollment,
configuration and data untouched; services and the updater task are registered again and older
programs are brought up to the installer's version (never downgraded). A PC with a database but no
configuration is refused: contact support, nothing is changed.

**Uninstall** (Windows Settings → Apps → Kuenta) removes the services, updater task, firewall rule,
programs and shortcuts, and keeps `env\`, the database, `backups\`, `caddy-data\`, `state\` and
`logs\`. Running the installer afterwards repairs the box with its data.

**Assisted or scripted install** (also what CI does):

```powershell
$env:KUENTA_SETUP_CODE = 'K7F2-9QXM'      # from Instalar caja; optional $env:KUENTA_SETUP_BOX_NAME
.\KuentaSetup-<version>.exe /VERYSILENT /SUPPRESSMSGBOXES /LOG=C:\kuenta-setup.log
#   /REPLACE        replace the organization's existing box (otherwise a 409 fails the install)
#   /CLOUDURL=URL   use another cloud than the one built in
```

Every failure exits non-zero and names its cause in the log; it never waits for input.

---

## 3. Cutting a release (developer)

An edge release is one `edge/v*` tag on the monorepo; it is independent of cloud deploys.

1. The PR job "Edge fingerprint" says whether the change needs one ("edge release needed:
   yes/no"). If `no`, there is nothing to release.
2. If the edge now depends on cloud behaviour the live cloud already has, bump `min_cloud` in
   `edge/compat.json` (it must name an existing cloud release).
3. Tag and push: `git tag edge/v1.3.0 && git push origin edge/v1.3.0`. CI
   (`.github/workflows/release-edge.yml`) refuses a tag whose fingerprint equals the last
   release's ("edge unchanged"), then publishes one GitHub Release with the appliance zip,
   `edge-node-windows-amd64-<v>.zip`, `next-standalone-windows-<v>.zip`, their `.sha256` files,
   `manifest.json` and `edge-fingerprint.txt`.
4. Boxes pick it up from the POS ("Actualizar ahora") once the live cloud satisfies its
   `min_cloud`. For scripts, Caddyfile or services changes, also run `scripts\install.ps1` on
   the box after fetching the appliance zip. `GET /api/edge/status` reports the running version.

   The same workflow builds `KuentaSetup-<v>.exe`, runs it silently against a stub cloud, and
   attaches it with its `.sha256`; a release whose installer fails that test is not published. Before
   the first release set the repository variable **`KUENTA_CLOUD_URL`** to the cloud's
   `PUBLIC_API_URL`. Signing runs when the secrets `KUENTA_SIGN_PFX_BASE64` and
   `KUENTA_SIGN_PFX_PASSWORD` exist; without them the installer is published unsigned and the notes say so.

5. **Before self-serve signup opens** (`self-serve-signup-trial`): once the `edge/v*` release that
   ships `expired-box-backoff` is out, raise the cloud's `MIN_EDGE_VERSION` to it. Older boxes don't
   pause when the cloud refuses them, so an expired trial box on one keeps calling every minute.

---

## 4. The script path (developer fallback)

What the installer runs, by hand, from an **elevated PowerShell** on a PC with the appliance files at
`C:\kuentapos` (the appliance zip of a release, extracted). No Go, pnpm or app source is needed.

```powershell
cd C:\kuentapos
.\scripts\fetch-artifacts.ps1                                  # Caddy, Node, Postgres, WinSW + the release's apps (sha256-checked)
```

Configuration: instead of editing files, pipe an enrollment result into the same script the installer uses:

```powershell
# enroll.ps1 reads {"code","replace","name","cloud_url"} on standard input (the code from Instalar caja) and prints the
# enrollment values; write-box-config.ps1 reads those on standard input and writes env\box.env and
# env\edge-node.env. They read the process's stdin, so pipe between powershell.exe processes.
'{"code":"K7F2-9QXM","replace":false,"name":"CAJA-1","cloud_url":"https://api.example.com"}' |
  powershell -NoProfile -File .\scripts\enroll.ps1 |
  powershell -NoProfile -File .\scripts\write-box-config.ps1
.\scripts\setup-box.ps1                                        # database, services, verification, shortcut
```

Or step by step: `init-postgres.ps1 -DbPassword <the DB_PASSWORD in env\edge-node.env>` (refuses an
existing data directory unless `-SkipIfInitialized`; `-Force` **wipes** it), `install.ps1`,
`verify-install.ps1` (`-WaitForSync` also waits for the first sync).

### Install the services + lock the firewall

**Elevated PowerShell.** Registers all four services in dependency order, stages the `.env`,
exports the network identity from `box.env`, sets the Postgres logon account to `NetworkService`,
verifies Caddy came up, and adds the firewall rule.

```powershell
cd C:\kuentapos
.\scripts\install.ps1
# The firewall defaults to EDGE_LAN_CIDR from box.env. Pass -AllowedTabletIPs ONLY to
# tighten to specific DHCP-reserved tablet IPs, e.g.:
# .\scripts\install.ps1 -AllowedTabletIPs '192.168.0.21','192.168.0.22'
```

The script is **re-runnable / self-healing** — if anything failed, fix it and run it again.

---

### Verify it's up

Run the smoke check (full version in [`VALIDATION.md`](VALIDATION.md)):

```powershell
Get-Service kuentapos-* | Format-Table Name, Status, StartType
# All four should be Running.

# Use the EDGE_LAN_IP you set in box.env (this box: 192.168.0.123).
curl.exe -k -i https://192.168.0.123/        # expect: 307, Location: /signin  (RELATIVE)
curl.exe -k -i https://192.168.0.123/signin  # expect: 200, HTML
```

> **`curl: (28) ... Couldn't connect`** = the IP isn't reachable: `EDGE_LAN_IP` in `box.env`
> doesn't match the address this box actually holds (run `Get-NetIPAddress -AddressFamily IPv4`).
> Fix `box.env` and re-run `install.ps1` — its preflight now catches this before it bites.

If `https://<EDGE_LAN_IP>/` redirects to `localhost:3000`, the Caddyfile `header_down Location`
fix is missing — see [`TROUBLESHOOTING.md`](TROUBLESHOOTING.md).

Unreleased code: `.\scripts\build-artifacts.ps1` builds both apps from source and needs Go, pnpm
and Developer Mode on a **development** machine, at the final path `C:\kuentapos`; never on a
customer's PC. A local build records `local-<sha>` in `artifacts\<app>\RELEASE`.

---

## 5. Network + tablets

1. **Router:** add a DHCP reservation binding the box's MAC → its `EDGE_LAN_IP` (this box:
   `192.168.0.123`) so the IP never changes. (The Caddy cert SAN and the firewall rule are tied
   to this IP.)
2. **Why HTTPS shows "not private" (`ERR_CERT_AUTHORITY_INVALID`):** the Caddyfile uses
   `tls internal`, so Caddy signs the LAN cert with its **own private CA**. The cert is valid;
   the client just doesn't trust the CA that signed it. Every client (dev PC + tablet) must
   install Caddy's **root CA** once. `install.ps1` already exports it to
   `C:\kuentapos\kuentapos-root-ca.crt` and trusts it on the box itself. If you need to re-export:
   ```powershell
   Copy-Item C:\kuentapos\caddy-data\pki\authorities\local\root.crt C:\kuentapos\kuentapos-root-ca.crt
   ```
3. **Dev PC (Windows):** copy `kuentapos-root-ca.crt` off the box, then in an elevated PowerShell:
   ```powershell
   Import-Certificate -FilePath kuentapos-root-ca.crt -CertStoreLocation Cert:\LocalMachine\Root
   ```
   Then **fully restart the browser** — Chrome caches its trust decision per-process and keeps
   background processes alive, so a plain close-and-reopen often isn't enough:
   ```powershell
   Get-Process chrome -ErrorAction SilentlyContinue | Stop-Process -Force
   ```
   Reopen `https://192.168.0.123`. Note: modern Chrome shows a **sliders/"tune" icon**, not a
   padlock — that icon is still "secure". Click it; it should read *"Connection is secure"*.
   If it still says "Not secure" after a full restart, confirm the bar shows `https://` (not
   `http://`) and that you imported the root on **this** machine (each client has its own store).
4. **Tablets:** install the same `kuentapos-root-ca.crt` as a trusted CA cert, then open
   `https://192.168.0.123`. See `certs\README.md` / `provisioning\README.md` for the per-tablet steps.

---

## Day-2 operations (quick reference)

```powershell
# Status / start / stop one service (use the WinSW wrapper):
C:\kuentapos\services\caddy.exe status
C:\kuentapos\services\caddy.exe stop
C:\kuentapos\services\caddy.exe start

# Swap ONE service to a freshly built/fetched version (minimal downtime):
.\scripts\update.ps1 -Service next        # rebuilds from frontend repo
.\scripts\update.ps1 -Service edge-node   # rebuilds from backend repo + refreshes .env
.\scripts\update.ps1 -Service caddy       # re-fetches pinned binary

# Remove everything (keeps artifacts\ and caddy-data\ so the CA root survives):
.\scripts\uninstall.ps1
```

To change config: edit `env\edge-node.env`, then `.\scripts\update.ps1 -Service edge-node`
(it re-stages the `.env` and restarts). Editing the `Caddyfile` only needs a Caddy reload —
see [`MONITORING.md`](MONITORING.md).
