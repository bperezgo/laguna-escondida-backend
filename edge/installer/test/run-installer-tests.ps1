<#
  run-installer-tests.ps1 -- the installer's silent-mode test, run by edge-ci.yml on every PR
  and by release-edge.yml before a release may publish. Needs an elevated Windows machine with
  the appliance ports free and nothing installed at C:\kuentapos.

  Cases: busy port; invalid code; box already enrolled (409, no /REPLACE); fresh install; re-run as
  repair (still one enroll call); uninstall keeps data; reinstall keeps data (still one enroll call);
  enroll.ps1 alone: a valid code, an invalid code, and a 409 then replace with the same code.
  A local standard user pre-creates the root with planted files and a link before the fresh install,
  and after the fresh install and the repair can neither change what SYSTEM runs nor read env\.
  -KeepInstalled leaves the reinstalled box running for the caller's next steps.
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory)] [string] $Installer,
  [string] $Work = (Join-Path $env:RUNNER_TEMP 'installer-test'),
  [int]    $Port = 18080,
  [int]    $TimeoutSeconds = 900,
  [switch] $KeepInstalled
)
$ErrorActionPreference = 'Stop'
$root = 'C:\kuentapos'
$code = 'STUB-C0DE'
$stub = Join-Path $PSScriptRoot 'stub-cloud.ps1'
New-Item -ItemType Directory -Force -Path $Work | Out-Null
function Step($m) { Write-Host ("`n==> " + $m) -ForegroundColor Cyan }
function Assert($cond, $msg) { if (-not $cond) { throw "FAIL: $msg" } else { Write-Host "  ok   $msg" -ForegroundColor Green } }

function Start-Stub([int] $p, [switch] $Conflict) {
  $log = Join-Path $Work "stub-$p.log"; $stop = Join-Path $Work "stub-$p.stop"
  Remove-Item $log, $stop -Force -ErrorAction SilentlyContinue
  $a = @('-NoProfile', '-File', $stub, '-Port', $p, '-LogFile', $log, '-StopFile', $stop)
  if ($Conflict) { $a += '-Conflict' }
  $proc = Start-Process pwsh -ArgumentList $a -PassThru -WindowStyle Hidden
  foreach ($i in 1..30) {
    try { Invoke-RestMethod "http://127.0.0.1:$p/api/auth/context" -TimeoutSec 2 | Out-Null; break } catch { Start-Sleep 1 }
  }
  return @{ proc = $proc; log = $log; stop = $stop; url = "http://127.0.0.1:$p" }
}
function Stop-Stub($s) { New-Item $s.stop -ItemType File -Force | Out-Null; $s.proc | Wait-Process -Timeout 15 -ErrorAction SilentlyContinue }
function Enroll-Calls($s) {
  if (-not (Test-Path $s.log)) { return 0 }
  @(Get-Content $s.log | ConvertFrom-Json | Where-Object { $_.path -eq '/api/edge/enroll-with-code' }).Count
}
function Get-Descendants([int] $id) {
  foreach ($c in @(Get-CimInstance Win32_Process -Filter "ParentProcessId = $id")) { $c; Get-Descendants $c.ProcessId }
}
# A hung installer must fail the test with its log and process tree, not run into the job timeout.
function Wait-Setup($p, [string] $log, [string] $what) {
  if ($p.WaitForExit($TimeoutSeconds * 1000)) { return }
  $tree = @(Get-Descendants $p.Id)
  Write-Host "::group::$what is still running after $TimeoutSeconds s"
  $tree | Format-List ProcessId, ParentProcessId, Name, CommandLine | Out-String -Width 400 | Write-Host
  if (Test-Path $log) { Get-Content $log -Tail 80 | Write-Host }
  Write-Host '::endgroup::'
  $tree | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
  Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
  throw "FAIL: $what did not exit within $TimeoutSeconds s"
}
function Install($s, [string] $name, [string[]] $extra = @(), [string] $useCode = $code) {
  $env:KUENTA_SETUP_CODE = $useCode
  $log = Join-Path $Work "$name.log"
  $p = Start-Process $Installer -PassThru -ArgumentList (@('/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', "/CLOUDURL=$($s.url)", "/LOG=$log") + $extra)
  # Without -Wait, ExitCode stays empty unless the handle is taken while the process runs.
  $null = $p.Handle
  Remove-Item Env:\KUENTA_SETUP_CODE -ErrorAction SilentlyContinue
  Wait-Setup $p $log "the $name install"
  return @{ code = $p.ExitCode; log = $log }
}
function Run-Enroll($s, [string] $useCode, [bool] $replace) {
  $line = @{ code = $useCode; replace = $replace; name = 'Caja CI'; cloud_url = $s.url } | ConvertTo-Json -Compress
  $out = $line | powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot '..\..\scripts\enroll.ps1')
  return @{ code = $LASTEXITCODE; out = ($out | Out-String) | ConvertFrom-Json }
}

