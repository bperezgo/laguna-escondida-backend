# Verification: edge-lan-hostname

Commit `cf685c7` · base `origin/main` · 2026-11-28

## Verdict
Needs work: the network rules in `lan-config.ps1` are proven by a unit test that passes. But no Windows check has run: the branch is not pushed, so edge-ci never ran, ISCC never compiled the wizard here, and the manual wizard pass (4.5) is open. Tasks 3.1, 3.2, 4.3 and 4.4 are checked although their verify steps could not have run yet. Scenarios 3 proven, 13 weak, 6 missing. Tasks 16/18 checked, 0 false claims (7 unverified). Checks: all green that could run here (unit test, parse, generator, greps, `openspec validate`); edge-ci, ISCC and the manual wizard pass not run.

## Read these yourself
1. `edge/installer/kuenta.iss` (`:593-640` NextButtonClick, `:697-705` silent check, `:749` write-box-config args, `:775-805` FinishAddresses, `:563-584` welcome checkbox): about 270 new lines of Pascal with no compile or run in evidence. Check the following. (a) The welcome-page `CheckNetwork('', '', '')` at `:611` runs even when the box is ticked, so a PC whose default-route address fails D4 (a public or CGNAT IP, no default route) can never reach the network page to pick another address. That follows the design (§4, D12), but it defeats the purpose of the advanced step in exactly the case that needs it. (b) `WelcomeLabel2.AutoSize := True`: does the checkbox overlap the text at high DPI? (c) Values going stale on Back/Next.
2. `edge/scripts/lan-config.ps1`: the single security control for the firewall CIDR (D4). `Test-InsidePrivateRange` (`:55-62`) and the order of the checks in `Test-LanIdentity` (`:67-115`). The unit test and my probes back them, but only on pwsh 7.4/Linux, not on the Windows PowerShell 5.1 the box runs. The pure path is never tested with `-cidrByIp` (the D3 adapter-prefix default), so the "Detected network" default has no test.
3. `edge/scripts/install.ps1:77-86` and `edge/scripts/write-box-config.ps1:25`: dot-sourcing `lan-config.ps1` (which has a `param()` block) rebinds `$LanIp/$LanCidr/$LanHost/$List/$Check/$NoRun` in the caller's scope. The author passes the caller's values back through to work around this. Confirm that no later line depends on a variable the dot-source overwrites. `set-lan-ip.ps1:42` dot-sources without passthrough. Its own variables are `$IP`/`$Cidr`, so nothing collides today, but this is fragile.
4. `edge/scripts/set-lan-ip.ps1:130-163`: the derive branch, the check before any write, the normalized `$Cidr` write, and `EDGE_SITE_ADDR`. None of it is tested. The only verify step is a grep plus a manual run on the owner's box.
5. `edge/installer/test/run-installer-tests.ps1:328-335, 356-367, 423`: the new asserts are the only automated proof of the installed box, and none of them has run yet.

