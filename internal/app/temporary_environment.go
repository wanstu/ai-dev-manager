package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/isolation"
	"ai-dev-manager-v2/internal/model"
)

const temporaryEnvironmentRetentionPolicy = "temporary_environment_ttl"

// CreateTemporaryEnvironment creates a fresh Environment whose temporary
// retention intent is persisted atomically with Environment identity. The
// caller supplies creatorSurface from the trusted application surface; it is
// not part of the user-controlled request model.
func (s *Service) CreateTemporaryEnvironment(ctx context.Context, creatorSurface string, request model.TemporaryEnvironmentCreateRequest) (model.TemporaryEnvironmentCreateResult, error) {
	if err := ctx.Err(); err != nil {
		return model.TemporaryEnvironmentCreateResult{}, err
	}
	request.WorkspaceID = strings.TrimSpace(request.WorkspaceID)
	request.SourceEnvironmentID = strings.TrimSpace(request.SourceEnvironmentID)
	request.Name = strings.TrimSpace(request.Name)
	request.OwnerID = strings.TrimSpace(request.OwnerID)
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.RunID = strings.TrimSpace(request.RunID)
	request.Mode = strings.TrimSpace(request.Mode)
	request.Root = strings.TrimSpace(request.Root)
	request.BaseRef = strings.TrimSpace(request.BaseRef)
	creatorSurface = strings.TrimSpace(creatorSurface)
	if creatorSurface == "" {
		creatorSurface = "core"
	}
	if request.Name == "" {
		return model.TemporaryEnvironmentCreateResult{}, fmt.Errorf("environment name is required")
	}
	if request.OwnerID == "" {
		return model.TemporaryEnvironmentCreateResult{}, fmt.Errorf("owner_id is required")
	}
	if request.TTLSeconds <= 0 {
		return model.TemporaryEnvironmentCreateResult{}, fmt.Errorf("ttl_seconds must be positive")
	}
	ttl := time.Duration(request.TTLSeconds) * time.Second
	if ttl <= 0 || int64(ttl/time.Second) != request.TTLSeconds {
		return model.TemporaryEnvironmentCreateResult{}, fmt.Errorf("ttl_seconds is out of range")
	}
	if request.Mode == "" {
		request.Mode = model.TemporaryEnvironmentModeExistingRoot
	}

	now := time.Now().UTC()
	expiresAt := now.Add(ttl)
	retention := model.ResourceRetention{
		Persistence:    model.PersistenceTemporary,
		CreatorSurface: creatorSurface,
		OwnerID:        request.OwnerID,
		SessionID:      request.SessionID,
		RunID:          request.RunID,
		CreatedAt:      &now,
		ExpiresAt:      &expiresAt,
		Policy:         temporaryEnvironmentRetentionPolicy,
	}

	switch request.Mode {
	case model.TemporaryEnvironmentModeExistingRoot:
		if request.WorkspaceID == "" {
			return model.TemporaryEnvironmentCreateResult{}, fmt.Errorf("workspace_id is required for existing_root mode")
		}
		if request.SourceEnvironmentID != "" {
			return model.TemporaryEnvironmentCreateResult{}, fmt.Errorf("source_environment_id is only valid for managed_worktree mode")
		}
		if request.BaseRef != "" {
			return model.TemporaryEnvironmentCreateResult{}, fmt.Errorf("base_ref is only valid for managed_worktree mode")
		}
		environment, err := s.Environments.CreateNewWithRetention(request.WorkspaceID, request.Name, request.Root, retention)
		if err != nil {
			return model.TemporaryEnvironmentCreateResult{}, err
		}
		return model.TemporaryEnvironmentCreateResult{Mode: request.Mode, Environment: environment}, nil
	case model.TemporaryEnvironmentModeManagedWorktree:
		if request.Root != "" {
			return model.TemporaryEnvironmentCreateResult{}, fmt.Errorf("root cannot be supplied for managed_worktree mode")
		}
		if (request.WorkspaceID == "") == (request.SourceEnvironmentID == "") {
			return model.TemporaryEnvironmentCreateResult{}, fmt.Errorf("managed_worktree mode requires exactly one of workspace_id or source_environment_id")
		}
		if s.Isolation == nil {
			return model.TemporaryEnvironmentCreateResult{}, fmt.Errorf("managed worktree isolation is unavailable")
		}
		var (
			created isolation.CreateResult
			err     error
		)
		if request.SourceEnvironmentID != "" {
			created, err = s.Isolation.CreateFromEnvironmentWithRetention(ctx, request.SourceEnvironmentID, request.Name, request.BaseRef, retention)
		} else {
			created, err = s.Isolation.CreateWithRetention(ctx, request.WorkspaceID, request.Name, request.BaseRef, retention)
		}
		if err != nil {
			return model.TemporaryEnvironmentCreateResult{}, err
		}
		managed := created.ManagedWorktree
		return model.TemporaryEnvironmentCreateResult{
			Mode:            request.Mode,
			Environment:     created.Environment,
			ManagedWorktree: &managed,
		}, nil
	default:
		return model.TemporaryEnvironmentCreateResult{}, fmt.Errorf("invalid temporary environment mode %q", request.Mode)
	}
}

