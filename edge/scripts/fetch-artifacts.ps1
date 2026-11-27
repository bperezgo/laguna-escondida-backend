<#
  fetch-artifacts.ps1 -- download the pinned binaries/bundles into artifacts\.
  Reads versions.json.

  INFRASTRUCTURE (Caddy, Alloy, Node, Postgres) are public upstream downloads.
  APP artifacts (edge-node, next) are the Windows builds CI attaches to an edge/vX.Y.Z GitHub
  Release of the monorepo (see .github/workflows/release-edge.yml); both carry that one
  version. Without -Tags the newest edge release (compatible with the live cloud, when it can
  be reached) is used. The download is checked against the release's .sha256 (and -Sha256,
  the manifest's value, when given) and REFUSED on mismatch. Nothing is compiled on the box
  (no Go / pnpm / source needed). scripts\build-artifacts.ps1 remains as a developer fallback
  for unreleased code.

  The installed version is recorded in artifacts\<app>\RELEASE, so re-running is a no-op until
  a newer edge release exists (or -Force). A newer release installed from the POS by
  scripts\updater.ps1 (-Tags) is kept rather than downgraded.

  Run:  .\scripts\fetch-artifacts.ps1                         # everything
        .\scripts\fetch-artifacts.ps1 -Only node,postgres     # subset
        .\scripts\fetch-artifacts.ps1 -Only next -DownloadOnly # download + verify into the cache only
        .\scripts\fetch-artifacts.ps1 -Only next -Tags @{ next = 'v1.3.0' }   # a specific edge version

  Installer build (release-edge.yml): -VerifyThirdParty checks every vendor download against the
  sha256 pinned in versions.json and fails on a mismatch; -AssetDir takes the edge-node and POS
  zips (each with its .sha256) from a folder instead of the GitHub release, which does not exist
  yet at that point.
#>
[CmdletBinding()]
param(
  # Which artifacts to fetch. Default: all of them.
  [ValidateSet('winsw','caddy','alloy','node','postgres','edge-node','next')]
  [string[]] $Only = @('winsw','caddy','alloy','node','postgres','edge-node','next'),
  [switch]   $Force,        # re-download/re-install even if the target already exists
  # App artifacts only: download + verify into the local cache, do not touch artifacts\.
  # update.ps1 uses this BEFORE stopping a service, so the download is not downtime.
  [switch]   $DownloadOnly,
  # App artifacts only: edge versions (vX.Y.Z) to use instead of the newest release.
  [hashtable] $Tags = @{},
  # App artifacts only: expected sha256 per app (from the release's manifest.json); a zip that
  # does not match is refused even if it matches its own .sha256 file.
  [hashtable] $Sha256 = @{},
  # Check each third-party download against versions.json's "sha256" pin; refuse a mismatch.
  [switch]   $VerifyThirdParty,
  # App artifacts only: a folder holding edge-node-windows-amd64-<tag>.zip and
  # next-standalone-windows-<tag>.zip with their .sha256 files, used instead of the release.
  [string]   $AssetDir
)

$ErrorActionPreference = 'Stop'
$Root      = Split-Path -Parent $PSScriptRoot
$Artifacts = Join-Path $Root 'artifacts'
$v         = Get-Content (Join-Path $Root 'versions.json') | ConvertFrom-Json
$tmp       = Join-Path $env:TEMP 'laguna-fetch'
# Verified app release zips, kept by tag so a staged download survives until install.
$cache     = Join-Path $Root 'artifacts\.cache'
. (Join-Path $PSScriptRoot 'releases.ps1')   # release repo, Get-EdgeBaseUrl, Select-EdgeRelease
$ProgressPreference = 'SilentlyContinue'   # PS 5.1's progress bar makes Invoke-WebRequest crawl

New-Item -ItemType Directory -Force -Path $Artifacts, $tmp, $cache | Out-Null
function Want($name) { $Only -contains $name }
function Step($m) { Write-Host ("`n==> " + $m) -ForegroundColor Cyan }

# Refuse a third-party download whose sha256 is not the one versions.json pins for $name.
function Assert-ThirdParty($name, $file) {
  if (-not $VerifyThirdParty) { return }
  $pin = "$($v.sha256.$name)".ToLower()
  if ($pin -notmatch '^[0-9a-f]{64}$') { throw "versions.json has no sha256 pin for $name." }
  $actual = (Get-FileHash $file -Algorithm SHA256).Hash.ToLower()
  if ($actual -ne $pin) {
    Remove-Item $file -Force -ErrorAction SilentlyContinue
    throw "CHECKSUM MISMATCH for third-party component ${name}: pinned $pin, downloaded $actual. Refusing to continue."
  }
  Write-Host "  sha256 ok ($name, pinned)" -ForegroundColor DarkGray
}

