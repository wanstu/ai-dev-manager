package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	kiticon "github.com/wanstu/wails-desktop-kit/icon"
)

func main() {
	root, err := findRepoRoot()
	if err != nil {
		fatal(err)
	}

	copies := [][2]string{
		{filepath.Join(root, "assets", "icons", "ai-dev-manager-app.png"), filepath.Join(root, "cmd", "ai-dev-manager-desktop", "build", "appicon.png")},
		{filepath.Join(root, "assets", "icons", "ai-dev-manager-window.png"), filepath.Join(root, "cmd", "ai-dev-manager-desktop", "frontend", "assets", "ai-dev-manager-window.png")},
	}
	for _, pair := range copies {
		if err := copyFile(pair[0], pair[1]); err != nil {
			fatal(err)
		}
	}

	traySource := filepath.Join(root, "assets", "icons", "ai-dev-manager-tray.png")
	trayTarget := filepath.Join(root, "cmd", "ai-dev-manager-desktop", "assets", "tray.png")
	options := kiticon.DefaultOptions()
	options.CanvasSize = 1024
	options.Fill = 0.94
	options.TrimAlpha = true
	options.AlphaThreshold = 8
	if err := kiticon.NormalizeFile(traySource, trayTarget, options); err != nil {
		fatal(fmt.Errorf("normalize tray icon with desktop-kit: %w", err))
	}

	if runtime.GOOS == "windows" {
		// Wails regenerates icon.ico from build/appicon.png when the previous
		// generated resource is absent.
		windowsIcon := filepath.Join(root, "cmd", "ai-dev-manager-desktop", "build", "windows", "icon.ico")
		if err := os.Remove(windowsIcon); err != nil && !os.IsNotExist(err) {
			fatal(fmt.Errorf("remove generated Windows icon: %w", err))
		}
	}

	fmt.Println("Prepared adm-desktop assets with Wails Desktop Kit for", runtime.GOOS)
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "assets", "icons", "ai-dev-manager-app.png")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repository root not found from %s", dir)
		}
		dir = parent
	}
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer input.Close()

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(destination), err)
	}
	output, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return fmt.Errorf("copy %s to %s: %w", source, destination, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close %s: %w", destination, err)
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "prepare desktop assets:", err)
	os.Exit(1)
}
