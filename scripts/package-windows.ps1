param(
    [Parameter(Mandatory = $true)]
    [string]$InputPath,
    [string]$OutputDir = 'dist',
    [string]$Version = 'dev',
    [string]$NSISPath = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot

if ($Version -notmatch '^(?:dev|v?\d+\.\d+\.\d+(?:-rc\.\d+)?)$') {
    throw "Invalid installer version: $Version"
}
if (-not [System.IO.Path]::IsPathRooted($InputPath)) {
    $InputPath = Join-Path $repoRoot $InputPath
}
if (-not [System.IO.Path]::IsPathRooted($OutputDir)) {
    $OutputDir = Join-Path $repoRoot $OutputDir
}
if (-not (Test-Path -LiteralPath $InputPath -PathType Leaf)) {
    throw "ADM Desktop EXE was not found: $InputPath"
}
if ([System.IO.Path]::GetExtension($InputPath) -ne '.exe') {
    throw "ADM Desktop installer requires a Windows .exe: $InputPath"
}
if ([string]::IsNullOrWhiteSpace($NSISPath)) {
    $command = Get-Command 'makensis.exe' -ErrorAction SilentlyContinue
    if ($null -ne $command) {
        $NSISPath = $command.Source
    } else {
        $candidates = @(
            (Join-Path ${env:ProgramFiles(x86)} 'NSIS\makensis.exe'),
            (Join-Path $env:ProgramFiles 'NSIS\makensis.exe')
        )
        $NSISPath = $candidates | Where-Object { $_ -and (Test-Path -LiteralPath $_ -PathType Leaf) } | Select-Object -First 1
    }
}
if ([string]::IsNullOrWhiteSpace($NSISPath) -or -not (Test-Path -LiteralPath $NSISPath -PathType Leaf)) {
    throw 'NSIS (makensis.exe) is required. Install NSIS before packaging ADM Desktop.'
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$assetBase = "adm-desktop-$Version"
$setupPath = Join-Path $OutputDir "$assetBase-windows-amd64-setup.exe"
$zipPath = Join-Path $OutputDir "$assetBase-windows-amd64.zip"

Push-Location $repoRoot
try {
    # Only package the existing production Wails EXE; never rebuild it.
    & go run 'github.com/wanstu/wails-desktop-kit/cmd/desktopkit@v0.11.3' package windows `
        --input $InputPath `
        --dist $OutputDir `
        --app-name 'adm-desktop' `
        --asset-base $assetBase `
        --package-version $Version.TrimStart('v') `
        --arch 'amd64' `
        --product-name 'AI Dev Manager' `
        --publisher 'wanstu' `
        --app-id 'com.wanstu.adm-desktop' `
        --install-scope 'user' `
        --start-menu-shortcut=true `
        --desktop-shortcut=false `
        --nsis $NSISPath
    if ($LASTEXITCODE -ne 0) {
        throw "Desktop Kit installer packaging failed (exit code $LASTEXITCODE)"
    }
} finally {
    Pop-Location
}

if (-not (Test-Path -LiteralPath $setupPath -PathType Leaf)) {
    throw "Expected Windows Setup artifact is missing: $setupPath"
}
if (-not (Test-Path -LiteralPath "$setupPath.sha256" -PathType Leaf)) {
    throw "Expected Windows Setup SHA256 sidecar is missing: $setupPath.sha256"
}
if ((Get-Item -LiteralPath $setupPath).Length -eq 0) {
    throw "Windows Setup artifact is empty: $setupPath"
}

# ZIP is for portable usage; the package installer is a separate asset.
Compress-Archive -LiteralPath $InputPath -DestinationPath $zipPath -CompressionLevel Optimal -Force
if (-not (Test-Path -LiteralPath $zipPath -PathType Leaf) -or (Get-Item -LiteralPath $zipPath).Length -eq 0) {
    throw "Windows Portable ZIP artifact is missing or empty: $zipPath"
}
Write-Host "ADM Desktop Setup: $setupPath"
Write-Host "ADM Desktop Portable ZIP: $zipPath"