function Get-Zip($name, $url, $zipName) {
  $zip = Join-Path $tmp $zipName
  Write-Host "  downloading $url"
  Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing
  Assert-ThirdParty $name $zip
  $out = Join-Path $tmp ([IO.Path]::GetFileNameWithoutExtension($zipName))
  if (Test-Path $out) { Remove-Item $out -Recurse -Force }
  Expand-Archive -Path $zip -DestinationPath $out -Force
  return $out
}

# --- WinSW (service wrapper; install.ps1 downloads it itself when this was not run) ------
if (Want 'winsw') {
  Step "WinSW $($v.winsw)"
  $dest = Join-Path $Root 'services\winsw.exe'
  if ((Test-Path $dest) -and -not $Force) {
    Write-Host "  present -- skip."
  } else {
    New-Item -ItemType Directory -Force -Path (Split-Path $dest) | Out-Null
    $dl = Join-Path $tmp 'WinSW-x64.exe'
    Invoke-WebRequest -Uri "https://github.com/winsw/winsw/releases/download/v$($v.winsw)/WinSW-x64.exe" -OutFile $dl -UseBasicParsing
    Assert-ThirdParty 'winsw' $dl
    Copy-Item $dl $dest -Force
    Write-Host "  -> $dest" -ForegroundColor Green
  }
}

# --- Caddy (official build) -------------------------------------------------------
if (Want 'caddy') {
  Step "Caddy $($v.caddy)"
  $dest = Join-Path $Artifacts 'caddy'
  if ((Test-Path (Join-Path $dest 'caddy.exe')) -and -not $Force) {
    Write-Host "  present -- skip (use -Force to refresh)."
  } else {
    $ex = Get-Zip 'caddy' "https://github.com/caddyserver/caddy/releases/download/v$($v.caddy)/caddy_$($v.caddy)_windows_amd64.zip" "caddy.zip"
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Copy-Item (Join-Path $ex 'caddy.exe') (Join-Path $dest 'caddy.exe') -Force
    Write-Host "  -> $dest\caddy.exe" -ForegroundColor Green
  }
}

# --- Grafana Alloy (telemetry sidecar) --------------------------------------------
if (Want 'alloy') {
  Step "Grafana Alloy $($v.alloy)"
  $dest = Join-Path $Artifacts 'alloy'
  if ((Test-Path (Join-Path $dest 'alloy.exe')) -and -not $Force) {
    Write-Host "  present -- skip."
  } else {
    # The zip holds a single 'alloy-windows-amd64.exe'; rename it to alloy.exe so
    # services\alloy.xml can point at a stable path across version bumps.
    $ex  = Get-Zip 'alloy' "https://github.com/grafana/alloy/releases/download/v$($v.alloy)/alloy-windows-amd64.exe.zip" "alloy.zip"
    $src = Join-Path $ex 'alloy-windows-amd64.exe'
    if (-not (Test-Path $src)) { throw "Unexpected Alloy archive layout under $ex" }
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Copy-Item $src (Join-Path $dest 'alloy.exe') -Force
    Write-Host "  -> $dest\alloy.exe" -ForegroundColor Green
  }
}

# --- Node runtime -----------------------------------------------------------------
if (Want 'node') {
  Step "Node $($v.node)"
  $dest = Join-Path $Artifacts 'node'
  if ((Test-Path (Join-Path $dest 'node.exe')) -and -not $Force) {
    Write-Host "  present -- skip."
  } else {
    $folder = "node-v$($v.node)-win-x64"
    $ex = Get-Zip 'node' "https://nodejs.org/dist/v$($v.node)/$folder.zip" "node.zip"
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    # The zip extracts to <folder>\node.exe (+ npm etc). Copy the whole runtime.
    Copy-Item (Join-Path $ex "$folder\*") $dest -Recurse -Force
    Write-Host "  -> $dest\node.exe" -ForegroundColor Green
  }
}

# --- Postgres (EnterpriseDB portable binaries) ------------------------------------
if (Want 'postgres') {
  Step "Postgres $($v.postgres)"
  $dest = Join-Path $Artifacts 'postgres'
  if ((Test-Path (Join-Path $dest 'bin\postgres.exe')) -and -not $Force) {
    Write-Host "  present -- skip."
  } else {
    # EDB 'binaries' zip extracts to a top-level 'pgsql\' (bin, lib, share). Flatten
    # it into artifacts\postgres\ so postgres.xml finds artifacts\postgres\bin\postgres.exe.
    $ex = Get-Zip 'postgres' "https://get.enterprisedb.com/postgresql/postgresql-$($v.postgres)-1-windows-x64-binaries.zip" "pg.zip"
    $pgsql = Join-Path $ex 'pgsql'
    if (-not (Test-Path $pgsql)) { throw "Unexpected Postgres archive layout under $ex" }
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Copy-Item (Join-Path $pgsql '*') $dest -Recurse -Force
    Write-Host "  -> $dest\bin\postgres.exe" -ForegroundColor Green
    Write-Host "  NOTE: the data dir is NOT created here. Initialize it with scripts\init-postgres.ps1." -ForegroundColor Yellow
  }
}

