package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ai-dev-manager/internal/controlplane"
	"ai-dev-manager/internal/daemon"
	"ai-dev-manager/internal/model"
	"ai-dev-manager/internal/workspace"
)

type upOutput struct {
	Workspace workspaceOutput      `json:"workspace"`
	Daemon    daemon.Metadata      `json:"daemon"`
	Runtime   daemon.RuntimeStatus `json:"runtime"`
}

type psItem struct {
	WorkspaceID    string              `json:"workspace_id"`
	Path           string              `json:"path"`
	RuntimeID      string              `json:"runtime_id,omitempty"`
	DesiredRunning bool                `json:"desired_running"`
	State          daemon.RuntimeState `json:"state"`
	Listen         string              `json:"listen,omitempty"`
	LocalEndpoint  string              `json:"local_endpoint,omitempty"`
	DockerEndpoint string              `json:"docker_endpoint,omitempty"`
	Error          string              `json:"error,omitempty"`
}

type psOutput struct {
	DaemonState daemon.State `json:"daemon_state"`
	Items       []psItem     `json:"items"`
}

func runUp(ctx context.Context, configRoot string, service *controlplane.Service, args []string, stdout, stderr io.Writer, jsonOutput bool) error {
	flags := flag.NewFlagSet("up", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dockerAccess := flags.Bool("docker", false, "expose MCP to local Docker via host.docker.internal")
	listen := flags.String("listen", "", "explicit MCP listen address")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) > 1 {
		return errors.New("usage: ai-dev-manager up [--docker|--listen ADDR] [path|workspace-id]")
	}
	if *dockerAccess && strings.TrimSpace(*listen) != "" {
		return errors.New("up accepts either --docker or --listen, not both")
	}
	target := ""
	if len(flags.Args()) == 1 {
		target = flags.Args()[0]
	}
	ws, err := resolveWorkspaceTarget(service, target, true)
	if err != nil {
		return err
	}
	meta, err := daemon.Start(ctx, configRoot, "")
	if err != nil {
		return err
	}
	options := daemon.RuntimeStartOptions{}
	if *dockerAccess {
		options.Listen = daemon.DockerRuntimeListen
		options.Exposed = true
	} else if strings.TrimSpace(*listen) != "" {
		options.Listen = strings.TrimSpace(*listen)
		options.Exposed = true
	}
	status, err := daemon.RuntimeStartWithOptions(ctx, configRoot, ws.ID, options)
	if err != nil {
		return err
	}
	result := upOutput{Workspace: newWorkspaceOutput(ws), Daemon: meta, Runtime: status}
	if jsonOutput {
		return writeJSON(stdout, result)
	}
	if _, err := fmt.Fprintf(stdout, "workspace %s %s\nruntime   %s\n", ws.ID, ws.Path, status.State); err != nil {
		return err
	}
	if status.LocalEndpoint != "" {
		if _, err := fmt.Fprintf(stdout, "local     %s\n", status.LocalEndpoint); err != nil {
			return err
		}
	}
	if status.DockerEndpoint != "" {
		if _, err := fmt.Fprintf(stdout, "docker    %s\n", status.DockerEndpoint); err != nil {
			return err
		}
	}
	return nil
}

func runDown(ctx context.Context, configRoot string, service *controlplane.Service, args []string, stdout, stderr io.Writer, jsonOutput bool) error {
	flags := flag.NewFlagSet("down", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) > 1 {
		return errors.New("usage: ai-dev-manager down [path|workspace-id]")
	}
	target := ""
	if len(flags.Args()) == 1 {
		target = flags.Args()[0]
	}
	ws, err := resolveWorkspaceTarget(service, target, false)
	if err != nil {
		return err
	}
	statusCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	_, statusErr := daemon.Status(statusCtx, configRoot)
	cancel()
	var status daemon.RuntimeStatus
	if errors.Is(statusErr, daemon.ErrNotRunning) {
		if err := daemon.NewDesiredStore(configRoot).Remove(ws.ID); err != nil {
			return err
		}
		status = stoppedRuntimeStatus(ws)
	} else if statusErr != nil {
		return statusErr
	} else {
		status, err = daemon.RuntimeStop(ctx, configRoot, ws.ID)
		if err != nil {
			return err
		}
	}
	if jsonOutput {
		return writeJSON(stdout, status)
	}
	_, err = fmt.Fprintf(stdout, "workspace %s stopped\n", ws.ID)
	return err
}

