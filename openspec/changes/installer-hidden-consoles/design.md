## Context

`RunPS` in `edge/installer/kuenta.iss` (around line 207) runs every installer script as
`WScript.Shell.Exec('cmd.exe /d /s /c ""powershell" … -File "<script>" <args> 2>"{tmp}\<script>.err""')`.
It writes `Input` to the child's stdin, reads stdout line by line (and, when `Live`, sets `WizardForm.StatusLabel` from `{"step","status":"start"}` lines), redirects stderr to a file so no undrained pipe can hang the child, and logs the stderr tail on failure.

`WScript.Shell.Exec` has no window-style parameter: every call opens a visible `cmd.exe` console. The calls are:
- `preflight.ps1` on the first page, where the owner sees the first console
- `enroll.ps1`, with the code on stdin
- `lock-root.ps1`
- `write-box-config.ps1`, with the enrollment result (node key) on stdin
- `setup-box.ps1`, with `Live` progress, which is the long console at the end

The plain `Exec` calls (stopping services on repair, VC++ runtime, uninstall) already pass `SW_HIDE` and show nothing.

CI builds with Inno Setup 6.4.3 (`edge/installer/build-installer.ps1`, `$InnoVersion`).

## Goals / Non-Goals

**Goals:**
- No console window at any point of install, repair or uninstall.
- Keep live progress, exit codes, the stderr tail in the log, and no hang.
- Keep the code and the node key off the command line, off disk and out of the log.

**Non-Goals:**
- Changing what the scripts do, their exit codes or messages.
- Hiding the "Abrir Kuenta" launch at the end. That is intended.
- A progress bar or new wizard pages.

## Decisions

### D1. Run scripts with Inno's `ExecAndLogOutput`, hidden

Inno Setup 6.3.0 added `ExecAndLogOutput(Filename, Params, WorkingDir, ShowCmd, Wait, var ResultCode, OnLog)`. It starts the process directly, without `cmd.exe`, and with `SW_HIDE` the new console is never shown. It drains stdout and stderr itself and calls `OnLog(S, Error, FirstLine)` for each line while the wizard keeps pumping messages, so the status label can change during the run.

The new `RunPS`:
- starts `powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "<script>" <args>` with `SW_HIDE`, `ewWaitUntilTerminated`
- always passes an `OnLog` callback. With a nil callback Inno writes every output line to the setup log, and `enroll.ps1` prints the node key on stdout, so a nil callback would leak it.
- In the callback, stdout lines are appended to a global `RunOutput`. When `RunLive` is set, they are logged as `progress …` and drive the status label, as today. Stderr lines are appended to a global `RunErr`, and its first 2000 characters are logged when the exit code is non-zero.
- `Live` is never set for `enroll.ps1` or `write-box-config.ps1`, as today.

The `{tmp}\<script>.err` file goes away: it existed only to avoid an undrained stderr pipe, and Inno now drains both pipes.

*Alternatives considered:*
- `ExecAndCaptureOutput`: also hidden, but it returns the output only after the process exits, so there is no live progress for setup-box. Rejected.
- Importing `CreateProcessW`, `CreatePipe`, `ReadFile` and `PeekNamedPipe` from kernel32 into Pascal Script with `CREATE_NO_WINDOW`: this would keep stdin. It needs `STARTUPINFO` and `PROCESS_INFORMATION` records and handle inheritance managed by hand in Pascal Script, which is fragile and hard to test. Rejected in favor of D2.
- `powershell -WindowStyle Hidden` under `WScript.Shell.Exec`: the `cmd.exe` console still opens, and PowerShell's own console flashes before it hides. Rejected.
- `WScript.Shell.Run(cmd, 0)`: hidden, but it has no pipes, so the output would have to go through files. Rejected.
- A small launcher exe shipped with the installer: it adds an unsigned binary to the supply chain. Rejected.

### D2. Secret input through a single-use environment variable

`ExecAndLogOutput` gives the child no stdin. Instead, the installer passes the same JSON line it writes to stdin today through the variable `KUENTA_SCRIPT_INPUT`:
1. Right before the call, `SetEnvironmentVariableW('KUENTA_SCRIPT_INPUT', Input)` in setup's own process. The import already exists in `kuenta.iss` for `ClearInheritedModulePath`.
2. The child inherits it at creation.
3. In a `try … finally` around the call, `SetEnvironmentVariableW('KUENTA_SCRIPT_INPUT', nil)` runs right after the call, so later children (lock-root, setup-box, services, the uninstall) never inherit it. The call runs on the wizard's thread, so no other child starts in between.
4. In `enroll.ps1` and `write-box-config.ps1`, the first statement reads `$env:KUENTA_SCRIPT_INPUT` and then runs `Remove-Item Env:\KUENTA_SCRIPT_INPUT`. It falls back to `[Console]::In.ReadLine()` when the variable is empty, so the runbook's stdin piping and CI's `Run-Enroll` keep working.

The variable is set only when `Input <> ''`, and never for other scripts.

