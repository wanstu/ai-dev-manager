$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName System.Drawing

$repoRoot = Split-Path -Parent $PSScriptRoot
$desktopRoot = Join-Path $repoRoot 'cmd\ai-dev-manager-desktop'
$appIconSource = Join-Path $repoRoot 'assets\icons\ai-dev-manager-app.png'
$windowIconSource = Join-Path $repoRoot 'assets\icons\ai-dev-manager-window.png'
$trayIconSource = Join-Path $repoRoot 'assets\icons\ai-dev-manager-tray.png'
$appIconTarget = Join-Path $desktopRoot 'build\appicon.png'
$windowIconTarget = Join-Path $desktopRoot 'frontend\assets\ai-dev-manager-window.png'
$trayIconTarget = Join-Path $desktopRoot 'assets\tray.png'
$windowsIconTarget = Join-Path $desktopRoot 'build\windows\icon.ico'

foreach ($source in @($appIconSource, $windowIconSource, $trayIconSource)) {
    if (-not (Test-Path $source)) {
        throw "Desktop icon source not found: $source"
    }
}

function Export-FittedTransparentPng {
    param(
        [Parameter(Mandatory = $true)][string]$Source,
        [Parameter(Mandatory = $true)][string]$Destination,
        [int]$CanvasSize = 1024,
        [double]$Fill = 0.94,
        [int]$AlphaThreshold = 8
    )

    $sourceBitmap = [System.Drawing.Bitmap]::FromFile($Source)
    try {
        $minX = $sourceBitmap.Width
        $minY = $sourceBitmap.Height
        $maxX = -1
        $maxY = -1

        for ($y = 0; $y -lt $sourceBitmap.Height; $y++) {
            for ($x = 0; $x -lt $sourceBitmap.Width; $x++) {
                if ($sourceBitmap.GetPixel($x, $y).A -gt $AlphaThreshold) {
                    if ($x -lt $minX) { $minX = $x }
                    if ($y -lt $minY) { $minY = $y }
                    if ($x -gt $maxX) { $maxX = $x }
                    if ($y -gt $maxY) { $maxY = $y }
                }
            }
        }

        if ($maxX -lt $minX -or $maxY -lt $minY) {
            throw "Icon source has no visible pixels: $Source"
        }

        $cropWidth = $maxX - $minX + 1
        $cropHeight = $maxY - $minY + 1
        $targetSize = $CanvasSize * $Fill
        $scale = [Math]::Min($targetSize / $cropWidth, $targetSize / $cropHeight)
        $drawWidth = [int][Math]::Round($cropWidth * $scale)
        $drawHeight = [int][Math]::Round($cropHeight * $scale)
        $drawX = [int][Math]::Round(($CanvasSize - $drawWidth) / 2)
        $drawY = [int][Math]::Round(($CanvasSize - $drawHeight) / 2)

        $sourceRect = New-Object System.Drawing.Rectangle($minX, $minY, $cropWidth, $cropHeight)
        $targetRect = New-Object System.Drawing.Rectangle($drawX, $drawY, $drawWidth, $drawHeight)
        $output = New-Object System.Drawing.Bitmap($CanvasSize, $CanvasSize, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
        try {
            $graphics = [System.Drawing.Graphics]::FromImage($output)
            try {
                $graphics.Clear([System.Drawing.Color]::Transparent)
                $graphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
                $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
                $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
                $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
                $graphics.DrawImage($sourceBitmap, $targetRect, $sourceRect, [System.Drawing.GraphicsUnit]::Pixel)
            } finally {
                $graphics.Dispose()
            }
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
            $output.Save($Destination, [System.Drawing.Imaging.ImageFormat]::Png)
        } finally {
            $output.Dispose()
        }
    } finally {
        $sourceBitmap.Dispose()
    }
}

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $appIconTarget) | Out-Null
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $windowIconTarget) | Out-Null
Copy-Item $appIconSource $appIconTarget -Force
Copy-Item $windowIconSource $windowIconTarget -Force
Export-FittedTransparentPng -Source $trayIconSource -Destination $trayIconTarget -Fill 0.94

# Wails regenerates the Windows ICO from build/appicon.png only when the old
# generated resource is absent. Removing it makes direct `wails build` and the
# repository build wrapper deterministic.
if (Test-Path $windowsIconTarget) {
    Remove-Item $windowsIconTarget -Force
}

Write-Host 'Prepared adm-desktop icons:'
Write-Host "  app:    $appIconSource"
Write-Host "  window: $windowIconSource"
Write-Host "  tray:   $trayIconSource -> $trayIconTarget"