## Scenarios → tests
| Scenario | Rating | Test | Assertion on THEN |
|---|---|---|---|
| edge-installer: Advanced option not ticked | weak | `run-installer-tests.ps1:358,360` (silent fresh install) | `(BoxEnv 'EDGE_LAN_HOST') -eq 'pos.kuenta.lan'`, `$siteAddr -eq "pos.kuenta.lan, $lanIp"`. Silent only: no wizard page-skip test, and the IP is never compared with the detected default-route IP |
| edge-installer: Owner picks another address | weak | `test-lan-config.ps1:80` | `Check 'listed network' $addrs[1].cidr '192.168.1.0/24'`. This proves the `-List` data. The combo `OnChange` and the served IP/CIDR are untested (manual 4.5, open) |
| edge-installer: Only addresses the PC holds can be chosen | weak | `test-lan-config.ps1:77` | `... -join ',') '10.0.0.7,192.168.1.50'` proves the filtering. "Does not accept typed text" (`csDropDownList`, `kuenta.iss:541`) has no test |
| edge-installer: Network that does not contain the IP | weak | `test-lan-config.ps1:34` | `CheckLike ... '*no está dentro de la red*'`. The rule is proven. "Stays on the screen, code not sent" has no test |
| edge-installer: Network outside the private ranges | weak | `test-lan-config.ps1:37` | `"La red $c debe ser una red privada*"` for `0.0.0.0/0` and others. The wizard staying on the page is untested |
| edge-installer: Invalid name | weak | `test-lan-config.ps1:51` | `'El nombre * no es válido.*'` for `pos kuenta`, `-pos.lan`. The wizard behaviour is untested |
| edge-installer: Name left blank | weak | `test-lan-config.ps1:46` | `Check 'none: no name' $r.lan_host ''`. The IP-only serving and the finish text are untested |
| edge-installer: Repair does not show the network screen | weak | `run-installer-tests.ps1:423` | `(Get-FileHash ... box.env).Hash -eq $boxEnvHash` proves that the values don't change. That the option and the screen are not shown is untested |
| edge-installer: Silent install with a name | missing | none | No test sets `KUENTA_SETUP_LAN_HOST` (manual 4.5, open) |
| edge-installer: Silent install with an IP the PC does not hold | proven (not yet run) | `run-installer-tests.ps1:332-335` | `$r.code -ne 0`, `-not (Test-Path $root)`, `Select-String ... '192.0.2.1'`, `(Enroll-Calls $ok) -eq $before`. It uses 192.0.2.1, not 192.168.77.77, and the log check matches the IP, not the "no tiene" wording |
| edge-installer: Final screen names the gateway record | missing | none | `FinishAddresses` (`kuenta.iss:775`) is never exercised |
| edge-installer: Printing is off (pre-existing) | missing | none in `edge/` | Not touched by this change, and no edge test hits `/api/device/print` |
| edge-installer: Values come from enrollment (pre-existing) | weak | `run-installer-tests.ps1:382` | `$status.online -eq $true`. Node N and the sync URL are not compared with the enrollment values |
| edge-installer: Detected network | weak | `run-installer-tests.ps1:358-362` | The host, site address and firewall are checked against whatever `box.env` holds. The derivation of the mask into a CIDR is never compared with the adapter's mask |
| edge-installer: Confirmed network | missing | none | No test passes confirmed IP, CIDR and name through the installer |
| edge-installer: Secrets are not world-readable (pre-existing) | proven | `run-installer-tests.ps1:262,305` | `$got -eq 'denied'`, `$denied.err -eq 5` |
| edge-lan-identity: Name and IP both work | weak | `run-installer-tests.ps1:367` | `$san -match 'pos.kuenta.lan' -and $san -match $lanIp` proves the SAN. No test loads `https://pos.kuenta.lan` without a warning |
| edge-lan-identity: Box without a name | missing | none | Manual 4.5, open |
| edge-lan-identity: The box is not a DNS server | proven (not yet run) | `run-installer-tests.ps1:365` | `-not @($dns \| Where-Object { $boxPids -contains [int] $_.OwningProcess })` |
| edge-lan-identity: Device outside the network | weak | `run-installer-tests.ps1:362` | `$fwRemote.Count -eq 1 -and (ConvertTo-Cidr $fwRemote[0]) -eq $lanCidr` checks the rule's configuration, but no connection from outside the CIDR is attempted |
| edge-lan-identity: Network wider than allowed is rejected | weak | `test-lan-config.ps1:37` | The rejection message is proven. "No firewall rule created" rests only on code order (`install.ps1:83` throws before `:339`), with no test |
| edge-lan-identity: IP changes, name keeps working | missing | none | `set-lan-ip.ps1` has no automated test. The verify is manual on the owner's box |

