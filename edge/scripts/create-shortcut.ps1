<#
  create-shortcut.ps1 -- "Kuenta" on the all-users desktop and Start menu, opening the POS in
  its own window (Edge app mode on http://localhost:3000). localhost, not the LAN address, so
  the shortcut survives the PC changing address. Without Edge it is a .url to the same address.
  Run elevated (all-users locations).
#>
[CmdletBinding()]
param(
  [string] $Root = (Split-Path -Parent $PSScriptRoot),
  [string] $Url = 'http://localhost:3000',
  # Tests point this at a missing key to exercise the fallback.
  [string] $EdgeRegistryKey = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe',
  [string] $DesktopDir  = [Environment]::GetFolderPath('CommonDesktopDirectory'),
  [string] $StartMenuDir = [Environment]::GetFolderPath('CommonPrograms')
)
$ErrorActionPreference = 'Stop'
$icon = Join-Path $Root 'installer\kuenta.ico'
$edge = $null
try { $edge = (Get-ItemProperty -Path $EdgeRegistryKey -ErrorAction Stop).'(default)' } catch {}
if ($edge -and -not (Test-Path $edge)) { $edge = $null }

foreach ($dir in $DesktopDir, $StartMenuDir) {
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
  foreach ($stale in 'Kuenta.lnk', 'Kuenta.url') { Remove-Item (Join-Path $dir $stale) -Force -ErrorAction SilentlyContinue }
  if ($edge) {
    $sh = (New-Object -ComObject WScript.Shell).CreateShortcut((Join-Path $dir 'Kuenta.lnk'))
    $sh.TargetPath = $edge
    $sh.Arguments = "--app=$Url"
    $sh.IconLocation = "$icon,0"
    $sh.Description = 'Kuenta'
    $sh.Save()
  } else {
    Set-Content (Join-Path $dir 'Kuenta.url') -Encoding ascii -Value @('[InternetShortcut]', "URL=$Url", "IconFile=$icon", 'IconIndex=0')
  }
}
Write-Host "shortcut: $(if ($edge) { "Edge app mode ($edge)" } else { 'default browser (.url)' })"
