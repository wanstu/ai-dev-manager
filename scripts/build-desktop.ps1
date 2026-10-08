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

# Wails uses wails.json Info for Windows EXE FileVersion/ProductVersion.
# The -ldflags version only updates the Go runtime. Inject build metadata
# into a transient wails.json and restore the tracked bytes afterwards.
$numericFileVersion = '0.0.0.0'
if ($resolvedVersion -match '^v?(\d+)\.(\d+)\.(\d+)(?:-rc\.(\d+))?$') {
    $rcNumber = if ($Matches[4]) { [int]$Matches[4] } else { 0 }
    $numericFileVersion = "$($Matches[1]).$($Matches[2]).$($Matches[3]).$rcNumber"
} elseif ($resolvedVersion -ne 'dev') {
    throw "Version must be vMAJOR.MINOR.PATCH, vMAJOR.MINOR.PATCH-rc.N, or dev: $resolvedVersion"
}
$wailsConfigPath = Join-Path $desktopRoot 'wails.json'
$originalConfigBytes = [System.IO.File]::ReadAllBytes($wailsConfigPath)
try {
    $config = Get-Content -LiteralPath $wailsConfigPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $config | Add-Member -NotePropertyName 'info' -NotePropertyValue @{
        companyName = 'wanstu'
        productName = 'AI Dev Manager'
        productVersion = $numericFileVersion
        comments = "ADM $resolvedVersion"
    } -Force
    [System.IO.File]::WriteAllText($wailsConfigPath, ($config | ConvertTo-Json -Depth 15), (New-Object System.Text.UTF8Encoding($false)))
    Push-Location $desktopRoot
    try {
        # wails.json owns Desktop pre-build preparation, including icon refresh.
        & go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 @wailsArgs
        if ($LASTEXITCODE -ne 0) {
            throw "Wails build failed (exit code $LASTEXITCODE)"
        }
    } finally {
        Pop-Location
    }
} finally {
    [System.IO.File]::WriteAllBytes($wailsConfigPath, $originalConfigBytes)
}

$builtDesktop = Join-Path $desktopRoot (Join-Path 'build\bin' $OutputName)
if (-not (Test-Path -LiteralPath $builtDesktop)) {
    throw "Wails Desktop output not found: $builtDesktop"
}
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$finalDesktop = Join-Path $OutputDir $OutputName
Copy-Item -LiteralPath $builtDesktop -Destination $finalDesktop -Force
# Fail the build rather than shipping another EXE whose Windows Explorer
# Properties disagree with the in-app/CLI product version.
if ($IsWindows) {
    $resourceInfo = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($finalDesktop)
    $fileVersion = '{0}.{1}.{2}.{3}' -f $resourceInfo.FileMajorPart, $resourceInfo.FileMinorPart, $resourceInfo.FileBuildPart, $resourceInfo.FilePrivatePart
    # Wails writes ProductVersion into StringFileInfo (Explorer Details),
    # while VS_FIXEDFILEINFO product-version numeric fields default to zero.
    $productVersion = [string]$resourceInfo.ProductVersion
    if ($fileVersion -ne $numericFileVersion -or $productVersion -ne $numericFileVersion) {
        throw "Windows EXE VERSIONINFO mismatch: FileVersion=$fileVersion ProductVersion=$productVersion expected=$numericFileVersion"
    }
    if ($resourceInfo.ProductName -ne 'AI Dev Manager') {
        throw "Windows EXE ProductName mismatch: $($resourceInfo.ProductName)"
    }
    if ($resourceInfo.Comments -ne "ADM $resolvedVersion") {
        throw "Windows EXE Comments mismatch: expected ADM $resolvedVersion, actual $($resourceInfo.Comments)"
    }
    Write-Host "Verified Windows FileVersion/ProductVersion: $fileVersion ($resolvedVersion)"
}
Write-Host "Desktop artifact: $finalDesktop"
