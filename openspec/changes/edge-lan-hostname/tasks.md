# Tasks

Windows PowerShell 5.1 targets. `test-lan-config.ps1` runs anywhere with `pwsh` (it passes fake adapters). The installer test runs only on Windows CI (`edge-ci.yml`, "Installer test (silent)"). Wizard UI scenarios have no automated driver, so they're checked by hand on a Windows PC in §4.5.

## 1. lan-config.ps1: one check for the box's network

- [x] 1.1 Create `edge/scripts/test-lan-config.ps1`, copying the structure of `edge/scripts/test-releases.ps1` (its `Check` helper and `$fail` count, exit 1 when `$fail -gt 0`). It dot-sources `lan-config.ps1 -NoRun` and calls `Test-LanIdentity`, `Get-DefaultLanCidr`, `Get-SiteAddr`, `ConvertTo-AsciiJson` and `Get-LanAddresses -Source <fake adapters> -DefaultIp 10.0.0.7` exactly as design.md D2, D12 and §1 define them. The fake adapters use the D12 fields. Cases, with held IPs `@('192.168.1.50','10.0.0.7')` and default IP `10.0.0.7`:
  - **Defaults:** blanks → `lan_ip 10.0.0.7`, and `lan_host pos.kuenta.lan`.
  - **IP:** `192.168.77.77` → error "Este equipo no tiene la IP 192.168.77.77…". `300.1.1.1` and `1` → "La IP … no es válida.".
  - **Network:**
    - `192.168.1.5/24` normalizes to `192.168.1.0/24`.
    - `192.168.2.0/24` with IP `192.168.1.50` → "no está dentro de la red".
    - `0.0.0.0/0`, `192.168.0.0/8`, `8.8.8.0/24` and `10.0.0.0/15` → the private-network error.
    - `192.168.0.0/16` and `192.168.1.50/32` pass.
    - `abc` → "no es válida".
  - **Name:**
    - `none` → `lan_host ""`.
    - `POS.Kuenta.LAN` → `pos.kuenta.lan`.
    - `pos kuenta`, `-pos.lan`, `pos` (one label), `192.168.1.50` and a 254-character name → the name error.
  - **Default network:** `Get-DefaultLanCidr 10.0.0.7 8` → `10.0.0.0/24`. `Get-DefaultLanCidr 192.168.1.50 24` → `192.168.1.0/24`.
  - **Site address:** `Get-SiteAddr 'pos.kuenta.lan' '192.168.1.50'` → `pos.kuenta.lan, 192.168.1.50`. `Get-SiteAddr '' '192.168.1.50'` → `192.168.1.50`.
  - **ASCII JSON:** `ConvertTo-AsciiJson @{ error = 'válida' }` → `{"error":"v\u00e1lida"}`.
  - **Address list:** fake adapters including `127.0.0.1`, `169.254.3.4` and a down interface → only `192.168.1.50` and `10.0.0.7` are listed, the default first, with `default` `"true"` on the first entry and `"false"` on the rest.

  Verify: `pwsh -File edge/scripts/test-lan-config.ps1` fails because `lan-config.ps1` does not exist.
