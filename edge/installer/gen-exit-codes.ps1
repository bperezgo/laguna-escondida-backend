<#
  gen-exit-codes.ps1 -- turn exit-codes.json into exit-codes.inc, the Pascal function the wizard
  includes, so the scripts' codes and the wizard's messages share one source. CI runs this and
  fails when the committed .inc differs.
#>
[CmdletBinding()]
param(
  [string] $Json = (Join-Path $PSScriptRoot 'exit-codes.json'),
  [string] $Out  = (Join-Path $PSScriptRoot 'exit-codes.inc')
)
$ErrorActionPreference = 'Stop'
$codes = Get-Content $Json -Raw -Encoding UTF8 | ConvertFrom-Json
$lines = @('function ExitMessage(Code: Integer): String;', 'begin', '  case Code of')
foreach ($p in $codes.PSObject.Properties) {
  $lines += "    $($p.Name): Result := '$($p.Value.message.Replace("'", "''"))';"
}
$lines += "  else Result := 'Error inesperado (código ' + IntToStr(Code) + ').';", '  end;', 'end;'
[IO.File]::WriteAllText($Out, ($lines -join "`n") + "`n", (New-Object Text.UTF8Encoding($true)))
