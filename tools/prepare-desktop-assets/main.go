package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	root, err := findRepoRoot()
	if err != nil {
		fatal(err)
	}

	if runtime.GOOS == "windows" {
		if err := runWindowsIconPreparation(root); err != nil {
			fatal(err)
		}
		return
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

	// The tray implementation is currently Windows-only, but main.go embeds
	// this asset on every platform. Keep the checked-in fitted tray image when
	// present; only seed it from the source asset for a fresh checkout/layout.
	trayTarget := filepath.Join(root, "cmd", "ai-dev-manager-desktop", "assets", "tray.png")
	if _, err := os.Stat(trayTarget); os.IsNotExist(err) {
		if err := copyFile(filepath.Join(root, "assets", "icons", "ai-dev-manager-tray.png"), trayTarget); err != nil {
			fatal(err)
		}
	} else if err != nil {
		fatal(err)
	}

	fmt.Println("Prepared adm-desktop assets for", runtime.GOOS)
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

func runWindowsIconPreparation(root string) error {
	script := filepath.Join(root, "scripts", "prepare-desktop-icons.ps1")
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("prepare Windows desktop icons: %w", err)
	}
	return nil
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
