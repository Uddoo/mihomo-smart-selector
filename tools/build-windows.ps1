[CmdletBinding()]
param(
    [ValidatePattern('^[A-Za-z0-9][A-Za-z0-9._-]*$')]
    [string]$Version = 'dev',
    [ValidateSet('amd64', 'arm64')]
    [string[]]$GoArch = @('amd64', 'arm64'),
    [string]$OutputPath = 'dist/windows'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true
$projectRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$portableGo = Join-Path $projectRoot '.tools/go/bin/go.exe'
$go = if (Test-Path -LiteralPath $portableGo) { $portableGo } else { (Get-Command go -ErrorAction Stop).Source }
$output = [IO.Path]::GetFullPath($OutputPath, $projectRoot)
New-Item -ItemType Directory -Force -Path $output | Out-Null
$stageRoot = Join-Path ([IO.Path]::GetTempPath()) ('mss-windows-build-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $stageRoot | Out-Null
$savedEnv = @{}
foreach ($name in @('GOOS', 'GOARCH', 'GOAMD64', 'CGO_ENABLED')) { $savedEnv[$name] = [Environment]::GetEnvironmentVariable($name, 'Process') }
$checksums = @()
Push-Location $projectRoot
try {
    $env:GOOS = 'windows'; $env:CGO_ENABLED = '0'; $env:GOAMD64 = 'v1'
    foreach ($arch in $GoArch) {
        $env:GOARCH = $arch
        $name = "mihomo-smart-selector-$Version-windows-$arch"
        $stage = Join-Path $stageRoot $name
        New-Item -ItemType Directory -Path $stage | Out-Null
        $binary = Join-Path $stage 'mihomo-smart-selector.exe'
        & $go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $binary './cmd/mihomo-smart-selector'
        & $go version -m $binary
        Copy-Item -LiteralPath (Join-Path $projectRoot 'config.example.yaml') -Destination $stage
        Copy-Item -LiteralPath (Join-Path $projectRoot 'LICENSE') -Destination $stage
        Copy-Item -LiteralPath (Join-Path $projectRoot 'docs/windows.md') -Destination (Join-Path $stage 'README-Windows.md')
        $archive = Join-Path $output ($name + '.zip')
        # Explicit contents: never include a user's config, database or credentials.
        $files = @('mihomo-smart-selector.exe', 'config.example.yaml', 'LICENSE', 'README-Windows.md') | ForEach-Object { Join-Path $stage $_ }
        Compress-Archive -LiteralPath $files -DestinationPath $archive -Force
        $checksums += ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() + '  ' + [IO.Path]::GetFileName($archive))
        Write-Host "Packaged $archive"
    }
    [IO.File]::WriteAllText((Join-Path $output 'SHA256SUMS.txt'), (($checksums -join "`n") + "`n"), [Text.UTF8Encoding]::new($false))
}
finally {
    Pop-Location
    foreach ($name in $savedEnv.Keys) { [Environment]::SetEnvironmentVariable($name, $savedEnv[$name], 'Process') }
    $resolvedStage = [IO.Path]::GetFullPath($stageRoot)
    if ([IO.Path]::GetDirectoryName($resolvedStage) -ne [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar) -or -not [IO.Path]::GetFileName($resolvedStage).StartsWith('mss-windows-build-')) { throw 'Unexpected staging path' }
    Remove-Item -LiteralPath $resolvedStage -Recurse -Force
}
