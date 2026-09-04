package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"ai-dev-manager/internal/controlplane"
	"ai-dev-manager/internal/model"
	"ai-dev-manager/internal/workspace"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	global := flag.NewFlagSet("ai-dev-manager", flag.ContinueOnError)
	global.SetOutput(stderr)
	configRoot := global.String("config-root", "", "configuration root (defaults to the user config directory)")
	jsonOutput := global.Bool("json", false, "emit JSON output")
	if err := global.Parse(args); err != nil {
		return err
	}
	remaining := global.Args()
	if len(remaining) == 0 {
		return errors.New("usage: ai-dev-manager [--config-root PATH] [--json] <workspace|inspect|serve> ...")
	}

	service, err := controlplane.New(*configRoot)
	if err != nil {
		return err
	}

	switch remaining[0] {
	case "workspace":
		return runWorkspace(service, remaining[1:], stdout, stderr, *jsonOutput)
	case "inspect":
		return runInspect(service, remaining[1:], stdout, stderr, *jsonOutput)
	case "serve":
		return runServe(ctx, service, remaining[1:], stdout, stderr, *jsonOutput)
	case "mcp":
		return runConfiguredMCP(ctx, service, remaining[1:], stdout, stderr, *jsonOutput)
	default:
		return fmt.Errorf("unknown command %q", remaining[0])
	}
}

type workspaceOutput struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	ProfileID string `json:"profile_id,omitempty"`
	RuntimeID string `json:"runtime_id,omitempty"`
}

func newWorkspaceOutput(value model.Workspace) workspaceOutput {
	return workspaceOutput{ID: value.ID, Path: value.Path, ProfileID: value.ProfileID, RuntimeID: value.RuntimeID}
}

func runWorkspace(service *controlplane.Service, args []string, stdout, stderr io.Writer, jsonOutput bool) error {
	if len(args) == 0 {
		return errors.New("usage: ai-dev-manager workspace <add|list|show> ...")
	}
	switch args[0] {
	case "add":
		flags := flag.NewFlagSet("workspace add", flag.ContinueOnError)
		flags.SetOutput(stderr)
		path := flags.String("path", "", "absolute workspace path")
		profileID := flags.String("profile", "", "profile id")
		runtimeID := flags.String("runtime", "", "runtime id")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*path) == "" {
			return errors.New("workspace add requires --path")
		}
		value, err := service.Registry().Add(workspace.Input{Path: *path, ProfileID: *profileID, RuntimeID: *runtimeID})
		if err != nil {
			return err
		}
		return writeWorkspace(stdout, newWorkspaceOutput(value), jsonOutput)
	case "list":
		if len(args) != 1 {
			return errors.New("usage: ai-dev-manager workspace list")
		}
		values, err := service.Registry().List()
		if err != nil {
			return err
		}
		outputs := make([]workspaceOutput, 0, len(values))
		for _, value := range values {
			outputs = append(outputs, newWorkspaceOutput(value))
		}
		if jsonOutput {
			return writeJSON(stdout, outputs)
		}
		for _, value := range outputs {
			if err := writeWorkspace(stdout, value, false); err != nil {
				return err
			}
		}
		return nil
	case "show":
		if len(args) != 2 {
			return errors.New("usage: ai-dev-manager workspace show <workspace-id>")
		}
		value, err := service.Registry().Get(args[1])
		if err != nil {
			return err
		}
		return writeWorkspace(stdout, newWorkspaceOutput(value), jsonOutput)
	default:
		return fmt.Errorf("unknown workspace command %q", args[0])
	}
}

func runInspect(service *controlplane.Service, args []string, stdout, stderr io.Writer, jsonOutput bool) error {
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(stderr)
	workspaceID := flags.String("workspace", "", "workspace id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*workspaceID) == "" {
		return errors.New("inspect requires --workspace")
	}
	snapshot, err := service.Inspect(*workspaceID, nil)
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(stdout, snapshot)
	}
	return writePrettyJSON(stdout, snapshot)
}

func runServe(ctx context.Context, service *controlplane.Service, args []string, stdout, stderr io.Writer, jsonOutput bool) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	workspaceID := flags.String("workspace", "", "workspace id")
	listen := flags.String("listen", "127.0.0.1:0", "loopback listen address")
	instanceID := flags.String("instance", "", "MCP instance id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*workspaceID) == "" {
		return errors.New("serve requires --workspace")
	}
	if strings.TrimSpace(*instanceID) == "" {
		*instanceID = "foreground:" + *workspaceID
	}
	instance, err := service.StartMCP(*instanceID, *workspaceID, nil, *listen)
	if err != nil {
		return err
	}
	if jsonOutput {
		if err := writeJSON(stdout, instance); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(stdout, "MCP %s %s\n", instance.ID, instance.Endpoint); err != nil {
			return err
		}
	}

	<-ctx.Done()
	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return service.StopMCP(stopCtx, instance.ID)
}

func runConfiguredMCP(ctx context.Context, service *controlplane.Service, args []string, stdout, stderr io.Writer, jsonOutput bool) error {
	if len(args) == 0 {
		return errors.New("usage: ai-dev-manager mcp <status|activate|stop> --workspace <workspace-id>")
	}
	flags := flag.NewFlagSet("mcp "+args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	workspaceID := flags.String("workspace", "", "workspace id")
	mcpID := flags.String("id", "", "configured MCP id")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*workspaceID) == "" {
		return fmt.Errorf("mcp %s requires --workspace", args[0])
	}

	switch args[0] {
	case "status":
		statuses, err := service.MCPStatuses(*workspaceID, nil)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(stdout, statuses)
		}
		return writePrettyJSON(stdout, statuses)
	case "activate":
		statuses, activateErr := service.ActivateConfiguredMCPs(ctx, *workspaceID, nil)
		defer service.StopConfiguredMCPs(*workspaceID)
		var writeErr error
		if jsonOutput {
			writeErr = writeJSON(stdout, statuses)
		} else {
			writeErr = writePrettyJSON(stdout, statuses)
		}
		return errors.Join(writeErr, activateErr)
	case "stop":
		var err error
		if strings.TrimSpace(*mcpID) == "" {
			err = service.StopConfiguredMCPs(*workspaceID)
		} else {
			err = service.StopConfiguredMCP(*workspaceID, *mcpID)
		}
		if err != nil {
			return err
		}
		result := map[string]any{"workspace_id": *workspaceID, "stopped": true}
		if strings.TrimSpace(*mcpID) != "" {
			result["id"] = *mcpID
		}
		if jsonOutput {
			return writeJSON(stdout, result)
		}
		return writePrettyJSON(stdout, result)
	default:
		return fmt.Errorf("unknown mcp command %q", args[0])
	}
}

func writeWorkspace(w io.Writer, value workspaceOutput, jsonOutput bool) error {
	if jsonOutput {
		return writeJSON(w, value)
	}
	runtimeID := value.RuntimeID
	if strings.TrimSpace(runtimeID) == "" {
		runtimeID = controlplane.NativeRuntimeID
	}
	_, err := fmt.Fprintf(w, "%s\t%s\tprofile=%s\truntime=%s\n", value.ID, value.Path, value.ProfileID, runtimeID)
	return err
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writePrettyJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
