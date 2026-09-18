param(
    [string]$Version = 'dev',
    [string]$OutputDir = '',
    [string]$Platform = 'linux/amd64',
    [string]$WebKitTag = 'webkit2_41'
)

$ErrorActionPreference = 'Stop'

if (-not $IsLinux) {
    throw 'build-linux-desktop.ps1 must run on Linux.'
}
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = 'dev'
}
$Version = $Version.Trim()
if ($Version -match '\s') {
    throw "Version must not contain whitespace: $Version"
}
if ($Platform -notin @('linux', 'linux/amd64', 'linux/arm64')) {
    throw "Unsupported Linux Wails platform: $Platform"
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
    'linux/amd64' { 'linux-amd64' }
    'linux/arm64' { 'linux-arm64' }
    default { "linux-$(& go env GOARCH)" }
}
$outputName = "adm-desktop-$Version-$archName"
$productVersionLdflag = "-X ai-dev-manager-v2/internal/version.Version=$Version"

New-Item -ItemType Directory -Force -Path $distRoot | Out-Null

Push-Location $desktopRoot
try {
    $args = @(
        'run', 'github.com/wailsapp/wails/v2/cmd/wails@v2.15.0',
        'build', '-skipbindings', '-clean', '-trimpath',
        '-platform', $Platform,
        '-ldflags', $productVersionLdflag,
        '-o', $outputName
    )
    if (-not [string]::IsNullOrWhiteSpace($WebKitTag)) {
        $args += @('-tags', $WebKitTag.Trim())
    }
    & go @args
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}

$builtDesktop = Join-Path (Join-Path (Join-Path $desktopRoot 'build') 'bin') $outputName
if (-not (Test-Path -LiteralPath $builtDesktop -PathType Leaf)) {
    throw "Wails Linux Desktop output not found: $builtDesktop"
}
$finalDesktop = Join-Path $distRoot $outputName
Copy-Item -LiteralPath $builtDesktop -Destination $finalDesktop -Force
Write-Host "Linux Desktop artifact: $finalDesktop"
