# Run in an elevated PowerShell. Does not start the task until you set the env file.
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$binary = Join-Path $root 'node-console-windows-amd64.exe'
if (-not (Test-Path -LiteralPath $binary)) { throw "Missing $binary" }
$target = Join-Path $env:ProgramData 'NodeConsoleAgent'
New-Item -Path $target -ItemType Directory -Force | Out-Null
Copy-Item -LiteralPath $binary -Destination (Join-Path $target 'node-console.exe') -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'run-agent.ps1') -Destination $target -Force
$envPath = Join-Path $target 'agent.env'
if (-not (Test-Path -LiteralPath $envPath)) { Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'agent.env.example') -Destination $envPath }
# Only SYSTEM and Administrators may read the bootstrap secret.
& icacls $target /inheritance:r /grant:r 'SYSTEM:(OI)(CI)F' 'Administrators:(OI)(CI)F' | Out-Null
$runner = Join-Path $target 'run-agent.ps1'
$act = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument ('-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "' + $runner + '"')
$trigger = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit (New-TimeSpan -Seconds 0) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
Register-ScheduledTask -TaskName 'NodeConsoleAgent' -Action $act -Trigger $trigger -Settings $settings -User 'SYSTEM' -RunLevel Highest -Force | Out-Null
Write-Host "Installed scheduled task NodeConsoleAgent. Edit $envPath then run Start-ScheduledTask -TaskName NodeConsoleAgent"
