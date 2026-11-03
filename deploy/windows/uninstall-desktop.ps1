# Administrator terminal. Retains %APPDATA%\NodeSwarmDesktop to avoid losing credentials unexpectedly.
$ErrorActionPreference = 'Stop'
$exe = Join-Path $env:ProgramFiles 'NodeSwarm\NodeSwarm-Desktop.exe'
if (Get-Process -Name 'NodeSwarm-Desktop' -ErrorAction SilentlyContinue) { throw 'Please exit NodeSwarm Desktop before uninstalling' }
if (Test-Path $exe) { Remove-Item -LiteralPath $exe -Force }
$dir = Split-Path -Parent $exe
if ((Test-Path $dir) -and -not (Get-ChildItem $dir -Force)) { Remove-Item $dir -Force }
@((Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\NodeSwarm Desktop.lnk'), (Join-Path $env:Public 'Desktop\NodeSwarm Desktop.lnk')) | ForEach-Object {
    if (Test-Path -LiteralPath $_) { Remove-Item -LiteralPath $_ -Force }
}
Write-Host 'Desktop application removed. User data/credentials remain in %APPDATA%\NodeSwarmDesktop.'
