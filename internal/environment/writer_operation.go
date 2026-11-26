package environment

import (
	"crypto/sha256"
	"fmt"
	"time"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/pathutil"
	"ai-dev-manager-v2/internal/store"
)

// BeginWriterOperation checks the lease both before and after cross-process
// admission. A lease change in between cannot start a stale mutation. A new
// Writer may be granted immediately, but its finite mutations cannot start
// until overlapping operations of a previous generation have drained.
func (s *Service) BeginWriterOperation(id, owner string, expected *model.WriterLease, kinds ...string) (model.Environment, *store.OperationGuard, error) {
	env, err := s.RequireWriter(id, owner)
	if err != nil {
		return model.Environment{}, nil, err
	}
	if expected != nil && !SameWriterLease(env.Writer, expected) {
		return model.Environment{}, nil, fmt.Errorf("writer lease was replaced for environment %s", id)
	}
	kind := "mutation"
	if len(kinds) > 0 {
		kind = kinds[0]
	}
	digest := writerOperationGeneration(env.Writer)
	guard, err := s.store.BeginOperation(store.OperationIdentity{Root: env.Root, EnvironmentID: id, Generation: digest, Kind: kind})
	if err != nil {
		return model.Environment{}, nil, err
	}
	current, err := s.RequireWriterLease(id, owner, env.Writer)
	if err != nil {
		guard.Close()
		return model.Environment{}, nil, err
	}
	if !pathutil.Same(env.Root, current.Root) {
		guard.Close()
		return model.Environment{}, nil, fmt.Errorf("environment root changed during writer operation admission")
	}
	return current, guard, nil
}

func writerOperationGeneration(lease *model.WriterLease) string {
	if lease == nil {
		return ""
	}
	generation := lease.ID
	if generation == "" {
		generation = fmt.Sprintf("legacy:%s:%d", lease.Owner, lease.AcquiredAt.UnixNano())
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(generation)))
}

type PendingWriterOperation struct {
	EnvironmentID string    `json:"environment_id"`
	Kind          string    `json:"kind"`
	HostPID       int       `json:"host_pid"`
	StartedAt     time.Time `json:"started_at"`
}

// PendingWriterOperations reports live previous-generation operations in
// overlapping roots, including the case where no Writer is currently held.
func (s *Service) PendingWriterOperations(env model.Environment) ([]PendingWriterOperation, error) {
	active, err := s.store.ActiveOperations()
	if err != nil {
		return nil, err
	}
	generation := writerOperationGeneration(env.Writer)
	out := []PendingWriterOperation{}
	for _, op := range active {
		if env.ID == op.EnvironmentID && generation == op.Generation {
			continue
		}
		if !writerRootsOverlap(env.Root, op.Root) {
			continue
		}
		out = append(out, PendingWriterOperation{op.EnvironmentID, op.Kind, op.HostPID, op.StartedAt})
	}
	return out, nil
}
