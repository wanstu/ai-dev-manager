param(
    [Parameter(Mandatory = $true)][string]$PreviousSetup,
    [Parameter(Mandatory = $true)][string]$NextSetup,
    [Parameter(Mandatory = $true)][string]$PreviousVersion,
    [Parameter(Mandatory = $true)][string]$NextVersion
)

$ErrorActionPreference = 'Stop'
$registryPath = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\com.wanstu.adm-desktop'
$installDir = Join-Path $env:RUNNER_TEMP 'adm-desktop-upgrade-acceptance'
$exe = Join-Path $installDir 'adm-desktop.exe'
$uninstall = Join-Path $installDir 'Uninstall.exe'
$marker = Join-Path $env:USERPROFILE '.config\adm\kit-rc4-installer-e2e.marker'

function Test-SetupChecksum([string]$setup) {
    if (-not (Test-Path -LiteralPath $setup -PathType Leaf)) { throw "Missing Setup: $setup" }
    $sidecar = "$setup.sha256"
    if (-not (Test-Path -LiteralPath $sidecar -PathType Leaf)) { throw "Missing SHA256 sidecar: $sidecar" }
    $text = Get-Content -LiteralPath $sidecar -Raw
    if ($text -notmatch '(?i)\b([0-9a-f]{64})\b') { throw "Malformed checksum: $sidecar" }
    $expected = $Matches[1].ToLowerInvariant()
    $actual = (Get-FileHash -LiteralPath $setup -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "Checksum mismatch for $setup" }
}

function Get-ADMInstall {
    foreach ($view in @([Microsoft.Win32.RegistryView]::Registry64, [Microsoft.Win32.RegistryView]::Registry32)) {
        $base = [Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::CurrentUser, $view)
        try {
            $key = $base.OpenSubKey($registryPath)
            if ($null -eq $key) { continue }
            try {
                return @{
                    InstallLocation = [string]$key.GetValue('InstallLocation')
                    DisplayVersion = [string]$key.GetValue('DisplayVersion')
                    DisplayIcon = [string]$key.GetValue('DisplayIcon')
                }
            } finally { $key.Dispose() }
        } finally { $base.Dispose() }
    }
    return $null
}

function Assert-Installed([string]$version) {
    if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) { throw "ADM executable is missing: $exe" }
    if (-not (Test-Path -LiteralPath $uninstall -PathType Leaf)) { throw "Uninstaller is missing: $uninstall" }
    $entry = Get-ADMInstall
    if ($null -eq $entry) { throw "HKCU uninstall entry is missing" }
    if ($entry.DisplayVersion -ne $version.TrimStart('v')) { throw "Unexpected DisplayVersion: $($entry.DisplayVersion), expected $version" }
    if (-not [string]::Equals([IO.Path]::GetFullPath($entry.InstallLocation), [IO.Path]::GetFullPath($installDir), [StringComparison]::OrdinalIgnoreCase)) {
        throw "Unexpected InstallLocation: $($entry.InstallLocation)"
    }
    if (-not $entry.DisplayIcon.Contains('adm-desktop.exe')) { throw "Unexpected DisplayIcon: $($entry.DisplayIcon)" }
}

function Invoke-Setup([string]$setup) {
    # Windows GUI processes invoked with & need not block or populate LASTEXITCODE.
    # Wait for NSIS explicitly before inspecting the installed files/registry.
    $absolute = (Resolve-Path -LiteralPath $setup).Path
    $process = Start-Process -FilePath $absolute -ArgumentList @('/S', "/D=$installDir") -Wait -PassThru
    if ($process.ExitCode -ne 0) { throw "NSIS Setup failed with code $($process.ExitCode): $setup" }
}

if (-not $env:RUNNER_TEMP) { throw 'Only run on an isolated Windows CI runner with RUNNER_TEMP' }
if (Test-Path -LiteralPath $installDir) { throw "Refusing to overwrite existing directory: $installDir" }
if (Test-Path -LiteralPath $marker) { throw "Refusing to overwrite existing business data: $marker" }
Test-SetupChecksum $PreviousSetup
Test-SetupChecksum $NextSetup

