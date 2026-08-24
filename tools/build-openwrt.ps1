[CmdletBinding()]
param(
    [ValidateSet('arm64', 'arm', 'mips', 'mipsle', '386', 'amd64')]
    [string]$GoArch = 'arm64',

    [string]$OutputPath = 'dist/linux'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$projectRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$portableGo = Join-Path $projectRoot '.tools\go\bin\go.exe'
$go = if (Test-Path -LiteralPath $portableGo) {
    $portableGo
} elseif (Get-Command go -ErrorAction SilentlyContinue) {
    (Get-Command go).Source
} else {
    throw 'Go was not found. Install Go or unpack an official project-local toolchain under .tools\go.'
}

$resolvedOutput = if ([IO.Path]::IsPathRooted($OutputPath)) {
    $OutputPath
} else {
    Join-Path $projectRoot $OutputPath
}
New-Item -ItemType Directory -Force -Path $resolvedOutput | Out-Null
$binary = Join-Path $resolvedOutput 'mihomo-smart-selector'

$previousCgo = $env:CGO_ENABLED
$previousOs = $env:GOOS
$previousArch = $env:GOARCH
try {
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'linux'
    $env:GOARCH = $GoArch
    & $go build -trimpath -ldflags '-s -w' -o $binary './cmd/mihomo-smart-selector'
    & $go version -m $binary
    Get-FileHash -LiteralPath $binary -Algorithm SHA256
} finally {
    $env:CGO_ENABLED = $previousCgo
    $env:GOOS = $previousOs
    $env:GOARCH = $previousArch
}

