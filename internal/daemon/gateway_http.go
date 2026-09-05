package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type gatewayRequest struct {
	Listen  string `json:"listen,omitempty"`
	Exposed bool   `json:"exposed,omitempty"`
}

func registerGatewayHandlers(mux *http.ServeMux, owner *GatewayOwner) {
	mux.HandleFunc("/gateway/up", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxControlBodyBytes)
		defer r.Body.Close()
		var request gatewayRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeControlError(w, http.StatusBadRequest, "invalid request")
			return
		}
		var status GatewayStatus
		var err error
		if request.Exposed {
			status, err = owner.UpDocker(r.Context())
		} else {
			status, err = owner.Up(r.Context(), strings.TrimSpace(request.Listen))
		}
		if err != nil {
			writeControlError(w, http.StatusConflict, err.Error())
			return
		}
		writeControlJSON(w, http.StatusOK, status)
	})

	mux.HandleFunc("/gateway/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		status, err := owner.Status()
		if err != nil {
			writeControlError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeControlJSON(w, http.StatusOK, status)
	})

	mux.HandleFunc("/gateway/down", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		status, err := owner.Down(r.Context())
		if err != nil {
			writeControlError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeControlJSON(w, http.StatusOK, status)
	})
}

func GatewayUp(ctx context.Context, root, listen string) (GatewayStatus, error) {
	return gatewayUpRequest(ctx, root, gatewayRequest{Listen: strings.TrimSpace(listen)})
}

func GatewayUpDocker(ctx context.Context, root string) (GatewayStatus, error) {
	return gatewayUpRequest(ctx, root, gatewayRequest{Exposed: true})
}

func gatewayUpRequest(ctx context.Context, root string, request gatewayRequest) (GatewayStatus, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return GatewayStatus{}, err
	}
	target, err := controlURL(meta.ControlEndpoint, "/gateway/up")
	if err != nil {
		return GatewayStatus{}, err
	}
	body, err := json.Marshal(request)
	if err != nil {
		return GatewayStatus{}, err
	}
	var result GatewayStatus
	if err := controlRequest(ctx, http.MethodPost, target, body, &result); err != nil {
		return GatewayStatus{}, err
	}
	return result, nil
}

func GatewayGetStatus(ctx context.Context, root string) (GatewayStatus, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return GatewayStatus{}, err
	}
	target, err := controlURL(meta.ControlEndpoint, "/gateway/status")
	if err != nil {
		return GatewayStatus{}, err
	}
	var result GatewayStatus
	if err := controlRequest(ctx, http.MethodGet, target, nil, &result); err != nil {
		return GatewayStatus{}, err
	}
	return result, nil
}

func GatewayDown(ctx context.Context, root string) (GatewayStatus, error) {
	meta, err := Status(ctx, root)
	if err != nil {
		return GatewayStatus{}, err
	}
	target, err := controlURL(meta.ControlEndpoint, "/gateway/down")
	if err != nil {
		return GatewayStatus{}, err
	}
	var result GatewayStatus
	if err := controlRequest(ctx, http.MethodPost, target, []byte(`{}`), &result); err != nil {
		return GatewayStatus{}, err
	}
	return result, nil
}
