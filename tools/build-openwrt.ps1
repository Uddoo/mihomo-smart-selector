<#
.SYNOPSIS
Cross-build the Linux binary for OpenWrt or iStoreOS.
.PARAMETER GoArch
Accepts arm64 (default), arm, 386, or amd64. Earlier versions also accepted
mips and mipsle, which the pinned SQLite and libc dependencies cannot build.
#>
[CmdletBinding()]
param(
    [ValidateSet('arm64', 'arm', '386', 'amd64', ErrorMessage = "Unsupported GOARCH '{0}'. Choose from {1}. The pinned modernc.org/sqlite and modernc.org/libc dependencies lack Linux mips and mipsle support with CGO_ENABLED=0.")]
    [string]$GoArch = 'arm64',

    [string]$OutputPath = 'dist/linux'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$projectRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$portableGo = Join-Path $projectRoot '.tools/go/bin/go.exe'
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
    $env:GOARCH = $GoArch.ToLowerInvariant()
    & $go -C $projectRoot build -trimpath -ldflags '-s -w' -o $binary './cmd/mihomo-smart-selector'
    & $go -C $projectRoot version -m $binary
    Get-FileHash -LiteralPath $binary -Algorithm SHA256
} finally {
    $env:CGO_ENABLED = $previousCgo
    $env:GOOS = $previousOs
    $env:GOARCH = $previousArch
}
