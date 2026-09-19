package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/desktop"
	"ai-dev-manager-v2/internal/dotenv"
	"ai-dev-manager-v2/internal/gateway"
	"ai-dev-manager-v2/internal/store"
	productversion "ai-dev-manager-v2/internal/version"

	desktopkit "github.com/wanstu/wails-desktop-kit"
	kitui "github.com/wanstu/wails-desktop-kit/ui"
)

//go:embed all:frontend
var embeddedFrontend embed.FS

//go:embed assets/tray.png
var trayIcon []byte

func main() {
	if err := dotenv.LoadDefaultFiles(); err != nil {
		fmt.Fprintln(os.Stderr, "desktop error:", err)
		os.Exit(1)
	}
	var err error
	if len(os.Args) > 1 && os.Args[1] == "--gateway-child" {
		err = runGatewayChild(os.Args[2:])
	} else {
		var startHidden bool
		startHidden, err = desktopLaunchOptions(os.Args[1:])
		if err == nil {
			err = runDesktop(startHidden)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "desktop error:", err)
		os.Exit(1)
	}
}

func desktopLaunchOptions(args []string) (bool, error) {
	launch, err := desktopkit.ParseLaunchOptions(args)
	if err != nil {
		return false, err
	}
	return launch.AutoStart, nil
}

func runGatewayChild(args []string) error {
	flags := flag.NewFlagSet("gateway-child", flag.ContinueOnError)
	listen := flags.String("listen", gateway.DefaultHTTPListen, "Gateway listen address")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("gateway child accepts only --listen")
	}
	statePath, err := store.DefaultPath()
	if err != nil {
		return err
	}
	service := app.New(statePath)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return gateway.RunHTTP(ctx, service, *listen)
}

func desktopTitle() string {
	return "adm-desktop — " + productversion.Current()
}

func runDesktop(startHidden bool) error {
	assets, err := frontendAssets()
	if err != nil {
		return err
	}
	adapter := desktop.NewClientAdapter()
	autoStart := newDesktopAutoStartProvider(adapter)
	window := desktopkit.DefaultWindowConfig()
	window.Width = 1120
	window.Height = 760
	window.MinWidth = 820
	window.MinHeight = 560
	window.HidePolicy = desktopHidePolicy(runtime.GOOS)
	window.StartHiddenOnAutoStart = true

	return desktopkit.Run(desktopkit.Config{
		ID:     "com.wanstu.adm-desktop",
		Title:  desktopTitle(),
		Assets: kitui.Mount(assets),
		Bind: []interface{}{
			adapter,
		},
		Launch: desktopkit.LaunchOptions{AutoStart: startHidden},
		Window: window,
		Theme:  desktopkit.DefaultThemeConfig(),
		Tray:   desktopTrayConfig(trayIcon, adapter, autoStart),
		Hooks: desktopkit.Hooks{
			Ready: autoStart.setController,
		},
		SingleInstance:       true,
		SecondInstancePolicy: desktopkit.SecondInstanceWakeManual,
	})
}

func desktopHidePolicy(goos string) desktopkit.HidePolicy {
	if goos == "linux" {
		return desktopkit.HideAlways
	}
	return desktopkit.HideSafe
}

func frontendAssets() (fs.FS, error) {
	return fs.Sub(embeddedFrontend, "frontend")
}
