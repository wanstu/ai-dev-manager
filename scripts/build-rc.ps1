param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$OutputDir = ''
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($Version)) {
    throw 'Version is required, for example v1.0.0-rc.4'
}
$Version = $Version.Trim()
if ($Version -match '\s') {
    throw "Version must not contain whitespace: $Version"
}

$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    $distRoot = Join-Path $repoRoot 'dist'
} elseif ([System.IO.Path]::IsPathRooted($OutputDir)) {
    $distRoot = $OutputDir
} else {
    $distRoot = Join-Path $repoRoot $OutputDir
}

$cliName = "adm-$Version-windows-amd64.exe"
$desktopName = "adm-desktop-$Version-windows-amd64.exe"
$cliPath = Join-Path $distRoot $cliName
$desktopPath = Join-Path $distRoot $desktopName
$checksumPath = Join-Path $distRoot "SHA256SUMS-$Version.txt"

New-Item -ItemType Directory -Force -Path $distRoot | Out-Null

Push-Location $repoRoot
try {
    $productVersionLdflag = "-X ai-dev-manager-v2/internal/version.Version=$Version"
    & go build -trimpath -ldflags $productVersionLdflag -o $cliPath ./cmd/ai-dev-manager
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

    & (Join-Path $PSScriptRoot 'build-desktop.ps1') -clean -trimpath -o $desktopName -OutputDir $distRoot -Version $Version
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

    if (-not (Test-Path -LiteralPath $desktopPath)) {
        throw "Desktop output not found in unified dist directory: $desktopPath"
    }

    function Get-Sha256Hex([string]$Path) {
        $sha = [System.Security.Cryptography.SHA256]::Create()
        try {
            $stream = [System.IO.File]::OpenRead($Path)
            try {
                $hash = $sha.ComputeHash($stream)
            } finally {
                $stream.Dispose()
            }
        } finally {
            $sha.Dispose()
        }
        return ([System.BitConverter]::ToString($hash)).Replace('-', '').ToLowerInvariant()
    }

    $lines = @(
        "$(Get-Sha256Hex $cliPath)  $cliName",
        "$(Get-Sha256Hex $desktopPath)  $desktopName"
    )
    [System.IO.File]::WriteAllLines($checksumPath, $lines, (New-Object System.Text.UTF8Encoding($false)))

    Write-Host "Release artifacts:"
    Write-Host "  $cliPath"
    Write-Host "  $desktopPath"
    Write-Host "  $checksumPath"
} finally {
    Pop-Location
}
