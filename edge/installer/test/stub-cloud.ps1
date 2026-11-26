<#
  stub-cloud.ps1 -- a tiny stand-in for the cloud, for the installer's CI test and for trying
  the wizard by hand. Payloads follow backend/docs/api (auth.md, edge-enrollment.md, sync.md).

    GET  /api/auth/context     {"mode":"cloud"}
    POST /api/edge/enroll-with-code   201 once for -Code (case, spaces and "-" ignored), 401 for
                               any other code; 409 when -Conflict, or after the first call,
                               unless "replace":true
    GET  /api/edge/status      {"version":"v0.0.0"}
    POST /api/sync/push, GET /api/sync/pull, GET /api/sync/pull/stock   empty valid pages

  Every request is appended to -LogFile as one JSON line {"method","path"}, so a test can count
  enroll calls. Stops when the file named by -StopFile appears, or on Ctrl+C.

  Run:  .\stub-cloud.ps1 -Port 18080 -LogFile C:\stub.log -StopFile C:\stub.stop [-Conflict]
#>
[CmdletBinding()]
param(
  [int]    $Port = 18080,
  [string] $LogFile = (Join-Path $env:TEMP 'stub-cloud.log'),
  [string] $StopFile = (Join-Path $env:TEMP 'stub-cloud.stop'),
  [switch] $Conflict,
  [string] $Code = 'STUB-C0DE'
)
$ErrorActionPreference = 'Stop'
$base = "http://127.0.0.1:$Port"
$listener = New-Object Net.HttpListener
$listener.Prefixes.Add("$base/")
$listener.Start()
Remove-Item $StopFile -Force -ErrorAction SilentlyContinue
$enrolled = $false
$nodeId = 'ce6ebef8-37cf-576b-b7a8-15aaaaae44a1'
$orgId  = 'bb0cad3e-be13-4bad-9a37-bba9525bfece'
Write-Host "stub cloud on $base"

function Send($ctx, [int] $status, $body) {
  $json = if ($null -eq $body) { '' } else { $body | ConvertTo-Json -Compress -Depth 6 }
  $bytes = [Text.Encoding]::UTF8.GetBytes($json)
  $ctx.Response.StatusCode = $status
  $ctx.Response.ContentType = 'application/json'
  $ctx.Response.Headers['Cache-Control'] = 'no-store'
  $ctx.Response.OutputStream.Write($bytes, 0, $bytes.Length)
  $ctx.Response.Close()
}

try {
  while (-not (Test-Path $StopFile)) {
    $task = $listener.GetContextAsync()
    while (-not $task.AsyncWaitHandle.WaitOne(500)) { if (Test-Path $StopFile) { return } }
    $ctx = $task.Result
    $method = $ctx.Request.HttpMethod
    $path = $ctx.Request.Url.AbsolutePath
    Add-Content $LogFile (@{ method = $method; path = $path } | ConvertTo-Json -Compress) -Encoding ascii
    $body = $null
    if ($ctx.Request.HasEntityBody) { $body = (New-Object IO.StreamReader($ctx.Request.InputStream)).ReadToEnd() | ConvertFrom-Json }
    $epoch = '2026-01-01T00:00:00Z'

    switch ("$method $path") {
      'GET /api/auth/context' { Send $ctx 200 @{ mode = 'cloud' } }
      'GET /api/edge/status'  { Send $ctx 200 @{ version = 'v0.0.0' } }
      'POST /api/edge/enroll-with-code' {
        $norm = { param($c) ("$c".ToUpper() -replace '[\s-]', '') }
        if ((& $norm $body.code) -ne (& $norm $Code)) { Send $ctx 401 @{ error = 'invalid or expired code' } }
        elseif (($Conflict -or $enrolled) -and -not $body.replace) {
          Send $ctx 409 @{ error = 'organization already has an active box'; code = 'box_already_enrolled'
            active_box = @{ name = 'Caja principal'; enrolled_at = $epoch; last_seen_at = $null } }
        } else {
          $enrolled = $true
          Send $ctx 201 @{ node_id = $nodeId; organization_id = $orgId; organization_name = 'Restaurante de Prueba'
            node_key = 'lgn_stubstubstubstubstubstubstubstubstubstub1'; cloud_sync_url = $base }
        }
      }
      'POST /api/sync/push'       { Send $ctx 200 @{ acked_op_ids = @(); acked_seqs = @() } }
      'GET /api/sync/pull'        { Send $ctx 200 @{ products = @(); users = @(); suppliers = @(); product_responsibilities = @(); product_ingredients = @(); cursor = $epoch } }
      'GET /api/sync/pull/stock'  { Send $ctx 200 @{ stock = @(); cursor = $epoch } }
      default                     { Send $ctx 404 @{ error = 'not found' } }
    }
  }
} finally {
  $listener.Stop()
}
