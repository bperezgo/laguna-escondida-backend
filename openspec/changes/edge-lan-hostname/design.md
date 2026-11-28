# Design

## Context

See proposal.md for the motivation. Today:

- `edge/scripts/write-box-config.ps1:43-58` takes the IP from the adapter of the default route. Line 66 then writes `box.env` with `EDGE_LAN_HOST=` (blank).
- `edge/scripts/install.ps1:52,78-86` builds `EDGE_SITE_ADDR` = `"<host>, <ip>"` (or `<ip>`) and sets it as a machine env var. The `Caddyfile` site is `{$EDGE_SITE_ADDR}`, so `tls internal` issues a certificate for each name and the IP in it, all from the same internal CA. It issues one per name, picked by SNI, not one for all: the first edge-ci run showed the certificate served on the IP names only the IP. `verify-install.ps1:139-143` already checks the name with `--resolve` when `EDGE_LAN_HOST` is set. No change is needed there.
- `edge/scripts/set-lan-ip.ps1:150-157` updates `EDGE_LAN_IP`/`EDGE_LAN_CIDR` but not `EDGE_SITE_ADDR`, so Caddy keeps the old address.
- The wizard (`edge/installer/kuenta.iss`) has two pages: welcome, then sign-in (`SignInPage`, created at `:390`). The single-use code is spent in `NextButtonClick` (`:426`) for an interactive install, and in `PrepareToInstall` (`:467-491`) for a silent one.
- The gateway (UniFi in the owner's restaurant) pins the box's IP and holds `pos.kuenta.lan -> <ip>`. The box never answers DNS.

## Goals / Non-Goals

**Goals:**
- One validation, in PowerShell, used by the wizard, the silent install, `write-box-config.ps1`, `install.ps1` and `set-lan-ip.ps1`.
- Network values are checked before the code reaches the cloud.

**Non-Goals:**
- Changing the name of an installed box. That's a manual `box.env` edit plus `install.ps1`.
- Several names from the wizard. `box.env` still accepts a comma list for manual setups.

## Decisions

### D1. Fixed default name `pos.kuenta.lan`

One constant, `$DefaultLanHost = 'pos.kuenta.lan'`, in `edge/scripts/lan-config.ps1`. Pascal never hard-codes the name: the wizard pre-fills its edit box from what `lan-config.ps1 -Check` returns when called with blanks. *Alternative: per-restaurant names.* Rejected: each install would need its own DNS work, and support would have to guess the name.

### D2. `lan-config.ps1`: one script for listing and checking

New file `edge/scripts/lan-config.ps1` (copy the header, `Fail` and stdout-only style from `write-box-config.ps1`). It targets Windows PowerShell 5.1 and has two modes.

**`-List`** prints one compressed JSON line per usable IPv4 address, the default-route address first, then the rest by interface metric:

```
{"ip":"192.168.1.50","cidr":"192.168.1.0/24","alias":"Ethernet","default":"true"}
```

- Excluded: `127.0.0.0/8`, `169.254.0.0/16`, and any address whose interface is down.
- `default` is the string `"true"` or `"false"`, because the Pascal `JsonField` reads strings only.
- `cidr` is the default network for that address (D3).
- No address at all: exit 37 with `{"ok":false,"error":"Este equipo no tiene ninguna dirección de red IPv4."}`.

**`-Check -LanIp <s> -LanCidr <s> -LanHost <s>`** fills in defaults, validates, and prints one line. Success is exit 0 with `{"ok":true,"lan_ip":"…","lan_cidr":"…","lan_host":"…"}`. Failure is exit 37 with `{"ok":false,"error":"<message>"}`. Every parameter is optional:

| Input | Blank | Rule |
|---|---|---|
| `LanIp` | the default-route address (same choice as `write-box-config.ps1` today); none → `Este equipo no tiene conexión a la red.` | must parse as IPv4 and be held by the PC (`Get-NetIPAddress -AddressFamily IPv4`) |
| `LanCidr` | the default network of `LanIp` (D3) | `a.b.c.d/n`, normalized to the network address; private and `/16`–`/32` (D4); contains `LanIp` |
| `LanHost` | `pos.kuenta.lan` | `none` → `""` (IP only). Otherwise it's lowercased and must match `^(?=.{1,253}$)[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$` and not parse as an IP address |

Messages, checked in this order (the first failure wins):

| Case | `error` |
|---|---|
| IP not IPv4 | `La IP {ip} no es válida.` |
| IP not held | `Este equipo no tiene la IP {ip}. Si la acaba de fijar en el router, reconecte la red o reinicie el equipo e intente de nuevo.` |
| CIDR malformed | `La red {cidr} no es válida. Escríbala como 192.168.1.0/24.` |
| CIDR not private or wider than /16 | `La red {cidr} debe ser una red privada (10.x, 172.16-31.x o 192.168.x) de máximo 65.536 direcciones (/16 o más).` |
| IP outside CIDR | `La IP {ip} no está dentro de la red {cidr}.` |
| Bad name | `El nombre {host} no es válido. Use letras minúsculas, números, guiones y puntos, por ejemplo pos.kuenta.lan.` |

The functions `Test-LanIdentity`, `Get-LanAddresses` and `Get-DefaultLanCidr` live in the script. A `-NoRun` switch only dot-sources them for the unit test, like `releases.ps1` is used by `test-releases.ps1`.

*Alternative: validate in Pascal.* Rejected: Inno Pascal has no regex or IP parsing, and `set-lan-ip.ps1` and `install.ps1` need the same rules.

### D3. Default network of an address

The default is the adapter's network (`IP & mask` / `PrefixLength`) when the prefix is `/16` or longer. Otherwise it's the `/24` that holds the IP. Narrowing is always safe, and the owner can widen up to `/16` on the network screen.

### D4. Private ranges only, at most /16

A CIDR passes only if its network address and prefix lie wholly inside `10.0.0.0/8`, `172.16.0.0/12` or `192.168.0.0/16`, with a prefix of 16 to 32. That keeps the firewall rule from opening the box beyond the LAN (see Security).

### D5. Wizard flow

```
wpWelcome  [ ] Configurar la red manualmente (avanzado)
   | Next: preflight (as today) -> lan-config -Check (blanks, or no-op if box ticked)
   v
NetworkPage   (skipped unless ticked; never in repair)
   | Next: char allowlist -> lan-config -Check (values)
   v
SignInPage    (enrollment spends the code here)
```

- **Checkbox.** A `TNewCheckBox` (`NetworkBox`) with parent `WizardForm.WelcomePage`, placed below `WizardForm.WelcomeLabel2` and the width of the label, with the caption `Configurar la red manualmente (avanzado)`. It's hidden when `Mode <> 'fresh'`.
- **Network page.** `NetworkPage := CreateCustomPage(wpWelcome, 'Red del local', 'Confirme cómo los equipos del restaurante llegan a la caja')`. `SignInPage` is created after `NetworkPage.ID` instead of `wpWelcome`. Its surface holds, top to bottom:
  - `IP de esta caja:` with `IpCombo: TNewComboBox`, `Style := csDropDownList`, items `"<ip> (<alias>)"`
  - `Red permitida:` with `CidrEdit: TNewEdit`
  - `Nombre local:` with `HostEdit: TNewEdit`
  - a `TNewStaticText` that reads `Deje el nombre vacío para usar solo la IP. El router debe tener este nombre apuntando a la IP elegida, y esa IP fija para este equipo.`
- **Filling the page.** On the first `CurPageChanged(NetworkPage.ID)`, run `lan-config.ps1 -List`. Split stdout on `#10`, and keep the arrays `AddrIp[]`/`AddrCidr[]` in combo order. Select index 0, set `CidrEdit.Text := AddrCidr[0]`, and set `HostEdit.Text := NetHost` (the default from the welcome-page check). `IpCombo.OnChange` sets `CidrEdit.Text := AddrCidr[IpCombo.ItemIndex]`.
  - If `-List` fails, `MsgBox` its `error`, and the page's Next stays refused.
- **Allowlist.** Before calling the script, Pascal rejects any `CidrEdit`/`HostEdit` character outside `0-9 a-z A-Z . - /`, using the matching message from D2 (`La red … no es válida…` / `El nombre … no es válido…`). This keeps the values safe to quote on the command line.
- **Empty name.** An empty `HostEdit` is passed as `none`.
- **Results.** `NetIp`, `NetCidr` and `NetHost` (globals) hold the last successful `-Check` result. Every later step uses them.

### D6. Silent mode

In `PrepareToInstall` (silent, `Mode = 'fresh'`), before `TryEnroll`, run `-Check` with `GetEnv('KUENTA_SETUP_LAN_IP')`, `GetEnv('KUENTA_SETUP_LAN_CIDR')` and `GetEnv('KUENTA_SETUP_LAN_HOST')`. Unset or empty means default, and `none` means no name. Inno can't tell an unset variable from an empty one, so `none` is the only way to ask for no name.

On failure: `Log('network check failed: ' + error)` and `Result := ExitMessage(37) + ' ' + error`. Setup then aborts non-zero before any enrollment.

### D7. `write-box-config.ps1` takes the confirmed values

- Replace the `-PrefixLength` parameter with `-LanCidr` and add `-LanHost`.
- Lines 43-58 become one call, `& (Join-Path $PSScriptRoot 'lan-config.ps1') -Check -LanIp $LanIp -LanCidr $LanCidr -LanHost $LanHost`. If it doesn't return `ok`, call `Fail` with its error and exit 37 (not 31).
- Line 66 writes `EDGE_LAN_HOST=<lan_host>`.
- Line 103 adds `lan_host` to the output.
- The installer (`kuenta.iss:530`) passes `-LanIp "<NetIp>" -LanCidr "<NetCidr>" -LanHost "<NetHost or none>"`.

### D8. `install.ps1` and `set-lan-ip.ps1` share the rules

- **`install.ps1`.** After `:75`, call `lan-config.ps1 -Check` with the `box.env` values. Pass only the first entry of a comma-separated `EDGE_LAN_HOST`, or `none` when it's blank. On failure, throw its `error`. This keeps hand-edited boxes inside D4.
- **`set-lan-ip.ps1`.** Before `:150`, run the same check with the new IP and CIDR, and throw on failure. After `:157`, rebuild `EDGE_SITE_ADDR` with `Get-SiteAddr` (dot-source `lan-config.ps1 -NoRun`; `install.ps1:78` switches to it too), using `$box['EDGE_LAN_HOST']` and `$IP`. Set it at Machine scope and in `$env:`. Fix the header comment at `:16` to name `{$EDGE_SITE_ADDR}`.

### D9. Finish page

`CurPageChanged(wpFinished)` appends the following when `FailCode = 0`. In repair mode it reads `NetIp`/`NetHost` from `env\box.env` (the first `EDGE_LAN_HOST` entry).

- With a name:

  ```
  Desde otros equipos del restaurante abra https://{host} (o https://{ip}).
  En el router, el nombre {host} debe apuntar a {ip}, y esa IP debe estar fija para este equipo.
  ```

- Without a name:

  ```
  Desde otros equipos del restaurante abra https://{ip}.
  En el router, esa IP debe estar fija para este equipo.
  ```

### D10. Exit code 37

Add to `exit-codes.json`: `"37": {"key": "network_invalid", "message": "La red elegida no sirve para la caja."}`. Regenerate `exit-codes.inc` with `gen-exit-codes.ps1`.

### D11. Drop Technitium

Delete `edge/scripts/setup-technitium-dns.ps1`, `edge/scripts/dev-test-here.ps1` (its only user), `edge/docs/DNS_SETUP_GUIDE.md` and `edge/docs/DNS_RESTORE_GUIDE.md`. The `edge/ai-plan/*` files are historical notes and stay unchanged.

### D12. Details settled by the readiness review

These override anything above that conflicts with them.

- **ASCII-only JSON.** `lan-config.ps1` defines `ConvertTo-AsciiJson($obj)`. It runs `ConvertTo-Json -Compress -Depth 3`, then replaces every character above `0x7F` with `\uXXXX` (four lowercase hex digits). Every JSON line that `lan-config.ps1` or `write-box-config.ps1` prints goes through it. The reason: Windows PowerShell 5.1 writes stdout in the OEM code page, and Pascal's `JsonField` already decodes `\uXXXX`. No script sets `[Console]::OutputEncoding`.
- **Script shape.** It starts with `param([switch]$List, [switch]$Check, [switch]$NoRun, [string]$LanIp, [string]$LanCidr, [string]$LanHost)`, then the functions, then `if ($NoRun) { return }`, then the modes, each ending in `exit 0` or `exit 37`. Callers dot-source it with `. (Join-Path $PSScriptRoot 'lan-config.ps1') -NoRun`. `test-releases.ps1` is the model only for the test file's `Check`/`$fail` structure, since `releases.ps1` has no `-NoRun`.
- **Strict IPv4.** An IP is valid only if it matches `^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$` and every octet is 255 or less. `[IPAddress]::Parse` alone accepts `1`.
- **`Get-LanAddresses([object[]]$Source, [string]$DefaultIp)`.** `$Source` items have the fields `IPAddress`, `PrefixLength`, `InterfaceAlias`, `InterfaceMetric` and `Up` (bool).
  - Without `-Source`, it builds them from `Get-NetIPAddress -AddressFamily IPv4`, joined to `Get-NetIPInterface -AddressFamily IPv4` (`InterfaceMetric`, and `Up` = `ConnectionState -eq 'Connected'`).
  - Without `-DefaultIp`, it takes the default-route address with the logic of today's `write-box-config.ps1:44-52`.
  - It drops `127.*`, `169.254.*` and `Up = $false`. It sorts `DefaultIp` first, then by `InterfaceMetric` ascending.
- **`Resolve-LanIdentity([string]$LanIp, [string]$LanCidr, [string]$LanHost)`.** Reads the held IPs and the default IP from the machine, using `Get-LanAddresses` without `-Source`, and returns `Test-LanIdentity …`. `-Check` prints `ConvertTo-AsciiJson (Resolve-LanIdentity …)`.
  - `write-box-config.ps1`, `install.ps1` and `set-lan-ip.ps1` dot-source the script and call `Resolve-LanIdentity` directly. They don't start a child process. `ok` is a real boolean.
- **`write-box-config.ps1` failures.**
  - `Fail` becomes `function Fail($msg, [int]$code = 31)`, and the `trap` keeps 31.
  - When the network check fails, it calls `Fail $r.error 37`, and `Fail` prints through `ConvertTo-AsciiJson`.
- **`install.ps1` / `set-lan-ip.ps1`.** Keep their existing English held-IP and CIDR-shape checks (`install.ps1:55-75`, `set-lan-ip.ps1:118-120`), and add the `Resolve-LanIdentity` call after them.
  - Only the first `EDGE_LAN_HOST` entry is validated: `($v -split ',')[0].Trim()`, or `none` when it's blank.
  - `EDGE_SITE_ADDR` always uses the **full** comma list: `Get-SiteAddr $box['EDGE_LAN_HOST'] $ip`, which trims the value but keeps every entry. This applies in both scripts, so a hand-made multi-name box keeps all its names after an IP change.
- **Pascal helpers.**
  - `ListAddresses` walks `Output` with `Pos(#10, …)` and `Copy`, trims `#13`, skips empty lines, and reads `ip`, `cidr`, `alias` with `JsonField`.
  - `function HostOrNone(const H: String): String` returns `'none'` for a blank `Trim(H)`, otherwise `Trim(H)`.
- **Welcome-page check.** In an interactive fresh install, `CheckNetwork('', '', '', Msg)` runs on **every** Next from `wpWelcome`. It sits after, and outside, the `if not PreflightDone` block (`kuenta.iss:414`), so going back and forward refreshes `NetIp`/`NetCidr`/`NetHost`.
- **Network page layout.** All controls have `Parent := NetworkPage.Surface`, `Left := 0` and `Width := NetworkPage.SurfaceWidth`. Each label's `Top` is the previous control's `Top + Height + ScaleY(12)` (the first is 0), and each input's `Top` is its label's `Top + Height + ScaleY(4)`. The hint is a `TNewStaticText` with `AutoSize := False`, `WordWrap := True` and `Height := ScaleY(48)`.
- **Firewall assert.** `Get-NetFirewallAddressFilter` returns `RemoteAddress` as `a.b.c.d/m.m.m.m`. The test converts the mask to a prefix by counting its set bits, and accepts a value already in `/n` form. The installer passes no `-AllowedTabletIPs`, so the rule holds exactly one entry, `EDGE_LAN_CIDR`.

- **`set-lan-ip.ps1` and D3/D4.**
  - When the old CIDR doesn't contain the new IP, the derive branch (`set-lan-ip.ps1:138`) uses `Get-DefaultLanCidr $IP $detectedPrefix` instead of `Get-NetworkCidr`.
  - If the CIDR that results, whether kept or derived, fails `Resolve-LanIdentity`, the script throws `"<error> Pass -Cidr with a private network of /16 or narrower."` and changes nothing. It never silently replaces an old CIDR.
- **`-List` failure, or no addresses.**
  - The network page stays as it is, and Next is refused. The user can go Back, untick the option, and take the defaults, or Cancel.
  - `NextButtonClick(NetworkPage.ID)` refuses when `IpCombo.ItemIndex < 0`, showing the `-List` error, or `Este equipo no tiene ninguna dirección de red IPv4.` when there is none.
  - `AddrAlias` is not kept. `alias` is read only for the combo label.
- **Welcome-page check with the box ticked (found by pr-verify).** A failed default check refuses Next only when the box is unticked. With it ticked, the owner continues to the network page, which runs its own check. If the default check failed, `HostEdit` is pre-filled from a Pascal constant `DefaultLanHost = 'pos.kuenta.lan'`, the one place outside `lan-config.ps1` that names it. Otherwise a PC whose default address fails D4 could never reach the page that lets it pick another.
- **Where the welcome-page check goes.** Insert it after the closing `end;` of the `if not PreflightDone` block (`kuenta.iss:415-423`), guarded by `if Result and (Mode = 'fresh') then`. The existing `MsgBox` in that block is the preflight failure. Leave it as it is.
- **`ShouldSkipPage`.** `NetworkBox` is always created in `InitializeWizard` (it's hidden when `Mode <> 'fresh'`), so it's never nil. The skip is `if PageID = NetworkPage.ID then Result := (Mode <> 'fresh') or not NetworkBox.Checked;`.
- **Callers of `write-box-config.ps1`.** Only `kuenta.iss:530` (no arguments today) and the runbook's manual command (`edge/docs/DEPLOYMENT_RUNBOOK.md:141`) call it. Nothing passes `-PrefixLength`.

## Implementation Map

### §1 lan-config.ps1 and its test
- **Create `edge/scripts/lan-config.ps1`.** Copy the style of `edge/scripts/write-box-config.ps1`. Signature:
  ```
  param([switch]$List, [switch]$Check, [switch]$NoRun, [string]$LanIp, [string]$LanCidr, [string]$LanHost)
  ```
  Functions:
  - `Get-LanAddresses` returns `[pscustomobject]@{ip; cidr; alias; default}` and takes an optional `-Source` of fake adapters for tests.
  - `Get-DefaultLanCidr([string]$ip, [int]$prefix)` returns a string.
  - `Test-LanIdentity($ip, $cidr, $lanHost, $heldIps, $defaultIp)` returns `@{ok; lan_ip; lan_cidr; lan_host; error}`. It's pure, so the test needs no network.
  - `Get-SiteAddr([string]$lanHost, [string]$ip)` returns `"$lanHost, $ip"`, or `$ip` when `$lanHost` is blank. It's the formula from `install.ps1:78`, so `install.ps1` and `set-lan-ip.ps1` both call it instead of repeating it.
- **Create `edge/scripts/test-lan-config.ps1`.** Copy `edge/scripts/test-releases.ps1` and dot-source `lan-config.ps1 -NoRun`.
- **Edit `.github/workflows/edge-ci.yml`.** Add a step `Unit: lan-config` before `Build the installer` (`:80`): `pwsh -File scripts/test-lan-config.ps1`.

### §2 Exit code and write-box-config
- **`edge/installer/exit-codes.json`**: add 37 (D10).
- **`edge/installer/exit-codes.inc`**: regenerate.
- **`edge/scripts/write-box-config.ps1`**: params `:12-18`, network block `:43-58`, `box.env` write `:66`, output `:103`. Update the header comment, which lists exit codes `(0, 31)`, to `(0, 31, 37)`.
- **`edge/env/box.env.example`**: the `EDGE_LAN_HOST` comment and value become `pos.kuenta.lan`. Add a line saying the gateway, not the box, holds the record.

### §3 install.ps1 and set-lan-ip.ps1
- **`edge/scripts/install.ps1`**: insert the check after `:75` (D8).
- **`edge/scripts/set-lan-ip.ps1`**: the check before `:150`, `EDGE_SITE_ADDR` after `:157`, and the header `:16`.
- **`edge/Caddyfile:11`**: the example becomes `"pos.kuenta.lan, 192.168.1.50"`.

### §4 Wizard
- **`edge/installer/kuenta.iss`**:
  - `[Files]` after `:61`: `Source: "..\scripts\lan-config.ps1"; DestDir: "{tmp}"; Flags: dontcopy`.
  - `var` block `:96-106`: `NetworkPage: TWizardPage; NetworkBox: TNewCheckBox; IpCombo: TNewComboBox; CidrEdit, HostEdit: TNewEdit; AddrIp, AddrCidr: TArrayOfString; NetIp, NetCidr, NetHost: String; NetListed: Boolean;`.
  - New functions after `DoPreflight` (`:336`): `function CheckNetwork(const Ip, Cidr, Host: String; var Msg: String): Boolean;`, which extracts `lan-config.ps1`, runs `-Check`, and sets `NetIp`/`NetCidr`/`NetHost` from `lan_ip`/`lan_cidr`/`lan_host`. Also `procedure ListAddresses;` and `procedure IpComboChange(Sender: TObject);`.
  - `InitializeWizard` `:388-399`: the checkbox, `NetworkPage`, and `SignInPage` after `NetworkPage.ID` (D5).
  - `ShouldSkipPage` `:401-405`: skip `NetworkPage` when `(Mode <> 'fresh') or not NetworkBox.Checked`.
  - `NextButtonClick` `:407`. In `wpWelcome`, after preflight succeeds and only when `Mode = 'fresh'` and `not NetworkBox.Checked`, call `CheckNetwork('', '', '', Msg)`. When `NetworkBox.Checked`, call `CheckNetwork('', '', '', Msg)` too, so `NetHost` holds the default name for the page. On failure, `MsgBox` and refuse. Add a new `NetworkPage.ID` branch: the allowlist, then `CheckNetwork(AddrIp[IpCombo.ItemIndex], CidrEdit.Text, HostOrNone)`.
  - `CurPageChanged` `:553`: list the addresses on the first visit to `NetworkPage`, and the finish text (D9).
  - `PrepareToInstall` `:476-478`: the silent check (D6).
  - `CurStepChanged` `:530`: the `write-box-config.ps1` arguments (D7).

### §5 Installer test
- **`edge/installer/test/run-installer-tests.ps1`**:
  - A new step after `Box already enrolled` (`:296-300`): `Network IP the PC does not hold`. It sets `$env:KUENTA_SETUP_LAN_IP = '192.0.2.1'`, runs `Install $ok 'bad-ip'`, and removes the variable. It asserts: non-zero exit, no `C:\kuentapos`, the log contains `192.0.2.1`, and `Enroll-Calls $ok` is unchanged.
  - After the fresh install (`:321`): assert that `C:\kuentapos\env\box.env` contains `EDGE_LAN_HOST=pos.kuenta.lan`, and that the machine variable `EDGE_SITE_ADDR` starts with `pos.kuenta.lan, `.
  - The `Install` helper at `:83` is unchanged.

### §6 Docs and cleanup
- **Delete** the files in D11.
- **Edit** `edge/README.md` (the diagram line at `:14`, the comment block at `:74-75`), `edge/docs/DEPLOYMENT_RUNBOOK.md` (`:135-141`, which mention the new `-LanIp`/`-LanCidr`/`-LanHost` parameters), `edge/provisioning/README.md` (`:8`, `:29-36`), `edge/certs/README.md` (`:5`, `:35`) and `edge/docs/LAN_SERVING_WALKTHROUGH.md`: `pos.laguna.lan` becomes `pos.kuenta.lan`. Replace the Technitium/router alternatives with one paragraph, "En el gateway (p. ej. UniFi): IP fija para la caja y registro DNS local `pos.kuenta.lan -> <ip>`".

## Security

The trust boundary is the box's LAN exposure.

- **Lower-trust principal.** Devices on the restaurant network, and anything that ends up inside a CIDR wider than intended.
- **Input that sets the boundary.** The CIDR, typed by an elevated administrator on the network screen, or through `KUENTA_SETUP_LAN_CIDR`, `box.env` or `set-lan-ip.ps1 -Cidr`.
- **Control.** `Test-LanIdentity` in `edge/scripts/lan-config.ps1` accepts only a private CIDR with a prefix of `/16` or longer (D4). It's called by every writer: the wizard, silent mode, `write-box-config.ps1`, `install.ps1` and `set-lan-ip.ps1`. The firewall rule (`install.ps1` step 4) and Caddy's `@notlan` guard consume the result.
- **Protected resource.** The POS on ports 443/80: sign-in and the restaurant's orders, and payments recorded on the box.
- **If the control fails.** A `0.0.0.0/0` rule would expose the POS to any network the PC joins, for example a public Wi-Fi when the PC is a laptop.

**Attack classes considered:**
- **Exposure through a mis-scoped allowlist.** Mitigated by D4.
- **Command-line injection through typed values passed to `powershell.exe -File`.** Mitigated by the D5 allowlist. The caller is already an elevated admin, so injection would not gain privilege. The allowlist mainly prevents broken quoting.
- **Name spoofing.** Another device answering for `pos.kuenta.lan` can't present a certificate signed by the box's CA, so a tablet that trusts only that CA sees a warning.
- **Same CA, new IP.** `set-lan-ip.ps1` keeps the CA, which is intended (D8).

**Not touched:** secrets (the KUENTA_SCRIPT_INPUT path is unchanged, and network values are not secret, so they go as arguments), enrollment, sync and auth.

**Needs validation:**
- *The UniFi local DNS record and fixed IP exist for the box.* Check with `nslookup pos.kuenta.lan` from a tablet on the restaurant Wi-Fi, and confirm the DHCP reservation in the UniFi console under Client > Fixed IP.
- *Tablets use the gateway as their DNS*, not a public resolver or Android Private DNS. Check under the tablet's Wi-Fi settings.
- *The GitHub `windows-latest` runner's address is private.* The default-path check fails if it isn't. Check the `Network check` log line of the first CI run.

**Review (task 6.1, focused, on the implementation diff):**
- *Every `EDGE_LAN_CIDR` writer is checked.* `write-box-config.ps1` writes only `Resolve-LanIdentity` output. `install.ps1` calls it on the `box.env` values before it sets the machine variables or the firewall rule. `set-lan-ip.ps1` calls it before any write. The wizard and silent mode reach `box.env` only through `write-box-config.ps1`, with the values of a passed `-Check`.
- *Fixed:* `set-lan-ip.ps1` checked the typed `-Cidr` but wrote it as typed. It now writes the normalized `lan_cidr`.
- *Allowlist before `RunPS`.* Typed values reach `RunPS` only through `CheckNetwork`. The network page runs the D5 allowlist on the CIDR and the name, and the IP comes from the `-List` output. Silent mode runs the same allowlist on all three variables, which is broader than D5. The `write-box-config.ps1` call carries only `NetIp`/`NetCidr`/`NetHost` from a passed check.
- *Accepted, out of scope:* `install.ps1 -AllowedTabletIPs` still lets an elevated admin set the firewall rule's addresses by hand. The installer never passes it.
- *CI runner address (validated):* the first edge-ci run's `Network check:` line shows `EDGE_LAN_IP=10.1.0.9 EDGE_LAN_CIDR=10.1.0.0/20`, a private /20 that passes D4.
- *Needs validation (still open):* the UniFi record and fixed IP, and tablet DNS. Both are checked on the owner's reinstall.

## Risks / Trade-offs

- **[The CI runner's adapter has a prefix shorter than /16, or a public IP]** → D3 narrows a short prefix to the `/24`. A public IP would fail every fresh install in CI, so the first CI run validates it (see Security).
- **[The owner sets a fixed IP in UniFi, but the PC still holds the old lease]** → the IP isn't in the list, and the page text says to reconnect or reboot. The check message says the same.
- **[A tablet uses Private DNS or a public resolver and can't resolve `pos.kuenta.lan`]** → `https://<ip>` still works, and the finish page and docs name both.
- **[A hand-made box with a CIDR wider than /16 fails `install.ps1` on its next repair]** → intended. The message names the rule. The only such box is the owner's, which is being reinstalled.
- **[`.lan` is not an IANA-reserved TLD]** → it is widely used for local names, and UniFi serves it. Fine for a closed LAN. `home.arpa` would be the strictly correct choice, but it's longer to type.

## Migration Plan

The owner's restaurant gets a fresh install:

1. In UniFi, give the PC a fixed IP and add the local DNS record `pos.kuenta.lan -> <that IP>`.
2. Reconnect the PC, or reboot it, so it holds the fixed IP.
3. Uninstall any old box, then run the new installer with the advanced option ticked. Confirm that the fixed IP is selected.
4. Install `C:\kuentapos\kuentapos-root-ca.crt` on each tablet, then open `https://pos.kuenta.lan`.

Rollback: the previous installer still works. It gives an IP-only box.