func runPS(ctx context.Context, configRoot string, service *controlplane.Service, stdout io.Writer, jsonOutput bool) error {
	workspaces, err := service.Registry().List()
	if err != nil {
		return err
	}
	desired, err := daemon.NewDesiredStore(configRoot).LoadRuntimes()
	if err != nil {
		return err
	}
	desiredByID := make(map[string]daemon.DesiredRuntime, len(desired))
	for _, runtime := range desired {
		desiredByID[runtime.WorkspaceID] = runtime
	}

	state := daemon.StateStopped
	observedByID := map[string]daemon.RuntimeStatus{}
	statusCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	meta, statusErr := daemon.Status(statusCtx, configRoot)
	cancel()
	if statusErr == nil {
		state = meta.State
		observed, listErr := daemon.RuntimeList(ctx, configRoot)
		if listErr != nil {
			return listErr
		}
		for _, runtime := range observed {
			observedByID[runtime.WorkspaceID] = runtime
		}
	} else if !errors.Is(statusErr, daemon.ErrNotRunning) {
		return statusErr
	}

	sort.Slice(workspaces, func(i, j int) bool { return strings.ToLower(workspaces[i].Path) < strings.ToLower(workspaces[j].Path) })
	items := make([]psItem, 0, len(workspaces))
	for _, ws := range workspaces {
		item := psItem{WorkspaceID: ws.ID, Path: ws.Path, RuntimeID: selectedWorkspaceRuntimeID(ws), State: daemon.RuntimeStopped}
		if desiredRuntime, ok := desiredByID[ws.ID]; ok {
			item.DesiredRunning = true
			item.Listen = desiredRuntime.Listen
		}
		if observed, ok := observedByID[ws.ID]; ok {
			item.DesiredRunning = observed.DesiredRunning
			item.State = observed.State
			item.Listen = observed.Listen
			item.LocalEndpoint = observed.LocalEndpoint
			item.DockerEndpoint = observed.DockerEndpoint
			item.Error = observed.Error
		}
		items = append(items, item)
	}
	result := psOutput{DaemonState: state, Items: items}
	if jsonOutput {
		return writeJSON(stdout, result)
	}
	if _, err := fmt.Fprintf(stdout, "daemon %s\n", state); err != nil {
		return err
	}
	for _, item := range items {
		endpoint := item.LocalEndpoint
		if item.DockerEndpoint != "" {
			endpoint = item.DockerEndpoint
		}
		if endpoint == "" {
			endpoint = "-"
		}
		if _, err := fmt.Fprintf(stdout, "%s\t%s\tdesired=%t\t%s\t%s\n", item.State, item.WorkspaceID, item.DesiredRunning, item.Path, endpoint); err != nil {
			return err
		}
	}
	return nil
}

func runCtl(ctx context.Context, configRoot string, args []string, stdout io.Writer, jsonOutput bool) error {
	if len(args) != 1 {
		return errors.New("usage: ai-dev-manager ctl <start|status|stop|restart|shutdown>")
	}
	switch args[0] {
	case "start":
		return runDaemonStart(ctx, configRoot, stdout, jsonOutput)
	case "status":
		return runDaemonStatus(ctx, configRoot, stdout, jsonOutput)
	case "stop":
		return runDaemonStop(ctx, configRoot, stdout, jsonOutput)
	case "restart":
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := daemon.Stop(stopCtx, configRoot)
		cancel()
		if err != nil {
			return err
		}
		meta, err := daemon.Start(ctx, configRoot, "")
		if err != nil {
			return err
		}
		return writeDaemon(stdout, meta, jsonOutput)
	case "shutdown":
		return runCtlShutdown(ctx, configRoot, stdout, jsonOutput)
	default:
		return fmt.Errorf("unknown ctl command %q", args[0])
	}
}

func runCtlShutdown(ctx context.Context, configRoot string, stdout io.Writer, jsonOutput bool) error {
	statusCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	_, statusErr := daemon.Status(statusCtx, configRoot)
	cancel()
	if statusErr == nil {
		statuses, err := daemon.RuntimeList(ctx, configRoot)
		if err != nil {
			return err
		}
		for _, status := range statuses {
			if _, err := daemon.RuntimeStop(ctx, configRoot, status.WorkspaceID); err != nil {
				return err
			}
		}
	} else if !errors.Is(statusErr, daemon.ErrNotRunning) {
		return statusErr
	}
	if err := daemon.NewDesiredStore(configRoot).ReplaceRuntimes(nil); err != nil {
		return err
	}
	stopCtx, stopCancel := context.WithTimeout(ctx, 5*time.Second)
	meta, err := daemon.Stop(stopCtx, configRoot)
	stopCancel()
	if err != nil {
		return err
	}
	return writeDaemon(stdout, meta, jsonOutput)
}

func resolveWorkspaceTarget(service *controlplane.Service, target string, create bool) (model.Workspace, error) {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "ws_") {
		return service.Registry().Get(target)
	}
	if target == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return model.Workspace{}, err
		}
		target = cwd
	}
	absolute, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return model.Workspace{}, err
	}
	workspaces, err := service.Registry().List()
	if err != nil {
		return model.Workspace{}, err
	}
	for _, ws := range workspaces {
		if strings.EqualFold(filepath.Clean(ws.Path), filepath.Clean(absolute)) {
			return ws, nil
		}
	}
	if !create {
		return model.Workspace{}, fmt.Errorf("workspace path not registered: %q", absolute)
	}
	return service.Registry().Add(workspace.Input{Path: absolute})
}

func stoppedRuntimeStatus(ws model.Workspace) daemon.RuntimeStatus {
	return daemon.RuntimeStatus{
		WorkspaceID: ws.ID,
		RuntimeID:   selectedWorkspaceRuntimeID(ws),
		State:       daemon.RuntimeStopped,
	}
}

func selectedWorkspaceRuntimeID(ws model.Workspace) string {
	if strings.TrimSpace(ws.RuntimeID) != "" {
		return ws.RuntimeID
	}
	return controlplane.NativeRuntimeID
}
