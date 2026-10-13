param([switch]$Test)
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $taskRoot
if (Test-Path '.tools\go\bin\go.exe') { $taskGo = Join-Path $taskRoot '.tools\go\bin\go.exe' } else { $taskGo = (Get-Command go -ErrorAction Stop).Source }
if (Test-Path '.tools\mingw64\bin') { $env:PATH = (Join-Path $taskRoot '.tools\mingw64\bin') + ';' + $env:PATH }
if (Test-Path '.tools\mingw64\bin\gcc.exe') { $env:CC = Join-Path $taskRoot '.tools\mingw64\bin\gcc.exe' }
$env:GOPATH = Join-Path $taskRoot '.tools\gopath'
$env:GOCACHE = Join-Path $taskRoot '.tools\gocache'
$env:CGO_ENABLED = '1'
& $taskGo run ./scripts/icons
if ($LASTEXITCODE -ne 0) { throw '图标生成失败' }
$taskRsrc = Join-Path $taskRoot '.tools\gopath\bin\rsrc.exe'
if (!(Test-Path -LiteralPath $taskRsrc)) {
    & $taskGo install github.com/akavel/rsrc@v0.10.2
    if ($LASTEXITCODE -ne 0) { throw '图标资源工具准备失败' }
}
& $taskRsrc -arch amd64 -ico internal/ui/assets/app.ico -o cmd/devopstool/icon_windows_amd64.syso
if ($LASTEXITCODE -ne 0) { throw 'Windows 图标嵌入失败' }
if ($Test) {
    & $taskGo test ./...
    if ($LASTEXITCODE -ne 0) { throw '测试失败' }
}
New-Item -ItemType Directory -Force 'dist' | Out-Null
& $taskGo build -trimpath -ldflags '-s -w -H=windowsgui -extldflags=-static' -o 'dist\DevOpsTool.exe' './cmd/devopstool'
if ($LASTEXITCODE -ne 0) { throw '构建失败' }
Get-Item 'dist\DevOpsTool.exe' | Select-Object FullName,Length
