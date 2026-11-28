<#
  lan-config.ps1 -- the box's LAN identity: which IPv4 address it serves on, which network may reach
  it, and its local name. One set of rules for the wizard, silent installs, write-box-config.ps1,
  install.ps1 and set-lan-ip.ps1.

    -List                                   one JSON line per usable IPv4 address, default route first
    -Check [-LanIp] [-LanCidr] [-LanHost]   fill in defaults, validate, print one JSON line
    -NoRun                                  only define the functions (dot-source it)

  Exit codes: installer\exit-codes.json (0, 37). On 37, stdout holds {"ok":false,"error"}.
  Saved as UTF-8 with a BOM: Windows PowerShell 5.1 reads a BOM-less script as ANSI.
#>
param([switch]$List, [switch]$Check, [switch]$NoRun, [string]$LanIp, [string]$LanCidr, [string]$LanHost)

$DefaultLanHost = 'pos.kuenta.lan'

# 5.1 writes stdout in the OEM code page; the wizard's JsonField decodes \uXXXX.
function ConvertTo-AsciiJson($obj) {
  $json = $obj | ConvertTo-Json -Compress -Depth 3
  [regex]::Replace($json, '[^\x00-\x7F]', { param($m) '\u{0:x4}' -f [int][char]$m.Value })
}

function ConvertTo-IpNumber([string]$ip) {
  if ($ip -notmatch '^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$') { return $null }
  $n = [int64]0
  foreach ($i in 1..4) {
    $o = [int64]$Matches[$i]
    if ($o -gt 255) { return $null }
    $n = $n * 256 + $o
  }
  return $n
}

function ConvertFrom-IpNumber([int64]$n) {
  (3..0 | ForEach-Object { [math]::Floor($n / [math]::Pow(256, $_)) % 256 }) -join '.'
}

function Get-PrefixMask([int]$prefix) {
  if ($prefix -eq 0) { return [int64]0 }
  return [int64](0xFFFFFFFFL -shl (32 - $prefix)) -band 0xFFFFFFFFL
}

function Get-DefaultLanCidr([string]$ip, [int]$prefix) {
  if ($prefix -lt 16 -or $prefix -gt 32) { $prefix = 24 }
  $net = (ConvertTo-IpNumber $ip) -band (Get-PrefixMask $prefix)
  return "$(ConvertFrom-IpNumber $net)/$prefix"
}

function Get-SiteAddr([string]$lanHost, [string]$ip) {
  $h = "$lanHost".Trim()
  if ($h) { return "$h, $ip" }
  return $ip
}

function Test-InsidePrivateRange([int64]$net, [int]$prefix) {
  if ($prefix -lt 16 -or $prefix -gt 32) { return $false }
  foreach ($r in @(@('10.0.0.0', 8), @('172.16.0.0', 12), @('192.168.0.0', 16))) {
    $mask = Get-PrefixMask $r[1]
    if (($net -band $mask) -eq (ConvertTo-IpNumber $r[0])) { return $true }
  }
  return $false
}

function New-LanFailure([string]$msg) { [ordered]@{ ok = $false; error = $msg } }

# Pure: the machine's addresses come in as $heldIps / $defaultIp / $cidrByIp, so the test needs no network.
function Test-LanIdentity($ip, $cidr, $lanHost, $heldIps, $defaultIp, [hashtable]$cidrByIp = @{}) {
  $ip = "$ip".Trim(); $cidr = "$cidr".Trim(); $lanHost = "$lanHost".Trim()

  if (-not $ip) {
    if (-not $defaultIp) { return New-LanFailure 'Este equipo no tiene conexión a la red.' }
    $ip = $defaultIp
  }
  $ipN = ConvertTo-IpNumber $ip
  if ($null -eq $ipN) { return New-LanFailure "La IP $ip no es válida." }
  if (@($heldIps) -notcontains $ip) {
    return New-LanFailure ("Este equipo no tiene la IP $ip. Si la acaba de fijar en el router, " +
      'reconecte la red o reinicie el equipo e intente de nuevo.')
  }

  if (-not $cidr) {
    $cidr = $cidrByIp[$ip]
    if (-not $cidr) { $cidr = Get-DefaultLanCidr $ip 24 }
  }
  $netN = $null; $prefix = -1
  if ($cidr -match '^([0-9.]+)/(\d{1,2})$') {
    $netN = ConvertTo-IpNumber $Matches[1]
    $prefix = [int]$Matches[2]
  }
  if ($null -eq $netN -or $prefix -gt 32) {
    return New-LanFailure "La red $cidr no es válida. Escríbala como 192.168.1.0/24."
  }
  $mask = Get-PrefixMask $prefix
  $netN = $netN -band $mask
  if (-not (Test-InsidePrivateRange $netN $prefix)) {
    return New-LanFailure ("La red $cidr debe ser una red privada (10.x, 172.16-31.x o 192.168.x) " +
      'de máximo 65.536 direcciones (/16 o más).')
  }
  if (($ipN -band $mask) -ne $netN) { return New-LanFailure "La IP $ip no está dentro de la red $cidr." }
  $normCidr = "$(ConvertFrom-IpNumber $netN)/$prefix"

  if (-not $lanHost) { $lanHost = $DefaultLanHost }
  if ($lanHost -eq 'none') {
    $lanHost = ''
  } else {
    $lanHost = $lanHost.ToLowerInvariant()
    $label = '[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?'
    $parsed = $null
    if ($lanHost -cnotmatch "^(?=.{1,253}$)$label(\.$label)+$" -or [Net.IPAddress]::TryParse($lanHost, [ref]$parsed)) {
      return New-LanFailure "El nombre $lanHost no es válido. Use letras minúsculas, números, guiones y puntos, por ejemplo pos.kuenta.lan."
    }
  }

  return [ordered]@{ ok = $true; lan_ip = $ip; lan_cidr = $normCidr; lan_host = $lanHost }
}

