package app

import (
	"context"
	"fmt"
	"time"

	"ai-dev-manager-v2/internal/runtime"
)

func (s *Service) GitLog(ctx context.Context, environmentID, ref string, maxCount int) (any, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	return rt.GitLog(ctx, ref, maxCount)
}

func (s *Service) GitShow(ctx context.Context, environmentID, ref string, maxOutputBytes int) (any, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	return rt.GitShow(ctx, ref, maxOutputBytes)
}

func (s *Service) GitRemoteList(ctx context.Context, environmentID string) (any, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	return rt.GitRemoteList(ctx)
}

func (s *Service) GitTagList(ctx context.Context, environmentID string, maxCount int) (any, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}
	return rt.GitTagList(ctx, maxCount)
}

func (s *Service) GitTagCreate(ctx context.Context, environmentID, owner, name, ref, message string, annotated bool) (any, error) {
	return withGitWriter(ctx, s, environmentID, owner, func(commandCtx context.Context, rt *runtime.Runtime) (any, error) {
		return rt.GitTagCreate(commandCtx, name, ref, message, annotated)
	})
}

func (s *Service) GitTagDelete(ctx context.Context, environmentID, owner, name string) (any, error) {
	return withGitWriter(ctx, s, environmentID, owner, func(commandCtx context.Context, rt *runtime.Runtime) (any, error) {
		return rt.GitTagDelete(commandCtx, name)
	})
}

func (s *Service) GitCherryPick(ctx context.Context, environmentID, owner string, refs []string) (any, error) {
	return withGitWriter(ctx, s, environmentID, owner, func(commandCtx context.Context, rt *runtime.Runtime) (any, error) {
		return rt.GitCherryPick(commandCtx, refs)
	})
}

func (s *Service) GitCherryPickContinue(ctx context.Context, environmentID, owner string) (any, error) {
	return withGitWriter(ctx, s, environmentID, owner, func(commandCtx context.Context, rt *runtime.Runtime) (any, error) {
		return rt.GitCherryPickContinue(commandCtx)
	})
}

func (s *Service) GitCherryPickAbort(ctx context.Context, environmentID, owner string) (any, error) {
	return withGitWriter(ctx, s, environmentID, owner, func(commandCtx context.Context, rt *runtime.Runtime) (any, error) {
		return rt.GitCherryPickAbort(commandCtx)
	})
}

func (s *Service) GitMerge(ctx context.Context, environmentID, owner, ref string, noFF bool) (any, error) {
	return withGitWriter(ctx, s, environmentID, owner, func(commandCtx context.Context, rt *runtime.Runtime) (any, error) {
		return rt.GitMerge(commandCtx, ref, noFF)
	})
}

func (s *Service) GitMergeAbort(ctx context.Context, environmentID, owner string) (any, error) {
	return withGitWriter(ctx, s, environmentID, owner, func(commandCtx context.Context, rt *runtime.Runtime) (any, error) {
		return rt.GitMergeAbort(commandCtx)
	})
}

// GitFetch and GitPush are intentionally wired through the Admin surface first.
// Remote transports can invoke credential helpers/SSH and therefore remain host
// authority until Sandbox Runtime or an ADM-owned remote credential binding is
// available for the Agent surface.
func (s *Service) GitFetch(ctx context.Context, environmentID, owner, remote string, tags, prune bool) (any, error) {
	return withGitWriter(ctx, s, environmentID, owner, func(commandCtx context.Context, rt *runtime.Runtime) (any, error) {
		return rt.GitFetch(commandCtx, remote, tags, prune, true)
	})
}

func (s *Service) GitPush(ctx context.Context, environmentID, owner, remote, branch string, tags []string, setUpstream bool) (any, error) {
	return withGitWriter(ctx, s, environmentID, owner, func(commandCtx context.Context, rt *runtime.Runtime) (any, error) {
		return rt.GitPush(commandCtx, remote, branch, tags, setUpstream, true)
	})
}

func withGitWriter(
	ctx context.Context,
	s *Service,
	environmentID, owner string,
	run func(context.Context, *runtime.Runtime) (any, error),
) (any, error) {
	if _, err := s.Environments.RequireWriter(environmentID, owner); err != nil {
		return nil, err
	}
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return nil, err
	}

	commandCtx, cancel := context.WithCancel(ctx)
	heartbeatDone := make(chan error, 1)
	go func() {
		interval := s.heartbeatInterval()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-commandCtx.Done():
				heartbeatDone <- nil
				return
			case <-ticker.C:
				if _, heartbeatErr := s.Environments.HeartbeatWriter(environmentID, owner); heartbeatErr != nil {
					heartbeatDone <- heartbeatErr
					cancel()
					return
				}
			}
		}
	}()

	result, runErr := run(commandCtx, rt)
	cancel()
	heartbeatErr := <-heartbeatDone
	if heartbeatErr != nil {
		return nil, fmt.Errorf("writer heartbeat failed: %w", heartbeatErr)
	}
	if runErr != nil {
		return nil, runErr
	}
	if err := s.Environments.Touch(environmentID, owner); err != nil {
		return nil, err
	}
	return result, nil
}
