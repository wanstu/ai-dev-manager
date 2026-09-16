param(
    [string]$Version = 'dev',
    [string]$OutputDir = '',
    [string]$Platform = 'darwin/universal'
)

$ErrorActionPreference = 'Stop'

if (-not $IsMacOS) {
    throw 'build-macos-desktop.ps1 must run on macOS.'
}
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = 'dev'
}
$Version = $Version.Trim()
if ($Version -match '\s') {
    throw "Version must not contain whitespace: $Version"
}
if ($Platform -notin @('darwin', 'darwin/amd64', 'darwin/arm64', 'darwin/universal')) {
    throw "Unsupported macOS Wails platform: $Platform"
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$desktopRoot = Join-Path (Join-Path $repoRoot 'cmd') 'ai-dev-manager-desktop'
if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    $distRoot = Join-Path $repoRoot 'dist'
} elseif ([System.IO.Path]::IsPathRooted($OutputDir)) {
    $distRoot = $OutputDir
} else {
    $distRoot = Join-Path $repoRoot $OutputDir
}

$archName = switch ($Platform) {
    'darwin/amd64' { 'darwin-amd64' }
    'darwin/arm64' { 'darwin-arm64' }
    'darwin/universal' { 'darwin-universal' }
    default { "darwin-$(& go env GOARCH)" }
}
$outputBase = "adm-desktop-$Version-$archName"
$productVersionLdflag = "-X ai-dev-manager-v2/internal/version.Version=$Version"

New-Item -ItemType Directory -Force -Path $distRoot | Out-Null

Push-Location $desktopRoot
try {
    & go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -skipbindings -clean -trimpath -platform $Platform -ldflags $productVersionLdflag -o $outputBase
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}

$binRoot = Join-Path (Join-Path $desktopRoot 'build') 'bin'
# Wails v2 keeps the bundle name from wails.json (`adm-desktop.app`). The
# `-o` value names the executable inside Contents/MacOS, not the .app folder.
$appPath = Join-Path $binRoot 'adm-desktop.app'
if (-not (Test-Path -LiteralPath $appPath -PathType Container)) {
    $candidate = Get-ChildItem -LiteralPath $binRoot -Directory -Filter '*.app' | Select-Object -First 1
    if ($null -eq $candidate) {
        throw "Wails macOS app bundle not found under $binRoot"
    }
    $appPath = $candidate.FullName
}

$executablePath = Join-Path (Join-Path (Join-Path $appPath 'Contents') 'MacOS') $outputBase
if (-not (Test-Path -LiteralPath $executablePath -PathType Leaf)) {
    throw "Wails macOS executable not found: $executablePath"
}
if ($Platform -eq 'darwin/universal') {
    $architectures = (& lipo -archs $executablePath).Trim().Split(' ', [System.StringSplitOptions]::RemoveEmptyEntries)
    if ($LASTEXITCODE -ne 0) {
        throw "lipo failed for $executablePath"
    }
    foreach ($requiredArchitecture in @('arm64', 'x86_64')) {
        if ($architectures -notcontains $requiredArchitecture) {
            throw "macOS universal executable is missing ${requiredArchitecture}: $($architectures -join ', ')"
        }
    }
    Write-Host "macOS universal architectures: $($architectures -join ', ')"
}

# Experience builds are not Developer-ID signed/notarized yet. Ad-hoc signing
# keeps the bundle internally consistent while Phase 27 later adds release signing.
& codesign --force --deep --sign - $appPath
if ($LASTEXITCODE -ne 0) {
    throw "Ad-hoc codesign failed for $appPath"
}
& codesign --verify --deep --strict $appPath
if ($LASTEXITCODE -ne 0) {
    throw "codesign verification failed for $appPath"
}

$zipName = "$outputBase.zip"
$zipPath = Join-Path $distRoot $zipName
if (Test-Path -LiteralPath $zipPath) {
    Remove-Item -LiteralPath $zipPath -Force
}
& ditto -c -k --sequesterRsrc --keepParent $appPath $zipPath
if ($LASTEXITCODE -ne 0) {
    throw "Failed to package macOS app: $zipPath"
}

Write-Host "macOS Desktop app: $appPath"
Write-Host "macOS Desktop artifact: $zipPath"