// TemporaryEnvironmentStatus returns retention/cleanup facts without exposing
// Environment-private Memory values. Application-only status deliberately keeps
// the Phase-15 runtime-observation blocker; Gateway-owner status can enrich it.
func (s *Service) TemporaryEnvironmentStatus(ctx context.Context, environmentID string) (model.TemporaryEnvironmentStatus, error) {
	environmentID = strings.TrimSpace(environmentID)
	environment, err := s.Environments.Get(environmentID)
	if err != nil {
		return model.TemporaryEnvironmentStatus{}, err
	}
	report, err := s.ResourceRetentionReport(ctx)
	if err != nil {
		return model.TemporaryEnvironmentStatus{}, err
	}
	item, err := retentionEnvironmentItem(report, environmentID)
	if err != nil {
		return model.TemporaryEnvironmentStatus{}, err
	}
	managed := false
	if s.Isolation != nil {
		_, managed, err = s.Isolation.GetByEnvironment(environmentID)
		if err != nil {
			return model.TemporaryEnvironmentStatus{}, err
		}
	}
	return temporaryEnvironmentStatus(environment, item, managed), nil
}

// PromoteTemporaryEnvironment changes retention only and requires the matching
// lifecycle owner. Environment identity, root, selections, private Memory and
// managed-worktree metadata remain untouched.
func (s *Service) PromoteTemporaryEnvironment(environmentID, ownerID string) (model.TemporaryEnvironmentStatus, error) {
	environmentID = strings.TrimSpace(environmentID)
	ownerID = strings.TrimSpace(ownerID)
	if environmentID == "" {
		return model.TemporaryEnvironmentStatus{}, fmt.Errorf("environment_id is required")
	}
	if ownerID == "" {
		return model.TemporaryEnvironmentStatus{}, fmt.Errorf("owner_id is required")
	}

	err := s.Store.Update(func(state *model.State) error {
		for i := range state.Environments {
			environment := &state.Environments[i]
			if environment.ID != environmentID {
				continue
			}
			if environment.Retention.Persistence != model.PersistenceTemporary {
				return fmt.Errorf("environment %s is not temporary", environmentID)
			}
			if strings.TrimSpace(environment.Retention.OwnerID) != ownerID {
				return fmt.Errorf("temporary environment %s belongs to owner %q", environmentID, environment.Retention.OwnerID)
			}
			now := time.Now().UTC()
			environment.Retention = promoteRetentionDurable(environment.Retention, model.ResourceRetentionUpdateRequest{
				CreatorSurface: environment.Retention.CreatorSurface,
			}, now)
			environment.UpdatedAt = now
			return nil
		}
		return fmt.Errorf("environment %q not found", environmentID)
	})
	if err != nil {
		return model.TemporaryEnvironmentStatus{}, err
	}
	return s.TemporaryEnvironmentStatus(context.Background(), environmentID)
}

func retentionEnvironmentItem(report model.ResourceRetentionReport, environmentID string) (model.ResourceRetentionItem, error) {
	for _, item := range report.Resources {
		if item.Kind == model.RetentionResourceEnvironment && item.ID == environmentID {
			return item, nil
		}
	}
	return model.ResourceRetentionItem{}, fmt.Errorf("retention environment %q not found", environmentID)
}

func temporaryEnvironmentStatus(environment model.Environment, item model.ResourceRetentionItem, managed bool) model.TemporaryEnvironmentStatus {
	return model.TemporaryEnvironmentStatus{
		EnvironmentID:   environment.ID,
		WorkspaceID:     environment.WorkspaceID,
		Name:            environment.Name,
		Root:            environment.Root,
		ManagedWorktree: managed,
		Retention:       item.Retention,
		CleanupState:    item.CleanupState,
		CleanupEligible: item.CleanupEligible,
		Blockers:        append([]string(nil), item.Blockers...),
		Uncertainties:   append([]string(nil), item.Uncertainties...),
	}
}