# --- Visual C++ runtime (Postgres's binaries need it and do not ship it) -------------
# The installer runs it; a PC that never got it from other software cannot start pg_ctl.
if (Want 'postgres') {
  Step 'Visual C++ Redistributable (x64)'
  $dest = Join-Path $Artifacts 'vcredist\VC_redist.x64.exe'
  if ((Test-Path $dest) -and -not $Force) {
    Write-Host "  present -- skip."
  } else {
    New-Item -ItemType Directory -Force -Path (Split-Path $dest) | Out-Null
    $dl = Join-Path $tmp 'VC_redist.x64.exe'
    Invoke-WebRequest -Uri $v.vcredist_url -OutFile $dl -UseBasicParsing
    Assert-ThirdParty 'vcredist' $dl
    Copy-Item $dl $dest -Force
    Write-Host "  -> $dest" -ForegroundColor Green
  }
}

# --- App artifacts (edge-node, next) from the edge GitHub Release -----------------
# Asset names match .github/workflows/release-edge.yml.
$AppAssets = @{
  'edge-node' = { param($tag) "edge-node-windows-amd64-$tag.zip" }
  'next'      = { param($tag) "next-standalone-windows-$tag.zip" }
}

# Download <asset> and <asset>.sha256 for $tag into the cache and verify. Returns the zip
# path. A cached zip is re-verified, never trusted blindly; a bad one is deleted.
function Get-VerifiedRelease($app, $tag) {
  if ($tag -notmatch '^v\d+\.\d+\.\d+$') {
    throw ("$app '$tag' is not an edge version (vX.Y.Z). Pass -Tags @{ $app = 'v1.3.0' } " +
           "or build from source with scripts\build-artifacts.ps1 -Only $app.")
  }
  $asset = & $AppAssets[$app] $tag
  $base  = $null
  $zip   = Join-Path $cache $asset
  $sumF  = "$zip.sha256"

  if ($AssetDir) {
    # Local folder: no network, same sha256 check as a download.
    Write-Host "  $AssetDir $tag"
    foreach ($f in $asset, "$asset.sha256") {
      if (-not (Test-Path (Join-Path $AssetDir $f))) { throw "$f not found in -AssetDir $AssetDir." }
    }
    Copy-Item (Join-Path $AssetDir "$asset.sha256") $sumF -Force
    Copy-Item (Join-Path $AssetDir $asset) $zip -Force
  } else {
    $base = Get-EdgeBaseUrl $tag
    Write-Host "  $ReleaseRepo $EdgePrefix$tag"
    try {
      Invoke-WebRequest -Uri "$base/$asset.sha256" -OutFile $sumF -UseBasicParsing
    } catch {
      # Offline: a zip verified earlier can still be reinstalled, e.g. by an updater rollback.
      if (-not (Test-Path $zip) -or -not (Test-Path $sumF)) { throw }
      Write-Host "  offline -- using the cached checksum" -ForegroundColor Yellow
    }
  }
  $expected = ((Get-Content $sumF -Raw).Trim() -split '\s+')[0].ToLower()
  if ($expected -notmatch '^[0-9a-f]{64}$') { throw "$asset.sha256 is not a sha256 checksum." }
  if ($Sha256.ContainsKey($app) -and "$($Sha256[$app])".ToLower() -ne $expected) {
    throw "MANIFEST MISMATCH for ${asset}: manifest says $($Sha256[$app]), the release's .sha256 says $expected. Refusing to install."
  }

  if (-not $AssetDir -and (-not (Test-Path $zip) -or ((Get-FileHash $zip -Algorithm SHA256).Hash.ToLower() -ne $expected))) {
    Write-Host "  downloading $base/$asset"
    Invoke-WebRequest -Uri "$base/$asset" -OutFile $zip -UseBasicParsing
  } else {
    Write-Host "  cached (checksum ok)"
  }
  $actual = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
  if ($actual -ne $expected) {
    Remove-Item $zip -Force -ErrorAction SilentlyContinue
    throw "CHECKSUM MISMATCH for $asset (expected $expected, got $actual). Refusing to install."
  }
  Write-Host "  sha256 ok  $actual" -ForegroundColor DarkGray
  return $zip
}

