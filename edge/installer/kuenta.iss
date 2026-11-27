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
DefaultDirName=C:\laguna-edge
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
Source: "{#StageDir}\*"; DestDir: "{app}"; Excludes: "artifacts\edge-node\*,artifacts\next\*,artifacts\.cache\*,env\box.env,env\edge-node.env,installer\test\*"; Flags: recursesubdirs createallsubdirs ignoreversion
Source: "{#StageDir}\artifacts\edge-node\*"; DestDir: "{app}\artifacts\edge-node"; Check: ShouldReplace('edge-node'); Flags: recursesubdirs createallsubdirs ignoreversion
Source: "{#StageDir}\artifacts\next\*"; DestDir: "{app}\artifacts\next"; Check: ShouldReplace('next'); Flags: recursesubdirs createallsubdirs ignoreversion
Source: "kuenta.ico"; DestDir: "{app}\installer"; Flags: ignoreversion
Source: "licenses\*"; DestDir: "{app}\licenses"; Flags: recursesubdirs ignoreversion
Source: "..\scripts\preflight.ps1"; DestDir: "{tmp}"; Flags: dontcopy
Source: "..\scripts\enroll.ps1"; DestDir: "{tmp}"; Flags: dontcopy

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
  Root = 'C:\laguna-edge';
  BuiltInCloudUrl = '{#CloudUrl}';

var
  SignInPage: TInputQueryWizardPage;
  Mode: String;          // fresh | repair | stop
  EnrollJson: String;    // the enrollment result, in memory only
  OrgName: String;
  Replace: Boolean;
  DeclinedReplace: Boolean;
  PreflightDone: Boolean;
  SyncWarning: Boolean;


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

// Runs a script with its stdin fed from Input. The enrollment code travels only through that pipe: it is
// never an argument and never logged.
function RunPS(const Script, Args, Input: String; var Output: String; Live: Boolean): Integer;
var Sh, Ex: Variant; Line, Lbl: String;
begin
  Output := '';
  Log('run ' + Script + ' ' + Args);
  Sh := CreateOleObject('WScript.Shell');
  Ex := Sh.Exec('"' + PowerShellExe + '" -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "' + Script + '" ' + Args);
  if Input <> '' then Ex.StdIn.Write(Input + #13#10);
  Ex.StdIn.Close;
  while not Ex.StdOut.AtEndOfStream do
  begin
    Line := Ex.StdOut.ReadLine;
    Output := Output + Line + #10;
    if Live then
    begin
      Log('progress ' + Line);
      Lbl := StepLabel(JsonField(Line, 'step'));
      if (Lbl <> '') and (JsonField(Line, 'status') = 'start') then
        WizardForm.StatusLabel.Caption := Lbl;
    end;
  end;
  while Ex.Status = 0 do Sleep(50);
  Result := Ex.ExitCode;
  Log('exit ' + IntToStr(Result));
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

function DoEnroll(const Code, BoxName: String; ReplaceBox: Boolean; var Output: String): Integer;
var Input, Flag: String;
begin
  ExtractTemporaryFile('enroll.ps1');
  if ReplaceBox then Flag := 'true' else Flag := 'false';
  Input := '{"code":"' + JsonEscape(Code) + '","replace":' + Flag +
           ',"name":"' + JsonEscape(BoxName) + '","cloud_url":"' + JsonEscape(CloudUrl) + '"}';
  Result := RunPS(ExpandConstant('{tmp}\enroll.ps1'), '', Input, Output, False);
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

procedure InitializeWizard;
begin
  SignInPage := CreateInputQueryPage(wpWelcome, 'Conecte la caja a Kuenta',
    'Escriba el código de instalación de su restaurante',
    'Encuéntralo en la nube: Instalar caja. El código vale 30 minutos y sirve una sola vez.');
  SignInPage.Add('Código:', False);
  SignInPage.Add('Nombre de la caja:', False);
  SignInPage.Values[1] := GetComputerNameString;
  if Mode = 'repair' then
    WizardForm.WelcomeLabel2.Caption := 'Este equipo ya tiene una caja de Kuenta. Se repetirá la instalación de los programas ' +
      'y servicios; sus datos y su configuración no cambian.' + #13#10#13#10 + 'Haga clic en Siguiente para continuar.';
end;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
  Result := False;
  if PageID = SignInPage.ID then Result := (Mode <> 'fresh') or (EnrollJson <> '');
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var Msg: String;
begin
  Result := True;
  // A silent run still "clicks" Next, and MsgBox ignores /SUPPRESSMSGBOXES; PrepareToInstall does these checks.
  if WizardSilent then Exit;
  if CurPageID = wpWelcome then
  begin
    if not PreflightDone then
    begin
      if DoPreflight(Msg) <> 0 then
      begin
        MsgBox(Msg, mbError, MB_OK);
        Result := False;
      end
      else
        PreflightDone := True;
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

procedure CancelButtonClick(CurPageID: Integer; var Cancel, Confirm: Boolean);
begin
  if DeclinedReplace then Confirm := False;
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
    Exec(PowerShellExe, '-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "Get-Service laguna-* -ErrorAction SilentlyContinue | Stop-Service -Force"',
      '', SW_HIDE, ewWaitUntilTerminated, RC);
end;

procedure CurStepChanged(CurStep: TSetupStep);
var Output, Progress: String; Code: Integer;
begin
  if CurStep <> ssPostInstall then Exit;
  if Mode = 'fresh' then
  begin
    WizardForm.StatusLabel.Caption := 'Escribiendo la configuración de la caja...';
    Code := RunPS(Root + '\scripts\write-box-config.ps1', '', EnrollJson, Output, False);
    EnrollJson := '';
    if Code <> 0 then
    begin
      Log('write-box-config failed: ' + ExitMessage(Code));
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
    RaiseException(ExitMessage(Code));
  end;
end;

procedure CurPageChanged(CurPageID: Integer);
var Txt: String;
begin
  if CurPageID <> wpFinished then Exit;
  if OrgName <> '' then
    Txt := 'Kuenta quedó instalado para ' + OrgName + '.'
  else
    Txt := 'Kuenta quedó instalado y funcionando.';
  if SyncWarning then
    Txt := Txt + #13#10#13#10 + 'Atención: ' + ExitMessage(34);
  WizardForm.FinishedLabel.Caption := Txt;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var RC: Integer;
begin
  if CurUninstallStep = usUninstall then
    Exec(PowerShellExe, '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "' + ExpandConstant('{app}\scripts\uninstall.ps1') + '"',
      '', SW_HIDE, ewWaitUntilTerminated, RC)
  else if CurUninstallStep = usDone then
    SuppressibleMsgBox('Kuenta se desinstaló. Los datos del restaurante (base de datos, configuración, copias de seguridad ' +
      'y certificados) se conservaron en ' + Root + '.', mbInformation, MB_OK, IDOK);
end;
