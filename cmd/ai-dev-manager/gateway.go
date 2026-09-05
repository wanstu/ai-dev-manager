package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"ai-dev-manager/internal/daemon"
)

func runGateway(ctx context.Context, configRoot string, args []string, stdout, stderr io.Writer, jsonOutput bool) error {
	if len(args) == 0 {
		return errors.New("usage: ai-dev-manager gateway <up|status|down> ...")
	}
	switch args[0] {
	case "up":
		flags := flag.NewFlagSet("gateway up", flag.ContinueOnError)
		flags.SetOutput(stderr)
		listen := flags.String("listen", "", "optional stable loopback listen address; first run otherwise selects and persists a free port")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if len(flags.Args()) != 0 {
			return errors.New("usage: ai-dev-manager gateway up [--listen 127.0.0.1:PORT]")
		}
		if _, err := daemon.Start(ctx, configRoot, daemonStartExecutable); err != nil {
			return err
		}
		status, err := daemon.GatewayUp(ctx, configRoot, strings.TrimSpace(*listen))
		if err != nil {
			return err
		}
		return writeGatewayStatus(stdout, status, jsonOutput)

	case "status":
		if len(args) != 1 {
			return errors.New("usage: ai-dev-manager gateway status")
		}
		status, err := daemon.GatewayGetStatus(ctx, configRoot)
		if err != nil {
			return err
		}
		return writeGatewayStatus(stdout, status, jsonOutput)

	case "down":
		if len(args) != 1 {
			return errors.New("usage: ai-dev-manager gateway down")
		}
		status, err := daemon.GatewayDown(ctx, configRoot)
		if err != nil {
			return err
		}
		return writeGatewayStatus(stdout, status, jsonOutput)

	default:
		return fmt.Errorf("unknown gateway command %q", args[0])
	}
}

func writeGatewayStatus(w io.Writer, status daemon.GatewayStatus, jsonOutput bool) error {
	if jsonOutput {
		return writeJSON(w, status)
	}
	_, err := fmt.Fprintf(w, "%s\tdesired=%t\tlisten=%s\tendpoint=%s", status.State, status.DesiredRunning, status.Listen, status.Endpoint)
	if err != nil {
		return err
	}
	if status.Error != "" {
		if _, err := fmt.Fprintf(w, "\terror=%s", status.Error); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(w)
	return err
}
