; Kuenta box installer. A thin wizard over the PowerShell scripts in edge\scripts: every step
; with logic lives there so CI can run it without the wizard (see openspec/changes/edge-installer).
;
; Compile:  ISCC.exe /DAppVersion=v1.4.0 /DStageDir=C:\stage /DCloudUrl=https://api.example.com kuenta.iss
;   AppVersion  the edge release tag, with its "v"        StageDir  the staged install root
;   CloudUrl    the cloud's public address, built in      SignTool  optional: a name configured with iscc /S

#ifndef AppVersion
  #error AppVersion is required (e.g. /DAppVersion=v1.4.0)
#endif
#ifndef StageDir
  #error StageDir is required
#endif
#ifndef CloudUrl
  #error CloudUrl is required
#endif

[Setup]
AppId={{6A1E8C52-3D0B-4B7E-9F4C-5C1D2E7A9B10}
AppName=Kuenta
AppVersion={#AppVersion}
AppPublisher=Kuenta
DefaultDirName=C:\kuentapos
DisableDirPage=yes
DisableProgramGroupPage=yes
DisableReadyPage=yes
UsePreviousAppDir=no
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
OutputDir=Output
OutputBaseFilename=KuentaSetup-{#AppVersion}
SetupIconFile=kuenta.ico
UninstallDisplayIcon={app}\installer\kuenta.ico
UninstallDisplayName=Kuenta
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
CloseApplications=no
RestartApplications=no
#ifdef SignTool
SignTool={#SignTool}
SignedUninstaller=yes
#endif

[Languages]
Name: "es"; MessagesFile: "compiler:Languages\Spanish.isl"

[Files]
; Everything except the two apps, which are replaced only by an equal or newer version, and the
; per-box files, which an install never overwrites.
Source: "{#StageDir}\*"; DestDir: "{app}"; Excludes: "artifacts\edge-node\*,artifacts\next\*,artifacts\.cache\*,artifacts\vcredist\*,env\box.env,env\edge-node.env,installer\test\*"; Flags: recursesubdirs createallsubdirs ignoreversion
Source: "{#StageDir}\artifacts\edge-node\*"; DestDir: "{app}\artifacts\edge-node"; Check: ShouldReplace('edge-node'); Flags: recursesubdirs createallsubdirs ignoreversion
Source: "{#StageDir}\artifacts\next\*"; DestDir: "{app}\artifacts\next"; Check: ShouldReplace('next'); Flags: recursesubdirs createallsubdirs ignoreversion
Source: "kuenta.ico"; DestDir: "{app}\installer"; Flags: ignoreversion
Source: "licenses\*"; DestDir: "{app}\licenses"; Flags: recursesubdirs ignoreversion
Source: "..\scripts\preflight.ps1"; DestDir: "{tmp}"; Flags: dontcopy
Source: "..\scripts\enroll.ps1"; DestDir: "{tmp}"; Flags: dontcopy
Source: "..\scripts\lock-root.ps1"; DestDir: "{tmp}"; Flags: dontcopy
Source: "..\scripts\lan-config.ps1"; DestDir: "{tmp}"; Flags: dontcopy
Source: "{#StageDir}\artifacts\vcredist\VC_redist.x64.exe"; DestDir: "{tmp}"; Flags: dontcopy

[InstallDelete]
; The POS is swapped as a whole folder so chunks of two builds never mix.
Type: filesandordirs; Name: "{app}\artifacts\next"; Check: ShouldReplace('next')

[Run]
Filename: "{commondesktop}\Kuenta.lnk"; Description: "Abrir Kuenta"; Flags: postinstall nowait skipifsilent shellexec; Check: ShortcutExists('Kuenta.lnk')
Filename: "{commondesktop}\Kuenta.url"; Description: "Abrir Kuenta"; Flags: postinstall nowait skipifsilent shellexec; Check: ShortcutExists('Kuenta.url')

[UninstallDelete]
; Programs only. env\, artifacts\postgres\data, backups\, caddy-data\, state\ and logs\ stay.
Type: filesandordirs; Name: "{app}\scripts"
Type: filesandordirs; Name: "{app}\services"
Type: filesandordirs; Name: "{app}\installer"
Type: filesandordirs; Name: "{app}\licenses"
Type: filesandordirs; Name: "{app}\artifacts\caddy"
Type: filesandordirs; Name: "{app}\artifacts\alloy"
Type: filesandordirs; Name: "{app}\artifacts\node"
Type: filesandordirs; Name: "{app}\artifacts\edge-node"
Type: filesandordirs; Name: "{app}\artifacts\next"
Type: filesandordirs; Name: "{app}\artifacts\.cache"
Type: filesandordirs; Name: "{app}\artifacts\postgres\bin"
Type: filesandordirs; Name: "{app}\artifacts\postgres\lib"
Type: filesandordirs; Name: "{app}\artifacts\postgres\share"
Type: files; Name: "{commondesktop}\Kuenta.lnk"
Type: files; Name: "{commondesktop}\Kuenta.url"
Type: files; Name: "{commonprograms}\Kuenta.lnk"
Type: files; Name: "{commonprograms}\Kuenta.url"

[Code]
const
  Root = 'C:\kuentapos';
  BuiltInCloudUrl = '{#CloudUrl}';
  DefaultLanHost = 'pos.kuenta.lan';  // only when the default network check failed; lan-config.ps1 owns it

var
  SignInPage: TInputQueryWizardPage;
  Mode: String;          // fresh | repair | stop
  EnrollJson: String;    // the enrollment result, in memory only
  OrgName: String;
  Replace: Boolean;
  DeclinedReplace: Boolean;
  PreflightDone: Boolean;
  SyncWarning: Boolean;
  FailCode: Integer;     // a post-install step's exit code; setup exits with it
  NetworkPage: TWizardPage;
  NetworkBox: TNewCheckBox;
  IpCombo: TNewComboBox;
  CidrEdit, HostEdit: TNewEdit;
  AddrIp, AddrCidr: TArrayOfString;
  NetIp, NetCidr, NetHost: String;  // the last network check that passed
  NetListed: Boolean;
  ListError: String;
  Purge: Boolean;        // uninstall: also remove the box's data
  BusyPage: TOutputMarqueeProgressWizardPage;
  Busy: Boolean;         // a script started from a wizard button is running
  BusyTimer: LongWord;
  BusyNextEnabled, BusyBackEnabled, BusyCancelEnabled: Boolean;

// lpValue 0 (NULL) removes the variable from this process and from everything it starts.
function SetEnvironmentVariable(lpName: String; lpValue: Cardinal): Boolean;
  external 'SetEnvironmentVariableW@kernel32.dll stdcall';

function SetEnvironmentVariableStr(lpName, lpValue: String): Boolean;
  external 'SetEnvironmentVariableW@kernel32.dll stdcall';

// Started from pwsh 7, setup inherits its PSModulePath, and Windows PowerShell 5.1 then loads
// PowerShell 7's modules and fails. Without the variable, 5.1 builds its own default.
procedure ClearInheritedModulePath;
begin
  if GetEnv('PSModulePath') <> '' then
  begin
    Log('clearing inherited PSModulePath');
    SetEnvironmentVariable('PSModulePath', 0);
  end;
end;


function HasSwitch(const Name: String): Boolean;
var I: Integer;
begin
  Result := False;
  for I := 1 to ParamCount do
    if CompareText(ParamStr(I), '/' + Name) = 0 then Result := True;
end;

function CloudUrl: String;
begin
  Result := ExpandConstant('{param:CLOUDURL|' + BuiltInCloudUrl + '}');
end;

function ShortcutExists(const Name: String): Boolean;
begin
  Result := FileExists(ExpandConstant('{commondesktop}\') + Name);
end;

function PowerShellExe: String;
begin
  Result := ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe');
end;

function JsonEscape(const S: String): String;
var I: Integer; C: Char;
begin
  Result := '';
  for I := 1 to Length(S) do
  begin
    C := S[I];
    if (C = '"') or (C = '\') then
      Result := Result + '\' + C
    else if (Ord(C) < 32) or (Ord(C) > 126) then
      Result := Result + '\u' + Format('%.4x', [Ord(C)])
    else
      Result := Result + C;
  end;
end;

// Value of a string field of one JSON line; '' when absent or null.
function JsonField(const Json, Key: String): String;
var P, I: Integer; C: Char; Needle: String;
begin
  Result := '';
  Needle := '"' + Key + '":"';
  P := Pos(Needle, Json);
  if P = 0 then Exit;
  I := P + Length(Needle);
  while I <= Length(Json) do
  begin
    C := Json[I];
    if C = '"' then Exit;
    if C = '\' then
    begin
      I := I + 1;
      C := Json[I];
      if C = 'u' then
      begin
        Result := Result + Chr(StrToIntDef('$' + Copy(Json, I + 1, 4), 63));
        I := I + 4;
      end
      else
        Result := Result + C;
    end
    else
      Result := Result + C;
    I := I + 1;
  end;
end;

#include "exit-codes.inc"

function StepLabel(const Step: String): String;
begin
  if Step = 'database' then Result := 'Preparando la base de datos...'
  else if Step = 'services' then Result := 'Registrando los servicios de la caja...'
  else if Step = 'verify' then Result := 'Verificando que la caja responde...'
  else if Step = 'sync' then Result := 'Esperando la primera sincronización con Kuenta (hasta 5 minutos)...'
  else if Step = 'shortcut' then Result := 'Creando el acceso directo de Kuenta...'
  else Result := '';
end;

// Per-run state of RunPS, read by RunLog: Inno's callback is a plain procedure, so it cannot capture locals.
var
  RunOutput: String;     // stdout of the current run
  RunErr: String;        // stderr of the current run
  RunLive: Boolean;      // log stdout as progress and drive the status label

// Called by ExecAndLogOutput for each output line. Passing a callback also keeps Inno from writing the
// lines to the setup log itself, which matters because enroll.ps1 prints the node key.
procedure RunLog(const S: String; const Error, FirstLine: Boolean);
var Lbl: String;
begin
  if Error then
  begin
    RunErr := RunErr + S + #10;
    Exit;
  end;
  RunOutput := RunOutput + S + #10;
  if Busy and not WizardSilent then BusyPage.Animate;
  if RunLive then
  begin
    Log('progress ' + S);
    Lbl := StepLabel(JsonField(S, 'step'));
    if (Lbl <> '') and (JsonField(S, 'status') = 'start') then
      WizardForm.StatusLabel.Caption := Lbl;
  end;
end;

// Runs a script with no console window. A script that takes secret input gets it, for the length of this
// one call, in KUENTA_SCRIPT_INPUT: never an argument, a file or a log line (the script removes it first).
function RunPS(const Script, Args, Input: String; var Output: String; Live: Boolean): Integer;
var RC: Integer;
begin
  RunOutput := '';
  RunErr := '';
  RunLive := Live;
  Output := '';
  Log('run ' + Script + ' ' + Args);
  if Input <> '' then SetEnvironmentVariableStr('KUENTA_SCRIPT_INPUT', Input);
  try
    if not ExecAndLogOutput(PowerShellExe, '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "' + Script + '" ' + Args,
      '', SW_HIDE, ewWaitUntilTerminated, RC, @RunLog) then
      RC := -1;
  finally
    if Input <> '' then SetEnvironmentVariable('KUENTA_SCRIPT_INPUT', 0);
  end;
  Output := RunOutput;
  Result := RC;
  Log('exit ' + IntToStr(Result));
  if (Result <> 0) and (Trim(RunErr) <> '') then
    Log('stderr ' + Copy(Trim(RunErr), 1, 2000));
end;

function SetTimer(hWnd, nIDEvent, uElapse: LongWord; lpTimerFunc: NativeInt): LongWord;
  external 'SetTimer@user32.dll stdcall';
function KillTimer(hWnd, uIDEvent: LongWord): Boolean;
  external 'KillTimer@user32.dll stdcall';

// The marquee moves only when Animate is called, so a timer drives it while a script runs.
procedure BusyTick(Wnd, Msg, IdEvent, Time: LongWord);
begin
  BusyPage.Animate;
end;

// Shows the working page and disables the buttons; a second call only changes the text.
procedure BeginBusy(const Msg: String);
begin
  if WizardSilent then
  begin
    Busy := True;
    Exit;
  end;
  if Busy then
  begin
    BusyPage.SetText(Msg, '');
    Exit;
  end;
  Busy := True;
  BusyNextEnabled := WizardForm.NextButton.Enabled;
  BusyBackEnabled := WizardForm.BackButton.Enabled;
  BusyCancelEnabled := WizardForm.CancelButton.Enabled;
  BusyPage.SetText(Msg, '');
  BusyPage.Show;
  // After Show, which may set the buttons itself.
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

function DetectMode: String;
var Lines: TArrayOfString; I: Integer; S: String; HasId: Boolean;
begin
  HasId := False;
  if LoadStringsFromFile(Root + '\env\edge-node.env', Lines) then
    for I := 0 to GetArrayLength(Lines) - 1 do
    begin
      S := Trim(Lines[I]);
      if (Pos('NODE_ID=', S) = 1) and (Trim(Copy(S, 9, Length(S))) <> '') then HasId := True;
    end;
  if HasId then Result := 'repair'
  else if DirExists(Root + '\artifacts\postgres\data') then Result := 'stop'
  else Result := 'fresh';
end;

function ParseVersion(const V: String; var Maj, Min, Pat: Integer): Boolean;
var S: String; P: Integer;
begin
  Result := False;
  S := V;
  if (Length(S) > 0) and (S[1] = 'v') then S := Copy(S, 2, Length(S));
  P := Pos('.', S); if P = 0 then Exit;
  Maj := StrToIntDef(Copy(S, 1, P - 1), -1); S := Copy(S, P + 1, Length(S));
  P := Pos('.', S); if P = 0 then Exit;
  Min := StrToIntDef(Copy(S, 1, P - 1), -1); Pat := StrToIntDef(Copy(S, P + 1, Length(S)), -1);
  Result := (Maj >= 0) and (Min >= 0) and (Pat >= 0);
end;

// An older installer never downgrades a box the in-POS updater moved forward.
function ShouldReplace(const App: String): Boolean;
var Raw: AnsiString; Installed: String; A1, A2, A3, B1, B2, B3: Integer;
begin
  Result := True;
  if not LoadStringFromFile(Root + '\artifacts\' + App + '\RELEASE', Raw) then Exit;
  Installed := Trim(String(Raw));
  if not ParseVersion('{#AppVersion}', A1, A2, A3) then Exit;
  if not ParseVersion(Installed, B1, B2, B3) then
  begin
    Result := False;
    Exit;
  end;
  Result := (A1 > B1) or ((A1 = B1) and ((A2 > B2) or ((A2 = B2) and (A3 >= B3))));
end;

function DoPreflight(var Msg: String): Integer;
var Output, Detail, Port: String; P: Integer;
begin
  ExtractTemporaryFile('preflight.ps1');
  Result := RunPS(ExpandConstant('{tmp}\preflight.ps1'), '-CloudUrl "' + CloudUrl + '"', '', Output, False);
  Msg := ExitMessage(Result);
  if Result = 13 then
  begin
    Detail := JsonField(Output, 'detail');
    P := Pos(' ', Detail);
    if P > 0 then
    begin
      Port := Copy(Detail, 1, P - 1);
      Msg := 'El puerto ' + Port + ' lo está usando el programa "' + Copy(Detail, P + 1, Length(Detail)) +
             '". Ciérrelo e intente de nuevo.';
    end;
  end;
  if Result <> 0 then Log('preflight failed: ' + Msg + ' ' + Trim(Output));
end;

function LanConfigScript: String;
begin
  ExtractTemporaryFile('lan-config.ps1');
  Result := ExpandConstant('{tmp}\lan-config.ps1');
end;

// Windows PowerShell drops an empty "" argument under -File, so a blank value is left out (its default applies).
function NetArg(const Name, Value: String): String;
begin
  Result := '';
  if Value <> '' then Result := ' -' + Name + ' "' + Value + '"';
end;

function CheckNetwork(const Ip, Cidr, Host: String; var Msg: String): Boolean;
var Output: String;
begin
  Msg := '';
  Result := RunPS(LanConfigScript, '-Check' + NetArg('LanIp', Ip) + NetArg('LanCidr', Cidr) + NetArg('LanHost', Host),
    '', Output, False) = 0;
  if Result then
  begin
    NetIp := JsonField(Output, 'lan_ip');
    NetCidr := JsonField(Output, 'lan_cidr');
    NetHost := JsonField(Output, 'lan_host');
    Log('network check: ' + NetIp + ' ' + NetCidr + ' ' + NetHost);
  end
  else
  begin
    Msg := JsonField(Output, 'error');
    if Msg = '' then Msg := ExitMessage(37);
  end;
end;

// Keeps typed values safe to quote on the command line; lan-config.ps1 does the real check.
function OnlyNetChars(const S: String): Boolean;
var I: Integer; C: Char;
begin
  Result := True;
  for I := 1 to Length(S) do
  begin
    C := S[I];
    if not (((C >= '0') and (C <= '9')) or ((C >= 'a') and (C <= 'z')) or ((C >= 'A') and (C <= 'Z')) or
            (C = '.') or (C = '-') or (C = '/')) then
    begin
      Result := False;
      Exit;
    end;
  end;
end;

// '' when the values may go to lan-config.ps1, otherwise its message for the offending one.
function NetCharsMessage(const Ip, Cidr, Host: String): String;
begin
  Result := '';
  if not OnlyNetChars(Ip) then Result := 'La IP ' + Ip + ' no es válida.'
  else if not OnlyNetChars(Cidr) then Result := 'La red ' + Cidr + ' no es válida. Escríbala como 192.168.1.0/24.'
  else if not OnlyNetChars(Host) then
    Result := 'El nombre ' + Host + ' no es válido. Use letras minúsculas, números, guiones y puntos, por ejemplo pos.kuenta.lan.';
end;

function HostOrNone(const H: String): String;
begin
  if Trim(H) = '' then Result := 'none' else Result := Trim(H);
end;

procedure ListAddresses;
var Output, Rest, Line: String; P, N: Integer;
begin
  NetListed := True;
  ListError := '';
  IpCombo.Items.Clear;
  SetArrayLength(AddrIp, 0);
  SetArrayLength(AddrCidr, 0);
  if RunPS(LanConfigScript, '-List', '', Output, False) <> 0 then
  begin
    ListError := JsonField(Output, 'error');
    if ListError = '' then ListError := ExitMessage(37);
    Exit;
  end;
  Rest := Output;
  while Rest <> '' do
  begin
    P := Pos(#10, Rest);
    if P = 0 then
    begin
      Line := Rest;
      Rest := '';
    end
    else
    begin
      Line := Copy(Rest, 1, P - 1);
      Rest := Copy(Rest, P + 1, Length(Rest));
    end;
    Line := Trim(Line);
    if JsonField(Line, 'ip') <> '' then
    begin
      N := GetArrayLength(AddrIp);
      SetArrayLength(AddrIp, N + 1);
      SetArrayLength(AddrCidr, N + 1);
      AddrIp[N] := JsonField(Line, 'ip');
      AddrCidr[N] := JsonField(Line, 'cidr');
      IpCombo.Items.Add(AddrIp[N] + ' (' + JsonField(Line, 'alias') + ')');
    end;
  end;
  if GetArrayLength(AddrIp) = 0 then
    ListError := 'Este equipo no tiene ninguna dirección de red IPv4.'
  else
  begin
    IpCombo.ItemIndex := 0;
    CidrEdit.Text := AddrCidr[0];
  end;
end;

procedure IpComboChange(Sender: TObject);
begin
  if IpCombo.ItemIndex >= 0 then CidrEdit.Text := AddrCidr[IpCombo.ItemIndex];
end;

function DoEnroll(const Code, BoxName: String; ReplaceBox: Boolean; var Output: String): Integer;
var Input, Flag: String;
begin
  ExtractTemporaryFile('enroll.ps1');
  if ReplaceBox then Flag := 'true' else Flag := 'false';
  Input := '{"code":"' + JsonEscape(Code) + '","replace":' + Flag +
           ',"name":"' + JsonEscape(BoxName) + '","cloud_url":"' + JsonEscape(CloudUrl) + '"}';
  BeginBusy('Conectando con Kuenta…');
  try
    Result := RunPS(ExpandConstant('{tmp}\enroll.ps1'), '', Input, Output, False);
  finally
    EndBusy;
  end;
end;

// Enrolls with the code; on a 409 asks (or reads /REPLACE) before replacing with the same code. Msg is
// '' when the owner declined, so the caller can end the wizard instead of showing an error.
function TryEnroll(const EnrollCode, BoxName: String; var Msg: String): Boolean;
var Output, Seen, Box: String; Code: Integer; Confirmed: Boolean;
begin
  Result := False;
  Msg := '';
  Code := DoEnroll(EnrollCode, BoxName, Replace, Output);
  if Code = 22 then
  begin
    Seen := JsonField(Output, 'last_seen_at');
    Box := JsonField(Output, 'name');
    if Seen = '' then Seen := 'nunca';
    if WizardSilent then
      Confirmed := HasSwitch('REPLACE')
    else
      Confirmed := SuppressibleMsgBox('La organización ya tiene una caja instalada: "' + Box + '" (última sincronización: ' + Seen +
        ').' + #13#10#13#10 + 'Si la reemplaza, lo que esa caja no haya sincronizado se perderá.' + #13#10#13#10 +
        '¿Desea reemplazarla con este equipo?', mbConfirmation, MB_YESNO or MB_DEFBUTTON2, IDNO) = IDYES;
    if not Confirmed then
    begin
      Log('box already enrolled (' + Box + '); replacement not confirmed');
      if WizardSilent then Msg := ExitMessage(22) + ' Use /REPLACE para reemplazarla.' else DeclinedReplace := True;
      Exit;
    end;
    Replace := True;
    Code := DoEnroll(EnrollCode, BoxName, True, Output);
  end;
  if Code = 0 then
  begin
    EnrollJson := Output;
    OrgName := JsonField(Output, 'organization_name');
    Result := True;
  end
  else
  begin
    Msg := ExitMessage(Code);
    Log('enroll failed: ' + JsonField(Output, 'error'));
  end;
end;

function InitializeSetup: Boolean;
begin
  ClearInheritedModulePath;
  Mode := DetectMode;
  Log('mode ' + Mode);
  Result := True;
  if Mode = 'stop' then
  begin
    SuppressibleMsgBox(ExitMessage(30), mbError, MB_OK, IDOK);
    Log('database without configuration');
    Result := False;
  end;
end;

function AddNetLabel(const Caption: String; Top: Integer): TNewStaticText;
begin
  Result := TNewStaticText.Create(NetworkPage);
  Result.Parent := NetworkPage.Surface;
  Result.Caption := Caption;
  Result.Left := 0;
  Result.Top := Top;
  Result.Width := NetworkPage.SurfaceWidth;
end;

function AddNetEdit(Lbl: TNewStaticText): TNewEdit;
begin
  Result := TNewEdit.Create(NetworkPage);
  Result.Parent := NetworkPage.Surface;
  Result.Left := 0;
  Result.Top := Lbl.Top + Lbl.Height + ScaleY(4);
  Result.Width := NetworkPage.SurfaceWidth;
end;

procedure CreateNetworkPage;
var Lbl, HintText: TNewStaticText;
begin
  NetworkPage := CreateCustomPage(wpWelcome, 'Red del local', 'Confirme cómo los equipos del restaurante llegan a la caja');
  Lbl := AddNetLabel('IP de esta caja:', 0);
  IpCombo := TNewComboBox.Create(NetworkPage);
  IpCombo.Parent := NetworkPage.Surface;
  IpCombo.Style := csDropDownList;
  IpCombo.Left := 0;
  IpCombo.Top := Lbl.Top + Lbl.Height + ScaleY(4);
  IpCombo.Width := NetworkPage.SurfaceWidth;
  IpCombo.OnChange := @IpComboChange;
  Lbl := AddNetLabel('Red permitida:', IpCombo.Top + IpCombo.Height + ScaleY(12));
  CidrEdit := AddNetEdit(Lbl);
  Lbl := AddNetLabel('Nombre local:', CidrEdit.Top + CidrEdit.Height + ScaleY(12));
  HostEdit := AddNetEdit(Lbl);
  HintText := TNewStaticText.Create(NetworkPage);
  HintText.Parent := NetworkPage.Surface;
  HintText.AutoSize := False;
  HintText.WordWrap := True;
  HintText.Left := 0;
  HintText.Top := HostEdit.Top + HostEdit.Height + ScaleY(12);
  HintText.Width := NetworkPage.SurfaceWidth;
  HintText.Height := ScaleY(48);
  HintText.Caption := 'Deje el nombre vacío para usar solo la IP. El router debe tener este nombre apuntando a la IP elegida, ' +
    'y esa IP fija para este equipo.';
end;

procedure InitializeWizard;
begin
  // The label fills the page by default; shrinking it to its text leaves room for the checkbox below.
  WizardForm.WelcomeLabel2.AutoSize := True;
  NetworkBox := TNewCheckBox.Create(WizardForm);
  NetworkBox.Parent := WizardForm.WelcomePage;
  NetworkBox.Left := WizardForm.WelcomeLabel2.Left;
  NetworkBox.Top := WizardForm.WelcomeLabel2.Top + WizardForm.WelcomeLabel2.Height + ScaleY(12);
  NetworkBox.Width := WizardForm.WelcomeLabel2.Width;
  NetworkBox.Caption := 'Configurar la red manualmente (avanzado)';
  NetworkBox.Visible := Mode = 'fresh';
  CreateNetworkPage;
  SignInPage := CreateInputQueryPage(NetworkPage.ID, 'Conecte la caja a Kuenta',
    'Escriba el código de instalación de su restaurante',
    'Encuéntralo en la nube: Instalar caja. El código vale 30 minutos y sirve una sola vez.');
  SignInPage.Add('Código:', False);
  SignInPage.Add('Nombre de la caja:', False);
  SignInPage.Values[1] := GetComputerNameString;
  BusyPage := CreateOutputMarqueeProgressPage('Un momento', 'Kuenta está trabajando. No cierre esta ventana.');
  if Mode = 'repair' then
    WizardForm.WelcomeLabel2.Caption := 'Este equipo ya tiene una caja de Kuenta. Se repetirá la instalación de los programas ' +
      'y servicios; sus datos y su configuración no cambian.' + #13#10#13#10 + 'Haga clic en Siguiente para continuar.';
end;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
  Result := False;
  if PageID = SignInPage.ID then Result := (Mode <> 'fresh') or (EnrollJson <> '');
  if PageID = NetworkPage.ID then Result := (Mode <> 'fresh') or not NetworkBox.Checked;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var Msg, Cidr, NetMsg: String;
begin
  Result := True;
  // A press that arrives while a script runs must not start a second check or enrollment.
  if Busy then
  begin
    Result := False;
    Exit;
  end;
  // A silent run still "clicks" Next, and MsgBox ignores /SUPPRESSMSGBOXES; PrepareToInstall does these checks.
  if WizardSilent then Exit;
  if CurPageID = wpWelcome then
  begin
    Msg := '';
    // Messages wait until the working page is gone, so they're never hidden behind it.
    try
      if not PreflightDone then
      begin
        BeginBusy('Revisando el equipo…');
        if DoPreflight(Msg) <> 0 then Result := False else PreflightDone := True;
      end;
      if Result and (Mode = 'fresh') then
      begin
        BeginBusy('Revisando la red…');
        // With the box ticked a failed default only means the owner must pick on the network page.
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
    if not Result then
      MsgBox(Msg, mbError, MB_OK)
    else if NetworkBox.Checked and (ListError <> '') then
      MsgBox(ListError, mbError, MB_OK);
  end
  else if CurPageID = NetworkPage.ID then
  begin
    if IpCombo.ItemIndex < 0 then
    begin
      if ListError <> '' then Msg := ListError else Msg := 'Este equipo no tiene ninguna dirección de red IPv4.';
      MsgBox(Msg, mbError, MB_OK);
      Result := False;
      Exit;
    end;
    Cidr := Trim(CidrEdit.Text);
    Msg := NetCharsMessage('', Cidr, Trim(HostEdit.Text));
    if Msg = '' then
    begin
      BeginBusy('Revisando la red…');
      try
        CheckNetwork(AddrIp[IpCombo.ItemIndex], Cidr, HostOrNone(HostEdit.Text), Msg);
      finally
        EndBusy;
      end;
    end;
    if Msg <> '' then
    begin
      MsgBox(Msg, mbError, MB_OK);
      Result := False;
    end;
  end
  else if CurPageID = SignInPage.ID then
  begin
    if (Trim(SignInPage.Values[0]) = '') or (Trim(SignInPage.Values[1]) = '') then
    begin
      MsgBox('Escriba el código y el nombre de la caja.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    if not TryEnroll(Trim(SignInPage.Values[0]), Trim(SignInPage.Values[1]), Msg) then
    begin
      Result := False;
      if DeclinedReplace then
      begin
        MsgBox('No se instaló nada. La caja existente sigue funcionando.', mbInformation, MB_OK);
        WizardForm.Close;
      end
      else
        MsgBox(Msg, mbError, MB_OK);
    end;
    if Result then SignInPage.Values[0] := '';
  end;
end;

function BackButtonClick(CurPageID: Integer): Boolean;
begin
  Result := not Busy;
end;

procedure CancelButtonClick(CurPageID: Integer; var Cancel, Confirm: Boolean);
begin
  if Busy then Cancel := False;
  if DeclinedReplace then Confirm := False;
end;

// Before any file is copied, so nothing a non-admin planted or can still change reaches the box.
function LockRoot: Boolean;
var Args, Output: String;
begin
  ExtractTemporaryFile('lock-root.ps1');
  Args := '-Root "' + Root + '"';
  if Mode = 'fresh' then Args := Args + ' -Fresh';
  Result := RunPS(ExpandConstant('{tmp}\lock-root.ps1'), Args, '', Output, False) = 0;
  Log('lock-root ' + Trim(Output));
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var Msg, EnrollCode, BoxName: String; RC: Integer;
begin
  Result := '';
  if WizardSilent then
  begin
    if DoPreflight(Msg) <> 0 then
    begin
      Result := Msg;
      Exit;
    end;
    if Mode = 'fresh' then
    begin
      EnrollCode := GetEnv('KUENTA_SETUP_CODE');
      BoxName := GetEnv('KUENTA_SETUP_BOX_NAME');
      if BoxName = '' then BoxName := GetComputerNameString;
      if EnrollCode = '' then
      begin
        Result := 'Falta KUENTA_SETUP_CODE.';
        Log(Result);
        Exit;
      end;
      Msg := NetCharsMessage(GetEnv('KUENTA_SETUP_LAN_IP'), GetEnv('KUENTA_SETUP_LAN_CIDR'), GetEnv('KUENTA_SETUP_LAN_HOST'));
      if Msg = '' then
        CheckNetwork(GetEnv('KUENTA_SETUP_LAN_IP'), GetEnv('KUENTA_SETUP_LAN_CIDR'), GetEnv('KUENTA_SETUP_LAN_HOST'), Msg);
      if Msg <> '' then
      begin
        Log('network check failed: ' + Msg);
        Result := ExitMessage(37) + ' ' + Msg;
        Exit;
      end;
      if not TryEnroll(EnrollCode, BoxName, Msg) then
      begin
        Result := Msg;
        Exit;
      end;
    end;
  end;
  if (Mode = 'fresh') and (EnrollJson = '') then
  begin
    Result := ExitMessage(23);
    Exit;
  end;
  // A repair replaces files the running services hold open.
  if Mode = 'repair' then
    Exec(PowerShellExe, '-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "Get-Service kuentapos-* -ErrorAction SilentlyContinue | Stop-Service -Force"',
      '', SW_HIDE, ewWaitUntilTerminated, RC);
  if not LockRoot then Result := ExitMessage(36);
end;

// Postgres's binaries need it and do not ship it. Always run: it returns 1638 at once when the
// same or a newer runtime is there, which also catches an old one without vcruntime140_1.dll.
function InstallVcRuntime: Boolean;
var RC: Integer;
begin
  WizardForm.StatusLabel.Caption := 'Instalando componentes de Windows...';
  ExtractTemporaryFile('VC_redist.x64.exe');
  Result := Exec(ExpandConstant('{tmp}\VC_redist.x64.exe'), '/install /quiet /norestart', '', SW_HIDE,
    ewWaitUntilTerminated, RC) and ((RC = 0) or (RC = 1638) or (RC = 3010));
  Log('Visual C++ runtime setup exit ' + IntToStr(RC));
end;

procedure CurStepChanged(CurStep: TSetupStep);
var Output, Progress: String; Code: Integer;
begin
  if CurStep <> ssPostInstall then Exit;
  if not InstallVcRuntime then
  begin
    FailCode := 35;
    RaiseException(ExitMessage(35));
  end;
  if Mode = 'fresh' then
  begin
    WizardForm.StatusLabel.Caption := 'Escribiendo la configuración de la caja...';
    Code := RunPS(Root + '\scripts\write-box-config.ps1', '-LanIp "' + NetIp + '" -LanCidr "' + NetCidr + '" -LanHost "' +
      HostOrNone(NetHost) + '"', EnrollJson, Output, False);
    EnrollJson := '';
    if Code <> 0 then
    begin
      Log('write-box-config failed: ' + ExitMessage(Code) + ' ' + Trim(Output));
      FailCode := Code;
      RaiseException(ExitMessage(Code));
    end;
  end;
  Progress := ExpandConstant('{tmp}\setup-progress.jsonl');
  Code := RunPS(Root + '\scripts\setup-box.ps1', '-ProgressFile "' + Progress + '"', '', Output, True);
  if Code = 34 then
  begin
    SyncWarning := True;
    Log(ExitMessage(34));
  end
  else if Code <> 0 then
  begin
    Log('setup-box failed: ' + ExitMessage(Code));
    FailCode := Code;
    RaiseException(ExitMessage(Code));
  end;
end;

// What other devices open, and what the gateway must hold. A repair reads it from env\box.env.
function FinishAddresses: String;
var Lines: TArrayOfString; I: Integer; S, Ip, Host: String;
begin
  Ip := NetIp;
  Host := NetHost;
  if Mode = 'repair' then
  begin
    Ip := '';
    Host := '';
    if LoadStringsFromFile(Root + '\env\box.env', Lines) then
      for I := 0 to GetArrayLength(Lines) - 1 do
      begin
        S := Trim(Lines[I]);
        if Pos('EDGE_LAN_IP=', S) = 1 then Ip := Trim(Copy(S, 13, Length(S)));
        if Pos('EDGE_LAN_HOST=', S) = 1 then
        begin
          Host := Trim(Copy(S, 15, Length(S)));
          if Pos(',', Host) > 0 then Host := Trim(Copy(Host, 1, Pos(',', Host) - 1));
        end;
      end;
  end;
  Result := '';
  if Ip = '' then Exit;
  if Host <> '' then
    Result := 'Desde otros equipos del restaurante abra https://' + Host + ' (o https://' + Ip + ').' + #13#10 +
      'En el router, el nombre ' + Host + ' debe apuntar a ' + Ip + ' (registro ' + Host + ' -> ' + Ip +
      '), y esa IP debe estar fija para este equipo.'
  else
    Result := 'Desde otros equipos del restaurante abra https://' + Ip + '.' + #13#10 +
      'En el router, esa IP debe estar fija para este equipo.';
end;

procedure CurPageChanged(CurPageID: Integer);
var Txt, Addr: String;
begin
  if CurPageID <> wpFinished then Exit;
  if FailCode <> 0 then
    Txt := 'Kuenta no se terminó de instalar: ' + ExitMessage(FailCode)
  else if OrgName <> '' then
    Txt := 'Kuenta quedó instalado para ' + OrgName + '.'
  else
    Txt := 'Kuenta quedó instalado y funcionando.';
  if FailCode = 0 then
  begin
    Addr := FinishAddresses;
    if Addr <> '' then Txt := Txt + #13#10#13#10 + Addr;
  end;
  if SyncWarning then
    Txt := Txt + #13#10#13#10 + 'Atención: ' + ExitMessage(34);
  WizardForm.FinishedLabel.Caption := Txt;
end;

// An exception in CurStepChanged is only shown; without this, setup would still exit 0.
function GetCustomSetupExitCode: Integer;
begin
  Result := FailCode;
end;

function InitializeUninstall: Boolean;
begin
  ClearInheritedModulePath;
  Result := True;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var RC: Integer; Args: String;
begin
  if CurUninstallStep = usUninstall then
  begin
    // After Inno's own confirmation, before anything is removed. No is the default, and a silent run keeps the data.
    Purge := HasSwitch('PURGE');
    if not Purge and not UninstallSilent then
      Purge := SuppressibleMsgBox('¿Desea borrar también los datos de esta caja (base de datos, configuración, copias de ' +
        'seguridad y certificados)?' + #13#10#13#10 + 'Lo que la caja no haya sincronizado con Kuenta se perderá. Si responde No, ' +
        'podrá reinstalar Kuenta y la caja seguirá igual.', mbConfirmation, MB_YESNO or MB_DEFBUTTON2, IDNO) = IDYES;
    Log('purge ' + IntToStr(Ord(Purge)));
    Args := '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "' + ExpandConstant('{app}\scripts\uninstall.ps1') + '"';
    if Purge then Args := Args + ' -Purge';
    Exec(PowerShellExe, Args, '', SW_HIDE, ewWaitUntilTerminated, RC);
  end
  // Inno has removed its own files and the services are gone, so nothing holds the folder open.
  else if (CurUninstallStep = usPostUninstall) and Purge then
  begin
    DelTree(Root, True, True, True);
    Log('purge folder removed: ' + IntToStr(Ord(not DirExists(Root))));
  end
  else if CurUninstallStep = usDone then
  begin
    // Inno may still be deleting unins000.* here, so the root folder itself is not the test.
    if not Purge then
      SuppressibleMsgBox('Kuenta se desinstaló. Los datos del restaurante (base de datos, configuración, copias de seguridad ' +
        'y certificados) se conservaron en ' + Root + '.', mbInformation, MB_OK, IDOK)
    else if not DirExists(Root + '\env') and not DirExists(Root + '\artifacts') then
      SuppressibleMsgBox('Kuenta se desinstaló y se borraron los datos de esta caja. La próxima instalación empezará desde cero.',
        mbInformation, MB_OK, IDOK)
    else
      SuppressibleMsgBox('Kuenta se desinstaló, pero no se pudo borrar todo en ' + Root + '. Reinicie el equipo y borre esa ' +
        'carpeta antes de instalar de nuevo.', mbError, MB_OK, IDOK);
  end;
end;
