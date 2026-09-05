package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"ai-dev-manager/internal/environment"
)

type environmentRequest struct {
	WorkspaceID    string `json:"workspace_id,omitempty"`
	EnvironmentID  string `json:"environment_id,omitempty"`
	Name           string `json:"name,omitempty"`
	Base           string `json:"base,omitempty"`
	Branch         string `json:"branch,omitempty"`
	IncludeChanges bool   `json:"include_changes,omitempty"`
	Force          bool   `json:"force,omitempty"`
	Owner          string `json:"owner,omitempty"`
}

func registerEnvironmentHandlers(mux *http.ServeMux, manager *environment.Manager) {
	mux.HandleFunc("/environment/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		request, ok := decodeEnvironmentRequest(w, r)
		if !ok {
			return
		}
		result, err := manager.Create(r.Context(), environment.CreateRequest{
			WorkspaceID:    request.WorkspaceID,
			Name:           request.Name,
			Base:           request.Base,
			Branch:         request.Branch,
			IncludeChanges: request.IncludeChanges,
		})
		if err != nil {
			writeEnvironmentError(w, err)
			return
		}
		writeControlJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("/environment/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		items, err := manager.List()
		if err != nil {
			writeEnvironmentError(w, err)
			return
		}
		writeControlJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("/environment/inspect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimSpace(r.URL.Query().Get("environment_id"))
		if id == "" {
			writeControlError(w, http.StatusBadRequest, "environment_id is required")
			return
		}
		result, err := manager.Inspect(r.Context(), id)
		if err != nil {
			writeEnvironmentError(w, err)
			return
		}
		writeControlJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("/environment/destroy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		request, ok := decodeEnvironmentRequest(w, r)
		if !ok {
			return
		}
		if request.EnvironmentID == "" {
			writeControlError(w, http.StatusBadRequest, "environment_id is required")
			return
		}
		result, err := manager.Destroy(r.Context(), request.EnvironmentID, request.Force)
		if err != nil {
			writeEnvironmentError(w, err)
			return
		}
		writeControlJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("/environment/writer/acquire", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		request, ok := decodeEnvironmentRequest(w, r)
		if !ok {
			return
		}
		if request.EnvironmentID == "" || request.Owner == "" {
			writeControlError(w, http.StatusBadRequest, "environment_id and owner are required")
			return
		}
		result, err := manager.AcquireWriter(r.Context(), request.EnvironmentID, request.Owner)
		if err != nil {
			writeEnvironmentError(w, err)
			return
		}
		writeControlJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("/environment/writer/release", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		request, ok := decodeEnvironmentRequest(w, r)
		if !ok {
			return
		}
		if request.EnvironmentID == "" || (!request.Force && request.Owner == "") {
			writeControlError(w, http.StatusBadRequest, "environment_id and owner are required unless force=true")
			return
		}
		result, err := manager.ReleaseWriter(request.EnvironmentID, request.Owner, request.Force)
		if err != nil {
			writeEnvironmentError(w, err)
			return
		}
		writeControlJSON(w, http.StatusOK, result)
	})
}

func decodeEnvironmentRequest(w http.ResponseWriter, r *http.Request) (environmentRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxControlBodyBytes)
	defer r.Body.Close()
	var request environmentRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeControlError(w, http.StatusBadRequest, "invalid request")
		return environmentRequest{}, false
	}
	request.WorkspaceID = strings.TrimSpace(request.WorkspaceID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.Name = strings.TrimSpace(request.Name)
	request.Base = strings.TrimSpace(request.Base)
	request.Branch = strings.TrimSpace(request.Branch)
	request.Owner = strings.TrimSpace(request.Owner)
	return request, true
}

func writeEnvironmentError(w http.ResponseWriter, err error) {
	status := http.StatusConflict
	var envErr *environment.Error
	if errors.As(err, &envErr) {
		switch envErr.Code {
		case environment.ErrInvalidInput:
			status = http.StatusBadRequest
		case environment.ErrNotFound, environment.ErrWorktreeMissing:
			status = http.StatusNotFound
		case environment.ErrStore, environment.ErrRuntime:
			status = http.StatusInternalServerError
		}
	}
	writeControlError(w, status, err.Error())
}

func EnvironmentCreate(ctx context.Context, root string, request environment.CreateRequest) (environment.InspectResult, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return environment.InspectResult{}, err
	}
	target, err := controlURL(meta.ControlEndpoint, "/environment/create")
	if err != nil {
		return environment.InspectResult{}, err
	}
	body, err := json.Marshal(environmentRequest{WorkspaceID: request.WorkspaceID, Name: request.Name, Base: request.Base, Branch: request.Branch, IncludeChanges: request.IncludeChanges})
	if err != nil {
		return environment.InspectResult{}, err
	}
	var result environment.InspectResult
	if err := controlRequest(ctx, http.MethodPost, target, body, &result); err != nil {
		return environment.InspectResult{}, err
	}
	return result, nil
}

func EnvironmentList(ctx context.Context, root string) ([]environment.Environment, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return nil, err
	}
	target, err := controlURL(meta.ControlEndpoint, "/environment/list")
	if err != nil {
		return nil, err
	}
	var result []environment.Environment
	if err := controlRequest(ctx, http.MethodGet, target, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func EnvironmentInspect(ctx context.Context, root, environmentID string) (environment.InspectResult, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return environment.InspectResult{}, err
	}
	target, err := controlURL(meta.ControlEndpoint, "/environment/inspect")
	if err != nil {
		return environment.InspectResult{}, err
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return environment.InspectResult{}, err
	}
	query := parsed.Query()
	query.Set("environment_id", strings.TrimSpace(environmentID))
	parsed.RawQuery = query.Encode()
	var result environment.InspectResult
	if err := controlRequest(ctx, http.MethodGet, parsed.String(), nil, &result); err != nil {
		return environment.InspectResult{}, err
	}
	return result, nil
}

func EnvironmentDestroy(ctx context.Context, root, environmentID string) (environment.Environment, error) {
	return EnvironmentDestroyWithForce(ctx, root, environmentID, false)
}

func EnvironmentDestroyWithForce(ctx context.Context, root, environmentID string, force bool) (environment.Environment, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return environment.Environment{}, err
	}
	target, err := controlURL(meta.ControlEndpoint, "/environment/destroy")
	if err != nil {
		return environment.Environment{}, err
	}
	body, err := json.Marshal(environmentRequest{EnvironmentID: strings.TrimSpace(environmentID), Force: force})
	if err != nil {
		return environment.Environment{}, err
	}
	var result environment.Environment
	if err := controlRequest(ctx, http.MethodPost, target, body, &result); err != nil {
		return environment.Environment{}, err
	}
	return result, nil
}

func EnvironmentWriterAcquire(ctx context.Context, root, environmentID, owner string) (environment.Environment, error) {
	return environmentWriterPost(ctx, root, "/environment/writer/acquire", environmentRequest{EnvironmentID: environmentID, Owner: owner})
}

func EnvironmentWriterRelease(ctx context.Context, root, environmentID, owner string, force bool) (environment.Environment, error) {
	return environmentWriterPost(ctx, root, "/environment/writer/release", environmentRequest{EnvironmentID: environmentID, Owner: owner, Force: force})
}

func environmentWriterPost(ctx context.Context, root, path string, request environmentRequest) (environment.Environment, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return environment.Environment{}, err
	}
	target, err := controlURL(meta.ControlEndpoint, path)
	if err != nil {
		return environment.Environment{}, err
	}
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.Owner = strings.TrimSpace(request.Owner)
	body, err := json.Marshal(request)
	if err != nil {
		return environment.Environment{}, err
	}
	var result environment.Environment
	if err := controlRequest(ctx, http.MethodPost, target, body, &result); err != nil {
		return environment.Environment{}, err
	}
	return result, nil
}
