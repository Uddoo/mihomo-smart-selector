[CmdletBinding()]
param([Parameter(Mandatory)][string]$ArchivePath)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true
if (-not $IsWindows -or [Runtime.InteropServices.RuntimeInformation]::OSArchitecture -ne 'X64') { throw 'This smoke test requires native x64 Windows.' }
$projectRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$archive = (Resolve-Path -LiteralPath $ArchivePath).Path
$run = Join-Path ([IO.Path]::GetTempPath()) ('mss-windows-test-' + [guid]::NewGuid().ToString('N'))
$install = Join-Path $run '中文安装 with spaces'
$other = Join-Path $run 'other working directory'
New-Item -ItemType Directory -Path $install, $other | Out-Null
Expand-Archive -LiteralPath $archive -DestinationPath $install
$binary = Join-Path $install 'mihomo-smart-selector.exe'
$children = [Collections.Generic.List[Diagnostics.Process]]::new()
$checks = [Collections.Generic.List[string]]::new()
$previousSecret = $env:MIHOMO_SECRET
function Assert-Check([bool]$Condition, [string]$Name) {
    if (-not $Condition) { throw "Failed: $Name" }
    $checks.Add($Name)
}
function Get-FreePort {
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    $listener.Start()
    try { return $listener.LocalEndpoint.Port } finally { $listener.Stop() }
}
function Start-Fixture([string]$File, [string[]]$Arguments, [string]$Name) {
    $parameters = @{FilePath=$File; WorkingDirectory=$other; WindowStyle='Hidden'; PassThru=$true; RedirectStandardOutput=(Join-Path $run "$Name.stdout.log"); RedirectStandardError=(Join-Path $run "$Name.stderr.log")}
    if ($Arguments.Count) { $parameters.ArgumentList = $Arguments }
    $process = Start-Process @parameters
    $children.Add($process)
    return $process
}
function Wait-Boot([string]$Previous = '') {
    $deadline = [DateTime]::UtcNow.AddSeconds(20)
    while ([DateTime]::UtcNow -lt $deadline) {
        if ($script:app.HasExited) { throw ('Packaged EXE stopped: ' + (Get-Content -LiteralPath (Join-Path $run 'app.stderr.log') -Raw)) }
        try {
            $state = Invoke-RestMethod "$script:base/api/v1/service" -TimeoutSec 2
            if ($state.status -eq 'running' -and $state.instance_id -and $state.instance_id -ne $Previous) { return $state }
        } catch { }
        Start-Sleep -Milliseconds 100
    }
    throw 'Packaged EXE did not become ready'
}
try {
    $env:MIHOMO_SECRET = $null
    $names = @(Get-ChildItem -LiteralPath $install -File | Select-Object -ExpandProperty Name | Sort-Object)
    Assert-Check (($names -join '|') -eq ((@('LICENSE','README-Windows.md','config.example.yaml','mihomo-smart-selector.exe') | Sort-Object) -join '|')) 'archive contains only the four intended public files'
    Push-Location $other
    try {
        $version = (& $binary -version | Out-String).Trim()
        Assert-Check ($version -match '^Mihomo Smart Selector .+') 'native EXE prints its embedded version'
        & $binary -init
        $configPath = Join-Path $install 'config.yaml'
        Assert-Check (Test-Path -LiteralPath $configPath) 'first config is beside EXE despite a different working directory'
        Assert-Check (-not (Test-Path -LiteralPath (Join-Path $other 'config.yaml'))) 'working directory remains free of configuration'
        $originalHash = (Get-FileHash -LiteralPath $configPath).Hash
        & $binary -init
        Assert-Check ((Get-FileHash -LiteralPath $configPath).Hash -eq $originalHash) 'initialization preserves existing config bytes'
    } finally { Pop-Location }
    $appPort = Get-FreePort
    $mockPort = Get-FreePort
    while ($mockPort -eq $appPort) { $mockPort = Get-FreePort }
    $config = (Get-Content -LiteralPath $configPath -Raw).Replace('127.0.0.1:8788', "127.0.0.1:$appPort").Replace('127.0.0.1:9090', "127.0.0.1:$mockPort")
    [IO.File]::WriteAllText($configPath, $config, [Text.UTF8Encoding]::new($false))
    $configHash = (Get-FileHash -LiteralPath $configPath).Hash
    $portableGo = Join-Path $projectRoot '.tools/go/bin/go.exe'
    $go = if (Test-Path -LiteralPath $portableGo) { $portableGo } else { (Get-Command go).Source }
    $mockBinary = Join-Path $run 'mihomo-mock.exe'
    Push-Location $projectRoot
    try { & $go build -o $mockBinary './cmd/mihomo-mock' } finally { Pop-Location }
    $mock = Start-Fixture $mockBinary @('-listen', "127.0.0.1:$mockPort") 'mock'
    $script:base = "http://127.0.0.1:$appPort"
    $script:app = Start-Fixture $binary @() 'app'
    $boot = Wait-Boot
    $health = Invoke-RestMethod "$base/api/v1/health" -TimeoutSec 5
    Assert-Check ($health.mihomo_connected -eq $true) 'no-argument EXE starts and connects to isolated Controller'
    $page = Invoke-WebRequest "$base/" -TimeoutSec 5
    Assert-Check ($page.Content -match '<script.+/assets/') 'packaged EXE serves embedded dashboard'
    $state = Invoke-RestMethod "$base/api/v1/connection" -TimeoutSec 5
    $update = @{revision=$state.revision; controller=$state.saved.controller; request_timeout_seconds=13; secret_action='none'} | ConvertTo-Json
    $saved = Invoke-RestMethod "$base/api/v1/connection" -Method Put -ContentType 'application/json' -Body $update -TimeoutSec 5
    Assert-Check $saved.restart_required 'packaged API persists pending connection configuration'
    $restart = @{instance_id=$boot.instance_id; confirm=$true} | ConvertTo-Json
    Invoke-RestMethod "$base/api/v1/service/restart" -Method Post -ContentType 'application/json' -Body $restart -TimeoutSec 5 | Out-Null
    $next = Wait-Boot $boot.instance_id
    $state = Invoke-RestMethod "$base/api/v1/connection" -TimeoutSec 5
    Assert-Check ((-not $state.restart_required) -and $state.active.request_timeout_seconds -eq 13) 'web restart applies saved values'
    Assert-Check (-not $app.HasExited) 'restart retains the original native process'
    Assert-Check ((Get-FileHash -LiteralPath $configPath).Hash -eq $configHash) 'startup and restart preserve YAML bytes'
    Assert-Check (Test-Path -LiteralPath (Join-Path $install 'data/selector.db.connection.json')) 'connection file resolves beside config'
    Assert-Check (-not (Test-Path -LiteralPath (Join-Path $other 'data'))) 'database does not follow working directory'
    $duplicate = Start-Fixture $binary @() 'duplicate'
    Assert-Check ($duplicate.WaitForExit(10000)) 'second instance exits promptly with redirected errors'
    Assert-Check ($duplicate.ExitCode -ne 0) 'port conflict reports failure'
    Assert-Check ((Get-Content -LiteralPath (Join-Path $run 'duplicate.stderr.log') -Raw) -match 'cannot listen') 'port conflict includes actionable diagnosis'
    Assert-Check ((Invoke-RestMethod "$base/api/v1/service" -TimeoutSec 5).instance_id -eq $next.instance_id) 'second instance leaves running service intact'
    # Once offline, the app still completes a web restart and exposes settings.
    Stop-Process -Id $mock.Id
    $mock.WaitForExit(5000) | Out-Null
    $restart = @{instance_id=$next.instance_id; confirm=$true} | ConvertTo-Json
    Invoke-RestMethod "$base/api/v1/service/restart" -Method Post -ContentType 'application/json' -Body $restart -TimeoutSec 5 | Out-Null
    $offline = Wait-Boot $next.instance_id
    Assert-Check ([bool]$offline.instance_id) 'restart recovers while Controller is offline'
    Assert-Check ([bool](Invoke-RestMethod "$base/api/v1/connection" -TimeoutSec 5).active.controller) 'offline settings remain accessible'
    [ordered]@{archive=$archive; version=$version; architecture='windows/amd64'; checks=$checks; count=$checks.Count} | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $run 'report.json') -Encoding utf8
    Write-Host "$($checks.Count) Windows package checks passed. Evidence: $run"
}
finally {
    foreach ($child in $children) {
        if (-not $child.HasExited) { Stop-Process -Id $child.Id -ErrorAction SilentlyContinue; $child.WaitForExit(5000) | Out-Null }
        $child.Dispose()
    }
    $env:MIHOMO_SECRET = $previousSecret
}
