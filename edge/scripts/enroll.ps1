<#
  enroll.ps1 -- enroll this PC as the organization's box with a code from "Instalar caja".

  Reads ONE JSON line on standard input: {"code","replace","name","cloud_url"}.
  Writes ONE JSON line on standard output, never the code:
    ok:    {"ok":true,"node_id","organization_id","organization_name","node_key","cloud_sync_url"}
    error: {"ok":false,"error":"bad_credentials|organization_inactive|no_connection|box_already_enrolled|enroll_failed","active_box":{...}|null}
  bad_credentials is an unknown, expired or used code; a 409 leaves the code usable for the retry with replace.
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

# A parse error quotes its input, the setup code; the installer logs this script's stderr.
# The installer passes the input in a single-use environment variable (it has no stdin to give); drop it
# before anything else starts so no child inherits it. Piped stdin still works for the runbook and CI.
$line = $env:KUENTA_SCRIPT_INPUT
Remove-Item Env:\KUENTA_SCRIPT_INPUT -ErrorAction SilentlyContinue
if (-not $line) { $line = [Console]::In.ReadLine() }
try { $in = $line | ConvertFrom-Json } catch { Done @{ ok = $false; error = 'enroll_failed' } 23 }
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

function Call($path, $payload) {
  return Invoke-RestMethod -Method Post -Uri "$base$path" -ContentType 'application/json' `
    -Body ($payload | ConvertTo-Json -Compress) -TimeoutSec 30 -UseBasicParsing
}

try {
  $r = Call '/api/edge/enroll-with-code' @{ code = $in.code; name = $in.name; replace = [bool]$in.replace }
  Done @{
    ok = $true; node_id = $r.node_id; organization_id = $r.organization_id
    organization_name = $r.organization_name; node_key = $r.node_key; cloud_sync_url = $r.cloud_sync_url
  } 0
} catch {
  $f = Get-Failure $_
  if (-not $f) { Done @{ ok = $false; error = 'no_connection' } 15 }
  if ($f.status -in 400, 401) { Done @{ ok = $false; error = 'bad_credentials' } 20 }
  if ($f.status -eq 403) { Done @{ ok = $false; error = 'organization_inactive' } 21 }
  if ($f.status -eq 409) {
    Done @{ ok = $false; error = 'box_already_enrolled'; active_box = $f.body.active_box } 22
  }
  Done @{ ok = $false; error = 'enroll_failed' } 23
} finally {
  $in = $null
}