# The standard user's probes run in this process under its network-logon token: a child process
# would get the admin's token, and Windows Server denies Users an interactive logon.
Add-Type -Namespace Ci -Name Native -MemberDefinition @'
[DllImport("advapi32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
public static extern bool LogonUser(string user, string domain, string password, int type, int provider,
  out Microsoft.Win32.SafeHandles.SafeAccessTokenHandle token);
'@
function New-StandardUser {
  $name = 'kuentaci'
  # 14 characters at most: net user asks to confirm a longer password, which fails without a console.
  $pw = 'Aa1!' + [guid]::NewGuid().ToString('N').Substring(0, 10)
  & net.exe user $name /delete 2>&1 | Out-Null
  $out = & net.exe user $name $pw /add 2>&1 | Out-String
  if ($LASTEXITCODE -ne 0) { throw "could not create the local user $name ($LASTEXITCODE): $($out.Trim())" }
  $sid = ([Security.Principal.NTAccount] $name).Translate([Security.Principal.SecurityIdentifier]).Value
  return @{ name = $name; pw = $pw; sid = $sid }
}
function As-User($u, [scriptblock] $block) {
  $tok = $null
  if (-not [Ci.Native]::LogonUser($u.name, '.', $u.pw, 3, 0, [ref] $tok)) {
    throw "LogonUser failed ($([Runtime.InteropServices.Marshal]::GetLastWin32Error()))"
  }
  try { [Security.Principal.WindowsIdentity]::RunImpersonated($tok, [Func[object]] $block) } finally { $tok.Dispose() }
}
# 'allowed', 'denied', or the error. Probes never change an existing file's content.
function Probe($u, [string] $kind, [string] $path) {
  As-User $u ({
    try {
      switch ($kind) {
        'write'  { [IO.File]::Open($path, 'Open', 'Write', 'ReadWrite').Dispose() }
        'create' { [IO.File]::Open($path, 'CreateNew', 'Write').Dispose() }
        'read'   { [IO.File]::OpenRead($path).Dispose() }
      }
      'allowed'
    } catch {
      $ex = $_.Exception; while ($ex.InnerException) { $ex = $ex.InnerException }
      if ($ex -is [UnauthorizedAccessException]) { 'denied' } else { "error: $($ex.Message)" }
    }
  }.GetNewClosure())
}
function Assert-Locked($u, [string] $when) {
  foreach ($t in @(
      @('write', "$root\scripts\updater.ps1"), @('write', "$root\services\edge-node.xml"),
      @('create', "$root\artifacts\postgres\bin\ci-probe.dll"), @('create', "$root\scripts\ci-probe.ps1"),
      @('read', "$root\env\edge-node.env"), @('read', "$root\env\alloy-node-key"),
      @('read', "$root\artifacts\edge-node\.env"),
      @('read', "$root\caddy-data\pki\authorities\local\root.key"))) {
    $got = Probe $u $t[0] $t[1]
    if ($got -eq 'allowed' -and $t[0] -eq 'create') { Remove-Item $t[1] -Force }
    Assert ($got -eq 'denied') "$when, a standard user's $($t[0]) of $($t[1]) is denied ($got)"
  }
}
function Owner([string] $path) { (Get-Acl $path).GetOwner([Security.Principal.SecurityIdentifier]).Value }

function NodeId { ((Get-Content "$root\env\edge-node.env" | Where-Object { $_ -match '^NODE_ID=' }) -split '=', 2)[1] }

$ok = Start-Stub $Port
$conflict = Start-Stub ($Port + 1) -Conflict
$user = $null
$canary = 'C:\ci-canary'
try {
  $user = New-StandardUser
  Step 'enroll.ps1 against the stub cloud'
  $e = Run-Enroll $conflict 'wrng-c0de' $false
  Assert ($e.code -eq 20 -and $e.out.error -eq 'bad_credentials') "an invalid code is bad_credentials ($($e.code))"
  $e = Run-Enroll $conflict 'stub c0de' $false
  Assert ($e.code -eq 22 -and $e.out.active_box.name -eq 'Caja principal') "a 409 reports the active box ($($e.code))"
  $e = Run-Enroll $conflict 'STUB-C0DE' $true
  Assert ($e.code -eq 0 -and $e.out.node_key) "the same code with replace enrolls ($($e.code))"
  Assert (-not ($e.out | ConvertTo-Json | Select-String -Pattern 'C0DE' -SimpleMatch -Quiet)) 'the output never echoes the code'

  Step 'Busy port: refused before anything is installed'
  $l = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Any, 5432); $l.Start()
  try { $r = Install $ok 'busy-port' } finally { $l.Stop() }
  Assert ($r.code -ne 0) "busy port exits non-zero ($($r.code))"
  Assert (-not (Test-Path $root)) 'nothing under C:\kuentapos'
  Assert (Select-String -Path $r.log -Pattern '5432' -Quiet) 'the log names port 5432'
  Assert ((Enroll-Calls $ok) -eq 0) 'no enroll call was made'

  Step 'Invalid code: the bad-credentials exit code, nothing installed'
  $r = Install $conflict 'bad-code' @() 'WRNG-C0DE'
  Assert ($r.code -ne 0) "an invalid code exits non-zero ($($r.code))"
  Assert (-not (Test-Path $root)) 'nothing under C:\kuentapos'
  Assert (Select-String -Path $r.log -Pattern 'bad_credentials' -Quiet) 'the log names bad_credentials'

  Step 'Box already enrolled, no /REPLACE'
  $r = Install $conflict 'conflict'
  Assert ($r.code -ne 0) "409 without /REPLACE exits non-zero ($($r.code))"
  Assert (-not (Test-Path $root)) 'nothing under C:\kuentapos'
  Assert (Select-String -Path $r.log -Pattern 'already enrolled' -Quiet) 'the log names the cause'

  Step 'A standard user pre-creates the root'
  Remove-Item $canary -Recurse -Force -ErrorAction SilentlyContinue
  $null = As-User $user ({
    [IO.Directory]::CreateDirectory("$root\artifacts\postgres\bin") | Out-Null
    [IO.Directory]::CreateDirectory("$root\scripts") | Out-Null
    [IO.File]::WriteAllText("$root\artifacts\postgres\bin\planted.dll", 'x')
    [IO.File]::WriteAllText("$root\scripts\planted.ps1", 'x')
    [IO.Directory]::CreateDirectory("$root\caddy-data\pki\authorities\local") | Out-Null
    [IO.File]::WriteAllText("$root\caddy-data\pki\authorities\local\root.key", 'planted')
    [IO.Directory]::CreateDirectory($canary) | Out-Null
    New-Item -ItemType Junction -Path "$root\ci-link" -Target $canary | Out-Null
  }.GetNewClosure())
  $userSid = $user.sid
  Assert ((Owner $root) -eq $userSid) 'the standard user owns the pre-created root'
  Assert (Test-Path "$root\scripts\planted.ps1") 'the planted files exist'

  Step 'Fresh install'
  $r = Install $ok 'fresh'
  Assert ($r.code -eq 0) "fresh install exits 0 ($($r.code))"
  Assert (Select-String -Path $r.log -Pattern 'Visual C\+\+ runtime setup exit (0|1638|3010)\b' -Quiet) 'the Visual C++ runtime setup ran'
  & "$root\scripts\verify-install.ps1" -TimeoutSeconds 120
  Assert ($LASTEXITCODE -eq 0) 'verify-install passes'
  $status = Invoke-RestMethod http://127.0.0.1:8080/api/edge/status
  Assert ($status.online -eq $true) 'the box synced with the cloud'
  Assert ((Enroll-Calls $ok) -eq 1) 'one enroll call'
  $nodeId = NodeId
  $hits = Get-ChildItem $root -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object { $_.Length -lt 5MB -and $_.Extension -in '.env', '.log', '.json', '.xml', '.txt', '.out', '.err' } |
    Select-String -Pattern $code -SimpleMatch -List
  $hits += Select-String -Path $r.log -Pattern $code -SimpleMatch -List
  Assert (-not $hits) 'no file or log holds the enrollment code'
  Assert ((Test-Path "$env:PUBLIC\Desktop\Kuenta.lnk") -or (Test-Path "$env:PUBLIC\Desktop\Kuenta.url")) 'the Kuenta shortcut exists'
  Assert (-not @(Get-Service laguna-* -ErrorAction SilentlyContinue)) 'no laguna service exists'
  Assert (-not (Get-ScheduledTask 'Laguna Updater' -ErrorAction SilentlyContinue)) 'no "Laguna Updater" task exists'
  Assert (-not (Get-NetFirewallRule -DisplayName 'Laguna POS - Caddy (LAN tablets)' -ErrorAction SilentlyContinue)) 'no "Laguna POS" firewall rule exists'
  Assert (Get-ScheduledTask 'Kuenta Updater' -ErrorAction SilentlyContinue) 'the "Kuenta Updater" task exists'
  Assert (Get-NetFirewallRule -DisplayName 'Kuenta - Caddy (LAN tablets)' -ErrorAction SilentlyContinue) 'the "Kuenta - Caddy (LAN tablets)" rule exists'
  Assert (Test-Path "$root\kuentapos-root-ca.crt") 'the root CA was exported to kuentapos-root-ca.crt'
  Assert (-not (Test-Path "$root\artifacts\postgres\bin\planted.dll")) 'the planted DLL is gone'
  Assert (-not (Test-Path "$root\scripts\planted.ps1")) 'the planted script is gone'
  Assert (-not (Test-Path "$root\ci-link")) 'the planted link is gone'
  Assert ((Get-Content "$root\caddy-data\pki\authorities\local\root.key" -Raw) -ne 'planted') 'Caddy made its own CA, not the planted one'
  Assert ((Owner $canary) -eq $userSid) 'the link target was not touched'
  Assert ((Owner $root) -eq 'S-1-5-32-544') 'Administrators own the root'
  Assert ((Get-Acl $root).AreAccessRulesProtected) 'the root does not inherit from C:\'
  # Not written by a box without telemetry; placed here so its read is probed, and its ACL set by the repair.
  if (-not (Test-Path "$root\env\alloy-node-key")) { [IO.File]::WriteAllText("$root\env\alloy-node-key", 'ci') }
  $got = Probe $user 'create' "$root\ci-probe.txt"
  Assert ($got -eq 'denied') "a standard user cannot create a file in the root ($got)"
  Assert-Locked $user 'after the fresh install'
  $dbPw = ((Get-Content "$root\env\edge-node.env" | Where-Object { $_ -match '^DB_PASSWORD=' }) -split '=', 2)[1]
  $env:PGPASSWORD = $dbPw
  & "$root\artifacts\postgres\bin\psql.exe" -h 127.0.0.1 -U laguna -d laguna_escondida -w -c 'CREATE TABLE ci_marker (v int); INSERT INTO ci_marker VALUES (42);' | Out-Null
  Assert ($LASTEXITCODE -eq 0) 'a marker row was written to the box database'

  Step 'Re-run: repair'
  $r = Install $ok 'repair'
  Assert ($r.code -eq 0) "repair exits 0 ($($r.code))"
  Assert ((Enroll-Calls $ok) -eq 1) 'still one enroll call'
  Assert ((NodeId) -eq $nodeId) 'the node id is unchanged'
  & "$root\scripts\verify-install.ps1" -TimeoutSeconds 120
  Assert ($LASTEXITCODE -eq 0) 'verify-install passes after repair'
  Assert-Locked $user 'after the repair'

  Step 'Uninstall keeps the data'
  $unins = Get-ChildItem $root -Filter 'unins*.exe' | Select-Object -First 1
  Assert ($unins -ne $null) 'an uninstaller was registered'
  Start-Process $unins.FullName -Wait -ArgumentList '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART'
  Assert (-not @(Get-Service kuentapos-* -ErrorAction SilentlyContinue)) 'no kuentapos service remains'
  Assert (-not (Get-ScheduledTask 'Kuenta Updater' -ErrorAction SilentlyContinue)) 'the "Kuenta Updater" task was removed'
  Assert (-not (Get-NetFirewallRule -DisplayName 'Kuenta - Caddy (LAN tablets)' -ErrorAction SilentlyContinue)) 'the firewall rule was removed'
  Assert (Test-Path "$root\env\edge-node.env") 'env\edge-node.env kept'
  Assert (Test-Path "$root\artifacts\postgres\data") 'the database directory kept'
  Assert (-not (Test-Path "$root\artifacts\postgres\bin")) 'the Postgres programs were removed'
  Assert (-not (Test-Path "$env:PUBLIC\Desktop\Kuenta.lnk") -and -not (Test-Path "$env:PUBLIC\Desktop\Kuenta.url")) 'the shortcut was removed'

  Step 'Reinstall: repair with the data intact'
  $r = Install $ok 'reinstall'
  Assert ($r.code -eq 0) "reinstall exits 0 ($($r.code))"
  Assert ((Enroll-Calls $ok) -eq 1) 'still one enroll call'
  Assert ((NodeId) -eq $nodeId) 'the node id is unchanged'
  $env:PGPASSWORD = $dbPw
  $v = (& "$root\artifacts\postgres\bin\psql.exe" -h 127.0.0.1 -U laguna -d laguna_escondida -w -tAc 'SELECT v FROM ci_marker' | Out-String).Trim()
  Assert ($v -eq '42') 'the marker row survived uninstall and reinstall'
  Remove-Item Env:\PGPASSWORD -ErrorAction SilentlyContinue

}
finally {
  if ($user) { & net.exe user $user.name /delete 2>&1 | Out-Null }
  Remove-Item $canary -Recurse -Force -ErrorAction SilentlyContinue
  Stop-Stub $conflict
  if (-not $KeepInstalled) { Stop-Stub $ok }
}
if ($KeepInstalled) { Write-Host "`nstub cloud left running on $($ok.url) (stop file $($ok.stop))" }
Write-Host "`nAll installer checks passed." -ForegroundColor Green