function Get-InstalledRelease($dest) {
  $f = Join-Path $dest 'RELEASE'
  if (Test-Path $f) { return (Get-Content $f -Raw).Trim() }
  return $null
}

# The edge version to install when -Tags does not name one: the newest release the live cloud
# can sync with (any, when the cloud cannot be reached, e.g. a first install without WAN).
function Get-DefaultEdgeVersion {
  $cloud = $null
  try {
    $envFile = Join-Path $Root 'env\edge-node.env'
    $line = Get-Content $envFile -ErrorAction Stop | Where-Object { $_ -match '^\s*CLOUD_SYNC_URL\s*=' } | Select-Object -First 1
    $url = (($line -split '=', 2)[1] -replace '\s+#.*$', '').Trim().Trim('"', "'").TrimEnd('/')
    $cloud = "$((Invoke-RestMethod "$url/api/edge/status" -TimeoutSec 10 -UseBasicParsing).version)"
  } catch { Write-Host "  cloud version unknown ($($_.Exception.Message)); not checking min_cloud" -ForegroundColor Yellow }
  # A release without a readable manifest (e.g. one a failed run left empty) is skipped, as the updater does.
  $r = Select-EdgeRelease (Get-EdgeReleases) $null $cloud {
    param($r)
    try { (Get-EdgeManifest (Get-EdgeVersion $r.tag_name)).min_cloud } catch { $null }
  }
  if (-not $r) { throw "no edge release is available$(if ($cloud) { " for cloud $cloud" })." }
  return Get-EdgeVersion $r.tag_name
}

foreach ($a in 'edge-node','next') {
  if (-not (Want $a)) { continue }
  if ($Tags.ContainsKey($a)) { $tag = "$($Tags[$a])" }
  elseif ($AssetDir) {
    $m = @(Get-ChildItem $AssetDir -Filter (& $AppAssets[$a] '*') | Where-Object { $_.Name -match 'v\d+\.\d+\.\d+\.zip$' })
    if ($m.Count -ne 1) { throw "-AssetDir $AssetDir must hold exactly one $a zip; found $($m.Count)." }
    $tag = [regex]::Match($m[0].Name, 'v\d+\.\d+\.\d+').Value
  }
  else {
    if (-not $defaultVersion) { $defaultVersion = Get-DefaultEdgeVersion }
    $tag = $defaultVersion
  }
  $dest = Join-Path $Artifacts $a
  Step "App artifact: $a $tag"
  $installed = Get-InstalledRelease $dest
  if (-not $DownloadOnly -and -not $Force) {
    if ($installed -eq $tag) {
      Write-Host "  $tag already installed -- skip (use -Force to reinstall)."
      continue
    }
    if (-not $Tags.ContainsKey($a) -and (Test-NewerTag $installed $tag)) {
      Write-Host "  $installed (installed from the POS) is newer than $tag -- skip."
      continue
    }
  }
  $zip = Get-VerifiedRelease $a $tag
  if ($DownloadOnly) { Write-Host "  staged: $zip" -ForegroundColor Green; continue }

  # Unpack to a scratch dir first, check the layout, THEN replace the live folder.
  $ex = Join-Path $cache "extract-$a-$tag"   # same volume as artifacts\, so Move-Item works
  if (Test-Path $ex) { Remove-Item $ex -Recurse -Force }
  Expand-Archive -Path $zip -DestinationPath $ex -Force

  if ($a -eq 'edge-node') {
    $src = Join-Path $ex 'edge-node.exe'
    if (-not (Test-Path $src)) { throw "Unexpected edge-node archive layout under $ex" }
    # Replace only the exe: artifacts\edge-node\.env (per-box config) stays as it is.
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Copy-Item $src (Join-Path $dest 'edge-node.exe') -Force
  } else {
    foreach ($p in 'server.js', '.next\static', 'node_modules\next\package.json') {
      if (-not (Test-Path (Join-Path $ex $p))) { throw "Unexpected next archive layout: $p missing under $ex" }
    }
    # Whole-folder swap: never mix chunks from two builds. Nothing per-box lives in here.
    if (Test-Path $dest) { Remove-Item $dest -Recurse -Force }
    Move-Item $ex $dest
  }
  Set-Content (Join-Path $dest 'RELEASE') $tag -Encoding ascii
  Write-Host "  -> $dest  ($tag)" -ForegroundColor Green
}

$verb = 'Fetched'; if ($DownloadOnly) { $verb = 'Staged' }
Write-Host "`nDone. ${verb}: $($Only -join ', ')" -ForegroundColor Green
