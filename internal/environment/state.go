package environment

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"ai-dev-manager/internal/adapter/runtimeadapter"
)

const (
	defaultStaleAfter       = 7 * 24 * time.Hour
	baseHintBehindThreshold = 10
	maxWriterOwnerLength    = 128
)

type gitRelation struct {
	LeftCommit  string `json:"left_commit"`
	RightCommit string `json:"right_commit"`
	Ahead       int    `json:"ahead"`
	Behind      int    `json:"behind"`
	Diverged    bool   `json:"diverged"`
}

func (m *Manager) AcquireWriter(ctx context.Context, id, owner string) (Environment, error) {
	owner = strings.TrimSpace(owner)
	if !validWriterOwner(owner) {
		return Environment{}, &Error{Code: ErrInvalidInput, EnvironmentID: strings.TrimSpace(id), Message: "writer owner is required and must be a short printable identifier"}
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	env, err := m.store.Get(strings.TrimSpace(id))
	if err != nil {
		return Environment{}, err
	}
	if _, _, _, err := m.validatedRuntimes(ctx, env); err != nil {
		return env, err
	}
	now := m.now()
	if env.Writer != nil && env.Writer.Owner != owner {
		return env, &Error{Code: ErrWriterConflict, EnvironmentID: env.ID, Message: fmt.Sprintf("environment writer is already held by %q", env.Writer.Owner)}
	}
	if env.Writer == nil {
		env.Writer = &WriterLease{Owner: owner, AcquiredAt: now, LastSeenAt: now}
	} else {
		env.Writer.LastSeenAt = now
	}
	env.UpdatedAt = now
	env.LastActivityAt = now
	if err := m.store.Put(env); err != nil {
		return Environment{}, err
	}
	return env, nil
}

func (m *Manager) ReleaseWriter(id, owner string, force bool) (Environment, error) {
	owner = strings.TrimSpace(owner)
	m.mu.Lock()
	defer m.mu.Unlock()

	env, err := m.store.Get(strings.TrimSpace(id))
	if err != nil {
		return Environment{}, err
	}
	if env.Writer == nil {
		if force {
			return env, nil
		}
		return env, &Error{Code: ErrWriterNotOwner, EnvironmentID: env.ID, Message: "environment has no active writer"}
	}
	if !force {
		if !validWriterOwner(owner) {
			return env, &Error{Code: ErrInvalidInput, EnvironmentID: env.ID, Message: "writer owner is required"}
		}
		if env.Writer.Owner != owner {
			return env, &Error{Code: ErrWriterNotOwner, EnvironmentID: env.ID, Message: fmt.Sprintf("environment writer is held by %q", env.Writer.Owner)}
		}
	}
	now := m.now()
	env.Writer = nil
	env.UpdatedAt = now
	env.LastActivityAt = now
	if err := m.store.Put(env); err != nil {
		return Environment{}, err
	}
	return env, nil
}

func (m *Manager) buildInspectResult(ctx context.Context, env Environment, baseRuntime, derived runtimeadapter.Runtime, worktree worktreeInfo) InspectResult {
	facts := []Fact{
		{Code: "worktree_available", Message: "Managed worktree is available", Value: true},
		{Code: "base_ref", Message: "Environment creation base reference", Value: env.BaseRef},
		{Code: "base_commit", Message: "Environment creation base commit", Value: env.BaseCommit},
		{Code: "worktree_path", Message: "Validated managed worktree path", Value: worktree.Path},
	}
	warnings := []Warning{}
	hints := []Hint{}

	if env.Metadata["changes_included"] == "true" {
		facts = append(facts, Fact{Code: "changes_included", Message: "Source checkout changes were included in this environment", Value: true})
	} else if env.Metadata["base_dirty_at_create"] == "true" {
		warnings = append(warnings, Warning{
			Code:    "changes_not_included",
			Message: "The base checkout had uncommitted changes when this environment was created; the environment was created from committed base content only.",
		})
	}

	if count, err := invokeSliceCount(ctx, derived, runtimeadapter.OpGitStatus, nil); err != nil {
		warnings = append(warnings, Warning{Code: "git_status_unavailable", Message: "Current Environment Git status could not be read."})
	} else {
		facts = append(facts,
			Fact{Code: "dirty", Message: "Environment has working tree or index changes", Value: count > 0},
			Fact{Code: "dirty_change_count", Message: "Environment Git status entry count", Value: count},
		)
	}

	if value, err := derived.Invoke(ctx, runtimeadapter.OpGitPushStatus, nil); err != nil {
		warnings = append(warnings, Warning{Code: "push_status_unavailable", Message: "Current Environment upstream status could not be read."})
	} else {
		var push gitPushStatus
		if err := decodeValue(value, &push); err != nil {
			warnings = append(warnings, Warning{Code: "push_status_unavailable", Message: "Current Environment upstream status could not be decoded."})
		} else {
			facts = append(facts,
				Fact{Code: "head_commit", Message: "Current Environment HEAD commit", Value: push.Head},
				Fact{Code: "has_upstream", Message: "Environment branch has an upstream", Value: push.HasUpstream},
				Fact{Code: "upstream_ahead", Message: "Environment commits ahead of configured upstream", Value: push.Ahead},
			)
			if push.Upstream != "" {
				facts = append(facts, Fact{Code: "upstream", Message: "Configured Environment upstream", Value: push.Upstream})
			}
		}
	}

	currentBaseCommit, err := invokeString(ctx, baseRuntime, runtimeadapter.OpGitResolveRef, map[string]any{"ref": env.BaseRef})
	if err != nil {
		facts = append(facts, Fact{Code: "current_base_available", Message: "Current base reference can be resolved", Value: false})
		warnings = append(warnings, Warning{Code: "base_ref_unavailable", Message: fmt.Sprintf("Base reference %q is no longer resolvable; the recorded base was not changed.", env.BaseRef)})
	} else {
		facts = append(facts,
			Fact{Code: "current_base_available", Message: "Current base reference can be resolved", Value: true},
			Fact{Code: "current_base_commit", Message: "Current commit resolved from the Environment base reference", Value: currentBaseCommit},
			Fact{Code: "base_moved", Message: "Current base reference differs from the Environment creation base commit", Value: currentBaseCommit != env.BaseCommit},
		)
		if value, relErr := baseRuntime.Invoke(ctx, runtimeadapter.OpGitRelation, map[string]any{"left": env.Branch, "right": env.BaseRef}); relErr != nil {
			warnings = append(warnings, Warning{Code: "base_relation_unavailable", Message: "Environment/base commit relation could not be calculated."})
		} else {
			var relation gitRelation
			if decodeErr := decodeValue(value, &relation); decodeErr != nil {
				warnings = append(warnings, Warning{Code: "base_relation_unavailable", Message: "Environment/base commit relation could not be decoded."})
			} else {
				facts = append(facts,
					Fact{Code: "ahead", Message: "Environment commits not present in current base", Value: relation.Ahead},
					Fact{Code: "behind", Message: "Current base commits not present in Environment", Value: relation.Behind},
					Fact{Code: "diverged", Message: "Environment and current base both contain unique commits", Value: relation.Diverged},
				)
				if relation.Diverged || relation.Behind >= baseHintBehindThreshold {
					hints = append(hints, Hint{
						Code:    "base_sync_may_need_confirmation",
						Message: "Base has advanced or diverged; consider confirming whether synchronization is needed before continuing.",
					})
				}
			}
		}
	}

	activityAt := env.LastActivityAt
	if activityAt.IsZero() {
		activityAt = env.CreatedAt
	}
	inactive := m.now().Sub(activityAt)
	if inactive < 0 {
		inactive = 0
	}
	stale := inactive >= defaultStaleAfter
	facts = append(facts,
		Fact{Code: "last_activity_at", Message: "Last recorded Environment development activity", Value: activityAt},
		Fact{Code: "inactive_for_seconds", Message: "Seconds since last recorded Environment development activity", Value: int64(inactive / time.Second)},
		Fact{Code: "stale_after_seconds", Message: "Current advisory inactivity threshold", Value: int64(defaultStaleAfter / time.Second)},
		Fact{Code: "stale", Message: "Environment has crossed the advisory inactivity threshold", Value: stale},
	)
	if stale {
		warnings = append(warnings, Warning{Code: "environment_inactive", Message: "Environment has been inactive for at least 7 days; no automatic cleanup or writer release will occur."})
	}

	if env.Writer == nil {
		facts = append(facts, Fact{Code: "writer_active", Message: "Environment currently has a writer lease", Value: false})
	} else {
		facts = append(facts,
			Fact{Code: "writer_active", Message: "Environment currently has a writer lease", Value: true},
			Fact{Code: "writer_owner", Message: "Current Environment writer owner", Value: env.Writer.Owner},
			Fact{Code: "writer_acquired_at", Message: "Current writer lease acquisition time", Value: env.Writer.AcquiredAt},
			Fact{Code: "writer_last_seen_at", Message: "Current writer lease last-seen time", Value: env.Writer.LastSeenAt},
		)
	}

	caps := append([]string(nil), derived.Capabilities()...)
	sort.Strings(caps)
	return InspectResult{Environment: env, Facts: facts, Warnings: warnings, Hints: hints, Capabilities: caps}
}

func validWriterOwner(owner string) bool {
	if owner == "" || len(owner) > maxWriterOwnerLength {
		return false
	}
	for _, r := range owner {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
