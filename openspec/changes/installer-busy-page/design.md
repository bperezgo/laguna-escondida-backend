# Design

## Context

See proposal.md for the motivation. Line numbers are on branch `feat/installer-busy-page-impl`, which is based on `installer-clean-uninstall` (PR #105) because both edit `kuenta.iss` and the installer test.

- `edge/installer/kuenta.iss` runs every script through `RunPS` (`:246`), which calls `ExecAndLogOutput` with `ewWaitUntilTerminated`. Its callback `RunLog` (`:226-243`) gets each output line.
- Waits started from a wizard button:
  - `NextButtonClick` (`:595-660`), `wpWelcome` branch (`:601-620`): `DoPreflight` (cloud reachability, up to 15 s, `preflight.ps1:59`) on the first Next, then `CheckNetwork('', '', '', Msg)` on every fresh Next.
  - `NetworkPage.ID` branch (`:621-638`): `CheckNetwork` with the typed values.
  - `SignInPage.ID` branch (`:639-659`): `TryEnroll` (`:465-502`), which calls `DoEnroll` (`:452-461`) once, and a second time after a 409 and the owner's yes (`SuppressibleMsgBox` at `:479-481`, between the two calls). `enroll.ps1:48` waits up to 30 s.
  - `CurPageChanged` (`:810-821`) calls `ListAddresses` (`:399-446`) on the first visit to the network page, and `ListAddresses` itself shows a `MsgBox` on failure (`:411`).
- Inno facts used here:
  - CI builds with Inno Setup **6.7.1**, preinstalled on the runner. That's from the "Locating Inno Setup" line in the release log; `build-installer.ps1:23` installs 6.4.3 only when none is present.
  - Setup is always a 32-bit process in 6.x (6.7.0 whatsnew: "Setup itself is currently always built as a 32-bit x86 binary").
  - `CreateOutputMarqueeProgressPage(const ACaption, ADescription: String): TOutputMarqueeProgressWizardPage`. Its help says: "To animate the progress bar, call the Animate method. Do not call the SetProgress method".
  - `CreateCallback(Method: AnyMethod): NativeInt`.
  - The docs don't say what `Show` does to the buttons, or whether `Exec` processes window messages while it waits. So this design does not rely on either.

## Goals / Non-Goals

**Goals:**
- Every wait above shows the working page, and no click during it reaches a script.

**Non-Goals:**
- Changing the silent path, `PrepareToInstall`, or the install step (`CurStepChanged`).

## Decisions

### D1. One marquee page, created in `InitializeWizard`

New globals in the `var` block (`:98-117`): `BusyPage: TOutputMarqueeProgressWizardPage; Busy: Boolean; BusyTimer: LongWord; BusyNextEnabled, BusyBackEnabled, BusyCancelEnabled: Boolean;`.

At the end of `InitializeWizard` (after `:582`, before the repair caption):

```
BusyPage := CreateOutputMarqueeProgressPage('Un momento', 'Kuenta está trabajando. No cierre esta ventana.');
```

Messages, written with the `…` character (U+2026):

| Wait | `Msg1` |
|---|---|
| `DoPreflight` | `Revisando el equipo…` |
| `CheckNetwork`, `ListAddresses` | `Revisando la red…` |
| `DoEnroll` | `Conectando con Kuenta…` |

`Msg2` is always `''`.

*Alternative: Inno's built-in output progress page with a fixed bar.* Rejected: a bar that never moves reads as frozen, which is the problem being solved.

### D2. `BeginBusy` / `EndBusy`, with a timer for the bar

These go right after `RunPS` (`:246-267`) and before `DoPreflight` (`:313`):

```
function SetTimer(hWnd, nIDEvent, uElapse: LongWord; lpTimerFunc: NativeInt): LongWord;
  external 'SetTimer@user32.dll stdcall';
function KillTimer(hWnd, uIDEvent: LongWord): Boolean;
  external 'KillTimer@user32.dll stdcall';

procedure BusyTick(Wnd, Msg, IdEvent, Time: LongWord);
begin
  BusyPage.Animate;
end;

procedure BeginBusy(const Msg: String);
begin
  if WizardSilent then begin Busy := True; Exit; end;
  if Busy then begin BusyPage.SetText(Msg, ''); Exit; end;
  Busy := True;
  BusyNextEnabled := WizardForm.NextButton.Enabled;
  BusyBackEnabled := WizardForm.BackButton.Enabled;
  BusyCancelEnabled := WizardForm.CancelButton.Enabled;
  BusyPage.SetText(Msg, '');
  BusyPage.Show;
  WizardForm.NextButton.Enabled := False;
  WizardForm.BackButton.Enabled := False;
  WizardForm.CancelButton.Enabled := False;
  BusyPage.Animate;
  BusyTimer := SetTimer(0, 0, 100, CreateCallback(@BusyTick));
end;

procedure EndBusy;
begin
  if not WizardSilent and Busy then
  begin
    if BusyTimer <> 0 then KillTimer(0, BusyTimer);
    BusyTimer := 0;
    BusyPage.Hide;
    WizardForm.NextButton.Enabled := BusyNextEnabled;
    WizardForm.BackButton.Enabled := BusyBackEnabled;
    WizardForm.CancelButton.Enabled := BusyCancelEnabled;
  end;
  Busy := False;
end;
```

- The buttons are disabled after `Show` and restored after `Hide`, in case either call changes them.
- `BeginBusy` while already busy only changes the text, so one span can cover several waits (D4).
- `RunLog` also calls `if Busy and not WizardSilent then BusyPage.Animate;` for each stdout line. That's a second source of movement if the timer never fires.
- The callback is passed as `NativeInt`, the type `CreateCallback` returns. Setup is 32-bit, so `LongWord` handles are pointer-sized.
- Every `BeginBusy` is paired with `EndBusy` in a `try … finally`, so an exception can't leave the buttons disabled.

*Alternative: let `Show` alone handle the buttons.* Rejected: the docs don't say what it does to them.

### D3. A press during a wait never reaches a script

- `NextButtonClick` starts, before the `WizardSilent` exit (`:599-600`), with `if Busy then begin Result := False; Exit; end;`.
- A new `function BackButtonClick(CurPageID: Integer): Boolean;` returns `not Busy`.
- `CancelButtonClick` (`:662-665`) adds `if Busy then Cancel := False;`.

These cover the case where Inno processes clicks while a script runs. The disabled buttons (D2) cover the case where clicks are queued and handled after the wait, because by then the guard has already returned.

### D4. Welcome page: one span, messages after it

The `wpWelcome` branch becomes:

```
Msg := '';
try
  if not PreflightDone then
  begin
    BeginBusy('Revisando el equipo…');
    if DoPreflight(Msg) <> 0 then Result := False else PreflightDone := True;
  end;
  if Result and (Mode = 'fresh') then
  begin
    BeginBusy('Revisando la red…');
    if CheckNetwork('', '', '', NetMsg) then
      NetMsg := ''
    else if not NetworkBox.Checked then
    begin
      Msg := NetMsg;
      Result := False;
    end;
    if Result and NetworkBox.Checked and not NetListed then
    begin
      ListAddresses;
      if NetHost <> '' then HostEdit.Text := NetHost else HostEdit.Text := DefaultLanHost;
    end;
  end;
finally
  EndBusy;
end;
if not Result then MsgBox(Msg, mbError, MB_OK)
else if NetworkBox.Checked and (ListError <> '') then MsgBox(ListError, mbError, MB_OK);
```

The working page opens only when a script runs, so a repair after Back and Next (preflight done, no network check) shows nothing, and the first `BeginBusy` call sets the right label. `EndBusy` is a no-op when nothing was started. `NetworkBox.Checked` matters only in fresh mode: the box is visible only then (`InitializeWizard`, `:575`), and `ShouldSkipPage` (`:592`) skips the network page otherwise. The `Mode = 'fresh'` guard above keeps the network calls to fresh installs.

`NetMsg: String` is added to `NextButtonClick`'s `var`. When the box is ticked, a failed default check still continues (as `c81271c` made it). A failed list still continues to the network page, whose Next refuses with the same error (`:623-629`).

### D5. `ListAddresses` moves out of `CurPageChanged`, and loses its `MsgBox`

- Delete the `MsgBox(ListError, mbError, MB_OK);` line (`:411`). The caller shows it after `EndBusy` (D4).
- Delete the `if CurPageID = NetworkPage.ID then … Exit; end;` block from `CurPageChanged` (`:813-820`). The list and the name prefill now happen in D4.

### D6. Network page and sign-in

- **Network page:** in the `NetworkPage.ID` branch, the line `if Msg = '' then CheckNetwork(…)` (`:632`) becomes:

  ```
  if Msg = '' then
  begin
    BeginBusy('Revisando la red…');
    try
      CheckNetwork(AddrIp[IpCombo.ItemIndex], Cidr, HostOrNone(HostEdit.Text), Msg);
    finally
      EndBusy;
    end;
  end;
  ```

- **Sign-in:** `DoEnroll` wraps its `RunPS` (`:460`) in `BeginBusy('Conectando con Kuenta…'); try … finally EndBusy; end;`. The confirm dialog in `TryEnroll` runs between the two `DoEnroll` calls, so it's never covered by the working page. The error `MsgBox` in `NextButtonClick` (`:656`) runs after `TryEnroll` returns.

### D7. A static check keeps the guards in place

`edge/installer/test/run-installer-tests.ps1` gets `function Test-BusyGuards([string] $text)`, right after `Test-HiddenLaunches` (`:57-68`). It returns a list of problems (empty when fine). It takes each routine's body as the text from `function <Name>` or `procedure <Name>` up to the first line that is exactly `end;`.

| Rule | Problem text |
|---|---|
| In `NextButtonClick`, `if Busy then` comes before `if WizardSilent then Exit` | `NextButtonClick does not refuse while busy` |
| `DoEnroll` contains `BeginBusy(` and `EndBusy` | `DoEnroll runs without the working page` |
| `ListAddresses` contains no `MsgBox(` | `ListAddresses shows a dialog` |
| a `BackButtonClick` exists and contains `not Busy` | `Back is not refused while busy` |

The "No console: static check on kuenta.iss" step (`:292-297`) adds:
- `Assert (-not @(Test-BusyGuards $iss)) 'the wizard refuses presses while it waits (…)'`
- one mutation check: `$iss` with `if Busy then` removed must give exactly one problem.

## Implementation Map

### §1 Static check (tests first)
- **`edge/installer/test/run-installer-tests.ps1`:** `Test-BusyGuards` after `:68`. Two asserts in the step at `:292-297`. The header's `No console:` sentence gains "and the wizard refuses presses while a check runs".

### §2 Wizard
- **`edge/installer/kuenta.iss`:**
  - `var` block (`:98-117`): the D1 globals
  - after `RunPS` (`:267`): `SetTimer`, `KillTimer`, `BusyTick`, `BeginBusy`, `EndBusy` (D2)
  - `RunLog` (`:226-243`): the `Animate` line (D2)
  - `ListAddresses` (`:411`): drop the `MsgBox` (D5)
  - `DoEnroll` (`:460`): the busy wrap (D6)
  - `InitializeWizard` (after `:582`): `BusyPage` (D1)
  - `NextButtonClick` (`:595-660`): the guard (D3), the welcome rewrite (D4), the network wrap (D6), and `NetMsg` in `var`
  - new `BackButtonClick` after `NextButtonClick` (D3)
  - `CancelButtonClick` (`:662-665`): the `Busy` line (D3)
  - `CurPageChanged` (`:813-820`): drop the network-page block (D5)

## Security — N/A

No trust boundary: the change is presentation and click handling inside an installer an elevated administrator runs. The enrollment code is still sent once per accepted press; D3 only removes extra presses.

## Risks / Trade-offs

- **[Inno doesn't process window messages while a script runs, so the timer never fires and the bar stands still]** → the page and its text still show, and the buttons stay disabled and guarded. A frozen bar is checked in §3. If seen, the fallback is to have `RunLog` animate it, which needs scripts to print progress (out of scope).
- **[Clicks queued during the wait are handled after `EndBusy`]** → the guard has returned by then, and the next page's buttons get the queued click. With `DisableReadyPage=yes`, a queued Next after sign-in would start the install sooner, which is the same as one deliberate press. Checked in §3.
- **[`Show` while on `wpWelcome` changes `CurPageID`]** → `Hide` returns to the page that was shown, the pattern Inno's own download-page example uses inside `NextButtonClick`. `CurPageChanged` no longer does work for the network page (D5), so a page-change event from `Show`/`Hide` triggers nothing.

## Migration Plan

None. Ships in `edge/v1.3.2`.
