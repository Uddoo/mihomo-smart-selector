[CmdletBinding()]
param()

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
    throw 'Go was not found on PATH or under .tools/go.'
}

$tempBase = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar)
$smokeRoot = Join-Path $tempBase ('mihomo-smart-selector-smoke-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $smokeRoot | Out-Null
$mockProcess = $null
$appProcess = $null
$previousSecret = [Environment]::GetEnvironmentVariable('MIHOMO_SECRET', 'Process')

function Get-FreeLoopbackPorts {
    param(
        [Parameter(Mandatory)]
        [int]$Count
    )

    $listeners = [System.Collections.Generic.List[Net.Sockets.TcpListener]]::new()
    try {
        for ($index = 0; $index -lt $Count; $index++) {
            $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
            $listener.Start()
            $listeners.Add($listener)
        }
        return @($listeners | ForEach-Object { ([Net.IPEndPoint]$_.LocalEndpoint).Port })
    }
    finally {
        foreach ($listener in $listeners) {
            $listener.Stop()
        }
    }
}

function Start-OwnedProcess {
    param(
        [Parameter(Mandatory)]
        [string]$FilePath,

        [Parameter(Mandatory)]
        [string[]]$ArgumentList,

        [Parameter(Mandatory)]
        [string]$StandardOutput,

        [Parameter(Mandatory)]
        [string]$StandardError
    )

    $parameters = @{
        FilePath               = $FilePath
        ArgumentList           = $ArgumentList
        PassThru               = $true
        RedirectStandardOutput = $StandardOutput
        RedirectStandardError  = $StandardError
    }
    if ($IsWindows) {
        $parameters.WindowStyle = 'Hidden'
    }
    return Start-Process @parameters
}

function Wait-HTTP {
    param(
        [Parameter(Mandatory)]
        [string]$Uri,

        [Parameter(Mandatory)]
        [Diagnostics.Process]$Process,

        [int]$TimeoutSeconds = 15
    )

    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        if ($Process.HasExited) {
            throw "Process $($Process.Id) exited before $Uri became ready."
        }
        try {
            $response = Invoke-WebRequest -Uri $Uri -TimeoutSec 2
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 500) {
                return
            }
        }
        catch {
            Start-Sleep -Milliseconds 100
        }
    } while ([DateTime]::UtcNow -lt $deadline)

    throw "Timed out waiting for $Uri"
}

function Invoke-JSON {
    param(
        [Parameter(Mandatory)]
        [ValidateSet('GET', 'POST', 'PUT')]
        [string]$Method,

        [Parameter(Mandatory)]
        [string]$Uri,

        [object]$Body
    )

    $parameters = @{
        Method     = $Method
        Uri        = $Uri
        TimeoutSec = 10
    }
    if ($null -ne $Body) {
        $parameters.ContentType = 'application/json'
        $parameters.Body = $Body | ConvertTo-Json -Depth 8 -Compress
    }
    return Invoke-RestMethod @parameters
}

function Assert-True {
    param(
        [Parameter(Mandatory)]
        [bool]$Condition,

        [Parameter(Mandatory)]
        [string]$Message
    )

    if (-not $Condition) {
        throw $Message
    }
}

function Write-LogTail {
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )

    if (Test-Path -LiteralPath $Path) {
        [Console]::Error.WriteLine("--- $([IO.Path]::GetFileName($Path)) ---")
        foreach ($line in Get-Content -LiteralPath $Path -Tail 40) {
            [Console]::Error.WriteLine($line)
        }
    }
}

