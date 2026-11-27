#requires -RunAsAdministrator
<#
  setup-box.ps1 -- everything the installer does after the box's configuration exists:
  database, services, verification, shortcut. The wizard's install page calls it; so does CI.

  Mode is decided from two files (design decision 5):
    env\edge-node.env with a NODE_ID | artifacts\postgres\data | mode
    no                               | no                       | fresh   (needs env\ written first)
    yes                              | yes or no                | repair
    no                               | yes                      | stop    (exit 30, nothing touched)
  -DetectOnly prints {"mode":"..."} and exits. Nothing here ever enrolls, rewrites the env files,
  or passes -Force to init-postgres.ps1.

  Writes one JSON line per step ({"step","status","detail"}) to stdout and, when given,
  appends it to -ProgressFile. Exit codes: installer\exit-codes.json (0, 30, 32, 33, 34).
#>
[CmdletBinding()]
param(
  [string] $Root,
  [switch] $DetectOnly,
  [string] $ProgressFile,
  [int]    $SyncWaitSeconds = 300,
  [int]    $VerifyTimeoutSeconds = 300
)
$ErrorActionPreference = 'Stop'
# Windows PowerShell 5.1 leaves $PSScriptRoot empty in param() defaults.
if (-not $Root) { $Root = Split-Path -Parent $PSScriptRoot }
$scripts = Join-Path $Root 'scripts'
$nodeEnv = Join-Path $Root 'env\edge-node.env'
$dataDir = Join-Path $Root 'artifacts\postgres\data'

function Get-Mode {
  $hasId = (Test-Path $nodeEnv) -and (Select-String -Path $nodeEnv -Pattern '^\s*NODE_ID\s*=\s*\S' -Quiet)
  $hasData = Test-Path $dataDir
  if ($hasId) { return 'repair' }
  if ($hasData) { return 'stop' }
  return 'fresh'
}
$mode = Get-Mode
if ($DetectOnly) { @{ mode = $mode } | ConvertTo-Json -Compress | Write-Output; exit 0 }

function Progress($step, $status, $detail = '') {
  $line = @{ step = $step; status = $status; detail = $detail } | ConvertTo-Json -Compress
  Write-Output $line
  if ($ProgressFile) { Add-Content -Path $ProgressFile -Value $line -Encoding ascii }
}
function Get-EnvValue($key) {
  $l = Get-Content $nodeEnv | Where-Object { $_ -match "^$key=" } | Select-Object -First 1
  if ($l) { return (($l -split '=', 2)[1] -replace '\s+#.*$', '').Trim() }
}
function Run-Step($name, [int] $code, [scriptblock] $body) {
  Progress $name 'start'
  try { & $body; Progress $name 'done' }
  catch { Progress $name 'failed' $_.Exception.Message; exit $code }
}

if ($mode -eq 'stop') { Progress 'mode' 'failed' 'database without configuration'; exit 30 }
if (-not (Test-Path $nodeEnv) -or -not (Test-Path (Join-Path $Root 'env\box.env'))) {
  Progress 'mode' 'failed' 'env files missing'; exit 32
}
Progress 'mode' 'done' $mode

New-Item -ItemType Directory -Force -Path (Join-Path $Root 'logs') | Out-Null
$log = Join-Path $Root 'logs\setup.log'

Run-Step 'database' 32 {
  $super = [Convert]::ToBase64String([Guid]::NewGuid().ToByteArray()).Replace('=', '').Replace('+', 'a').Replace('/', 'b')
  $dbPassword = Get-EnvValue 'DB_PASSWORD'
  if (-not $dbPassword) { throw 'DB_PASSWORD missing from env\edge-node.env' }
  # Its transcript is kept in a log with the generated superuser password masked.
  $output = & (Join-Path $scripts 'init-postgres.ps1') -DbPassword $dbPassword -SuperPassword $super -SkipIfInitialized *>&1 |
    ForEach-Object { "$_".Replace($super, '***').Replace($dbPassword, '***') }
  $output | Add-Content $log -Encoding ascii
}

Run-Step 'services' 32 {
  & (Join-Path $scripts 'install.ps1') *>&1 | Add-Content $log -Encoding ascii
}

Run-Step 'verify' 33 {
  & (Join-Path $scripts 'verify-install.ps1') -TimeoutSeconds $VerifyTimeoutSeconds *>&1 | Add-Content $log -Encoding ascii
  if ($LASTEXITCODE -ne 0) { throw "verify-install.ps1 exited $LASTEXITCODE" }
}

# Not a failure when it times out: the box serves, it only has not reached the cloud yet.
Progress 'sync' 'start'
& (Join-Path $scripts 'verify-install.ps1') -WaitForSync -SyncWaitSeconds $SyncWaitSeconds -SkipChecks *>&1 | Add-Content $log -Encoding ascii
$syncCode = $LASTEXITCODE

Run-Step 'shortcut' 32 {
  & (Join-Path $scripts 'create-shortcut.ps1') -Root $Root *>&1 | Add-Content $log -Encoding ascii
}

if ($syncCode -eq 34) { Progress 'sync' 'warning' 'serving, not yet synced'; exit 34 }
if ($syncCode -ne 0) { Progress 'sync' 'failed' "exit $syncCode"; exit 33 }
Progress 'sync' 'done'
exit 0
