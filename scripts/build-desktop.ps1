param(
    [Alias('o')]
    [string]$OutputName = 'adm-desktop-windows-amd64.exe',
    [string]$OutputDir = '',
    [string]$Version = '',
    [switch]$clean,
    [switch]$trimpath
)

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$desktopRoot = Join-Path $repoRoot 'cmd\ai-dev-manager-desktop'
if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    $OutputDir = Join-Path $repoRoot 'dist'
} elseif (-not [System.IO.Path]::IsPathRooted($OutputDir)) {
    $OutputDir = Join-Path $repoRoot $OutputDir
}

$resolvedVersion = $Version.Trim()
if ([string]::IsNullOrWhiteSpace($resolvedVersion)) {
    $resolvedVersion = 'dev'
    if ($null -ne (Get-Command git -ErrorAction SilentlyContinue)) {
        Push-Location $repoRoot
        try {
            $tags = @(& git tag --points-at HEAD)
            if ($LASTEXITCODE -eq 0) {
                $tag = $tags | Where-Object { $_ -like 'v*' } | Select-Object -First 1
                if ([string]::IsNullOrWhiteSpace($tag)) {
                    $tag = $tags | Select-Object -First 1
                }
                if (-not [string]::IsNullOrWhiteSpace($tag)) {
                    $resolvedVersion = $tag.Trim()
                }
            }
        } finally {
            Pop-Location
        }
    }
}
if ($resolvedVersion -match '\s') {
    throw "Version must not contain whitespace: $resolvedVersion"
}
$productVersionLdflag = "-X ai-dev-manager-v2/internal/version.Version=$resolvedVersion"

$wailsArgs = @('build', '-skipbindings', '-ldflags', $productVersionLdflag)
if ($clean) { $wailsArgs += '-clean' }
if ($trimpath) { $wailsArgs += '-trimpath' }
$wailsArgs += @('-o', $OutputName)

Push-Location $desktopRoot
try {
    # wails.json owns Desktop pre-build preparation, including icon refresh.
    & go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 @wailsArgs
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}

$builtDesktop = Join-Path $desktopRoot (Join-Path 'build\bin' $OutputName)
if (-not (Test-Path -LiteralPath $builtDesktop)) {
    throw "Wails Desktop output not found: $builtDesktop"
}
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$finalDesktop = Join-Path $OutputDir $OutputName
Copy-Item -LiteralPath $builtDesktop -Destination $finalDesktop -Force
Write-Host "Desktop artifact: $finalDesktop"
