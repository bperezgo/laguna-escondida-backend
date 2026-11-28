<#
  write-box-config.ps1 -- build env\box.env and env\edge-node.env for a freshly enrolled box.

  Reads ONE JSON line on standard input: the enrollment result of enroll.ps1
  (node_id, organization_id, node_key, cloud_sync_url). Values are taken as given. The node key
  reaches disk only here. The database password is generated here (32 random bytes).
  Then env\ is restricted to Administrators and SYSTEM.

  Refuses to run when env\edge-node.env already holds a NODE_ID: an existing box is never
  reconfigured. Exit codes: installer\exit-codes.json (0, 31, 37). On failure, stdout holds {"ok":false,"error"}.
  The network (-LanIp, -LanCidr, -LanHost) is checked by lan-config.ps1; blanks take its defaults.
#>
[CmdletBinding()]
param(
  [string] $Root,
  [string] $LanIp,
  [string] $LanCidr,
  [string] $LanHost
)
$ErrorActionPreference = 'Stop'
# Windows PowerShell 5.1 leaves $PSScriptRoot empty in param() defaults.
if (-not $Root) { $Root = Split-Path -Parent $PSScriptRoot }

# Dot-sourcing binds lan-config's own -LanIp/-LanCidr/-LanHost in this scope, so pass ours through.
. (Join-Path $PSScriptRoot 'lan-config.ps1') -NoRun -LanIp $LanIp -LanCidr $LanCidr -LanHost $LanHost

# The installer reads only stdout, so failures are reported there.
function Fail($msg, [int]$code = 31) { ConvertTo-AsciiJson ([ordered]@{ ok = $false; error = $msg }) | Write-Output; exit $code }
trap { Fail "line $($_.InvocationInfo.ScriptLineNumber): $($_.Exception.Message)" }

# Parsed apart so a parse error, which quotes its input, never prints the node key.
# The installer passes the input in a single-use environment variable (it has no stdin to give); drop it
# before anything else starts so no child inherits it. Piped stdin still works for the runbook and CI.
$line = $env:KUENTA_SCRIPT_INPUT
Remove-Item Env:\KUENTA_SCRIPT_INPUT -ErrorAction SilentlyContinue
if (-not $line) { $line = [Console]::In.ReadLine() }
try { $enr = $line | ConvertFrom-Json } catch { Fail 'the enrollment result is not JSON' }
foreach ($k in 'node_id', 'organization_id', 'node_key', 'cloud_sync_url') {
  if (-not $enr.$k) { Fail "enrollment result lacks $k" }
}
$envDir = Join-Path $Root 'env'
$nodeEnv = Join-Path $envDir 'edge-node.env'
if ((Test-Path $nodeEnv) -and (Select-String -Path $nodeEnv -Pattern '^\s*NODE_ID\s*=\s*\S' -Quiet)) {
  Fail 'env\edge-node.env already holds a box identity; not overwriting it'
}

$net = Resolve-LanIdentity $LanIp $LanCidr $LanHost
if (-not $net.ok) { Fail $net.error 37 }

# --- database password: 32 random bytes, base64url -------------------------------------
$rnd = New-Object byte[] 32
$rng = [Security.Cryptography.RandomNumberGenerator]::Create(); $rng.GetBytes($rnd); $rng.Dispose()
$dbPassword = [Convert]::ToBase64String($rnd).TrimEnd('=').Replace('+', '-').Replace('/', '_')

New-Item -ItemType Directory -Force -Path $envDir | Out-Null
Set-Content (Join-Path $envDir 'box.env') -Encoding ascii -Value @("EDGE_LAN_IP=$($net.lan_ip)", "EDGE_LAN_CIDR=$($net.lan_cidr)", "EDGE_LAN_HOST=$($net.lan_host)")

# Same approach as the CI smoke test: start from the template so a variable the backend
# starts requiring, or a renamed one, fails the build instead of a customer's box.
$set = [ordered]@{
  PORT = '8080'
  DB_HOST = '127.0.0.1'; DB_PORT = '5432'; DB_USER = 'laguna'; DB_PASSWORD = $dbPassword
  DB_NAME = 'laguna_escondida'; DB_SSLMODE = 'disable'
  ORGANIZATION_ID = $enr.organization_id; NODE_ID = $enr.node_id
  CLOUD_SYNC_URL = $enr.cloud_sync_url; NODE_SYNC_KEY = $enr.node_key
  PRINTER_TRANSPORT = 'none'; PRINTER_TARGET = ''
  OBSERVABILITY_ENABLED = 'false'
}
$out = @(); $seen = @{}; $required = @()
foreach ($line in Get-Content (Join-Path $envDir 'edge-node.env.example')) {
  # A key may be commented out in the template (NODE_ID); only a bare "# KEY=" line counts.
  if ($line -match '^\s*#?\s*([A-Z][A-Z0-9_]*)=' -and $set.Contains($Matches[1]) -and -not $seen[$Matches[1]]) {
    $k = $Matches[1]; $seen[$k] = $true
    $out += "$k=$($set[$k])"; continue
  }
  if ($line -match '^\s*([A-Z][A-Z0-9_]*)=.*\(required\)') { $required += $Matches[1] }
  $out += $line
}
foreach ($k in $set.Keys) { if (-not $seen[$k]) { $out += "$k=$($set[$k])" } }
$out | Set-Content $nodeEnv -Encoding ascii

# Every required key must now hold a value and no placeholder may remain.
$written = @{}
foreach ($l in Get-Content $nodeEnv) { if ($l -match '^([A-Z][A-Z0-9_]*)=(.*)$') { $written[$Matches[1]] = ($Matches[2] -replace '\s+#.*$', '').Trim() } }
$missing = @(($required + 'ORGANIZATION_ID', 'NODE_ID', 'NODE_SYNC_KEY', 'CLOUD_SYNC_URL', 'DB_PASSWORD') | Where-Object { -not $written[$_] } | Sort-Object -Unique)
if ($missing) { Remove-Item $nodeEnv -Force; Fail "no value for: $($missing -join ', ')" }
if (Select-String -Path $nodeEnv -Pattern 'CHANGE_ME' -Quiet) { Remove-Item $nodeEnv -Force; Fail 'CHANGE_ME left in edge-node.env' }

# Administrators and SYSTEM only, inheritance removed (S-1-5-32-544 / S-1-5-18, locale-proof).
& icacls.exe $envDir /inheritance:r /grant:r '*S-1-5-32-544:(OI)(CI)F' '*S-1-5-18:(OI)(CI)F' | Out-Null
if ($LASTEXITCODE -ne 0) { Fail "icacls failed ($LASTEXITCODE)" }

ConvertTo-AsciiJson ([ordered]@{ lan_ip = $net.lan_ip; lan_cidr = $net.lan_cidr; lan_host = $net.lan_host }) | Write-Output
exit 0