Push-Location $projectRoot
try {
    $ports = @(Get-FreeLoopbackPorts -Count 2)
    $mockPort = $ports[0]
    $appPort = $ports[1]
    $suffix = if ($IsWindows) { '.exe' } else { '' }
    $mockBinary = Join-Path $smokeRoot ('mihomo-mock' + $suffix)
    $appBinary = Join-Path $smokeRoot ('mihomo-smart-selector' + $suffix)
    $configPath = Join-Path $smokeRoot 'config.yaml'
    $mockOut = Join-Path $smokeRoot 'mock.stdout.log'
    $mockErr = Join-Path $smokeRoot 'mock.stderr.log'
    $appOut = Join-Path $smokeRoot 'app.stdout.log'
    $appErr = Join-Path $smokeRoot 'app.stderr.log'

    & $go build -trimpath -o $mockBinary ./cmd/mihomo-mock
    & $go build -trimpath -o $appBinary ./cmd/mihomo-smart-selector

    $configuration = @"
http:
  listen: 127.0.0.1:$appPort
mihomo:
  controller: http://127.0.0.1:$mockPort
  secret_env: MIHOMO_SECRET
  request_timeout_seconds: 2
storage:
  path: data/smoke-selector.db
scanner:
  concurrency: 2
  batch_size: 2
  max_total_candidates: 20
  samples: 1
  timeout_ms: 1000
  min_success_rate: 0.95
  median_target_ms: 300
  p95_target_ms: 800
  jitter_target_ms: 200
  probes:
    - name: smoke-reachability
      url: https://example.test/generate_204
      expected_status: "204"
regions:
  - code: JP
    name: Japan
    aliases: [JP, Japan, Tokyo, Osaka]
  - code: US
    name: United States
    aliases: [US, USA, Los Angeles]
  - code: KR
    name: Korea
    aliases: [KR, Korea, Seoul]
region_overrides: {}
egress_verification:
  enabled: false
auto_switch:
  enabled: false
"@
    [IO.File]::WriteAllText($configPath, $configuration, [Text.UTF8Encoding]::new($false))
    [Environment]::SetEnvironmentVariable('MIHOMO_SECRET', 'smoke-test-only', 'Process')

    $mockProcess = Start-OwnedProcess -FilePath $mockBinary -ArgumentList @('-listen', "127.0.0.1:$mockPort") -StandardOutput $mockOut -StandardError $mockErr
    Wait-HTTP -Uri "http://127.0.0.1:$mockPort/version" -Process $mockProcess
    $appProcess = Start-OwnedProcess -FilePath $appBinary -ArgumentList @('-config', $configPath) -StandardOutput $appOut -StandardError $appErr
    $baseUri = "http://127.0.0.1:$appPort"
    Wait-HTTP -Uri "$baseUri/api/v1/health" -Process $appProcess

    $health = Invoke-JSON -Method GET -Uri "$baseUri/api/v1/health"
    Assert-True ($health.status -eq 'ok' -and $health.mihomo_connected) 'Health did not report a connected Mihomo Controller.'

    $shell = Invoke-WebRequest -Uri "$baseUri/" -TimeoutSec 10
    Assert-True ($shell.StatusCode -eq 200 -and $shell.Content.Contains('Mihomo Smart Selector')) 'Embedded SPA shell was not served.'

    $groups = @((Invoke-JSON -Method GET -Uri "$baseUri/api/v1/groups") | ForEach-Object { foreach ($item in $_) { $item } })
    Assert-True (@($groups | Where-Object { $_.name -eq '🤖 ChatGPT' }).Count -eq 1) 'Expected ChatGPT Selector was not discovered.'
    $providers = @((Invoke-JSON -Method GET -Uri "$baseUri/api/v1/providers") | ForEach-Object { foreach ($item in $_) { $item } })
    Assert-True (@($providers | Where-Object { $_.name -eq 'Sakura Network' }).Count -eq 1) 'Expected provider was not discovered.'
    $nodes = @((Invoke-JSON -Method GET -Uri "$baseUri/api/v1/nodes") | ForEach-Object { foreach ($item in $_) { $item } })
    Assert-True ($nodes.Count -ge 5) "Expected at least five leaf nodes, got: $($nodes | ConvertTo-Json -Depth 4 -Compress)"

    $request = @{
        target_group = '🤖 ChatGPT'
        regions      = @('JP')
        providers    = @('Sakura Network')
        mode         = 'quick'
    }
    $preview = Invoke-JSON -Method POST -Uri "$baseUri/api/v1/scans/preflight" -Body $request
    Assert-True ($preview.ready -and $preview.candidate_count -eq 3 -and $preview.profile.id -eq 'chatgpt') 'Scan preflight did not expose the expected three ChatGPT candidates.'

    $created = Invoke-JSON -Method POST -Uri "$baseUri/api/v1/scans" -Body $request
    Assert-True (-not [string]::IsNullOrWhiteSpace($created.id)) 'Scan creation returned no ID.'
    $deadline = [DateTime]::UtcNow.AddSeconds(15)
    do {
        $scan = Invoke-JSON -Method GET -Uri "$baseUri/api/v1/scans/$($created.id)"
        if ($scan.status -eq 'complete') {
            break
        }
        if ($scan.status -in @('failed', 'cancelled')) {
            throw "Smoke scan ended with status $($scan.status): $($scan.error)"
        }
        Start-Sleep -Milliseconds 100
    } while ([DateTime]::UtcNow -lt $deadline)
    Assert-True ($scan.status -eq 'complete') 'Smoke scan did not complete before the deadline.'

    $results = @($scan.results | Sort-Object rank)
    Assert-True ($results.Count -eq 3) 'Smoke scan did not return three ranked results.'
    $best = $results[0]
    Assert-True ($best.name -eq 'JP-Tokyo-03' -and $best.rank -eq 1) 'Smoke scan did not rank the lowest-delay node first.'

    $selection = Invoke-JSON -Method POST -Uri "$baseUri/api/v1/scans/$($created.id)/select" -Body @{ node = $best.name }
    Assert-True ($selection.selected -eq $best.name -and $selection.group -eq '🤖 ChatGPT') 'Selection endpoint did not return the expected audit event.'
    $controllerState = Invoke-JSON -Method GET -Uri "http://127.0.0.1:$mockPort/proxies"
    $selectedGroup = $controllerState.proxies.PSObject.Properties['🤖 ChatGPT'].Value
    Assert-True ($selectedGroup.now -eq $best.name) 'Mock Controller did not receive the Selector change.'
    $history = @((Invoke-JSON -Method GET -Uri "$baseUri/api/v1/history?limit=5") | ForEach-Object { foreach ($item in $_) { $item } })
    Assert-True ($history.Count -ge 1 -and $history[0].selected -eq $best.name) 'Selection history was not persisted.'

    # Prove next-start connection overrides through a real process restart.
    $connection = Invoke-JSON -Method GET -Uri "$baseUri/api/v1/connection"
    $originalAddress = $connection.active.controller
    $connectionDraft = @{
        revision = $connection.revision
        controller = "http://localhost:$mockPort"
        request_timeout_seconds = 7
        secret_action = 'replace'
        secret = 'smoke-ui-only'
    }
    $connectionTest = Invoke-JSON -Method POST -Uri "$baseUri/api/v1/connection/test" -Body $connectionDraft
    Assert-True ($connectionTest.connected -and $connectionTest.version -eq 'dev-mock') 'Connection draft test failed.'
    $savedConnection = Invoke-JSON -Method PUT -Uri "$baseUri/api/v1/connection" -Body $connectionDraft
    Assert-True ($savedConnection.restart_required -and $savedConnection.active.controller -eq $originalAddress) 'Connection save changed the active Controller before restart.'
    Assert-True (-not ($savedConnection | ConvertTo-Json -Depth 5).Contains('smoke-ui-only')) 'Connection response exposed the secret.'

    Stop-Process -Id $appProcess.Id -Force
    Assert-True ($appProcess.WaitForExit(5000)) 'Original smoke service did not stop.'
    $appProcess = Start-OwnedProcess -FilePath $appBinary -ArgumentList @('-config', $configPath) -StandardOutput $appOut -StandardError $appErr
    Wait-HTTP -Uri "$baseUri/api/v1/health" -Process $appProcess
    $appliedConnection = Invoke-JSON -Method GET -Uri "$baseUri/api/v1/connection"
    Assert-True (-not $appliedConnection.restart_required -and $appliedConnection.active.controller -eq $connectionDraft.controller -and $appliedConnection.active.secret_source -eq 'custom') 'Saved connection did not survive restart.'
    $oldScan = Invoke-JSON -Method GET -Uri "$baseUri/api/v1/scans/$($created.id)"
    Assert-True ($oldScan.results[0].selection_reason -eq '扫描不属于当前 Controller，请重新扫描') 'Old scan was usable against the changed Controller.'

    $null = Invoke-JSON -Method PUT -Uri "$baseUri/api/v1/connection" -Body @{ revision = $appliedConnection.revision; use_server = $true }
    Stop-Process -Id $appProcess.Id -Force
    Assert-True ($appProcess.WaitForExit(5000)) 'Overridden smoke service did not stop.'
    $appProcess = Start-OwnedProcess -FilePath $appBinary -ArgumentList @('-config', $configPath) -StandardOutput $appOut -StandardError $appErr
    Wait-HTTP -Uri "$baseUri/api/v1/health" -Process $appProcess
    $restoredConnection = Invoke-JSON -Method GET -Uri "$baseUri/api/v1/connection"
    Assert-True (-not $restoredConnection.override -and -not $restoredConnection.restart_required -and $restoredConnection.active.controller -eq $originalAddress) 'YAML defaults did not survive restart.'

    [ordered]@{
        status          = 'passed'
        candidate_count = $preview.candidate_count
        result_count    = $results.Count
        best_node       = $best.name
        selected_node   = $selection.selected
        connection_restart = 'passed'
    } | ConvertTo-Json -Compress
}
catch {
    Write-LogTail -Path (Join-Path $smokeRoot 'mock.stderr.log')
    Write-LogTail -Path (Join-Path $smokeRoot 'app.stderr.log')
    throw
}
finally {
    foreach ($process in @($appProcess, $mockProcess)) {
        if ($null -ne $process) {
            try {
                if (-not $process.HasExited) {
                    Stop-Process -Id $process.Id -Force
                    Wait-Process -Id $process.Id -Timeout 5 -ErrorAction SilentlyContinue
                }
            }
            catch {
                [Console]::Error.WriteLine("Could not stop smoke-test process $($process.Id): $($_.Exception.Message)")
            }
        }
    }
    [Environment]::SetEnvironmentVariable('MIHOMO_SECRET', $previousSecret, 'Process')
    Pop-Location

    if (Test-Path -LiteralPath $smokeRoot) {
        $resolvedSmokeRoot = (Resolve-Path -LiteralPath $smokeRoot).Path
        if (-not $resolvedSmokeRoot.StartsWith($tempBase + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
            throw "Refusing to remove unexpected smoke-test path: $resolvedSmokeRoot"
        }
        [IO.Directory]::Delete($resolvedSmokeRoot, $true)
    }
}
