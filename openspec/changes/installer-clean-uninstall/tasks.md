# Tasks

Windows PowerShell 5.1 targets. The installer test runs only on Windows CI (`edge-ci.yml`, "Installer test (silent)"). The uninstall question has no automated driver, so it is checked by hand in §5.

## 1. Installer test: removal and fresh reinstall

- [x] 1.1 In `edge/installer/test/run-installer-tests.ps1`, add the two steps from design.md D5 after the `the marker row survived uninstall and reinstall` assert (`:454-455`), before the `try` block's closing `}` (`:457`):
  - `Uninstall removing everything (/PURGE)` with the thumbprint, decoy, uninstall, 30 s wait and every assert listed in D5.
  - `Fresh install after removing everything`: `Install $ok 'after-purge' @('/REPLACE')`, exit 0, `mode fresh` in the log, `Enroll-Calls $ok` equals 3, `verify-install.ps1` passes, and `to_regclass('ci_marker')` is empty, all as D5 states.
  - Declare `$decoyTp = $null` before the `try` (`:275`), and in `finally` (`:458`) remove the decoy from `Cert:\LocalMachine\Root` by `$decoyTp` when it is set.
  - Add the case to the header's `Cases:` list.

  Verify: `pwsh -NoProfile -Command "[ScriptBlock]::Create((Get-Content -Raw edge/installer/test/run-installer-tests.ps1)) | Out-Null"` parses without error. On edge-ci the `/PURGE` step fails (`$root` remains) until §3 lands.

## 2. uninstall.ps1 -Purge

- [x] 2.1 Edit `edge/scripts/uninstall.ps1` per design.md D2: the `param([switch]$Purge)` block, the root certificate removal by thumbprint (both files), the five machine variables, the `Purged:` line, the last line and the header. Verify: the script parses (the 1.1 command, on this file), and `grep -n "Subject" edge/scripts/uninstall.ps1` prints nothing (no match by subject).

## 3. Uninstaller in kuenta.iss

- [x] 3.1 In `edge/installer/kuenta.iss`, per design.md D1, D3 and D4:
  - add `Purge: Boolean;` to the `var` block (`:98-116`)
  - in `CurUninstallStepChanged` (`:850-860`): the D1 question and `/PURGE` at the start of `usUninstall`, ` -Purge` appended to the `uninstall.ps1` arguments when `Purge`, a `usPostUninstall` branch with `DelTree(Root, True, True, True)` and its log line, and the D4 `usDone` messages
  - every `Exec` keeps `SW_HIDE`

  Verify: edge-ci "Installer test (silent)" passes, including both 1.1 steps, and its `No console: static check on kuenta.iss` step reports no problems.

## 4. Docs

- [x] 4.1 Update `edge/docs/DEPLOYMENT_RUNBOOK.md:71-73` and `edge/README.md:42` per design.md §4. Verify: `grep -n "PURGE\|Purge" edge/docs/DEPLOYMENT_RUNBOOK.md edge/README.md` shows both.

## 5. Check by hand

- [ ] 5.1 On a Windows PC with the new release installed, uninstall from Settings → Apps → Kuenta:
  - **Default:** the question appears with No preselected, Enter keeps the data, and the last screen says the data was kept in `C:\kuentapos`.
  - **Yes:** reinstall, uninstall again and answer Sí. The last screen says the data was removed, `C:\kuentapos` is gone, and Kuenta is no longer in Apps.
  - **Fresh again:** run the installer. The welcome screen shows "Configurar la red manualmente (avanzado)", and the code screen asks to replace the existing box.

  Record the results in the PR description.

## Workflow follow-up

- Run the spec-readiness skill on this change and fix every blocking question before implementing.
- Implement after `installer-busy-page` merges (both edit `kuenta.iss` and `run-installer-tests.ps1`), or before it, but not in parallel.
- Release in `edge/v1.3.2`, then archive the change.
