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
	usage := "usage: ai-dev-manager gateway <up|status|down> ...\n       ai-dev-manager gateway up [--listen 127.0.0.1:PORT | --docker]"
	if len(args) == 0 {
		return errors.New(usage)
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, err := fmt.Fprintln(stdout, usage)
		return err
	}
	switch args[0] {
	case "up":
		flags := flag.NewFlagSet("gateway up", flag.ContinueOnError)
		flags.SetOutput(stderr)
		listen := flags.String("listen", "", "optional stable loopback listen address; first run otherwise selects and persists a free port")
		dockerAccess := flags.Bool("docker", false, "expose the stable Gateway port to local Docker clients as host.docker.internal")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if len(flags.Args()) != 0 || (*dockerAccess && strings.TrimSpace(*listen) != "") {
			return errors.New("usage: ai-dev-manager gateway up [--listen 127.0.0.1:PORT | --docker]")
		}
		if _, err := daemon.Start(ctx, configRoot, daemonStartExecutable); err != nil {
			return err
		}
		var status daemon.GatewayStatus
		var err error
		if *dockerAccess {
			status, err = daemon.GatewayUpDocker(ctx, configRoot)
		} else {
			status, err = daemon.GatewayUp(ctx, configRoot, strings.TrimSpace(*listen))
		}
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
	_, err := fmt.Fprintf(w, "%s\tdesired=%t\texposed=%t\tlisten=%s", status.State, status.DesiredRunning, status.Exposed, status.Listen)
	if err != nil {
		return err
	}
	if status.LocalEndpoint != "" {
		if _, err := fmt.Fprintf(w, "\tlocal=%s", status.LocalEndpoint); err != nil {
			return err
		}
	} else if status.Endpoint != "" {
		if _, err := fmt.Fprintf(w, "\tendpoint=%s", status.Endpoint); err != nil {
			return err
		}
	}
	if status.DockerEndpoint != "" {
		if _, err := fmt.Fprintf(w, "\tdocker=%s", status.DockerEndpoint); err != nil {
			return err
		}
	}
	if status.Error != "" {
		if _, err := fmt.Fprintf(w, "\terror=%s", status.Error); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(w)
	return err
}
