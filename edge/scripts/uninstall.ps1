#requires -RunAsAdministrator
<#
  uninstall.ps1 — stop & remove all edge services and the firewall rule.
  Run from an ELEVATED PowerShell:  .\scripts\uninstall.ps1 [-Purge]
  Does NOT delete artifacts\, caddy-data\, state\ or backups\ (so the CA root survives a reinstall).
  -Purge also removes the box's root CA from the trust store and the machine variables the scripts
  set; the installer's uninstaller then deletes the whole folder.
#>
param([switch] $Purge)
# Not SilentlyContinue: a skipped stop leaves a service holding handles on artifacts\.
$ErrorActionPreference = 'Stop'
$Root     = Split-Path -Parent $PSScriptRoot
$Services = Join-Path $Root 'services'
# Reverse of install. 'alloy' is always listed so a box that dropped env\alloy.env is cleaned too.
$order    = 'caddy','next','edge-node','postgres','alloy'

# First, so the updater cannot start an install while the services are being removed.
Unregister-ScheduledTask -TaskName 'Kuenta Updater' -Confirm:$false -ErrorAction SilentlyContinue

foreach ($name in $order) {
  $svcId = "kuentapos-$name"
  $exe   = Join-Path $Services "$name.exe"

  # Stop via SCM, not the wrapper exe, so a registration whose exe is already gone is still removed.
  if (Get-CimInstance Win32_Service -Filter "Name='$svcId'" -ErrorAction SilentlyContinue) {
    Write-Host "Stopping & removing service: $svcId"
    & sc.exe stop   $svcId | Out-Host
    try { (Get-Service $svcId).WaitForStatus('Stopped', (New-TimeSpan -Seconds 20)) }
    catch { Write-Warning "$svcId did not reach 'Stopped' within 20s; continuing to delete." }
    & sc.exe delete $svcId | Out-Host
  }

  # The wrapper process can hold <name>.exe briefly after 'Stopped', so retry the delete.
  if (Test-Path $exe) {
    $removed = $false
    foreach ($attempt in 1..10) {
      try { Remove-Item $exe -Force -ErrorAction Stop; $removed = $true; break }
      catch { Start-Sleep -Milliseconds 500 }
    }
    if (-not $removed) { Remove-Item $exe -Force }
  }
}

Get-NetFirewallRule -DisplayName 'Kuenta - Caddy (LAN tablets)' -ErrorAction SilentlyContinue |
  Remove-NetFirewallRule

if ($Purge) {
  # By thumbprint only: a developer PC can trust other Caddy CAs with the same name.
  foreach ($crt in (Join-Path $Root 'kuentapos-root-ca.crt'), (Join-Path $Root 'caddy-data\pki\authorities\local\root.crt')) {
    if (-not (Test-Path $crt)) { continue }
    $tp = (New-Object Security.Cryptography.X509Certificates.X509Certificate2($crt)).Thumbprint
    Remove-Item "Cert:\LocalMachine\Root\$tp" -ErrorAction SilentlyContinue
  }
  foreach ($name in 'EDGE_LAN_IP', 'EDGE_LAN_CIDR', 'EDGE_SITE_ADDR', 'EDGE_NODE_KEY_FILE', 'EDGE_TELEMETRY_BUFFER_DIR') {
    [Environment]::SetEnvironmentVariable($name, $null, 'Machine')
  }
  Write-Host 'Purged: root certificate and machine variables removed.'
  Write-Host 'Uninstalled.'
} else {
  Write-Host "Uninstalled. (artifacts\, caddy-data\, alloy-data\, state\ and backups\ left intact.)"
}
