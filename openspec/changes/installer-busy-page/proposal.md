## Why

When the owner presses "Siguiente" in the installer wizard, the screen freezes for a few seconds with no sign of work, while a script checks the PC or calls the Kuenta cloud. The owner thinks the installer is stuck and presses the button again. Seen on the first `edge/v1.3.1` reinstall.

## What Changes

- Every wizard step that waits on a script shows a "working" screen with a moving bar and a Spanish message until the wait ends:
  - Welcome screen, Next: `Revisando el equipo…` (the PC check), then `Revisando la red…` (the network check, and the address list when the owner ticked the manual network option).
  - Network screen, Next: `Revisando la red…`.
  - Sign-in screen, Next: `Conectando con Kuenta…` (each enrollment call; the confirm dialog for replacing a box still appears between the two calls).
- While that screen is up, the wizard's buttons can't be used, so a second press can't start a second check or a second enrollment.
- When the wait ends, the wizard moves on, or it returns to the same screen and shows the same error message as today.
- The address list for the network screen is now loaded on the welcome screen's Next, under the working screen, instead of when the network screen first opens.
- Silent installs are unchanged.

## Non-goals

- No progress percentage or time estimate: the scripts report none.
- No change to the install step itself (`Instalando…`), which already shows Inno's own progress and the status label.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `edge-installer`: adds the requirements "The wizard shows that it is working while it waits" and "A press during a wait does nothing".

## Impact

- No backend, frontend, API, sync or config change. Ships in an `edge/vX.Y.Z` release (planned `edge/v1.3.2`).
- CI (`edge-ci.yml`) runs only the silent install, which has no screens. The new screen is checked by hand on Windows. The static `Test-HiddenLaunches` check in the installer test must keep passing.
- Trust boundary: none. The enrollment code is still sent once per press. The change only stops a second press from reaching the scripts.
- Touches:
  - `edge/installer/kuenta.iss`
  - `edge/installer/test/run-installer-tests.ps1` (static check only)
- Depends on: none.
