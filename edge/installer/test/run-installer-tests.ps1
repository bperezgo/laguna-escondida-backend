<#
  run-installer-tests.ps1 -- the installer's silent-mode test, run by edge-ci.yml on every PR
  and by release-edge.yml before a release may publish. Needs an elevated Windows machine with
  the appliance ports free and nothing installed at C:\laguna-edge.

  Cases: busy port; box already enrolled (409, no /REPLACE); fresh install; re-run as repair
  (still one enroll call); uninstall keeps data; reinstall keeps data (still one enroll call).
  -KeepInstalled leaves the reinstalled box running for the caller's next steps.
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory)] [string] $Installer,
  [string] $Work = (Join-Path $env:RUNNER_TEMP 'installer-test'),
  [int]    $Port = 18080,
  [switch] $KeepInstalled
)
$ErrorActionPreference = 'Stop'
$root = 'C:\laguna-edge'
$password = 'stub-password'
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
  @(Get-Content $s.log | ConvertFrom-Json | Where-Object { $_.path -eq '/api/edge/enroll' }).Count
}
function Install($s, [string] $name, [string[]] $extra = @()) {
  $env:KUENTA_SETUP_EMAIL = 'owner@stub.test'; $env:KUENTA_SETUP_PASSWORD = $password
  $log = Join-Path $Work "$name.log"
  $p = Start-Process $Installer -Wait -PassThru -ArgumentList (@('/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', "/CLOUDURL=$($s.url)", "/LOG=$log") + $extra)
  Remove-Item Env:\KUENTA_SETUP_EMAIL, Env:\KUENTA_SETUP_PASSWORD -ErrorAction SilentlyContinue
  return @{ code = $p.ExitCode; log = $log }
}
function NodeId { ((Get-Content "$root\env\edge-node.env" | Where-Object { $_ -match '^NODE_ID=' }) -split '=', 2)[1] }

$ok = Start-Stub $Port
$conflict = Start-Stub ($Port + 1) -Conflict
try {
  Step 'Busy port: refused before anything is installed'
  $l = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Any, 5432); $l.Start()
  try { $r = Install $ok 'busy-port' } finally { $l.Stop() }
  Assert ($r.code -ne 0) "busy port exits non-zero ($($r.code))"
  Assert (-not (Test-Path $root)) 'nothing under C:\laguna-edge'
  Assert (Select-String -Path $r.log -Pattern '5432' -Quiet) 'the log names port 5432'
  Assert ((Enroll-Calls $ok) -eq 0) 'no enroll call was made'

  Step 'Box already enrolled, no /REPLACE'
  $r = Install $conflict 'conflict'
  Assert ($r.code -ne 0) "409 without /REPLACE exits non-zero ($($r.code))"
  Assert (-not (Test-Path $root)) 'nothing under C:\laguna-edge'
  Assert (Select-String -Path $r.log -Pattern 'already enrolled' -Quiet) 'the log names the cause'

  Step 'Fresh install'
  $r = Install $ok 'fresh'
  Assert ($r.code -eq 0) "fresh install exits 0 ($($r.code))"
  & "$root\scripts\verify-install.ps1" -TimeoutSeconds 120
  Assert ($LASTEXITCODE -eq 0) 'verify-install passes'
  $status = Invoke-RestMethod http://127.0.0.1:8080/api/edge/status
  Assert ($status.online -eq $true) 'the box synced with the cloud'
  Assert ((Enroll-Calls $ok) -eq 1) 'one enroll call'
  $nodeId = NodeId
  $hits = Get-ChildItem $root -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object { $_.Length -lt 5MB -and $_.Extension -in '.env', '.log', '.json', '.xml', '.txt', '.out', '.err' } |
    Select-String -Pattern $password, 'stub-token-' -SimpleMatch -List
  $hits += Select-String -Path $r.log -Pattern $password, 'stub-token-' -SimpleMatch -List
  Assert (-not $hits) 'no file or log holds the password or the user token'
  Assert ((Test-Path "$env:PUBLIC\Desktop\Kuenta.lnk") -or (Test-Path "$env:PUBLIC\Desktop\Kuenta.url")) 'the Kuenta shortcut exists'
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

  Step 'Uninstall keeps the data'
  $unins = Get-ChildItem $root -Filter 'unins*.exe' | Select-Object -First 1
  Assert ($unins -ne $null) 'an uninstaller was registered'
  Start-Process $unins.FullName -Wait -ArgumentList '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART'
  Assert (-not @(Get-Service laguna-* -ErrorAction SilentlyContinue)) 'no laguna service remains'
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
  Stop-Stub $conflict
  if (-not $KeepInstalled) { Stop-Stub $ok }
}
if ($KeepInstalled) { Write-Host "`nstub cloud left running on $($ok.url) (stop file $($ok.stop))" }
Write-Host "`nAll installer checks passed." -ForegroundColor Green
