<#
  test-releases.ps1 -- scripted check of the updater's release choice (Select-EdgeRelease)
  against a fake release list. Run anywhere with PowerShell:  pwsh edge/scripts/test-releases.ps1
#>
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'releases.ps1')

function R($tag) { [pscustomobject]@{ tag_name = $tag; draft = $false; prerelease = $false } }
$min = @{ 'edge/v1.3.0' = 'v1.2.6'; 'edge/v1.4.0' = 'v1.3.0'; 'edge/v1.5.0' = 'v1.4.0' }
$getMin = { param($r) $min[$r.tag_name] }

$fail = 0
function Check($name, $got, $want) {
  if ("$got" -eq "$want") { Write-Host "ok    $name" } else { Write-Host "FAIL  $name : got '$got', want '$want'"; $script:fail++ }
}
function Pick($releases, $installed, $cloud) {
  $r = Select-EdgeRelease $releases $installed $cloud $getMin
  if ($r) { return $r.tag_name } else { return '' }
}

# 1. a cloud release is not an edge release: nothing to offer
Check 'cloud-only release offers nothing' (Pick @((R 'cloud/v1.2.7'), (R 'landing/v1.0.0'), (R 'v1.2.7'), (R 'edge/v1.3.0')) 'v1.3.0' 'v1.2.7') ''

# 2. newest edge needs a cloud that is not live yet: offer the older compatible one
$list = @((R 'edge/v1.3.0'), (R 'edge/v1.4.0'), (R 'edge/v1.5.0'))
Check 'release above the cloud -> older compatible one' (Pick $list 'v1.2.0' 'v1.3.0') 'edge/v1.4.0'
Check 'nothing compatible and newer'                    (Pick $list 'v1.4.0' 'v1.3.0') ''

# 3. compatible newest release is offered
Check 'compatible release offered'    (Pick $list 'v1.4.0' 'v1.4.5') 'edge/v1.5.0'
Check 'already on the newest'         (Pick $list 'v1.5.0' 'v1.9.0') ''
Check 'no installed version = newest' (Pick $list $null 'v1.9.0') 'edge/v1.5.0'
Check 'no cloud version skips min_cloud' (Pick $list 'v1.0.0' $null) 'edge/v1.5.0'
Check 'drafts and prereleases ignored' (Pick @([pscustomobject]@{ tag_name = 'edge/v1.6.0'; draft = $true; prerelease = $false }) 'v1.0.0' 'v9.9.9') ''
Check 'numeric, not text, ordering'   (Pick @((R 'edge/v1.9.0'), (R 'edge/v1.10.0')) 'v1.0.0' $null) 'edge/v1.10.0'

if ($fail) { throw "$fail check(s) failed" }
Write-Host 'all checks passed'