*Decided (owner, on PR #85):* the single-use environment variable replaces stdin for the installer's runs, accepting that setup's elevated process holds the input for the length of one script run.

### D3. Tests

In `run-installer-tests.ps1`:
- **No `cmd.exe` and no console host shown.** During the fresh install and the repair, a background job samples `Get-Descendants $p.Id` every 200 ms.
  - It asserts that no `cmd.exe` appears.
  - For every descendant pid, it calls `EnumWindows`/`IsWindowVisible` (through `Add-Type`, as the test already does for `LogonUser`) and asserts that none of their windows is visible.
  - If the runner has no interactive desktop, the visibility half cannot fail meaningfully. The test then logs "no desktop: visibility not checked", and the `cmd.exe` check plus a static check stand.
- **Static check.** `kuenta.iss` contains no `WScript.Shell`, and every `Exec`/`ExecAndLogOutput`/`ExecAndCaptureOutput` call passes `SW_HIDE`.
- **No secret leaks.** The existing "no file or log holds the enrollment code" assert is extended to the node key the stub cloud returns. The sampler also records descendants' command lines and asserts that neither secret appears in them.
- **Manual check.** A manual check on a real Windows PC with a desktop is part of the release checklist: run the wizard and see no console.

## Security

**Boundary: other local processes and users → the box's secrets while the installer passes them to its scripts.**
- *Lower-trust principal:* a signed-in non-administrator on the PC, or any non-elevated process, while the installer runs.
- *Input or action it controls:* it can list processes and read command lines, environment blocks and files that its token can open, and it can read the setup log if the log's ACL allows.
- *Intended control:*
  - The code and the node key never appear in arguments or files. `RunPS` sets `KUENTA_SCRIPT_INPUT` only around one call and clears it in `finally` (`kuenta.iss`, `RunPS`).
  - The scripts drop the variable before starting anything (`enroll.ps1` and `write-box-config.ps1`, first statement).
  - The `OnLog` callback never logs non-`Live` output, and `Live` is never set for those two scripts.
  - The child processes run elevated, so reading their environment needs `PROCESS_VM_READ` on a high-integrity process, which a non-administrator does not get.
- *Protected resource:* the enrollment code (single-use, 30 minutes) and the node key, which lets its holder act as the box toward the cloud for the organization.
- *If the control fails:* a non-admin who reads the node key can impersonate the box to the cloud. A leaked code can enroll a rogue box until it is used or expires.
- *Attack classes considered* (`DESKTOP-MOBILE-AND-LOCAL-IPC.md`):
  - credential-store and local-secret boundary mismatch: the log, `{tmp}` and the process command lines
  - IPC lifecycle: inherited environment reaching grandchildren and later children
  - install path trust: scripts run from `{tmp}` and from the locked-down root, unchanged by this change

**Validated in CI** (edge-ci run 46087334874, Inno Setup 6.4.3, fresh install on a Windows runner; the installer compiled and ran every hidden script):
- Output is not echoed into the setup log when a callback is set: the fresh-install setup log has 0 matches for `"node_key"`, `"node_id"`, `lgn_stub` and the enrollment code, although `enroll.ps1` printed all of them on stdout (the log shows only `run ...enroll.ps1` and `exit 0`).
- Inno does not log the child's environment: 0 matches for `KUENTA_SCRIPT_INPUT` in the setup log.
- Sampling setup's process tree every 200 ms during the fresh install showed: no command line with the code or the node key; `KUENTA_SCRIPT_INPUT` only in setup itself, the two scripts that read it and the console host created for them (`conhost.exe`, made while setup still holds the variable); the box's own processes (postgres, edge-node, next, node, caddy) never had it; a standard user is denied (error 5) when it opens an elevated process to read its environment, while the elevated test user can read the canary.
- The only `cmd.exe` processes are Postgres's own (`pg_ctl` and `initdb` start `postgres.exe` through `cmd /c`); none has setup as its parent.

**Security review (guidance mode over the diff):**
- Fixed: `RunPS` now clears `KUENTA_SCRIPT_INPUT` before every run, so a value inherited from whoever started setup never reaches a script that does not expect it.
- Accepted: the `conhost.exe` that Windows creates for `enroll.ps1` and `write-box-config.ps1` holds the variable for a moment. It is a system process in the same elevated boundary.
- Not part of this change, found by the new tests: `artifacts\edge-node\.env` (a copy of `env\edge-node.env` staged by `install.ps1`) holds the node key outside `env\`, and after the fresh install a standard user can read Caddy's `caddy-data\pki\authorities\local\root.key`. Both are in the `edge-kuentapos-naming` lock-down; the test allows the first as a known copy and fails on the second.

## Risks / Trade-offs

- [An environment variable is a weaker channel than a pipe: the setup process holds it for the length of one script run] → setup runs elevated and the window is seconds long. It is cleared in `finally`, and the script removes its own copy first.
- [A future script that starts a child before reading the variable would pass it on] → the variable is set only for `enroll.ps1` and `write-box-config.ps1`, and the D3 command-line and secret asserts cover the install.
- [`OnLog` is a global callback, so `RunPS` keeps per-run state in globals] → runs are sequential on the wizard thread. `RunPS` resets the globals at the start of each run.
- [The runner may have no desktop, so the window check is weaker in CI] → the static check plus the `cmd.exe` check catch the known cause. The release checklist adds the manual check.

## Migration Plan

Ships in the next `edge/v*` installer. There is no data or box state to migrate. Rollback: an earlier installer shows the consoles again.

## Open Questions

(none)
