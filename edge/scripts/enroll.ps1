<#
  enroll.ps1 -- sign in to the cloud and enroll this PC as the organization's box.

  Reads ONE JSON line on standard input: {"email","password","replace","name","cloud_url"}.
  Writes ONE JSON line on standard output, never the password or the user token:
    ok:    {"ok":true,"node_id","organization_id","organization_name","node_key","cloud_sync_url"}
    error: {"ok":false,"error":"bad_credentials|not_admin|no_connection|box_already_enrolled|enroll_failed","active_box":{...}|null}
  Exit codes: installer\exit-codes.json (0, 15, 20, 21, 22, 23).
  Nothing is written to disk and no transcript is kept.
#>
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

function Done([hashtable] $result, [int] $code) {
  # ASCII only: the wizard reads this through the console code page, which would garble accents.
  $json = $result | ConvertTo-Json -Compress -Depth 5
  $json = [regex]::Replace($json, '[^\x00-\x7F]', { param($m) '\u{0:x4}' -f [int][char]$m.Value })
  Write-Output $json
  exit $code
}

$in = [Console]::In.ReadLine() | ConvertFrom-Json
$base = "$($in.cloud_url)".TrimEnd('/')

# Status code and parsed body of a failed call, for PS 5.1 and 7 alike.
function Get-Failure($err) {
  $resp = $err.Exception.Response
  if (-not $resp) { return $null }
  $body = $null
  try {
    if ($err.ErrorDetails.Message) { $body = $err.ErrorDetails.Message | ConvertFrom-Json }
    else { $body = (New-Object IO.StreamReader($resp.GetResponseStream())).ReadToEnd() | ConvertFrom-Json }
  } catch {}
  return @{ status = [int]$resp.StatusCode; body = $body }
}

function Call($path, $payload, $token) {
  $headers = @{}
  if ($token) { $headers['Authorization'] = "Bearer $token" }
  return Invoke-RestMethod -Method Post -Uri "$base$path" -Headers $headers -ContentType 'application/json' `
    -Body ($payload | ConvertTo-Json -Compress) -TimeoutSec 30 -UseBasicParsing
}

$token = $null
try {
  $signin = Call '/api/auth/signin' @{ email = $in.email; password = $in.password } $null
  $token = $signin.token
} catch {
  $f = Get-Failure $_
  if (-not $f) { Done @{ ok = $false; error = 'no_connection' } 15 }
  if ($f.status -in 400, 401) { Done @{ ok = $false; error = 'bad_credentials' } 20 }
  Done @{ ok = $false; error = 'enroll_failed' } 23
}

try {
  $r = Call '/api/edge/enroll' @{ name = $in.name; replace = [bool]$in.replace } $token
  Done @{
    ok = $true; node_id = $r.node_id; organization_id = $r.organization_id
    organization_name = $r.organization_name; node_key = $r.node_key; cloud_sync_url = $r.cloud_sync_url
  } 0
} catch {
  $f = Get-Failure $_
  if (-not $f) { Done @{ ok = $false; error = 'no_connection' } 15 }
  if ($f.status -eq 403) { Done @{ ok = $false; error = 'not_admin' } 21 }
  if ($f.status -eq 409) {
    Done @{ ok = $false; error = 'box_already_enrolled'; active_box = $f.body.active_box } 22
  }
  Done @{ ok = $false; error = 'enroll_failed' } 23
} finally {
  $token = $null
  $in = $null
}