## Task claims
| Task | Code | Verify | Result |
|---|---|---|---|
| 1.1 | `edge/scripts/test-lan-config.ps1:1-84` | The test fails while `lan-config.ps1` is missing | unverified: one commit holds both files, so the red state can't be seen. Every case listed in 1.1 is present |
| 1.2 | `edge/scripts/lan-config.ps1:1-187` | `pwsh -File edge/scripts/test-lan-config.ps1` | ran, passed (34 `ok`, "all checks passed"). Run with pwsh 7.4 in the `mcr.microsoft.com/powershell:7.4-ubuntu-22.04` container, not Windows PowerShell 5.1 |
| 1.3 | `.github/workflows/edge-ci.yml:77-78` | grep shows the step before "Build the installer" | ran, passed (`:78` < `:83`) |
| 2.1 | `run-installer-tests.ps1:33-56, 356-368, 423` | The script parses | ran, passed (`Parser.ParseFile`, 0 errors). Asserts (a)-(f) are present, and (b) is stricter (equality) |
| 2.2 | `exit-codes.json:73-76`, `exit-codes.inc:22` | The `.inc` has the 37 line, and regenerating changes nothing | ran, passed (regenerated a scratch copy, and the diff is identical) |
| 2.3 | `write-box-config.ps1:14-19, 25, 29, 47-48, 56, 93` | Windows run writes `pos.kuenta.lan`, and `-LanIp 192.0.2.1` exits 37 | unverified: needs Windows (`Get-NetIPAddress`, icacls) |
| 2.4 | `edge/env/box.env.example:27-32` | `grep laguna` prints nothing | ran, passed |
| 3.1 | `install.ps1:77-86` | edge-ci installer test, plus a manual `/8` run | unverified: the branch is not pushed and no edge-ci run exists. Checked before its verify could run |
| 3.2 | `set-lan-ip.ps1:16, 42, 133, 139-145, 158-163` | grep for `EDGE_SITE_ADDR`, plus a manual run on the owner's box | grep ran, passed (`:160` Machine scope). The manual part is unverified, and the task is checked anyway |
| 3.3 | `edge/Caddyfile:11` | `grep laguna` prints nothing | ran, passed |
| 4.1 | `run-installer-tests.ps1:328-335` | The script parses | ran, passed |
| 4.2 | `kuenta.iss:61, 107-114, 332-449, 516-561` | ISCC compiles, and `Test-HiddenLaunches` passes | unverified: no ISCC/Windows here |
| 4.3 | `kuenta.iss:563-584, 586-591, 593-640, 697-705, 749-750, 809-817` | edge-ci installer test passes | unverified: edge-ci never ran. Checked before its verify could run |
| 4.4 | `kuenta.iss:775-805, 826-830` | ISCC compiles, and the 4.5 wizard pass shows the text | unverified: 4.5 is unchecked, so this verify cannot have been done |
| 4.5 | n/a | Manual wizard pass | not checked |
| 5.1 | the four files deleted | grep for the deleted names | ran, passed (no output) |
| 5.2 | `edge/README.md:14,74-81`, `DEPLOYMENT_RUNBOOK.md:145-148`, other docs | `grep "pos.laguna.lan\|Technitium"` | ran, passed (no output) |
| 6.1 | design.md "Review (task 6.1…)" section exists | `openspec validate edge-lan-hostname` | not checked. The review notes are written, and validate ran and passed (one warning: requirement text >500 chars). The needs-validation items are still open |

## Test integrity
none. No test or assertion was deleted, loosened or changed in its expected value, and no skip, mock or golden file was touched. The `Install` helper is unchanged. The deleted files (`setup-technitium-dns.ps1`, `dev-test-here.ps1`, two DNS guides) are tooling and docs, not tests, and D11 justifies them.

## Checks run
- No `backend/` or `frontend/` change, so no Go or pnpm checks.
- `pwsh -NoProfile -File /edge/scripts/test-lan-config.ps1` (container `powershell:7.4-ubuntu-22.04`, `edge/` mounted read-only, no network): passed, "all checks passed".
- PowerShell `Parser.ParseFile` on `run-installer-tests.ps1`, `lan-config.ps1`, `write-box-config.ps1`, `install.ps1`, `set-lan-ip.ps1`, `test-lan-config.ps1` (same container): passed, 0 errors each.
- Extra probes of `Test-LanIdentity` (same container): `172.32.0.0/16` and `172.16.0.0/12` rejected, `172.20.0.0/16` passes, `/33`, `/024` and `"0 /24"` rejected as malformed, the comma-list host rejected, and the `cidrByIp` default from a `/24` adapter works: passed.
- `gen-exit-codes.ps1` on a scratch copy, then diff against `edge/installer/exit-codes.inc`: identical.
- `openspec validate edge-lan-hostname`: valid (1 warning, long requirement text).
- The grep verify steps for 1.3, 2.4, 3.2, 3.3, 5.1 and 5.2: passed.
- Not run: edge-ci (`gh run list --branch feat/edge-lan-hostname` is empty and the branch is not on origin), ISCC compile, the silent installer test, and Windows PowerShell 5.1.
- The working tree has unrelated uncommitted changes (`docs/FACTURACION_ELECTRONICA_DIAN.md`, and untracked `openspec/changes/dian-direct-einvoice/` and `tips/`). None of them is under `edge/`, so they don't affect these results.

