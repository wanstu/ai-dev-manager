package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"ai-dev-manager/internal/controlplane"
	"ai-dev-manager/internal/daemon"
	"ai-dev-manager/internal/environment"
)

func runEnvironment(ctx context.Context, configRoot string, service *controlplane.Service, args []string, stdout, stderr io.Writer, jsonOutput bool) error {
	if len(args) == 0 {
		return errors.New("usage: ai-dev-manager env <create|list|inspect|destroy|writer> ...")
	}
	switch args[0] {
	case "create":
		flags := flag.NewFlagSet("env create", flag.ContinueOnError)
		flags.SetOutput(stderr)
		name := flags.String("name", "", "environment display name")
		base := flags.String("base", "", "optional base branch, tag, or commit")
		branch := flags.String("branch", "", "optional environment branch")
		includeChanges := flags.Bool("include-changes", false, "include staged, unstaged, and untracked non-ignored changes")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("env create requires --name")
		}
		if len(flags.Args()) > 1 {
			return errors.New("usage: ai-dev-manager env create --name NAME [--base REF] [--branch BRANCH] [--include-changes] [path|workspace-id]")
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
		result, err := daemon.EnvironmentCreate(ctx, configRoot, environment.CreateRequest{
			WorkspaceID:    ws.ID,
			Name:           strings.TrimSpace(*name),
			Base:           strings.TrimSpace(*base),
			Branch:         strings.TrimSpace(*branch),
			IncludeChanges: *includeChanges,
		})
		if err != nil {
			return err
		}
		return writeEnvironmentInspect(stdout, result, jsonOutput)

	case "list":
		if len(args) != 1 {
			return errors.New("usage: ai-dev-manager env list")
		}
		items, err := daemon.EnvironmentList(ctx, configRoot)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(stdout, items)
		}
		for _, item := range items {
			if err := writeEnvironment(stdout, item); err != nil {
				return err
			}
		}
		return nil

	case "inspect":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return errors.New("usage: ai-dev-manager env inspect <environment-id>")
		}
		result, err := daemon.EnvironmentInspect(ctx, configRoot, args[1])
		if err != nil {
			return err
		}
		return writeEnvironmentInspect(stdout, result, jsonOutput)

	case "destroy":
		flags := flag.NewFlagSet("env destroy", flag.ContinueOnError)
		flags.SetOutput(stderr)
		force := flags.Bool("force", false, "destroy even when the environment contains dirty or unpushed work")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if len(flags.Args()) != 1 || strings.TrimSpace(flags.Args()[0]) == "" {
			return errors.New("usage: ai-dev-manager env destroy [--force] <environment-id>")
		}
		item, err := daemon.EnvironmentDestroyWithForce(ctx, configRoot, flags.Args()[0], *force)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(stdout, item)
		}
		_, err = fmt.Fprintf(stdout, "destroyed\t%s\tworkspace=%s\tbranch=%s\n", item.ID, item.WorkspaceID, item.Branch)
		return err

	case "writer":
		return runEnvironmentWriter(ctx, configRoot, args[1:], stdout, stderr, jsonOutput)

	default:
		return fmt.Errorf("unknown env command %q", args[0])
	}
}

func runEnvironmentWriter(ctx context.Context, configRoot string, args []string, stdout, stderr io.Writer, jsonOutput bool) error {
	if len(args) == 0 {
		return errors.New("usage: ai-dev-manager env writer <acquire|release> ...")
	}
	switch args[0] {
	case "acquire":
		flags := flag.NewFlagSet("env writer acquire", flag.ContinueOnError)
		flags.SetOutput(stderr)
		owner := flags.String("owner", "", "writer owner identifier")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if len(flags.Args()) != 1 || strings.TrimSpace(flags.Args()[0]) == "" || strings.TrimSpace(*owner) == "" {
			return errors.New("usage: ai-dev-manager env writer acquire --owner OWNER <environment-id>")
		}
		item, err := daemon.EnvironmentWriterAcquire(ctx, configRoot, flags.Args()[0], *owner)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(stdout, item)
		}
		return writeEnvironment(stdout, item)

	case "release":
		flags := flag.NewFlagSet("env writer release", flag.ContinueOnError)
		flags.SetOutput(stderr)
		owner := flags.String("owner", "", "writer owner identifier")
		force := flags.Bool("force", false, "clear the current writer lease without matching owner")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if len(flags.Args()) != 1 || strings.TrimSpace(flags.Args()[0]) == "" || (!*force && strings.TrimSpace(*owner) == "") {
			return errors.New("usage: ai-dev-manager env writer release [--owner OWNER | --force] <environment-id>")
		}
		item, err := daemon.EnvironmentWriterRelease(ctx, configRoot, flags.Args()[0], *owner, *force)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeJSON(stdout, item)
		}
		return writeEnvironment(stdout, item)

	default:
		return fmt.Errorf("unknown env writer command %q", args[0])
	}
}

func writeEnvironmentInspect(w io.Writer, result environment.InspectResult, jsonOutput bool) error {
	if jsonOutput {
		return writeJSON(w, result)
	}
	if err := writeEnvironment(w, result.Environment); err != nil {
		return err
	}
	for _, warning := range result.Warnings {
		if _, err := fmt.Fprintf(w, "warning\t%s\t%s\n", warning.Code, warning.Message); err != nil {
			return err
		}
	}
	for _, hint := range result.Hints {
		if _, err := fmt.Fprintf(w, "hint\t%s\t%s\n", hint.Code, hint.Message); err != nil {
			return err
		}
	}
	return nil
}

func writeEnvironment(w io.Writer, item environment.Environment) error {
	if _, err := fmt.Fprintf(w, "%s\t%s\tworkspace=%s\tbranch=%s\tbase=%s\tpath=%s",
		item.State, item.ID, item.WorkspaceID, item.Branch, item.BaseCommit, item.WorktreePath); err != nil {
		return err
	}
	if item.Writer != nil {
		if _, err := fmt.Fprintf(w, "\twriter=%s", item.Writer.Owner); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}
