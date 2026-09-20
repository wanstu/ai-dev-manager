package app

import (
	"context"
	"os/exec"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// execUsageCommandTransport wraps the SDK command transport so execution usage
// is recorded only after exec.Cmd.Start succeeds. MCP initialization may still
// fail later; the subprocess was nevertheless actually started and must count.
type execUsageCommandTransport struct {
	inner         *mcp.CommandTransport
	service       *Service
	environmentID string
	executable    string
	surface       string
	once          sync.Once
}

func (s *Service) TrackMCPCommandTransport(command *exec.Cmd, environmentID, executable, surface string) mcp.Transport {
	return &execUsageCommandTransport{
		inner:         &mcp.CommandTransport{Command: command},
		service:       s,
		environmentID: environmentID,
		executable:    executable,
		surface:       surface,
	}
}

func (t *execUsageCommandTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	connection, err := t.inner.Connect(ctx)
	if t.inner.Command != nil && t.inner.Command.Process != nil {
		t.once.Do(func() {
			if t.service != nil {
				_ = t.service.RecordExecUsage(t.environmentID, t.executable, t.surface)
			}
		})
	}
	return connection, err
}
