<#
  preflight.ps1 -- is this PC fit for the appliance? Changes nothing.
  Exit codes: installer\exit-codes.json (10 os, 11 elevation, 12 disk, 13 port, 14 policy,
  15 cloud). The first failing check prints one JSON line {"check","code","detail"} and stops.

  Run:  .\scripts\preflight.ps1 -CloudUrl https://api.example.com
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory)] [string] $CloudUrl,
  [string] $Drive = 'C:',
  [long]   $MinFreeBytes = 3GB,
  [int[]]  $Ports = @(80, 443, 3000, 5432, 8080)
)
$ErrorActionPreference = 'Stop'

function Stop-With([int] $code, [string] $check, [string] $detail) {
  @{ check = $check; code = $code; detail = $detail } | ConvertTo-Json -Compress | Write-Output
  exit $code
}
# The installer reads only stdout, so an unexpected error must land there too.
trap { Stop-With 1 'unexpected' "line $($_.InvocationInfo.ScriptLineNumber): $($_.Exception.Message)" }

# 1. Windows 10/11, 64-bit (build 10240 is Windows 10 1507).
$os = Get-CimInstance Win32_OperatingSystem
if (-not [Environment]::Is64BitOperatingSystem -or [version]$os.Version -lt [version]'10.0.10240') {
  Stop-With 10 'os' "$($os.Caption) $($os.OSArchitecture)"
}

# 2. Elevation.
$id = [Security.Principal.WindowsIdentity]::GetCurrent()
if (-not ([Security.Principal.WindowsPrincipal]$id).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  Stop-With 11 'elevation' 'not an administrator'
}

# 3. Free space.
$free = (Get-PSDrive -Name $Drive.TrimEnd(':') -PSProvider FileSystem).Free
if ($free -lt $MinFreeBytes) { Stop-With 12 'disk' "$([math]::Round($free / 1GB, 1)) GB free" }

# 4. Ports, ignoring the appliance's own services (a repair runs with them up).
$ownPids = @(Get-CimInstance Win32_Service -Filter "Name LIKE 'laguna-%'" | Where-Object ProcessId | ForEach-Object ProcessId)
# winsw wrappers own a child process; include the children of our service wrappers too.
$ownPids += @(Get-CimInstance Win32_Process | Where-Object { $ownPids -contains $_.ParentProcessId } | ForEach-Object ProcessId)
$busy = Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
  Where-Object { $Ports -contains $_.LocalPort -and $ownPids -notcontains $_.OwningProcess } |
  Sort-Object LocalPort | Select-Object -First 1
if ($busy) {
  $name = (Get-Process -Id $busy.OwningProcess -ErrorAction SilentlyContinue).ProcessName
  if (-not $name) { $name = 'System' }
  Stop-With 13 'port' "$($busy.LocalPort) $name"
}

# 5. Script policy: a machine-level Group Policy beats -ExecutionPolicy Bypass.
$policy = Get-ExecutionPolicy -Scope MachinePolicy
if ($policy -in 'Restricted', 'AllSigned') { Stop-With 14 'policy' "MachinePolicy=$policy" }

# 6. The cloud answers.
try {
  $null = Invoke-RestMethod -Uri ($CloudUrl.TrimEnd('/') + '/api/auth/context') -TimeoutSec 15 -UseBasicParsing
} catch {
  Stop-With 15 'cloud' $_.Exception.Message
}

@{ check = 'all'; code = 0; detail = '' } | ConvertTo-Json -Compress | Write-Output
exit 0
