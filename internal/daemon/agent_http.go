package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"ai-dev-manager/internal/agent"
)

type agentRequest struct {
	WorkspaceID string         `json:"workspace_id,omitempty"`
	RunID       string         `json:"run_id,omitempty"`
	Executor    string         `json:"executor,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
}

func registerAgentHandlers(mux *http.ServeMux, manager *agent.Manager) {
	mux.HandleFunc("/agent/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		request, ok := decodeAgentRequest(w, r)
		if !ok {
			return
		}
		if request.WorkspaceID == "" {
			writeControlError(w, http.StatusBadRequest, "workspace_id is required")
			return
		}
		status, err := manager.StartRequest(agent.StartRequest{WorkspaceID: request.WorkspaceID, Executor: request.Executor, Input: request.Input})
		if err != nil {
			writeControlError(w, http.StatusConflict, err.Error())
			return
		}
		writeControlJSON(w, http.StatusOK, status)
	})

	mux.HandleFunc("/agent/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeControlJSON(w, http.StatusOK, manager.List())
	})

	mux.HandleFunc("/agent/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		runID := strings.TrimSpace(r.URL.Query().Get("run_id"))
		if runID == "" {
			writeControlError(w, http.StatusBadRequest, "run_id is required")
			return
		}
		status, err := manager.Get(runID)
		if err != nil {
			writeControlError(w, http.StatusNotFound, err.Error())
			return
		}
		writeControlJSON(w, http.StatusOK, status)
	})

	mux.HandleFunc("/agent/cancel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		request, ok := decodeAgentRequest(w, r)
		if !ok {
			return
		}
		if request.RunID == "" {
			writeControlError(w, http.StatusBadRequest, "run_id is required")
			return
		}
		status, err := manager.Cancel(request.RunID)
		if err != nil {
			code := http.StatusConflict
			if errors.Is(err, agent.ErrRunNotFound) {
				code = http.StatusNotFound
			}
			writeControlError(w, code, err.Error())
			return
		}
		writeControlJSON(w, http.StatusOK, status)
	})
}

func decodeAgentRequest(w http.ResponseWriter, r *http.Request) (agentRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxControlBodyBytes)
	defer r.Body.Close()
	var request agentRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeControlError(w, http.StatusBadRequest, "invalid request")
		return agentRequest{}, false
	}
	request.WorkspaceID = strings.TrimSpace(request.WorkspaceID)
	request.RunID = strings.TrimSpace(request.RunID)
	request.Executor = strings.TrimSpace(request.Executor)
	return request, true
}

func AgentRun(ctx context.Context, root, workspaceID string) (agent.RunStatus, error) {
	return AgentRunWithExecutor(ctx, root, workspaceID, "")
}

func AgentRunWithExecutor(ctx context.Context, root, workspaceID, executor string) (agent.RunStatus, error) {
	return AgentRunRequest(ctx, root, workspaceID, executor, nil)
}

func AgentRunRequest(ctx context.Context, root, workspaceID, executor string, input map[string]any) (agent.RunStatus, error) {
	return agentPost(ctx, root, "/agent/run", agentRequest{WorkspaceID: workspaceID, Executor: executor, Input: input})
}

func AgentCancel(ctx context.Context, root, runID string) (agent.RunStatus, error) {
	return agentPost(ctx, root, "/agent/cancel", agentRequest{RunID: runID})
}

func AgentStatus(ctx context.Context, root, runID string) (agent.RunStatus, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return agent.RunStatus{}, err
	}
	target, err := controlURL(meta.ControlEndpoint, "/agent/status")
	if err != nil {
		return agent.RunStatus{}, err
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return agent.RunStatus{}, err
	}
	query := parsed.Query()
	query.Set("run_id", strings.TrimSpace(runID))
	parsed.RawQuery = query.Encode()
	var status agent.RunStatus
	if err := controlRequest(ctx, http.MethodGet, parsed.String(), nil, &status); err != nil {
		return agent.RunStatus{}, err
	}
	return status, nil
}

func AgentList(ctx context.Context, root string) ([]agent.RunStatus, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return nil, err
	}
	target, err := controlURL(meta.ControlEndpoint, "/agent/list")
	if err != nil {
		return nil, err
	}
	var statuses []agent.RunStatus
	if err := controlRequest(ctx, http.MethodGet, target, nil, &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func agentPost(ctx context.Context, root, path string, request agentRequest) (agent.RunStatus, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return agent.RunStatus{}, err
	}
	target, err := controlURL(meta.ControlEndpoint, path)
	if err != nil {
		return agent.RunStatus{}, err
	}
	body, err := json.Marshal(request)
	if err != nil {
		return agent.RunStatus{}, err
	}
	var status agent.RunStatus
	if err := controlRequest(ctx, http.MethodPost, target, body, &status); err != nil {
		return agent.RunStatus{}, err
	}
	return status, nil
}