- [x] 1.2 Create `edge/scripts/lan-config.ps1` per design.md D1-D4, D12 and §1, including `Resolve-LanIdentity` and `ConvertTo-AsciiJson`: the `-List`, `-Check` and `-NoRun` modes, the exact messages from D2 in the D2 order, exit 37 on failure, and compressed JSON on stdout only. Verify: `pwsh -File edge/scripts/test-lan-config.ps1` prints only `ok` lines and exits 0.
- [x] 1.3 In `.github/workflows/edge-ci.yml`, add a step `Unit: lan-config` before `Build the installer` that runs `pwsh -File scripts/test-lan-config.ps1` (the job's working directory is `edge`). Verify: `grep -n "test-lan-config" .github/workflows/edge-ci.yml` shows the step placed before "Build the installer".

## 2. write-box-config.ps1 writes the confirmed network

- [x] 2.1 In `edge/installer/test/run-installer-tests.ps1`, after the `fresh install exits 0` assert (`:321`), add these asserts:
  - (a) `C:\kuentapos\env\box.env` has the line `EDGE_LAN_HOST=pos.kuenta.lan`.
  - (b) `[Environment]::GetEnvironmentVariable('EDGE_SITE_ADDR','Machine')` starts with `pos.kuenta.lan, ` and ends with the `EDGE_LAN_IP` from `box.env`.
  - (c) The firewall rule `Kuenta - Caddy (LAN tablets)` has exactly one remote address, and it equals `EDGE_LAN_CIDR` from `box.env` after mask-to-prefix normalization. Add a local `function ConvertTo-Cidr([string]$addr)` in `run-installer-tests.ps1` next to `Assert`: `a.b.c.d/m.m.m.m` becomes `a.b.c.d/<set bits>`, and `/n` is returned unchanged.
  - (d) No process whose `ExecutablePath` is under `C:\kuentapos` listens on port 53: check `Get-NetTCPConnection -LocalPort 53 -State Listen` and `Get-NetUDPEndpoint -LocalPort 53` against their `OwningProcess`.

  - (e) The certificate served on `https://<EDGE_LAN_IP>` has a subjectAltName (OID `2.5.29.17`) containing `EDGE_LAN_IP`, and the one served for SNI `pos.kuenta.lan` contains `pos.kuenta.lan` (`tls internal` issues one certificate per name). Copy the read from `edge/scripts/set-lan-ip.ps1:204-215` (`TcpClient` + `SslStream`, match by OID).

  The repair assert comes last because it needs the repair step. After the `repair exits 0` assert (`:375`), add: (f) the SHA-256 of `box.env` equals the hash taken right after the fresh install.

  Verify: `pwsh -NoProfile -Command "[ScriptBlock]::Create((Get-Content -Raw edge/installer/test/run-installer-tests.ps1)) | Out-Null"` parses without error. Assert (a) and (b) fail on the next edge-ci run until 2.3 lands.
- [x] 2.2 Add exit code 37 to `edge/installer/exit-codes.json` (design.md D10), then run `pwsh -File edge/installer/gen-exit-codes.ps1`. Verify: `edge/installer/exit-codes.inc` contains `37: Result := 'La red elegida no sirve para la caja.';`, and re-running the generator leaves `git diff --stat edge/installer/exit-codes.inc` unchanged.
- [x] 2.3 Edit `edge/scripts/write-box-config.ps1` per design.md D7:
  - **Parameters:** `-LanIp`, `-LanCidr` and `-LanHost` (remove `-PrefixLength`).
  - **Network block:** dot-source `lan-config.ps1 -NoRun` and replace the block at `:43-58` with `$r = Resolve-LanIdentity $LanIp $LanCidr $LanHost`. On failure, call `Fail $r.error 37`. `Fail` gains the `$code` parameter and prints through `ConvertTo-AsciiJson`, as in D12.
  - **Output:** `box.env` gets `EDGE_LAN_HOST=<lan_host>`, the output JSON gains `lan_host`, and the header names exit codes `(0, 31, 37)`.

  Verify, on a Windows PC:
  - `'{"node_id":"n","organization_id":"o","node_key":"k","cloud_sync_url":"https://x"}' | powershell -File edge\scripts\write-box-config.ps1 -Root $tmp` (where `$tmp` holds a copy of `env\edge-node.env.example`) writes `box.env` with `EDGE_LAN_HOST=pos.kuenta.lan`.
  - The same run with `-LanIp 192.0.2.1` exits 37 and writes no `box.env`.
- [x] 2.4 Update `edge/env/box.env.example` per design.md §2: `EDGE_LAN_HOST=pos.kuenta.lan`, with the comment saying the gateway, not the box, holds the record. Verify: `grep -n "laguna" edge/env/box.env.example` prints nothing.

## 3. install.ps1 and set-lan-ip.ps1 share the check (parallel-safe with §4)

- [x] 3.1 In `edge/scripts/install.ps1`:
  - After `:75`, keeping the existing checks, dot-source `lan-config.ps1 -NoRun`. Call `Resolve-LanIdentity` with the `box.env` IP, the CIDR, and the first `EDGE_LAN_HOST` entry (`($LanHost -split ',')[0].Trim()`, or `none` when it's blank). Throw its `error` when it isn't `ok`.
  - Replace `:78` with `$SiteAddr = Get-SiteAddr $LanHost $LanIp`, using the full list (D12).

  Verify: edge-ci "Installer test (silent)" passes, including assert 2.1(b). Then on the Windows PC, set `EDGE_LAN_CIDR=192.168.0.0/8` in a test box's `box.env`. `install.ps1` must throw the D2 private-network message and leave the firewall rule unchanged.
- [x] 3.2 In `edge/scripts/set-lan-ip.ps1` (design.md D8):
  - Before `:150`, dot-source `lan-config.ps1 -NoRun` and run `Resolve-LanIdentity $IP $Cidr <first EDGE_LAN_HOST entry, or none>`. Throw on failure.
  - After `:157`, set `EDGE_SITE_ADDR` (Machine and `$env:`) to `Get-SiteAddr $box['EDGE_LAN_HOST'] $IP`, using the full list (D12).
  - In the derive branch (`:138`), use `Get-DefaultLanCidr $IP $detectedPrefix`. A kept or derived CIDR that fails gets the D12 throw.
  - Fix the header comment at `:16` to name `{$EDGE_SITE_ADDR}`.

  Verify: `grep -n "EDGE_SITE_ADDR" edge/scripts/set-lan-ip.ps1` shows the Machine-scope set. Then, on the owner's box after reinstall (§4.5), with UniFi moved to a new fixed IP: `set-lan-ip.ps1 -IP <new>` reports "Certificate SAN confirms <new>", the served certificate's SAN no longer contains the old IP (read it with the same `SslStream` code), and `https://pos.kuenta.lan` loads on a provisioned tablet with no warning.
- [x] 3.3 Change the example at `edge/Caddyfile:11` to `"pos.kuenta.lan, 192.168.1.50"`. Verify: `grep -n "laguna" edge/Caddyfile` prints nothing.

## 4. Wizard: advanced network step, silent variables, finish page

- [x] 4.1 In `edge/installer/test/run-installer-tests.ps1`, add a step after "Box already enrolled, no /REPLACE" (`:296-300`), per design.md §5, named `Network IP the PC does not hold`:
  - Set `$env:KUENTA_SETUP_LAN_IP = '192.0.2.1'`, run `Install $ok 'bad-ip'`, then remove the variable in a `finally`.
  - Assert: exit code non-zero, `C:\kuentapos` absent, the log contains `192.0.2.1`, and `Enroll-Calls $ok` equals its value before the step.

  Verify: the script parses (the same command as 2.1). On edge-ci this step fails until 4.3 lands.
- [x] 4.2 In `edge/installer/kuenta.iss`, add the declarations and helpers from design.md §4:
  - **Declarations:** the `lan-config.ps1` `dontcopy` `[Files]` line, and the new `var` entries.
  - **`CheckNetwork`:** extracts the script, runs `-Check -LanIp "<ip>" -LanCidr "<cidr>" -LanHost "<host>"` through `RunPS`, and on success sets `NetIp`/`NetCidr`/`NetHost` from `JsonField`. On failure, `Msg := JsonField(Output, 'error')`.
  - **`ListAddresses`:** runs `-List` and splits stdout by hand (D12: `Pos(#10)`/`Copy`, trim `#13`, skip empty lines) into `AddrIp`/`AddrCidr` and the combo items `"<ip> (<alias>)"`.
  - **`HostOrNone`.**
  - **Network page controls:** laid out as in D12.
  - **`IpComboChange`.**

  Verify: `ISCC.exe /DAppVersion=v0.0.0 /DStageDir=<stage> /DCloudUrl=http://127.0.0.1:18080 edge\installer\kuenta.iss` compiles with no error. Also, `run-installer-tests.ps1`'s `Test-HiddenLaunches` check still reports no problems.
- [x] 4.3 Wire the flow in `edge/installer/kuenta.iss` per design.md D5, D6 and D7:
  - **`InitializeWizard`:** the checkbox (hidden unless `Mode = 'fresh'`), `NetworkPage`, and `SignInPage` after `NetworkPage.ID`.
  - **`ShouldSkipPage`:** skips `NetworkPage` unless it's a fresh install with the box ticked.
  - **`NextButtonClick`:** for `wpWelcome`, call `CheckNetwork('', '', '', Msg)` on every Next, after `:423`, guarded by `if Result and (Mode = 'fresh')` (D12). `MsgBox` and refuse on failure. For `NetworkPage.ID`, refuse when `IpCombo.ItemIndex < 0` (D12). For `NetworkPage.ID`, apply the D5 allowlist, map an empty host to `none`, and call `CheckNetwork`.
  - **`CurPageChanged(NetworkPage.ID)`:** calls `ListAddresses` once and pre-fills `HostEdit.Text := NetHost`.
  - **`PrepareToInstall`:** in silent fresh mode, before `TryEnroll`, run `CheckNetwork(GetEnv('KUENTA_SETUP_LAN_IP'), GetEnv('KUENTA_SETUP_LAN_CIDR'), GetEnv('KUENTA_SETUP_LAN_HOST'), Msg)`. On failure, log `network check failed: <Msg>` and return `ExitMessage(37) + ' ' + Msg`.
  - **`CurStepChanged`:** passes `-LanIp "<NetIp>" -LanCidr "<NetCidr>" -LanHost "<NetHost, or none>"` to `write-box-config.ps1`.

  Verify: edge-ci "Installer test (silent)" passes with asserts 2.1(a)-(f) and the 4.1 step green.
- [x] 4.4 Finish page per design.md D9. In `CurPageChanged(wpFinished)`, when `FailCode = 0`, append the D9 text. In repair mode, first read `EDGE_LAN_IP` and the first `EDGE_LAN_HOST` entry from `C:\kuentapos\env\box.env` with `LoadStringsFromFile`. Verify: the ISCC compile from 4.2 passes, and the wizard check in 4.5 shows the text.
- [ ] 4.5 Check the wizard by hand on a Windows PC that holds two IPv4 addresses (Ethernet plus Wi-Fi), building it with `edge\installer\build-installer.ps1` against the stub cloud (`edge\installer\test\stub-cloud.ps1`). Verify each `edge-installer` scenario:
  - **Unticked:** the welcome page goes straight to sign-in, and `box.env` ends up with the detected IP and `pos.kuenta.lan`.
  - **Ticked:** the network page lists only the PC's addresses, the dropdown can't be typed into, and picking the other address updates "Red permitida".
  - **Rejected values:** `192.168.2.0/24` (wrong network), `0.0.0.0/0`, `pos kuenta` and `-pos.lan` each keep the wizard on the page with the D2 message, and the stub log shows no `/api/edge/enroll-with-code` call.
  - **Blank name:** an IP-only box, whose finish page shows only `https://<ip>`, and whose served certificate's SAN lists only the IP.
  - **Silent with a name:** a silent install with `KUENTA_SETUP_LAN_HOST=caja.kuenta.lan` gives a `box.env` with `EDGE_LAN_HOST=caja.kuenta.lan`, and `https://caja.kuenta.lan` loads, using a hosts-file entry on the test PC.
  - **Finish page:** shows `https://pos.kuenta.lan`, `https://<ip>` and `pos.kuenta.lan -> <ip>`.
  - **Repair:** re-running the installer shows neither the checkbox nor the network page.

  Record the results in the PR description.

## 5. Docs and dropping Technitium (parallel-safe with §1-§4)

- [x] 5.1 Delete `edge/scripts/setup-technitium-dns.ps1`, `edge/scripts/dev-test-here.ps1`, `edge/docs/DNS_SETUP_GUIDE.md` and `edge/docs/DNS_RESTORE_GUIDE.md` (design.md D11). Verify: `grep -rn "setup-technitium\|dev-test-here\|DNS_SETUP_GUIDE\|DNS_RESTORE_GUIDE" edge .github --exclude-dir=ai-plan` prints nothing.
- [x] 5.2 Update `edge/README.md` (diagram line `:14`, comment block `:74-75`), `edge/docs/DEPLOYMENT_RUNBOOK.md` (`:135-141`: the new `write-box-config.ps1` parameters), `edge/provisioning/README.md`, `edge/certs/README.md` and `edge/docs/LAN_SERVING_WALKTHROUGH.md` (`:67`, `:264-265`, `:280`, `:313-324` (the DNS section to replace), `:349`, `:369-371`) per design.md §6:
  - `pos.laguna.lan` → `pos.kuenta.lan`.
  - Replace the router/Technitium alternatives with the gateway paragraph (fixed IP for the box, plus the local DNS record `pos.kuenta.lan -> <ip>`).
  - Mention the installer's "Configurar la red manualmente (avanzado)" step in `edge/README.md`.

  Verify, after 2.4 and 3.3 have landed: `grep -rn "pos.laguna.lan\|Technitium" edge --exclude-dir=ai-plan --exclude-dir=artifacts` prints nothing.

## 6. Security review

- [ ] 6.1 Run the security-audit skill in guidance mode (a focused review, not the full audit) over this change's diff, against design.md "Security". Check that every writer of `EDGE_LAN_CIDR` goes through `Test-LanIdentity`: the wizard, silent mode, `write-box-config.ps1`, `install.ps1` and `set-lan-ip.ps1`. Also check that the D5 allowlist runs before every `RunPS` that carries typed values. Fix the confirmed findings, and record the needs-validation results in design.md Security: UniFi record and fixed IP, tablet DNS, and the CI runner's private address from the first edge-ci log. Verify: the review notes are in the PR, and `openspec validate edge-lan-hostname` passes.

## Workflow follow-up

- Run the spec-readiness skill on this change and fix every blocking question before implementing.
- After merge, release `edge/vX.Y.Z` and reinstall the owner's restaurant per design.md Migration Plan.
- Archive the change.
