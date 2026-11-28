# Design

## Context

See proposal.md for the motivation. Today:

- `edge/installer/kuenta.iss:850-860` (`CurUninstallStepChanged`): at `usUninstall` it runs `{app}\scripts\uninstall.ps1` hidden. At `usDone` it shows "Kuenta se desinstaló. Los datos del restaurante … se conservaron en C:\kuentapos."
- `kuenta.iss:72-89` (`[UninstallDelete]`) removes only the program folders (`scripts`, `services`, `installer`, `licenses`, the program parts of `artifacts`) and the shortcuts. `env\`, `artifacts\postgres\data`, `backups\`, `caddy-data\`, `state\` and `logs\` stay.
- `edge/scripts/uninstall.ps1` removes the updater task, the five services and the firewall rule. It leaves the machine variables and the trusted root certificate.
- What the appliance puts on the PC beyond `C:\kuentapos`:
  - machine variables `EDGE_LAN_IP`, `EDGE_LAN_CIDR`, `EDGE_SITE_ADDR`, `EDGE_NODE_KEY_FILE` and `EDGE_TELEMETRY_BUFFER_DIR` (the only `SetEnvironmentVariable(…, 'Machine')` calls in `edge/scripts/*.ps1`)
  - Caddy's root CA in `Cert:\LocalMachine\Root` (`install.ps1:354`), exported to `C:\kuentapos\kuentapos-root-ca.crt` (`install.ps1:348-352`)
  - the Inno uninstall entry, which Inno removes itself
- Every ACL the scripts set keeps Administrators with full control (`lock-root.ps1`, `install.ps1:122-222`, `write-box-config.ps1:90`), so the elevated uninstaller can delete the whole folder.
- `DetectMode` (`kuenta.iss:269-282`) returns `fresh` when `env\edge-node.env` has no `NODE_ID` and `artifacts\postgres\data` doesn't exist. With the folder gone, the next install is fresh.

## Goals / Non-Goals

**Goals:**
- One question, keep is the default, and the whole removal happens inside the normal uninstall.

**Non-Goals:**
- Wiping anything outside the list in Context.
- Releasing the box in the cloud.

## Decisions

### D1. Ask at `usUninstall`, before anything is removed

A global `Purge: Boolean` is added to the `[Code]` `var` block (`kuenta.iss:98-116`). At the start of the `usUninstall` branch:

```
Purge := HasSwitch('PURGE');
if not Purge and not UninstallSilent then
  Purge := SuppressibleMsgBox(PurgeQuestion, mbConfirmation, MB_YESNO or MB_DEFBUTTON2, IDNO) = IDYES;
Log('purge ' + IntToStr(Ord(Purge)));
```

`PurgeQuestion` is:

```
¿Desea borrar también los datos de esta caja (base de datos, configuración, copias de seguridad y certificados)?

Lo que la caja no haya sincronizado con Kuenta se perderá. Si responde No, podrá reinstalar Kuenta y la caja seguirá igual.
```

(`#13#10#13#10` between the two paragraphs.) `MB_DEFBUTTON2` makes No the default button, and `/SUPPRESSMSGBOXES` returns `IDNO`.

*Alternative: ask in `InitializeUninstall`.* Rejected: that runs before Inno's own "¿Desea eliminar Kuenta?" confirmation, so an owner who then cancels would have answered a data question for nothing.

### D2. `uninstall.ps1 -Purge` cleans what lives outside the folder

`uninstall.ps1` gains `param([switch]$Purge)`. After the firewall rule (`uninstall.ps1:41-42`), when `$Purge`:

1. **Root certificate, by thumbprint only.** For each of `$Root\kuentapos-root-ca.crt` and `$Root\caddy-data\pki\authorities\local\root.crt` that exists, read it with `New-Object Security.Cryptography.X509Certificates.X509Certificate2($path)`. Then remove `Cert:\LocalMachine\Root\<Thumbprint>` if present. Never match by subject: a developer PC can trust other Caddy CAs with the same subject.
2. **Machine variables.** Set each of `EDGE_LAN_IP`, `EDGE_LAN_CIDR`, `EDGE_SITE_ADDR`, `EDGE_NODE_KEY_FILE` and `EDGE_TELEMETRY_BUFFER_DIR` to `$null` at Machine scope.
3. Print `Purged: root certificate and machine variables removed.` The folder itself is not deleted here (D3).

The last line changes to print `Uninstalled.` plus, without `-Purge`, the existing "left intact" note. The header comment says `-Purge` also removes the root certificate and the machine variables, and that the installer's uninstaller then deletes the folder.

`kuenta.iss` passes `-Purge` when `Purge` is set: the `usUninstall` `Exec` argument string becomes `… -File "<…>\uninstall.ps1"` followed by `' -Purge'` when `Purge`.

### D3. The folder is deleted at `usPostUninstall`

A new `usPostUninstall` branch, when `Purge`: `DelTree(Root, True, True, True)`, then `Log('purge folder removed: ' + IntToStr(Ord(not DirExists(Root))))`. At this point Inno has removed its own files and the services are gone, so nothing holds files open. The uninstaller runs from a temporary copy, and Inno deletes `unins000.exe` and `unins000.dat` itself, so `DelTree` may leave those two for Inno to remove.

*Alternative: delete the folder in `uninstall.ps1`.* Rejected: that runs at `usUninstall`, before Inno removes its own files and while it still reads `unins000.dat`.

### D4. Last screen

The `usDone` message depends on what happened:

| Case | Message |
|---|---|
| kept (`not Purge`) | unchanged: `Kuenta se desinstaló. Los datos del restaurante (base de datos, configuración, copias de seguridad y certificados) se conservaron en C:\kuentapos.` |
| removed (`Purge`, and neither `Root\env` nor `Root\artifacts` exists) | `Kuenta se desinstaló y se borraron los datos de esta caja. La próxima instalación empezará desde cero.` |
| partly removed (`Purge`, and `Root\env` or `Root\artifacts` still exists) | `Kuenta se desinstaló, pero no se pudo borrar todo en C:\kuentapos. Reinicie el equipo y borre esa carpeta antes de instalar de nuevo.` |

Inno may still be removing `unins000.*` at `usDone`, so the root folder itself is not the test.

### D5. The installer test proves the removal and the fresh reinstall

In `edge/installer/test/run-installer-tests.ps1`, two steps follow "Reinstall: repair with the data intact" (`:447-455`), before the closing `}` of the `try` (`:457`):

- **`Uninstall removing everything (/PURGE)`:**
  1. Read the thumbprint of `$root\kuentapos-root-ca.crt`, and assert `Cert:\LocalMachine\Root\<tp>` exists.
  2. Create a decoy: `New-SelfSignedCertificate -Subject 'CN=Caddy Local Authority - CI decoy' -CertStoreLocation Cert:\LocalMachine\My`. Export it with `Export-Certificate` to `$Work\decoy.cer`, `Import-Certificate` it into `Cert:\LocalMachine\Root`, and remove the `My` copy.
  3. Run `unins*.exe` with `/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /PURGE`, using `-Wait`.
  4. Wait up to 30 s for `Test-Path $root` to be false.
  5. Assert:
     - `$root` is gone
     - `Cert:\LocalMachine\Root\<tp>` is gone, and the decoy is still in `Root`
     - each of the five machine variables is `$null`
     - no `kuentapos-*` service, no `Kuenta Updater` task, and no `Kuenta - Caddy (LAN tablets)` rule remain
     - neither uninstall key under `HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\{6A1E8C52-3D0B-4B7E-9F4C-5C1D2E7A9B10}_is1` (also under `WOW6432Node`) exists
  6. `$decoyTp = $null` is declared before the `try` (`:275`), so the `finally` (`:458`) can run `if ($decoyTp) { Remove-Item "Cert:\LocalMachine\Root\$decoyTp" -ErrorAction SilentlyContinue }`. Step 2 sets `$decoyTp` to the decoy's thumbprint.
- **`Fresh install after removing everything`:**
  1. `$r = Install $ok 'after-purge' @('/REPLACE')`. The `$ok` stub answers 409 to every enrollment after its first one unless the call sends `"replace": true` (`stub-cloud.ps1:6-8,64`). That is the cloud still holding the old box, so `/REPLACE` is the silent form of the owner's "Sí".
  2. Assert:
     - `$r.code` is 0
     - `(Get-Content $r.log) -match 'mode fresh'` is non-empty (the line comes from `Log('mode ' + Mode)` in `InitializeSetup`, `kuenta.iss:507`)
     - `Enroll-Calls $ok` is 3: the fresh install's 1, plus this install's 409 and its replace (the count is cumulative over the stub's log, `:85-88`)
  3. Run `& "$root\scripts\verify-install.ps1" -TimeoutSeconds 120` and assert `$LASTEXITCODE` is 0.
  4. Assert the `ci_marker` table is gone. Read the new password as `:419-420` do (`$dbPw = ((Get-Content "$root\env\edge-node.env" | Where-Object { $_ -match '^DB_PASSWORD=' }) -split '=', 2)[1]`, then `$env:PGPASSWORD = $dbPw`). Run the `psql.exe` call from `:453` with `-tAc "SELECT to_regclass('ci_marker')"`, assert its trimmed output is empty, then remove `PGPASSWORD` as `:454` does.

  The box stays installed for the CI steps after the test (`-KeepInstalled`).
- The header's `Cases:` list gains "an uninstall with /PURGE removes everything, and the next install is fresh".

## Implementation Map

### §1 Installer test
- **`edge/installer/test/run-installer-tests.ps1`:** the two steps in D5 after `:455`, the decoy cleanup in `finally` (`:458-463`), and the header `Cases:` (`:6-15`).

### §2 uninstall.ps1
- **`edge/scripts/uninstall.ps1`:** the `param([switch]$Purge)` block before `:7`, the D2 block after `:42`, the last line (`:44`) and the header (`:1-6`).

### §3 Uninstaller in kuenta.iss
- **`edge/installer/kuenta.iss`:**
  - the `var` block (`:98-116`): `Purge: Boolean;`
  - `CurUninstallStepChanged` (`:850-860`): D1 at the start of `usUninstall`, the `-Purge` argument, a new `usPostUninstall` branch (D3) and the `usDone` message table (D4)
  - `HasSwitch` (`:136`) is reused as is

### §4 Docs
- **`edge/docs/DEPLOYMENT_RUNBOOK.md:71-73`:** the Uninstall paragraph names the question, `/PURGE`, and that a fresh install afterwards asks to replace the box in the cloud.
- **`edge/README.md:42`:** the `uninstall.ps1` row reads `Remove all services + firewall rule (-Purge: also the root certificate and machine variables)`.

## Security — N/A

No trust boundary: only an elevated administrator runs the uninstaller, and the removal targets files and settings the appliance itself created. The one shared resource, the machine's trusted root store, is changed only by the box's own certificate thumbprint (D2), and the decoy check in D5 covers it.

## Risks / Trade-offs

- **[The owner answers yes by mistake and loses unsynced orders]** → No is the default button, the question names the loss, and a silent uninstall never purges without `/PURGE`.
- **[Inno's uninstaller does not see `/PURGE` in `ParamStr`, since it relaunches itself from a temporary copy]** → Inno passes the original arguments to the second phase. The D5 step is the check: if `/PURGE` is lost, `$root` remains and the step fails in CI.
- **[A file stays locked and the folder survives]** → the services are already removed, D4 names the leftover, and the next install would be refused as "data without configuration" (exit 30) only if `artifacts\postgres\data` survives without `env\`. The message tells the owner to reboot and delete the folder.
- **[The cloud still has the old box]** → accepted. The fresh install asks to replace it (`TryEnroll`, `kuenta.iss:464-501`).

## Migration Plan

None. Existing boxes get the new uninstaller with their next install or update. Rollback is the previous release, whose uninstaller keeps the data.
