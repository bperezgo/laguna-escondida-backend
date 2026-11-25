<#
  build-installer.ps1 -- build KuentaSetup-<version>.exe from the tracked files of edge\ plus the
  release's apps and the pinned third-party programs. Used by release-edge.yml and edge-ci.yml.

    .\build-installer.ps1 -Version v1.4.0 -CloudUrl https://api.example.com -OutDir C:\out `
        -AssetDir C:\parts           # the release's edge-node and next zips (+ .sha256); omit to
                                     # take the published edge release named by -Version
    -SignPfx <file> -SignPassword <pw>   sign the installer and its uninstaller (all or nothing)

  Third-party downloads are checked against edge\versions.json (fetch-artifacts.ps1
  -VerifyThirdParty); a mismatch fails the build and nothing is produced.
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory)] [ValidatePattern('^v\d+\.\d+\.\d+$')] [string] $Version,
  [Parameter(Mandatory)] [string] $CloudUrl,
  [Parameter(Mandatory)] [string] $OutDir,
  [string] $AssetDir,
  [string] $StageDir = (Join-Path $env:RUNNER_TEMP 'kuenta-stage'),
  [string] $SignPfx,
  [string] $SignPassword,
  # Inno Setup is installed at this version when the runner image lacks it.
  [string] $InnoVersion = '6.4.3'
)
$ErrorActionPreference = 'Stop'
$edge = Split-Path -Parent $PSScriptRoot
function Step($m) { Write-Host ("`n==> " + $m) -ForegroundColor Cyan }

Step 'exit-codes.inc matches exit-codes.json'
$inc = Join-Path $PSScriptRoot 'exit-codes.inc'
$before = (Get-FileHash $inc).Hash
& (Join-Path $PSScriptRoot 'gen-exit-codes.ps1')
if ((Get-FileHash $inc).Hash -ne $before) { throw 'exit-codes.inc was out of date; regenerate it with installer\gen-exit-codes.ps1 and commit.' }

Step "Staging tracked edge files at $StageDir"
if (Test-Path $StageDir) { Remove-Item $StageDir -Recurse -Force }
New-Item -ItemType Directory -Force -Path $StageDir | Out-Null
$skip = '^(docs|ai-plan|incidents|installer/test)/'
foreach ($f in (& git -C $edge ls-files)) {
  if ($f -match $skip -or $f -match '\.md$') { continue }
  $dest = Join-Path $StageDir $f
  New-Item -ItemType Directory -Force -Path (Split-Path $dest) | Out-Null
  Copy-Item (Join-Path $edge $f) $dest -Force
}

Step 'Fetching programs (third-party sha256 pinned)'
$fetch = @{ VerifyThirdParty = $true }
if ($AssetDir) { $fetch.AssetDir = $AssetDir }
else { $fetch.Tags = @{ 'edge-node' = $Version; next = $Version } }   # an already published release
& (Join-Path $StageDir 'scripts\fetch-artifacts.ps1') @fetch
if (-not (Test-Path (Join-Path $StageDir 'services\winsw.exe'))) { throw 'winsw.exe missing from the stage' }
foreach ($app in 'edge-node', 'next') {
  $got = (Get-Content (Join-Path $StageDir "artifacts\$app\RELEASE") -Raw).Trim()
  if ($got -ne $Version) { throw "$app is $got, expected ${Version}: the zips in -AssetDir are not this release's." }
}

Step 'Trimming Postgres to bin, lib, share'
Get-ChildItem (Join-Path $StageDir 'artifacts\postgres') -Force |
  Where-Object { $_.Name -notin 'bin', 'lib', 'share' } | Remove-Item -Recurse -Force
Remove-Item (Join-Path $StageDir 'artifacts\.cache') -Recurse -Force -ErrorAction SilentlyContinue

Step 'Locating Inno Setup'
$iscc = Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe'
if (-not (Test-Path $iscc)) {
  & choco install innosetup --version=$InnoVersion -y --no-progress
  if ($LASTEXITCODE -ne 0) { throw "choco install innosetup failed ($LASTEXITCODE)" }
}
if (-not (Test-Path $iscc)) { throw "ISCC.exe not found at $iscc" }
& $iscc /? 2>&1 | Select-Object -First 1 | Write-Host

Step 'Compiling'
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$isccArgs = @("/DAppVersion=$Version", "/DStageDir=$StageDir", "/DCloudUrl=$CloudUrl", "/O$OutDir")
$signed = $false
if ($SignPfx) {
  $signtool = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin" -Recurse -Filter signtool.exe -ErrorAction SilentlyContinue |
    Where-Object { $_.FullName -like '*\x64\*' } | Sort-Object FullName -Descending | Select-Object -First 1
  if (-not $signtool) { throw 'signtool.exe not found; cannot sign' }
  $isccArgs += "/Skuenta=`"$($signtool.FullName)`" sign /f `"$SignPfx`" /p `"$SignPassword`" /fd sha256 /tr http://timestamp.digicert.com /td sha256 `$f"
  $isccArgs += '/DSignTool=kuenta'
  $signed = $true
}
& $iscc @isccArgs (Join-Path $PSScriptRoot 'kuenta.iss')
if ($LASTEXITCODE -ne 0) { throw "ISCC failed ($LASTEXITCODE)" }

$exe = Join-Path $OutDir "KuentaSetup-$Version.exe"
if (-not (Test-Path $exe)) { throw "expected $exe was not produced" }
if ($signed -and (Get-AuthenticodeSignature $exe).Status -ne 'Valid') { throw "$exe is not validly signed" }
$hash = (Get-FileHash $exe -Algorithm SHA256).Hash.ToLower()
"$hash  KuentaSetup-$Version.exe" | Set-Content "$exe.sha256" -Encoding ascii
if (-not $signed) { Set-Content (Join-Path $OutDir 'installer-unsigned.txt') 'unsigned' -Encoding ascii }
Write-Host ("`n{0}  {1:N1} MB  {2}  ({3})" -f (Split-Path $exe -Leaf), ((Get-Item $exe).Length / 1MB), $hash, $(if ($signed) { 'signed' } else { 'unsigned' }))
