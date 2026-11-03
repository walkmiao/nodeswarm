# Install the native Wails desktop app (no browser and no SYSTEM service).
# Run from an elevated PowerShell terminal to install for all users.
param(
    [string]$Binary = '',
    [switch]$NoDesktopShortcut
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
if (-not $Binary) { $Binary = Join-Path $root 'desktop\build\bin\NodeSwarm-Desktop.exe' }
if (-not (Test-Path -LiteralPath $Binary)) { throw "Desktop executable not found: $Binary" }
$destination = Join-Path $env:ProgramFiles 'NodeSwarm'
New-Item -ItemType Directory -Path $destination -Force | Out-Null
$target = Join-Path $destination 'NodeSwarm-Desktop.exe'
Copy-Item -LiteralPath $Binary -Destination $target -Force
$shortcuts = @((Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\NodeSwarm Desktop.lnk'))
if (-not $NoDesktopShortcut) { $shortcuts += (Join-Path $env:Public 'Desktop\NodeSwarm Desktop.lnk') }
$shell = New-Object -ComObject WScript.Shell
foreach ($path in $shortcuts) {
    $link = $shell.CreateShortcut($path)
    $link.TargetPath = $target
    $link.WorkingDirectory = $destination
    $link.Description = 'NodeSwarm Windows Client'
    $link.Save()
}
Write-Host "Installed NodeSwarm Desktop to $target"
Write-Host 'Launch via the Start Menu. Device credentials are stored per user under %APPDATA%\NodeSwarmDesktop.'
Write-Host 'Windows 11 needs Microsoft Edge WebView2 Runtime. For offline clients, install the official WebView2 offline runtime separately.'
