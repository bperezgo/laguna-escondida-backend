# Tasks

Windows-only behaviour. CI (`edge-ci.yml`, "Installer test (silent)") runs the static check and a silent install; the working page itself has no automated driver, so it is checked by hand in §3. Build this branch after `installer-clean-uninstall` (PR #105): both edit `kuenta.iss` and `run-installer-tests.ps1`.

## 1. Static check (tests first)

- [x] 1.1 In `edge/installer/test/run-installer-tests.ps1`, add `function Test-BusyGuards([string] $text)` after `Test-HiddenLaunches` (`:57-68`), with the four rules and problem texts of design.md D7. In the step "No console: static check on kuenta.iss" (`:292-297`), add `Assert (-not @(Test-BusyGuards $iss))` with the problems in the message, plus a mutation check: `Test-BusyGuards ($iss -replace 'if Busy then', 'if False then')` returns exactly 1 problem. Add "and the wizard refuses presses while a check runs" to the header's `No console:` sentence. Verify: the script parses (`[System.Management.Automation.Language.Parser]::ParseFile` gives 0 errors), and `Test-BusyGuards` run on the current `edge/installer/kuenta.iss` returns all 4 problems. The new assert is meant to stay red until 2.2 lands; don't change it to pass earlier.

## 2. Wizard

- [x] 2.1 In `edge/installer/kuenta.iss`, add the D1 globals, and the D2 `SetTimer`/`KillTimer` declarations, `BusyTick`, `BeginBusy` and `EndBusy` after `RunPS`. Add the `Animate` line to `RunLog`, and create `BusyPage` at the end of `InitializeWizard`. Verify: `Test-HiddenLaunches` on the file reports no problems, and ISCC compiles `edge/installer/kuenta.iss` (edge-ci's "Build the installer" step, or `edge\installer\build-installer.ps1` on Windows).
- [x] 2.2 In `edge/installer/kuenta.iss`, apply design.md D3 to D6:
  - the guard at the top of `NextButtonClick`
  - the new `BackButtonClick`
  - the `Busy` line in `CancelButtonClick`
  - the welcome rewrite with `NetMsg`
  - the network-page wrap
  - the `DoEnroll` wrap
  - removing the `MsgBox` from `ListAddresses` and the network-page block from `CurPageChanged`

  Verify: `Test-BusyGuards` and `Test-HiddenLaunches` on the file return no problems. edge-ci "Installer test (silent)" passes, which compiles the wizard with ISCC and runs every silent case unchanged.

## 3. Check by hand

- [ ] 3.1 On a Windows PC, build with `edge\installer\build-installer.ps1 -Version v1.3.1 -CloudUrl http://127.0.0.1:18080`, start `edge\installer\test\stub-cloud.ps1 -Port 18080 -LogFile C:\stub.log -StopFile C:\stub.stop` (add `-Conflict` for the 409 case), and run the wizard. Each enrollment call is one JSON line with `"path":"/api/edge/enroll-with-code"` in `C:\stub.log`:
  - **Welcome:** Next shows "Un momento" with `Revisando el equipo…`, then `Revisando la red…`, and the bar moves.
  - **Ticked:** with the manual network option ticked, the network page opens already filled, with no extra wait.
  - **Network page:** a valid value shows `Revisando la red…`. An invalid one returns to the page with the same message as before.
  - **Sign-in:** a valid code shows `Conectando con Kuenta…`, then the install starts. A wrong code returns to the sign-in page with the same error.
  - **409:** with the stub's `-Conflict`, the replace question appears with no working page behind it. After Sí, the working page shows again.
  - **Double press:** pressing Next or Enter several times during `Conectando con Kuenta…` gives one enrollment. The stub log shows one `/api/edge/enroll-with-code` per accepted press, and the wizard does not skip a page.

  Record in the PR description whether the bar moved during each wait.

## Workflow follow-up

- Run the spec-readiness skill on this change and fix every blocking question before implementing.
- Release in `edge/v1.3.2` with `installer-clean-uninstall`, then archive the change.
