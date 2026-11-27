<#
  lock-root.ps1 -- make the install root writable only by Administrators and SYSTEM. The installer
  runs it before it copies a file (openspec/changes/edge-kuentapos-naming, design D2).

  The root is owned by Administrators, grants full control to Administrators and SYSTEM and read
  and execute to Users, and does not inherit from C:\. env\, state\, backups\ and caddy-data\ (the
  tablets' CA key) stay restricted to Administrators and SYSTEM throughout. Links found under the
  root are deleted, never followed. -Fresh empties a root that already existed: without a box,
  anything in it was put there by someone else.

  Prints one line per step. Exit codes: installer\exit-codes.json (0, 36).
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory)] [string] $Root,
  [switch] $Fresh
)
$ErrorActionPreference = 'Stop'

function Fail($msg) { Write-Output "error $msg"; exit 36 }
trap { Fail "line $($_.InvocationInfo.ScriptLineNumber): $($_.Exception.Message)" }

$Admins  = New-Object Security.Principal.SecurityIdentifier 'S-1-5-32-544'
$System  = New-Object Security.Principal.SecurityIdentifier 'S-1-5-18'
$Users   = New-Object Security.Principal.SecurityIdentifier 'S-1-5-32-545'
$Network = New-Object Security.Principal.SecurityIdentifier 'S-1-5-20'
$Restricted = 'env', 'state', 'backups', 'caddy-data'
# Any right that changes a folder, its contents or its ACL, including generic write and all.
$WriteRights = 0x000D0156 -bor 0x50000000

function Icacls([string[]] $IcaclsArgs) {
  $out = & icacls.exe @IcaclsArgs 2>&1 | Out-String
  Write-Output "icacls $($IcaclsArgs -join ' ') -> $LASTEXITCODE"
  if ($LASTEXITCODE -ne 0) { Fail "icacls $($IcaclsArgs -join ' '): $($out.Trim())" }
}

function Test-Link([string] $Path) {
  ([IO.File]::GetAttributes($Path) -band [IO.FileAttributes]::ReparsePoint) -ne 0
}

# Deletes the link itself: RemoveDirectory and DeleteFile never touch a link's target.
function Remove-Link([string] $Path) {
  if ([IO.File]::GetAttributes($Path) -band [IO.FileAttributes]::Directory) { [IO.Directory]::Delete($Path, $false) }
  else { [IO.File]::Delete($Path) }
  Write-Output "removed link $Path"
}

# Trusted: owned by Administrators or SYSTEM, and nobody else can write.
function Test-Trusted([string] $Dir) {
  try { $acl = [IO.Directory]::GetAccessControl($Dir) } catch { return $false }
  $owner = $acl.GetOwner([Security.Principal.SecurityIdentifier])
  if ($owner -ne $Admins -and $owner -ne $System) { return $false }
  foreach ($r in $acl.GetAccessRules($true, $true, [Security.Principal.SecurityIdentifier])) {
    if ($r.AccessControlType -ne 'Allow') { continue }
    if ($r.IdentityReference -in $Admins, $System, $Network) { continue }
    if (([int] $r.FileSystemRights -band $WriteRights) -ne 0) { return $false }
  }
  return $true
}

# Replaces the whole DACL in one write, so no moment exposes the folder.
function Set-ExactAcl([string] $Dir, [bool] $UsersRead) {
  $sec = New-Object Security.AccessControl.DirectorySecurity
  $sec.SetAccessRuleProtection($true, $false)
  $inherit = [Security.AccessControl.InheritanceFlags] 'ContainerInherit, ObjectInherit'
  $full = [Security.AccessControl.FileSystemRights]::FullControl
  foreach ($sid in $Admins, $System) {
    $sec.AddAccessRule((New-Object Security.AccessControl.FileSystemAccessRule($sid, $full, $inherit, 'None', 'Allow')))
  }
  if ($UsersRead) {
    $rx = [Security.AccessControl.FileSystemRights]::ReadAndExecute
    $sec.AddAccessRule((New-Object Security.AccessControl.FileSystemAccessRule($Users, $rx, $inherit, 'None', 'Allow')))
  }
  [IO.Directory]::SetAccessControl($Dir, $sec)
  Write-Output "exact acl $Dir"
}

# /L acts on a link itself, so a folder swapped for a link meanwhile is not followed.
# A link found here is deleted; the caller checks that the folder still exists.
function Lock-Dir([string] $Dir, [bool] $IsRoot) {
  Icacls @($Dir, '/setowner', '*S-1-5-32-544', '/L')
  if (Test-Link $Dir) {
    if ($IsRoot) { Fail "$Dir is a link" }
    Remove-Link $Dir
    return
  }
  $restricted = ((Split-Path -Parent $Dir) -eq $Root) -and ((Split-Path -Leaf $Dir) -in $Restricted)
  if ($IsRoot) { Set-ExactAcl $Dir $true }
  elseif ($restricted) { Set-ExactAcl $Dir $false }
  else { Icacls @($Dir, '/reset', '/L') }
}

if ((Test-Path -LiteralPath $Root) -and (Test-Link $Root)) { Fail "$Root is a link" }
$existed = Test-Path -LiteralPath $Root
New-Item -ItemType Directory -Force -Path $Root | Out-Null
Lock-Dir $Root $true

# Top-down: each folder is locked before it is listed, so nothing new can appear in it.
$queue = New-Object Collections.Queue
$queue.Enqueue($Root)
while ($queue.Count -gt 0) {
  $dir = $queue.Dequeue()
  try { $entries = [IO.Directory]::GetFileSystemEntries($dir) }
  catch { Lock-Dir $dir $false; $entries = [IO.Directory]::GetFileSystemEntries($dir) }
  foreach ($entry in $entries) {
    if (Test-Link $entry) { Remove-Link $entry; continue }
    if (-not ([IO.File]::GetAttributes($entry) -band [IO.FileAttributes]::Directory)) { continue }
    $restrictedTop = ($dir -eq $Root) -and ((Split-Path -Leaf $entry) -in $Restricted)
    if ($restrictedTop -or -not (Test-Trusted $entry)) {
      Lock-Dir $entry $false
      if (-not (Test-Path -LiteralPath $entry)) { continue }
    }
    $queue.Enqueue($entry)
  }
}

if ($Fresh -and $existed) {
  foreach ($e in Get-ChildItem -LiteralPath $Root -Force) {
    Remove-Item -LiteralPath $e.FullName -Recurse -Force
    Write-Output "removed $($e.FullName)"
  }
}

# No link is left, so recursing is safe. Restricted folders keep their own DACL and only their
# contents inherit it.
Icacls @($Root, '/setowner', '*S-1-5-32-544', '/T', '/C', '/L', '/Q')
foreach ($e in Get-ChildItem -LiteralPath $Root -Force) {
  if ($e.PSIsContainer -and $e.Name -in $Restricted) {
    if (Get-ChildItem -LiteralPath $e.FullName -Force) { Icacls @("$($e.FullName)\*", '/reset', '/T', '/C', '/L', '/Q') }
  } else {
    Icacls @($e.FullName, '/reset', '/T', '/C', '/L', '/Q')
  }
}

$keyFile = Join-Path $Root 'env\alloy-node-key'
if (Test-Path -LiteralPath $keyFile) {
  Icacls @($keyFile, '/inheritance:r', '/grant:r', '*S-1-5-18:(R)', '*S-1-5-32-544:(F)', '/L')
}
Write-Output 'ok'
exit 0
