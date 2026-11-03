$ErrorActionPreference = 'Stop'
$envPath = Join-Path $PSScriptRoot 'client.env'
if (-not (Test-Path -LiteralPath $envPath)) { throw "Missing $envPath" }
Get-Content -LiteralPath $envPath | ForEach-Object {
    if ($_ -match '^\s*([A-Z_][A-Z_0-9]*)=(.*)$') {
        [Environment]::SetEnvironmentVariable($Matches[1], $Matches[2], 'Process')
    }
}
$env:NODE_MANAGER_AGENT_DATA = Join-Path $PSScriptRoot 'data'
& (Join-Path $PSScriptRoot 'nodeswarm-client.exe') -mode client
exit $LASTEXITCODE
