param(
  [Parameter(Mandatory = $true)]
  [string]$Confirm
)

$ErrorActionPreference = 'Stop'
$requiredConfirmation = 'RESET clivegformer AND clivegformer-data'
if ($Confirm -cne $requiredConfirmation) {
  throw "Confirmation must exactly equal: $requiredConfirmation"
}

$root = Split-Path -Parent $PSScriptRoot
$envPath = Join-Path $root '.env'
$values = @{}
Get-Content -LiteralPath $envPath -Encoding utf8 | Where-Object { $_ -and -not $_.StartsWith('#') } | ForEach-Object {
  $key, $value = $_ -split '=', 2
  $values[$key.Trim()] = $value.Trim()
  [Environment]::SetEnvironmentVariable($key.Trim(), $value.Trim(), 'Process')
}

if ($values.MYSQL_DATABASE -cne 'clivegformer') { throw "Refusing database: $($values.MYSQL_DATABASE)" }
if ($values.CEPH_ENDPOINT -cne 'http://192.168.10.130:80') { throw "Refusing Ceph endpoint: $($values.CEPH_ENDPOINT)" }
if ($values.CEPH_BUCKET -cne 'clivegformer-data') { throw "Refusing Ceph bucket: $($values.CEPH_BUCKET)" }

& (Join-Path $PSScriptRoot 'stop-local.ps1')
& (Join-Path $PSScriptRoot 'bootstrap-db.ps1')

& (Join-Path $root 'bin/purge_ceph.exe') -confirm 'PURGE clivegformer-data AT 192.168.10.130:80'
if ($LASTEXITCODE -ne 0) { throw 'Ceph purge failed; database was not cleared.' }

& (Join-Path $root 'bin/resetdev.exe') -confirm 'RESET clivegformer DATA AND PROJECT REDIS KEYS'
if ($LASTEXITCODE -ne 0) { throw 'MySQL/Redis reset failed.' }

& (Join-Path $root 'bin/purge_ceph.exe') -verify-only
if ($LASTEXITCODE -ne 0) { throw 'Ceph verification failed.' }
& (Join-Path $root 'bin/resetdev.exe') -verify-only
if ($LASTEXITCODE -ne 0) { throw 'MySQL/Redis verification failed.' }

& (Join-Path $PSScriptRoot 'run-local.ps1')
Write-Host 'Development data reset completed; schemas, Ceph bucket, RocketMQ and containers were preserved.'
