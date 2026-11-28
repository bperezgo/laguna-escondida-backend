<#
  test-lan-config.ps1 -- scripted check of the box's network rules (lan-config.ps1) against fake
  adapters, so it needs no network. Run anywhere with PowerShell:  pwsh edge/scripts/test-lan-config.ps1
#>
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'lan-config.ps1') -NoRun

$held = @('192.168.1.50', '10.0.0.7')
$default = '10.0.0.7'

$fail = 0
function Check($name, $got, $want) {
  if ("$got" -eq "$want") { Write-Host "ok    $name" } else { Write-Host "FAIL  $name : got '$got', want '$want'"; $script:fail++ }
}
function CheckLike($name, $got, $want) {
  if ("$got" -like $want) { Write-Host "ok    $name" } else { Write-Host "FAIL  $name : got '$got', want like '$want'"; $script:fail++ }
}
function T($ip, $cidr, $lanHost) { Test-LanIdentity $ip $cidr $lanHost $held $default }

# Defaults
$r = T '' '' ''
Check 'defaults: ok' $r.ok $true
Check 'defaults: the default-route IP' $r.lan_ip '10.0.0.7'
Check 'defaults: pos.kuenta.lan' $r.lan_host 'pos.kuenta.lan'

# IP
CheckLike 'IP not held' (T '192.168.77.77' '' '').error 'Este equipo no tiene la IP 192.168.77.77.*'
Check 'IP octet above 255' (T '300.1.1.1' '' '').error 'La IP 300.1.1.1 no es válida.'
Check 'IP of one number' (T '1' '' '').error 'La IP 1 no es válida.'

# Network
$r = T '192.168.1.50' '192.168.1.5/24' ''
Check 'network normalized' $r.lan_cidr '192.168.1.0/24'
CheckLike 'IP outside the network' (T '192.168.1.50' '192.168.2.0/24' '').error '*no está dentro de la red*'
foreach ($c in '0.0.0.0/0', '192.168.0.0/8', '8.8.8.0/24', '10.0.0.0/15') {
  $ip = if ($c -like '10.*') { '10.0.0.7' } else { '192.168.1.50' }
  CheckLike "network $c refused" (T $ip $c '').error "La red $c debe ser una red privada*"
}
Check 'network /16 passes' (T '192.168.1.50' '192.168.0.0/16' '').ok $true
Check 'network /32 passes' (T '192.168.1.50' '192.168.1.50/32' '').ok $true
CheckLike 'network abc malformed' (T '192.168.1.50' 'abc' '').error 'La red abc no es válida*'

# Name
$r = T '' '' 'none'
Check 'none: ok' $r.ok $true
Check 'none: no name' $r.lan_host ''
Check 'name lowercased' (T '' '' 'POS.Kuenta.LAN').lan_host 'pos.kuenta.lan'
$long = ('a' * 63 + '.') * 3 + ('a' * 62)
Check 'the long name is 254 characters' $long.Length 254
foreach ($h in 'pos kuenta', '-pos.lan', 'pos', '192.168.1.50', $long) {
  CheckLike "name '$($h.Substring(0, [Math]::Min(20, $h.Length)))' refused" (T '' '' $h).error 'El nombre * no es válido.*'
}

# Default network
Check 'default network of a /8 is the /24' (Get-DefaultLanCidr '10.0.0.7' 8) '10.0.0.0/24'
Check 'default network of a /24' (Get-DefaultLanCidr '192.168.1.50' 24) '192.168.1.0/24'

# Site address
Check 'site address with a name' (Get-SiteAddr 'pos.kuenta.lan' '192.168.1.50') 'pos.kuenta.lan, 192.168.1.50'
Check 'site address without a name' (Get-SiteAddr '' '192.168.1.50') '192.168.1.50'

# ASCII JSON
Check 'non-ASCII escaped' (ConvertTo-AsciiJson @{ error = 'válida' }) '{"error":"v\u00e1lida"}'

# Address list
function A($ip, $prefix, $alias, $metric, $up) {
  [pscustomobject]@{ IPAddress = $ip; PrefixLength = $prefix; InterfaceAlias = $alias; InterfaceMetric = $metric; Up = $up }
}
$fake = @(
  (A '127.0.0.1' 8 'Loopback' 75 $true),
  (A '192.168.1.50' 24 'Ethernet' 25 $true),
  (A '169.254.3.4' 16 'Bluetooth' 65 $true),
  (A '192.168.99.9' 24 'Down NIC' 5 $false),
  (A '10.0.0.7' 8 'Wi-Fi' 50 $true)
)
$addrs = @(Get-LanAddresses -Source $fake -DefaultIp '10.0.0.7')
Check 'only usable addresses listed' (($addrs | ForEach-Object { $_.ip }) -join ',') '10.0.0.7,192.168.1.50'
Check 'the default first' $addrs[0].default 'true'
Check 'the rest not default' $addrs[1].default 'false'
Check 'listed network' $addrs[1].cidr '192.168.1.0/24'
Check 'listed alias' $addrs[1].alias 'Ethernet'

if ($fail -gt 0) { Write-Host "$fail check(s) failed"; exit 1 }
Write-Host 'all checks passed'