## Flow from the code
```
Owner/CI           kuenta.iss                         lan-config.ps1                 write-box-config / install.ps1
  |-- Next (welcome) -> NextButtonClick :593
  |                    DoPreflight (unchanged)
  |                    CheckNetwork('','','') :611 --> -Check :179
  |                                                    Resolve-LanIdentity :155
  |                                                    Get-LanAddresses :128 (route, adapters)
  |                                                    Test-LanIdentity :67 -> JSON
  |                    <- NetIp/NetCidr/NetHost :352-354
  |  [ticked] ShouldSkipPage :590 shows NetworkPage
  |                    CurPageChanged :809 -> ListAddresses :397 --> -List :169
  |-- Next (network) -> :620 ItemIndex<0 refuse
  |                    :628 NetCharsMessage (allowlist)
  |                    :629 CheckNetwork(AddrIp[i], cidr, HostOrNone) --> -Check :179
  |-- Next (sign-in) -> enroll (unchanged; code spent here)
  |  [silent] PrepareToInstall :697 NetCharsMessage(env) + CheckNetwork(env)
  |                    fail -> Log 'network check failed' + ExitMessage(37), before TryEnroll :706
  |                    CurStepChanged :749 -> write-box-config.ps1 -LanIp -LanCidr -LanHost(none)
  |                                                    write-box-config :47 Resolve-LanIdentity (re-check)
  |                                                    :56 box.env EDGE_LAN_IP/CIDR/HOST
  |                    :760 setup-box.ps1 -> install.ps1 :78-83 Resolve-LanIdentity(box.env, first host)
  |                                                    :86 Get-SiteAddr (full list) -> :91 EDGE_SITE_ADDR (Machine)
  |                                                    :339 firewall RemoteAddress = EDGE_LAN_CIDR
  |                                                    Caddyfile {$EDGE_SITE_ADDR} tls internal
  |                    wpFinished :826-830 FinishAddresses :775 (repair: reads box.env)
```

Differs from design.md:
- D9: the finish text adds `(registro <host> -> <ip>)` to the "En el router…" line. The spec scenario "Final screen names the gateway record" justifies it.
- D5/D6: silent mode runs the allowlist on the IP as well (`NetCharsMessage`, `kuenta.iss:697`), which is broader than D5. The design's Security review already records this.
- Added, not in design: `NetArg` (`kuenta.iss:338-343`) leaves out blank arguments because Windows PowerShell drops `""` under `-File`. Blank values then take their defaults inside the script, which matches D6.
- Added, not in design: `install.ps1:78` and `write-box-config.ps1:25` pass their own `-LanIp/-LanCidr/-LanHost` into the dot-source. D12 says to dot-source with `-NoRun` only, which would blank the caller's variables, so the deviation is needed.
- Followed but risky: D12 asks for the welcome-page default check on every Next, ticked or not. A PC whose default address fails D4 can't open the network page at all.

## Out of plan
- Files no task covers: none. `edge/docs/README.md` (the dropped table row) and `edge/docs/DEPLOYMENT_RUNBOOK.md` fall under 5.1/5.2.
- Tasks with no code: 4.5 (manual wizard pass) and 6.1 (unchecked, though its review notes are already in design.md).
- Open questions in design.md Security "Needs validation", all still open:
  - the UniFi record and fixed IP
  - the tablets' DNS (no Private DNS)
  - the CI runner's private address: the `Network check:` line from the first edge-ci run, which hasn't happened
- Workflow follow-ups still open: the edge release and the owner's reinstall (Migration Plan).
