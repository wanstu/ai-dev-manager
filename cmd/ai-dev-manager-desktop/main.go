package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/desktop"
	"ai-dev-manager-v2/internal/dotenv"
	"ai-dev-manager-v2/internal/gateway"
	"ai-dev-manager-v2/internal/store"
	productversion "ai-dev-manager-v2/internal/version"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var embeddedFrontend embed.FS

//go:embed assets/tray.png
var trayIcon []byte

func main() {
	if err := dotenv.LoadFromExecutableDir(); err != nil {
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
	flags := flag.NewFlagSet("desktop", flag.ContinueOnError)
	autostart := flags.Bool("autostart", false, "start hidden after Windows login")
	if err := flags.Parse(args); err != nil {
		return false, err
	}
	if flags.NArg() != 0 {
		return false, fmt.Errorf("desktop accepts only --autostart")
	}
	return *autostart, nil
}

func runGatewayChild(args []string) error {
	fs := flag.NewFlagSet("gateway-child", flag.ContinueOnError)
	listen := fs.String("listen", gateway.DefaultHTTPListen, "Gateway listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
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
	tray := newTrayManager(trayIcon, adapter)
	return wails.Run(&options.App{
		Title:             desktopTitle(),
		Width:             1120,
		Height:            760,
		MinWidth:          820,
		MinHeight:         560,
		StartHidden:       startHidden && traySupported,
		HideWindowOnClose: traySupported,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 246, G: 247, B: 249, A: 1},
		OnStartup:        tray.Startup,
		OnDomReady:       tray.DomReady,
		OnShutdown:       tray.Shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "adm-desktop-v1",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				tray.ShowWindow()
			},
		},
		Bind: []interface{}{
			adapter,
		},
	})
}

func frontendAssets() (fs.FS, error) {
	return fs.Sub(embeddedFrontend, "frontend")
}
