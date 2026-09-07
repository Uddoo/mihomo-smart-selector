[CmdletBinding()]
param(
    [switch]$Race,
    [switch]$SkipSecurity
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$projectRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$portableGo = Join-Path $projectRoot '.tools/go/bin/go.exe'
$portableGofmt = Join-Path $projectRoot '.tools/go/bin/gofmt.exe'
$go = if (Test-Path -LiteralPath $portableGo) {
    $portableGo
} elseif (Get-Command go -ErrorAction SilentlyContinue) {
    (Get-Command go).Source
} else {
    throw 'Go was not found on PATH or under .tools/go.'
}
$gofmt = if (Test-Path -LiteralPath $portableGofmt) {
    $portableGofmt
} elseif (Get-Command gofmt -ErrorAction SilentlyContinue) {
    (Get-Command gofmt).Source
} else {
    throw 'gofmt was not found beside Go or on PATH.'
}
if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) {
    throw 'pnpm was not found on PATH.'
}

function Get-ContentHash {
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )

    if (-not (Test-Path -LiteralPath $Path)) {
        return '<missing>'
    }
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash
}

Push-Location $projectRoot
try {
    Write-Host 'Checking Go formatting...'
    $goFiles = @(git ls-files -- '*.go')
    $unformatted = @(& $gofmt -l -- $goFiles)
    if ($unformatted.Count -ne 0) {
        throw "gofmt is required for: $($unformatted -join ', ')"
    }

    Write-Host 'Checking Go module consistency...'
    $goModBefore = Get-ContentHash -Path (Join-Path $projectRoot 'go.mod')
    $goSumBefore = Get-ContentHash -Path (Join-Path $projectRoot 'go.sum')
    & $go mod tidy
    $goModAfter = Get-ContentHash -Path (Join-Path $projectRoot 'go.mod')
    $goSumAfter = Get-ContentHash -Path (Join-Path $projectRoot 'go.sum')
    if ($goModBefore -cne $goModAfter -or $goSumBefore -cne $goSumAfter) {
        throw 'go mod tidy changed go.mod or go.sum; review and commit the normalized module files.'
    }
    & $go mod verify

    Write-Host 'Running Go vet and tests...'
    & $go vet ./...
    $testArguments = @('test', '-cover')
    if ($Race) {
        $cgoEnabled = (& $go env CGO_ENABLED).Trim()
        if ($cgoEnabled -ne '1') {
            throw '-Race requires CGO_ENABLED=1 and a supported C compiler. CI provides both on Ubuntu.'
        }
        $testArguments += '-race'
    }
    $testArguments += './...'
    & $go @testArguments

    Write-Host 'Installing and checking the Vue application...'
    pnpm --dir web install --frozen-lockfile
    pnpm --dir web exec vue-tsc --noEmit
    pnpm --dir web test
    & (Join-Path $PSScriptRoot 'check-web-assets.ps1')

    Write-Host 'Checking the OpenWrt init script syntax...'
    $shellCommand = Get-Command sh -ErrorAction SilentlyContinue
    $shellPath = if ($shellCommand) { $shellCommand.Source } else { $null }
    if (-not $shellPath -and $IsWindows) {
        $gitShell = 'C:\Program Files\Git\bin\sh.exe'
        if (Test-Path -LiteralPath $gitShell) {
            $shellPath = $gitShell
        }
    }
    if (-not $shellPath) {
        throw 'A POSIX sh implementation was not found for init-script syntax checking.'
    }
    & $shellPath -n 'deploy/openwrt/mihomo-smart-selector.init'

    Write-Host 'Linting GitHub Actions workflows...'
    & $go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12

    if (-not $SkipSecurity) {
        Write-Host 'Running dependency and secret scans...'
        pnpm --dir web audit --audit-level high
        & $go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
        & $go run github.com/zricethezav/gitleaks/v8@v8.30.1 git --no-banner --redact=100 .
        & $go run github.com/zricethezav/gitleaks/v8@v8.30.1 dir --no-banner --redact=100 .
    }

    git diff --check
    git diff --cached --check
    Write-Host 'Repository verification passed.'
}
finally {
    Pop-Location
}