function Get-DefaultRouteIp {
  $route = Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue |
    Sort-Object { $_.RouteMetric + (Get-NetIPInterface -InterfaceIndex $_.InterfaceIndex -AddressFamily IPv4).InterfaceMetric } |
    Select-Object -First 1
  if (-not $route) { return $null }
  $addr = Get-NetIPAddress -InterfaceIndex $route.InterfaceIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue |
    Where-Object { $_.IPAddress -notlike '169.254.*' } | Select-Object -First 1
  if ($addr) { return $addr.IPAddress }
  return $null
}

function Get-LanAddresses([object[]]$Source, [string]$DefaultIp) {
  if (-not $PSBoundParameters.ContainsKey('Source')) {
    $ifs = @{}
    foreach ($i in @(Get-NetIPInterface -AddressFamily IPv4 -ErrorAction SilentlyContinue)) { $ifs[[int]$i.InterfaceIndex] = $i }
    $Source = foreach ($a in @(Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue)) {
      $i = $ifs[[int]$a.InterfaceIndex]
      $metric = 9999; $up = $false
      if ($i) { $metric = [int]$i.InterfaceMetric; $up = ("$($i.ConnectionState)" -eq 'Connected') }
      [pscustomobject]@{ IPAddress = $a.IPAddress; PrefixLength = [int]$a.PrefixLength
        InterfaceAlias = $a.InterfaceAlias; InterfaceMetric = $metric; Up = $up }
    }
  }
  if (-not $PSBoundParameters.ContainsKey('DefaultIp')) { $DefaultIp = Get-DefaultRouteIp }

  $usable = @($Source | Where-Object {
    $_.Up -and $_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*' -and
      $null -ne (ConvertTo-IpNumber $_.IPAddress)
  })
  $sorted = $usable | Sort-Object @{ Expression = { $_.IPAddress -ne $DefaultIp } }, @{ Expression = { [int]$_.InterfaceMetric } }
  foreach ($a in $sorted) {
    $isDefault = 'false'
    if ($a.IPAddress -eq $DefaultIp) { $isDefault = 'true' }
    [pscustomobject][ordered]@{ ip = $a.IPAddress; cidr = (Get-DefaultLanCidr $a.IPAddress $a.PrefixLength)
      alias = "$($a.InterfaceAlias)"; default = $isDefault }
  }
}

function Resolve-LanIdentity([string]$LanIp, [string]$LanCidr, [string]$LanHost) {
  $addrs = @(Get-LanAddresses)
  $default = $null
  $cidrByIp = @{}
  foreach ($a in $addrs) {
    $cidrByIp[$a.ip] = $a.cidr
    if ($a.default -eq 'true') { $default = $a.ip }
  }
  $held = @($addrs | ForEach-Object { $_.ip })
  return Test-LanIdentity $LanIp $LanCidr $LanHost $held $default $cidrByIp
}

if ($NoRun) { return }

if ($List) {
  $addrs = @(Get-LanAddresses)
  if (-not $addrs) {
    ConvertTo-AsciiJson (New-LanFailure 'Este equipo no tiene ninguna dirección de red IPv4.') | Write-Output
    exit 37
  }
  foreach ($a in $addrs) { ConvertTo-AsciiJson $a | Write-Output }
  exit 0
}

if ($Check) {
  $r = Resolve-LanIdentity $LanIp $LanCidr $LanHost
  ConvertTo-AsciiJson $r | Write-Output
  if ($r.ok) { exit 0 }
  exit 37
}

ConvertTo-AsciiJson (New-LanFailure 'use -List or -Check') | Write-Output
exit 37
