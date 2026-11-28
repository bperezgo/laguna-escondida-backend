## Why

When the owner runs the Kuenta installer, a black console window opens over the wizard at the start (the preflight check) and again at the end (setting up the box). To an owner who is not technical, this looks as if something is taking over their PC, at the exact moment they decide whether to trust the product.

The cause is `RunPS` in `edge/installer/kuenta.iss`. It starts every script through `WScript.Shell.Exec` (`cmd.exe` → `powershell.exe`), and that API always opens a visible console. It has no option to hide it. The other `Exec` calls in the installer already pass `SW_HIDE` and show nothing.

## What Changes

- The installer runs its PowerShell scripts with no window: preflight, enroll, lock-root, write-box-config and setup-box during install and repair. Uninstall already runs hidden and stays that way.
- Progress keeps showing in the wizard's own status line, from the same JSON lines setup-box prints today.
- Inputs that carry secrets (the enrollment code into `enroll.ps1`, and the enrollment result with the node key into `write-box-config.ps1`) move from stdin to a single-use environment variable set only for that child process. They are still never a command-line argument, never written to disk and never logged. Both scripts keep reading stdin when the variable is absent, so CI, the runbook and manual runs work unchanged.
- The installer test checks that no `cmd.exe` runs under setup, that no window under setup becomes visible, and that neither the code nor the node key reaches a log or file.
- Unchanged: the scripts' exit codes and messages, the stderr tail logged on failure, and the "Abrir Kuenta" launch at the end, which is intended.

## Capabilities

### New Capabilities
- `edge-installer-quiet-run`: how the installer runs its scripts. No visible console, live progress in the wizard, and how secret inputs reach the scripts.

### Modified Capabilities
(none: the installer's behavior is specified by the unarchived `edge-installer` change, not by a main spec)

## Impact

- `edge/installer/kuenta.iss`: `RunPS` is rewritten on Inno Setup's `ExecAndLogOutput`. CI installs Inno Setup 6.4.3 (`edge/installer/build-installer.ps1`), and the function exists since 6.3.0.
- `edge/scripts/enroll.ps1` and `write-box-config.ps1`: read their input from the variable first, then from stdin.
- `edge/installer/test/run-installer-tests.ps1`: no-console, no-visible-window and no-secret-in-log asserts.
- `edge/docs/DEPLOYMENT_RUNBOOK.md`: one line on the new input channel.
- Trust boundary: yes, the box's secrets in transit inside the installer (the enrollment code and the node key) and what the setup log records. It does not touch auth, tenancy, sync or any endpoint.
