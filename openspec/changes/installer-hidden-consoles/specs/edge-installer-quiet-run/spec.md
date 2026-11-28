## Purpose
Defines how the Kuenta installer runs its PowerShell scripts: no visible console window, progress shown in the wizard, and how inputs that carry secrets reach the scripts without being exposed.

## ADDED Requirements

### Requirement: The installer shows no console window

During install, repair and uninstall, the installer SHALL start its scripts without a visible window, and SHALL NOT start them through `cmd.exe` or `WScript.Shell`. The only windows the owner sees SHALL be the installer's own wizard and, at the end, Kuenta itself when the owner keeps "Abrir Kuenta" checked.

#### Scenario: Fresh install
- **WHEN** the owner runs the installer on a PC with no box and completes the wizard
- **THEN** no console window appears at any step, including the preflight check and setting up the box, and no `cmd.exe` runs under setup

#### Scenario: Repair
- **WHEN** the installer runs on a PC that already has a box
- **THEN** no console window appears while it stops the services, locks the root and sets up the box

#### Scenario: Uninstall
- **WHEN** the owner uninstalls Kuenta
- **THEN** no console window appears

### Requirement: Progress stays visible in the wizard

While the box is set up, the wizard's status line SHALL show the step that setup-box reports as started, as it does today. A failed script SHALL still stop setup with that script's exit code and message, and the first 2000 characters of its stderr SHALL be written to the setup log.

#### Scenario: Setup reports its steps
- **WHEN** setup-box prints a `{"step":…,"status":"start"}` line
- **THEN** the status line changes to that step's label before the step finishes

#### Scenario: A script fails
- **WHEN** a script exits with a non-zero code and writes to stderr
- **THEN** setup exits with that code, the finish page shows its message, and the setup log holds the stderr tail

### Requirement: Secret inputs are never exposed

The enrollment code SHALL reach `enroll.ps1`, and the enrollment result (which holds the node key) SHALL reach `write-box-config.ps1`, only through a channel that is not a command-line argument, not a file, and not logged. The installer SHALL make the input available only for the duration of that one script run. The script SHALL remove it from its own environment before it starts any other process. Without the installer's input, both scripts SHALL read stdin as they do today.

#### Scenario: No secret in a log or file
- **WHEN** a fresh install completes
- **THEN** neither the enrollment code nor the node key appears in the setup log, in `{tmp}`, under `logs\`, or in the command line of any process started by setup

#### Scenario: Input does not outlive its script
- **WHEN** `enroll.ps1` or `write-box-config.ps1` has returned
- **THEN** the setup process's environment no longer holds the input, and any process that the script or later steps start does not inherit it

#### Scenario: Non-administrator cannot read the input
- **WHEN** a signed-in user who is not an administrator tries to read the environment or command line of the installer's script processes while they run
- **THEN** Windows denies access, because those processes run elevated

#### Scenario: Direct runs keep working
- **WHEN** an operator pipes JSON into `enroll.ps1` or `write-box-config.ps1` on stdin, as the runbook shows
- **THEN** the script behaves as it does today
