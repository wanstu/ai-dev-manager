package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"ai-dev-manager/internal/agent"
	"ai-dev-manager/internal/controlplane"
	"ai-dev-manager/internal/daemon"
)

func runAgent(ctx context.Context, configRoot string, service *controlplane.Service, args []string, stdout, stderr io.Writer, jsonOutput bool) error {
	if len(args) == 0 {
		return errors.New("usage: ai-dev-manager agent <run|list|status|cancel> ...")
	}
	switch args[0] {
	case "run":
		flags := flag.NewFlagSet("agent run", flag.ContinueOnError)
		flags.SetOutput(stderr)
		workflow := flags.String("workflow", "", "agent workflow executor (for example: verify, gsd, parallel-verify)")
		keepWorktrees := flags.Bool("keep-worktrees", false, "preserve parallel-verify managed worktrees after the run")
		var lanes laneFlags
		flags.Var(&lanes, "lane", "parallel-verify lane in name:branch form (repeatable)")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if len(flags.Args()) > 1 {
			return errors.New("usage: ai-dev-manager agent run [--workflow NAME] [--lane name:branch ...] [--keep-worktrees] [path|workspace-id]")
		}
		workflowName := strings.TrimSpace(*workflow)
		input, err := parallelWorkflowInput(workflowName, lanes, *keepWorktrees)
		if err != nil {
			return err
		}
		target := ""
		if len(flags.Args()) == 1 {
			target = flags.Args()[0]
		}
		ws, err := resolveWorkspaceTarget(service, target, true)
		if err != nil {
			return err
		}
		if _, err := daemon.Start(ctx, configRoot, daemonStartExecutable); err != nil {
			return err
		}
		status, err := daemon.AgentRunRequest(ctx, configRoot, ws.ID, workflowName, input)
		if err != nil {
			return err
		}
		return writeAgentStatus(stdout, status, jsonOutput)

	case "list":
		if len(args) != 1 {
			return errors.New("usage: ai-dev-manager agent list")
		}
		statuses, err := daemon.AgentList(ctx, configRoot)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(stdout, statuses)
		}
		for _, status := range statuses {
			if err := writeAgentStatus(stdout, status, false); err != nil {
				return err
			}
		}
		return nil

	case "status":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return errors.New("usage: ai-dev-manager agent status <run-id>")
		}
		status, err := daemon.AgentStatus(ctx, configRoot, args[1])
		if err != nil {
			return err
		}
		return writeAgentStatus(stdout, status, jsonOutput)

	case "cancel":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return errors.New("usage: ai-dev-manager agent cancel <run-id>")
		}
		status, err := daemon.AgentCancel(ctx, configRoot, args[1])
		if err != nil {
			return err
		}
		return writeAgentStatus(stdout, status, jsonOutput)

	default:
		return fmt.Errorf("unknown agent command %q", args[0])
	}
}

type laneFlags []string

func (l *laneFlags) String() string { return strings.Join(*l, ",") }

func (l *laneFlags) Set(value string) error {
	*l = append(*l, value)
	return nil
}

func parallelWorkflowInput(workflow string, values []string, keepWorktrees bool) (map[string]any, error) {
	if workflow != "parallel-verify" {
		if len(values) > 0 || keepWorktrees {
			return nil, errors.New("--lane and --keep-worktrees require --workflow parallel-verify")
		}
		return nil, nil
	}
	lanes := make([]agent.ParallelLaneSpec, 0, len(values))
	for _, value := range values {
		name, branch, ok := strings.Cut(value, ":")
		name = strings.TrimSpace(name)
		branch = strings.TrimSpace(branch)
		if !ok || name == "" || branch == "" {
			return nil, fmt.Errorf("invalid --lane %q: expected name:branch", value)
		}
		lanes = append(lanes, agent.ParallelLaneSpec{Name: name, Branch: branch})
	}
	if len(lanes) < 2 || len(lanes) > 8 {
		return nil, errors.New("parallel-verify requires between 2 and 8 --lane values")
	}
	return map[string]any{"lanes": lanes, "keep_worktrees": keepWorktrees}, nil
}

func writeAgentStatus(w io.Writer, status agent.RunStatus, jsonOutput bool) error {
	if jsonOutput {
		return writeJSON(w, status)
	}
	if _, err := fmt.Fprintf(w, "%s\t%s\tworkspace=%s\texecutor=%s", status.State, status.RunID, status.WorkspaceID, status.Executor); err != nil {
		return err
	}
	if status.Stage != "" {
		if _, err := fmt.Fprintf(w, "\tstage=%s", status.Stage); err != nil {
			return err
		}
	}
	if status.Review != nil {
		if _, err := fmt.Fprintf(w, "\treview=%s\t%s", status.Review.Decision, status.Review.Summary); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}
