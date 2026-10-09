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
    & $setup '/S' "/D=$installDir"
    if ($LASTEXITCODE -ne 0) { throw "NSIS Setup failed with code $LASTEXITCODE : $setup" }
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

    Write-Host "=== Silent overwrite upgrade $PreviousVersion -> $NextVersion ==="
    Invoke-Setup $NextSetup
    Assert-Installed $NextVersion
    $after = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash
    if ($before -eq $after) { throw 'Installed Desktop executable did not change on upgrade' }
    if ((Get-Content -LiteralPath $marker -Raw) -ne 'keep-business-data-across-kit-upgrade-and-uninstall') {
        throw 'Business data was modified during upgrade'
    }

    Write-Host '=== Silent uninstall preserves business data ==='
    & $uninstall '/S'
    if ($LASTEXITCODE -ne 0) { throw "Silent uninstall failed with $LASTEXITCODE" }
    if (Test-Path -LiteralPath $exe) { throw 'Desktop executable retained after uninstall' }
    if ($null -ne (Get-ADMInstall)) { throw 'Uninstall registration retained after uninstall' }
    if ((Get-Content -LiteralPath $marker -Raw) -ne 'keep-business-data-across-kit-upgrade-and-uninstall') {
        throw 'Business data was not preserved after uninstall'
    }
    Write-Host 'PASS: real Windows install, overwrite, uninstall, registry, version and data retention'
} finally {
    if (Test-Path -LiteralPath $uninstall -PathType Leaf) {
        & $uninstall '/S' | Out-Null
    }
    if (Test-Path -LiteralPath $marker -PathType Leaf) { Remove-Item -LiteralPath $marker -Force }
}
