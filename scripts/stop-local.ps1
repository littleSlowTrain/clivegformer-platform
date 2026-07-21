$root = Split-Path -Parent $PSScriptRoot
$runDir = Join-Path $root 'tmp/run'
$serviceNames = @('user_srv.exe','file_srv.exe','storage_srv.exe','gin_web.exe')
Get-Process user_srv,file_srv,storage_srv,gin_web -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
$portPattern = ':(50051|50052|50053|50054|8080|5173)\s'
netstat -ano | Select-String -Pattern $portPattern | ForEach-Object {
  if ($_.Line -match 'LISTENING\s+(\d+)\s*$') { Stop-Process -Id ([int]$Matches[1]) -Force -ErrorAction SilentlyContinue }
}
try {
  Get-CimInstance Win32_Process -ErrorAction Stop | Where-Object {
    $_.Name -in $serviceNames -or
    ($_.Name -match '^python(\.exe)?$' -and $_.CommandLine -like '*clivegformer-platform*analysis_srv*server.py*') -or
    ($_.Name -match '^node(\.exe)?$' -and $_.CommandLine -like '*clivegformer-platform*web_ui*vite*')
  } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
} catch { }
Get-ChildItem -LiteralPath $runDir -Filter '*.pid' -ErrorAction SilentlyContinue | ForEach-Object {
  $processId = [int](Get-Content -LiteralPath $_.FullName -Raw)
  Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $_.FullName -Force
}
Write-Host 'Local platform processes stopped.'
