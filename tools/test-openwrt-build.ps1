[CmdletBinding()]
param(
    [Parameter(Mandatory, ParameterSetName = 'Build')]
    [ValidateSet('amd64', 'arm64', 'arm', '386')]
    [string]$GoArch,

    [Parameter(Mandatory, ParameterSetName = 'Reject')]
    [switch]$CheckRejected
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$build = Join-Path $PSScriptRoot 'build-openwrt.ps1'
$run = Join-Path ([IO.Path]::GetTempPath()) ('mss-openwrt-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $run | Out-Null
try {
    if ($CheckRejected) {
        foreach ($arch in 'mips', 'mipsle') {
            try {
                & $build -GoArch $arch -OutputPath (Join-Path $run $arch)
                throw "Unexpectedly accepted linux/$arch."
            } catch [System.Management.Automation.ParameterBindingException] {
                if ($_.FullyQualifiedErrorId -ne 'ParameterArgumentValidationError,build-openwrt.ps1' -or
                    $_.Exception.ParameterName -ne 'GoArch' -or
                    $_.Exception.Message -notlike "*'$arch'*" -or
                    $_.Exception.Message -notlike '*modernc.org/sqlite*' -or
                    $_.Exception.Message -notlike '*modernc.org/libc*') { throw }
            }
            if (@(Get-ChildItem -LiteralPath $run -Force).Count) {
                throw "Rejecting linux/$arch created output."
            }
            Write-Host "linux/$arch rejected during parameter binding, with no output."
        }
        return
    }

    $output = @(& $build -GoArch $GoArch -OutputPath $run)
    $output | Out-Host
    $binary = Join-Path $run 'mihomo-smart-selector'
    $header = Get-Content -LiteralPath $binary -AsByteStream -TotalCount 20
    $machines = @{ amd64 = 62; arm64 = 183; arm = 40; '386' = 3 }
    $class = if ($GoArch -in 'amd64', 'arm64') { 2 } else { 1 }
    if ($header.Count -ne 20 -or [BitConverter]::ToString([byte[]]$header, 0, 4) -ne '7F-45-4C-46' -or
        $header[4] -ne $class -or $header[5] -ne 1 -or
        ($header[18] + 256 * $header[19]) -ne $machines[$GoArch]) {
        throw "Expected a little-endian ELF binary for linux/$GoArch."
    }
    if (@($output -match '^\s*mod\s+github\.com/Uddoo/mihomo-smart-selector\s').Count -ne 1) {
        throw 'Built binary does not contain the canonical module path.'
    }
    foreach ($setting in 'GOOS=linux', "GOARCH=$GoArch", 'CGO_ENABLED=0') {
        if (-not ($output -match "^\s*build\s+$setting\s*$")) { throw "Missing build setting $setting." }
    }
    if ($output -match 'vcs\.modified=true|\+dirty') { throw 'Built binary reports a dirty source tree.' }
    Write-Host "Verified ELF$($class * 32) machine $($machines[$GoArch]) and Go build settings for linux/$GoArch."
} finally {
    Remove-Item -LiteralPath $run -Recurse -Force
}
