# Administrator PowerShell. Install in observe mode first and verify enrollment.
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$binary = Join-Path $root 'nodeswarm-client-windows-amd64.exe'
if (-not (Test-Path -LiteralPath $binary)) {throw "Missing $binary. Build using make win."}
$target = Join-Path $env:ProgramData 'NodeSwarmClient'
New-Item -ItemType Directory -Path $target -Force | Out-Null
Copy-Item -LiteralPath $binary -Destination (Join-Path $target 'nodeswarm-client.exe') -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'run-client.ps1') -Destination $target -Force
$envPath = Join-Path $target 'client.env'
if (-not (Test-Path -LiteralPath $envPath)) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'client.env.example') -Destination $envPath
}
# Caution: SYSTEM has a different Windows session than interactive users.
# Observe mode works under SYSTEM, but an external GUI-driven proxy engine may
# require its own interactive session. Do not enable proxybridge until verified.
& icacls $target /inheritance:r /grant:r 'SYSTEM:(OI)(CI)F' 'Administrators:(OI)(CI)F' | Out-Null
$runner = Join-Path $target 'run-client.ps1'
$act = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument ('-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "' + $runner + '"')
$trigger = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit (New-TimeSpan -Seconds 0) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
Register-ScheduledTask -TaskName 'NodeSwarmClient' -Action $act -Trigger $trigger -Settings $settings -User 'SYSTEM' -RunLevel Highest -Force | Out-Null
Write-Host "Installed NodeSwarmClient in observe mode. Edit $envPath then Start-ScheduledTask -TaskName NodeSwarmClient"
