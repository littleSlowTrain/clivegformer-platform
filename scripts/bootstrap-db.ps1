$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$envPath = Join-Path $root '.env'
$values = @{}
Get-Content -LiteralPath $envPath | Where-Object { $_ -and -not $_.StartsWith('#') } | ForEach-Object {
  $key, $value = $_ -split '=', 2
  $values[$key] = $value
}
$grant = "CREATE USER IF NOT EXISTS '$($values.MYSQL_USER)'@'%' IDENTIFIED BY '$($values.MYSQL_PASSWORD)'; ALTER USER '$($values.MYSQL_USER)'@'%' IDENTIFIED BY '$($values.MYSQL_PASSWORD)'; GRANT SELECT,INSERT,UPDATE,DELETE ON $($values.MYSQL_DATABASE).* TO '$($values.MYSQL_USER)'@'%'; FLUSH PRIVILEGES;"
docker exec -e "MYSQL_PWD=$($values.MYSQL_ROOT_PASSWORD)" mysql-master mysql -uroot -e $grant
Get-ChildItem -LiteralPath (Join-Path $root 'migrations') -Filter '*.sql' | Sort-Object Name | ForEach-Object {
  Write-Host "Applying migration $($_.Name)"
  Get-Content -LiteralPath $_.FullName -Raw | docker exec -i -e "MYSQL_PWD=$($values.MYSQL_ROOT_PASSWORD)" mysql-master mysql -uroot
  if ($LASTEXITCODE -ne 0) { throw "Migration failed: $($_.Name)" }
}
Write-Host 'Database, tables, and application account are ready.'
