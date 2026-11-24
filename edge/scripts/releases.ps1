<#
  releases.ps1 -- which edge release a box takes. Dot-source it:  . (Join-Path $PSScriptRoot 'releases.ps1')

  Releases come from the monorepo's GitHub Releases, tags edge/vX.Y.Z (see
  .github/workflows/release-edge.yml). cloud/* and landing/* releases are never looked at.
  Each edge release carries one version for the whole appliance (edge-node + next) and a
  manifest.json with min_cloud: the lowest cloud version it can sync with.
#>
$ReleaseOwner = 'bperezgo'
$ReleaseRepo  = 'laguna-escondida-backend'
$EdgePrefix   = 'edge/'

function Test-Tag($t) { return "$t" -match '^v\d+\.\d+\.\d+$' }

# True if release version $a is newer than $b. Anything that is not vX.Y.Z is never newer.
function Test-NewerTag($a, $b) {
  if (-not (Test-Tag $a) -or -not (Test-Tag $b)) { return $false }
  return [version]$a.Substring(1) -gt [version]$b.Substring(1)
}

# edge/v1.3.0 -> v1.3.0 (the version stamped into edge-node and recorded in artifacts\<app>\RELEASE)
function Get-EdgeVersion($tagName) { return "$tagName" -replace '^edge/', '' }

# Where an edge release's files are downloaded from.
function Get-EdgeBaseUrl($version) {
  return "https://github.com/$ReleaseOwner/$ReleaseRepo/releases/download/$EdgePrefix$version"
}

# The newest release that is newer than $Installed (null = any) and whose min_cloud the cloud
# satisfies. $Cloud null skips the min_cloud check (nothing to compare with, e.g. a first
# install). $GetMinCloud is called with a release and returns its min_cloud ($null if it has no
# readable manifest, which makes the release ineligible).
#   $Releases: GitHub API objects (tag_name, draft, prerelease)
function Select-EdgeRelease($Releases, $Installed, $Cloud, [scriptblock] $GetMinCloud) {
  $candidates = @($Releases | Where-Object {
    -not $_.draft -and -not $_.prerelease -and "$($_.tag_name)" -match '^edge/v\d+\.\d+\.\d+$'
  } | Sort-Object { [version](Get-EdgeVersion $_.tag_name).Substring(1) } -Descending)
  foreach ($r in $candidates) {
    $v = Get-EdgeVersion $r.tag_name
    if ($Installed -and -not (Test-NewerTag $v $Installed)) { continue }
    if ($Cloud) {
      $min = "$(& $GetMinCloud $r)"
      if (-not (Test-Tag $min)) { continue }
      if (Test-NewerTag $min $Cloud) { continue }
    }
    return $r
  }
  return $null
}

function Get-EdgeReleases {
  Invoke-RestMethod "https://api.github.com/repos/$ReleaseOwner/$ReleaseRepo/releases?per_page=50" `
    -TimeoutSec 20 -UseBasicParsing
}

function Get-EdgeManifest($version) {
  Invoke-RestMethod "$(Get-EdgeBaseUrl $version)/manifest.json" -TimeoutSec 20 -UseBasicParsing
}
