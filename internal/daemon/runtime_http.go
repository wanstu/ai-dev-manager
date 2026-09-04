package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxControlBodyBytes = 64 << 10

type runtimeRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Listen      string `json:"listen,omitempty"`
	Exposed     bool   `json:"exposed,omitempty"`
}

type controlError struct {
	Error string `json:"error"`
}

func registerRuntimeHandlers(mux *http.ServeMux, owner *RuntimeOwner) {
	mux.HandleFunc("/runtime/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		request, ok := decodeRuntimeRequest(w, r)
		if !ok {
			return
		}
		status, err := owner.StartWithOptions(r.Context(), request.WorkspaceID, RuntimeStartOptions{Listen: request.Listen, Exposed: request.Exposed})
		if err != nil {
			writeControlError(w, http.StatusConflict, status.Error)
			return
		}
		writeControlJSON(w, http.StatusOK, status)
	})

	mux.HandleFunc("/runtime/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		workspaceID := strings.TrimSpace(r.URL.Query().Get("workspace_id"))
		if workspaceID == "" {
			writeControlError(w, http.StatusBadRequest, "workspace_id is required")
			return
		}
		status, err := owner.Status(workspaceID)
		if err != nil {
			writeControlError(w, http.StatusNotFound, err.Error())
			return
		}
		writeControlJSON(w, http.StatusOK, status)
	})

	mux.HandleFunc("/runtime/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		request, ok := decodeRuntimeRequest(w, r)
		if !ok {
			return
		}
		status, err := owner.Stop(r.Context(), request.WorkspaceID)
		if err != nil {
			writeControlError(w, http.StatusConflict, status.Error)
			return
		}
		writeControlJSON(w, http.StatusOK, status)
	})

	mux.HandleFunc("/runtime/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeControlJSON(w, http.StatusOK, owner.List())
	})
}

func decodeRuntimeRequest(w http.ResponseWriter, r *http.Request) (runtimeRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxControlBodyBytes)
	defer r.Body.Close()
	var request runtimeRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeControlError(w, http.StatusBadRequest, "invalid request")
		return runtimeRequest{}, false
	}
	request.WorkspaceID = strings.TrimSpace(request.WorkspaceID)
	request.Listen = strings.TrimSpace(request.Listen)
	if request.WorkspaceID == "" {
		writeControlError(w, http.StatusBadRequest, "workspace_id is required")
		return runtimeRequest{}, false
	}
	return request, true
}

func writeControlJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}

func writeControlError(w http.ResponseWriter, statusCode int, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "control request failed"
	}
	writeControlJSON(w, statusCode, controlError{Error: message})
}

func RuntimeStart(ctx context.Context, root, workspaceID string) (RuntimeStatus, error) {
	return RuntimeStartWithOptions(ctx, root, workspaceID, RuntimeStartOptions{})
}

func RuntimeStartWithOptions(ctx context.Context, root, workspaceID string, options RuntimeStartOptions) (RuntimeStatus, error) {
	return runtimePost(ctx, root, "/runtime/start", runtimeRequest{WorkspaceID: workspaceID, Listen: options.Listen, Exposed: options.Exposed})
}

func RuntimeStop(ctx context.Context, root, workspaceID string) (RuntimeStatus, error) {
	return runtimePost(ctx, root, "/runtime/stop", runtimeRequest{WorkspaceID: workspaceID})
}

func RuntimeGetStatus(ctx context.Context, root, workspaceID string) (RuntimeStatus, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return RuntimeStatus{}, err
	}
	target, err := controlURL(meta.ControlEndpoint, "/runtime/status")
	if err != nil {
		return RuntimeStatus{}, err
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return RuntimeStatus{}, err
	}
	query := parsed.Query()
	query.Set("workspace_id", workspaceID)
	parsed.RawQuery = query.Encode()
	var status RuntimeStatus
	if err := controlRequest(ctx, http.MethodGet, parsed.String(), nil, &status); err != nil {
		return RuntimeStatus{}, err
	}
	return status, nil
}

func RuntimeList(ctx context.Context, root string) ([]RuntimeStatus, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return nil, err
	}
	target, err := controlURL(meta.ControlEndpoint, "/runtime/list")
	if err != nil {
		return nil, err
	}
	var statuses []RuntimeStatus
	if err := controlRequest(ctx, http.MethodGet, target, nil, &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func runtimePost(ctx context.Context, root, path string, request runtimeRequest) (RuntimeStatus, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return RuntimeStatus{}, err
	}
	target, err := controlURL(meta.ControlEndpoint, path)
	if err != nil {
		return RuntimeStatus{}, err
	}
	body, err := json.Marshal(request)
	if err != nil {
		return RuntimeStatus{}, err
	}
	var status RuntimeStatus
	if err := controlRequest(ctx, http.MethodPost, target, body, &status); err != nil {
		return RuntimeStatus{}, err
	}
	return status, nil
}

func controlRequest(ctx context.Context, method, target string, body []byte, output any) error {
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(requestCtx, method, target, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure controlError
		if decodeErr := json.NewDecoder(resp.Body).Decode(&failure); decodeErr == nil && strings.TrimSpace(failure.Error) != "" {
			return errors.New(failure.Error)
		}
		return fmt.Errorf("daemon control request returned %s", resp.Status)
	}
	if output == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(output)
}
