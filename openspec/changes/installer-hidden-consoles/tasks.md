## 1. Hidden script runs

- [ ] 1.1 Rewrite `RunPS` in `edge/installer/kuenta.iss` on `ExecAndLogOutput` with `SW_HIDE` and `ewWaitUntilTerminated`, starting `powershell.exe` directly (no `cmd.exe`, no `WScript.Shell`):
  - an `OnLog` callback that appends stdout to `RunOutput` and stderr to `RunErr`
  - when `RunLive` is set, the callback logs `progress …` and sets `WizardForm.StatusLabel`
  - the globals are reset at the start of each run
  - the first 2000 characters of stderr are logged on a non-zero exit
  - the `{tmp}\<script>.err` file is dropped
  - Verify the installer compiles with Inno 6.4.3 and installer test groups 1–3 pass
- [ ] 1.2 Pass `Input` through `KUENTA_SCRIPT_INPUT`: set it with `SetEnvironmentVariableW` only when `Input <> ''`, and clear it in a `try … finally` around the call. Verify with group 3 (enroll and write-box-config succeed)
- [ ] 1.3 In `enroll.ps1` and `write-box-config.ps1`, read `$env:KUENTA_SCRIPT_INPUT` first, then `Remove-Item Env:\KUENTA_SCRIPT_INPUT`, and fall back to `[Console]::In.ReadLine()` when it is empty. Verify that the test's `Run-Enroll` (stdin) still passes
- [ ] 1.4 Confirm that every other `Exec` in `kuenta.iss` (repair stop, VC++ runtime, uninstall) passes `SW_HIDE`. Update the stdin line in `edge/docs/DEPLOYMENT_RUNBOOK.md` to mention the variable. Verify by reading the diff

## 2. Tests

- [ ] 2.1 Add a static check to `run-installer-tests.ps1` (or to edge-ci's lint step): `kuenta.iss` contains no `WScript.Shell`, and every `Exec*` call passes `SW_HIDE`. Verify that it fails on a copy that still has `WScript.Shell`
- [ ] 2.2 Add a process sampler around the fresh install and the repair: every 200 ms it collects setup's descendants, asserts that no `cmd.exe` appears, and uses `EnumWindows`/`IsWindowVisible` to assert that no descendant window is visible. Log "no desktop: visibility not checked" when the runner has none. Verify in edge-ci
- [ ] 2.3 Negative test, secrets. Extend "no file or log holds the enrollment code" to the stub cloud's node key, and assert that neither secret appears in any sampled descendant command line. Verify in edge-ci
- [ ] 2.4 Negative test, input lifetime. After the fresh install, assert that the `lock-root`, `setup-box` and service processes sampled during the run never had `KUENTA_SCRIPT_INPUT` set, by checking their environment through the sampler while the processes are alive. Verify in edge-ci
- [ ] 2.5 Negative test, non-admin reader. While a test script that sleeps runs elevated, use the existing `As-User` helper to try to read its environment and command line as the standard user, and assert that access is denied. Verify in edge-ci
- [ ] 2.6 Add "run the wizard on a real Windows PC with a desktop: no console window appears at any step" to the release checklist in `edge/docs/DEPLOYMENT_RUNBOOK.md`

## 3. Security review

- [ ] 3.1 Run the security-audit skill in guidance mode over this change's diff against design.md's Security section: the input lifetime, the `OnLog` logging path, inherited environment and the command lines. Fix confirmed findings, and record the two needs-validation results (no output and no environment in the 6.4.3 setup log) in design.md with what was checked