try {
    Write-Host "=== Fresh install $PreviousVersion ==="
    Invoke-Setup $PreviousSetup
    Assert-Installed $PreviousVersion
    $before = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash
    $dataDir = Split-Path -Parent $marker
    New-Item -ItemType Directory -Force -Path $dataDir | Out-Null
    Set-Content -LiteralPath $marker -Value 'keep-business-data-across-kit-upgrade-and-uninstall' -NoNewline

    Write-Host '=== Reproduce: same-EXE --gateway-child blocks silent Setup ==='
    # Only a disposable CI runner enters this test. The child is started by
    # this script from the freshly installed EXE; no user process is touched.
    $testGateway = $null
    $gatewayOutput = Join-Path $env:RUNNER_TEMP 'adm-gateway-upgrade-e2e.stdout.log'
    $gatewayErrors = Join-Path $env:RUNNER_TEMP 'adm-gateway-upgrade-e2e.stderr.log'
    try {
        $testGateway = Start-Process -FilePath $exe -ArgumentList @('--gateway-child', '--listen', '127.0.0.1:0') -PassThru -RedirectStandardOutput $gatewayOutput -RedirectStandardError $gatewayErrors
        Start-Sleep -Seconds 2
        $testGateway.Refresh()
        if ($testGateway.HasExited) {
            throw "Test Gateway child exited unexpectedly ($($testGateway.ExitCode)): $(Get-Content -LiteralPath $gatewayErrors -Raw)"
        }
        $nextAbsolute = (Resolve-Path -LiteralPath $NextSetup).Path
        $blockedSetup = Start-Process -FilePath $nextAbsolute -ArgumentList @('/S', "/D=$installDir") -Wait -PassThru
        if ($blockedSetup.ExitCode -ne 3) {
            throw "Expected Kit Setup to refuse running --gateway-child with exit 3, got $($blockedSetup.ExitCode)"
        }
        Assert-Installed $PreviousVersion
        if ((Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash -ne $before) {
            throw 'Blocked Setup modified the installed Desktop EXE'
        }
    } finally {
        if ($null -ne $testGateway) {
            $testGateway.Refresh()
            if (-not $testGateway.HasExited) {
                $testGateway.Kill()  # Dedicated CI test child, never an existing service.
                if (-not $testGateway.WaitForExit(10000)) { throw 'Test Gateway child did not exit' }
            }
            $testGateway.Dispose()
        }
    }

    Write-Host "=== Silent overwrite upgrade $PreviousVersion -> $NextVersion after child exit ==="
    Invoke-Setup $NextSetup
    Assert-Installed $NextVersion
    $after = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash
    if ($before -eq $after) { throw 'Installed Desktop executable did not change on upgrade' }
    if ((Get-Content -LiteralPath $marker -Raw) -ne 'keep-business-data-across-kit-upgrade-and-uninstall') {
        throw 'Business data was modified during upgrade'
    }

    Write-Host '=== Silent uninstall preserves business data ==='
    $process = Start-Process -FilePath $uninstall -ArgumentList '/S' -Wait -PassThru
    if ($process.ExitCode -ne 0) { throw "Silent uninstall failed with $($process.ExitCode)" }
    if (Test-Path -LiteralPath $exe) { throw 'Desktop executable retained after uninstall' }
    if ($null -ne (Get-ADMInstall)) { throw 'Uninstall registration retained after uninstall' }
    if ((Get-Content -LiteralPath $marker -Raw) -ne 'keep-business-data-across-kit-upgrade-and-uninstall') {
        throw 'Business data was not preserved after uninstall'
    }
    Write-Host 'PASS: real Windows install, overwrite, uninstall, registry, version and data retention'
} finally {
    if (Test-Path -LiteralPath $uninstall -PathType Leaf) {
        $null = Start-Process -FilePath $uninstall -ArgumentList '/S' -Wait -PassThru
    }
    if (Test-Path -LiteralPath $marker -PathType Leaf) { Remove-Item -LiteralPath $marker -Force }
}
