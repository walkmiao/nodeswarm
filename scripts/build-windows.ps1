# Run at repository root with Go 1.23.2+ installed.
$ErrorActionPreference = 'Stop'
Write-Host '[1/3] Go tests'
go test ./... -count=1
if ($LASTEXITCODE -ne 0) { throw 'go test failed' }
Write-Host '[2/3] Static validation'
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
Write-Host '[3/3] Build Windows amd64 single binary'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go build -trimpath -o node-console-windows-amd64.exe .
if ($LASTEXITCODE -ne 0) { throw 'go build failed' }
Get-FileHash .\node-console-windows-amd64.exe -Algorithm SHA256
