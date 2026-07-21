$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$runDir = Join-Path $root 'tmp/run'
New-Item -ItemType Directory -Force -Path $runDir | Out-Null

# Some terminal hosts expose both PATH and Path. Start-Process treats those as
# duplicate dictionary keys, so normalize them before launching child services.
$canonicalPath = [Environment]::GetEnvironmentVariable('Path', 'Process')
[Environment]::SetEnvironmentVariable('PATH', $null, 'Process')
[Environment]::SetEnvironmentVariable('Path', $canonicalPath, 'Process')

Get-Content -LiteralPath (Join-Path $root '.env') -Encoding utf8 | Where-Object { $_ -and -not $_.StartsWith('#') } | ForEach-Object {
  $key, $value = $_ -split '=', 2
  [Environment]::SetEnvironmentVariable($key.Trim(), $value.Trim(), 'Process')
}

function Get-ListenerPid([int]$Port) {
  $match = netstat -ano -p tcp | Select-String -Pattern ":$Port\s+.*LISTENING\s+(\d+)\s*$" | Select-Object -First 1
  if ($match -and $match.Line -match 'LISTENING\s+(\d+)\s*$') { return [int]$Matches[1] }
  return $null
}

$node = (Get-Command node.exe -ErrorAction Stop).Source
$services = @(
  @{ Name='user'; Port=50051; File=(Join-Path $root 'bin/user_srv.exe'); Work=$root; Arguments=@() },
  @{ Name='file'; Port=50052; File=(Join-Path $root 'bin/file_srv.exe'); Work=$root; Arguments=@() },
  @{ Name='storage'; Port=50053; File=(Join-Path $root 'bin/storage_srv.exe'); Work=$root; Arguments=@() },
  @{ Name='analysis'; Port=50054; File=(Join-Path $root 'analysis_srv/.venv/Scripts/python.exe'); Work=(Join-Path $root 'analysis_srv'); Arguments=@('server.py') },
  @{ Name='gateway'; Port=8080; File=(Join-Path $root 'bin/gin_web.exe'); Work=$root; Arguments=@() },
  @{ Name='web'; Port=5173; File=$node; Work=(Join-Path $root 'web_ui'); Arguments=@('node_modules/vite/bin/vite.js','--host','127.0.0.1') }
)

foreach ($service in $services) {
  $existingPid = Get-ListenerPid $service.Port
  if ($existingPid) {
    Set-Content -LiteralPath (Join-Path $runDir "$($service.Name).pid") -Value $existingPid
    Write-Host "$($service.Name) already listening on $($service.Port) (PID $existingPid)"
    continue
  }
  if (-not (Test-Path -LiteralPath $service.File -PathType Leaf)) { throw "Executable not found: $($service.File)" }
  $start = @{
    FilePath = $service.File
    WorkingDirectory = $service.Work
    WindowStyle = 'Hidden'
    RedirectStandardOutput = (Join-Path $runDir "$($service.Name).out.log")
    RedirectStandardError = (Join-Path $runDir "$($service.Name).err.log")
    PassThru = $true
  }
  if ($service.Arguments.Count -gt 0) { $start.ArgumentList = $service.Arguments }
  $process = Start-Process @start
  Set-Content -LiteralPath (Join-Path $runDir "$($service.Name).pid") -Value $process.Id
  Write-Host "Started $($service.Name) on $($service.Port) (PID $($process.Id))"
}
