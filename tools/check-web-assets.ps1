[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$projectRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$committedRoot = (Resolve-Path -LiteralPath (Join-Path $projectRoot 'internal/api/static')).Path
$tempBase = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar)
$outputRoot = Join-Path $tempBase ('mihomo-smart-selector-web-' + [guid]::NewGuid().ToString('N'))

function Get-AssetManifest {
    param(
        [Parameter(Mandatory)]
        [string]$Root
    )

    $manifest = [ordered]@{}
    foreach ($file in Get-ChildItem -LiteralPath $Root -File -Recurse | Sort-Object FullName) {
        $relative = [IO.Path]::GetRelativePath($Root, $file.FullName).Replace('\', '/')
        $manifest[$relative] = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash
    }
    return $manifest
}

Push-Location $projectRoot
try {
    pnpm --dir web exec vite build --outDir $outputRoot

    $generated = Get-AssetManifest -Root $outputRoot
    $committed = Get-AssetManifest -Root $committedRoot
    $generatedPaths = @($generated.Keys)
    $committedPaths = @($committed.Keys)
    if (($generatedPaths -join "`n") -cne ($committedPaths -join "`n")) {
        throw "Embedded web asset paths are stale. Generated: $($generatedPaths -join ', '); committed: $($committedPaths -join ', ')"
    }
    foreach ($path in $generatedPaths) {
        if ($generated[$path] -cne $committed[$path]) {
            throw "Embedded web asset is stale: $path"
        }
    }

    Write-Host "Embedded web assets match the Vue production build ($($generated.Count) files)."
}
finally {
    Pop-Location
    if (Test-Path -LiteralPath $outputRoot) {
        $resolvedOutput = (Resolve-Path -LiteralPath $outputRoot).Path
        if (-not $resolvedOutput.StartsWith($tempBase + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
            throw "Refusing to remove unexpected web-build path: $resolvedOutput"
        }
        [IO.Directory]::Delete($resolvedOutput, $true)
    }
}
