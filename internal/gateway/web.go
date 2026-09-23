package gateway

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	desktopfrontend "ai-dev-manager-v2/cmd/ai-dev-manager-desktop/frontend"
	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/management"
	"ai-dev-manager-v2/internal/model"

	kitui "github.com/wanstu/wails-desktop-kit/ui"
)

const (
	webSessionCookieName = "adm_web_session"
	webSessionTTL        = 12 * time.Hour
)

type webManagementHandler struct {
	app        *app.Service
	management *management.Service
	owner      *runtimeOwner
	sessions   *webSessionStore
	assets     fs.FS
	static     http.Handler
}

type webSession struct {
	Username  string
	ExpiresAt time.Time
}

type webSessionStore struct {
	mu       sync.Mutex
	sessions map[[32]byte]webSession
}

type webAuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type webAuthResponse struct {
	Initialized         bool   `json:"initialized"`
	Authenticated       bool   `json:"authenticated"`
	Username            string `json:"username,omitempty"`
	RegistrationAllowed bool   `json:"registration_allowed"`
}

type webCallRequest struct {
	Method string            `json:"method"`
	Args   []json.RawMessage `json:"args,omitempty"`
}

type webWorkspaceInput struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

type webEnvironmentInput struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Root        string `json:"root,omitempty"`
}

type webMCPInput struct {
	Name           string                `json:"name"`
	Transport      string                `json:"transport"`
	AuthMode       string                `json:"auth_mode"`
	Endpoint       string                `json:"endpoint,omitempty"`
	HeaderRefs     map[string]string     `json:"header_refs,omitempty"`
	Executable     string                `json:"executable,omitempty"`
	Args           []string              `json:"args,omitempty"`
	EnvRefs        map[string]string     `json:"env_refs,omitempty"`
	HealthPolicy   model.MCPHealthPolicy `json:"health_policy"`
	DefaultInclude bool                  `json:"default_include_in_environment,omitempty"`
}

type webMCPImportInput struct {
	Format         string   `json:"format"`
	Content        string   `json:"content"`
	SelectedNames  []string `json:"selected_names,omitempty"`
	ConflictPolicy string   `json:"conflict_policy,omitempty"`
	DefaultInclude bool     `json:"default_include_in_environment,omitempty"`
	SourceScope    string   `json:"source_scope,omitempty"`
}

type webSkillSourceInput struct {
	Root           string   `json:"root"`
	SupportRoots   []string `json:"support_roots,omitempty"`
	DefaultInclude bool     `json:"default_include_in_environment,omitempty"`
}

func newWebManagementHandler(service *app.Service, owner *runtimeOwner) http.Handler {
	assets, err := desktopfrontend.Assets()
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "web management assets unavailable", http.StatusInternalServerError)
		})
	}
	mounted := kitui.Mount(assets)
	return &webManagementHandler{
		app:        service,
		management: management.New(service),
		owner:      owner,
		sessions:   &webSessionStore{sessions: map[[32]byte]webSession{}},
		assets:     mounted,
		static:     http.FileServer(http.FS(mounted)),
	}
}

func (h *webManagementHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/web/auth/status":
		h.handleAuthStatus(w, r)
		return
	case "/api/web/auth/register":
		h.handleRegister(w, r)
		return
	case "/api/web/auth/login":
		h.handleLogin(w, r)
		return
	case "/api/web/auth/logout":
		h.handleLogout(w, r)
		return
	case "/api/web/manage":
		h.handleManagementCall(w, r)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		if _, ok := h.authenticatedUsername(r); !ok {
			h.serveAsset(w, r, "web-auth.html")
			return
		}
		h.serveAsset(w, r, "index.html")
		return
	}
	h.static.ServeHTTP(w, r)
}

func (h *webManagementHandler) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	status, err := h.app.WebAdminStatus()
	if err != nil {
		writeWebError(w, http.StatusInternalServerError, err)
		return
	}
	username, authenticated := h.authenticatedUsername(r)
	writeWebJSON(w, http.StatusOK, webAuthResponse{
		Initialized:         status.Initialized,
		Authenticated:       authenticated,
		Username:            username,
		RegistrationAllowed: !status.Initialized && webBootstrapRequestAllowed(r),
	})
}

func (h *webManagementHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if !webMutationRequestAllowed(r) {
		writeWebError(w, http.StatusForbidden, errors.New("web mutation request denied"))
		return
	}
	if !webBootstrapRequestAllowed(r) {
		writeWebError(w, http.StatusForbidden, errors.New("first web administrator can only be initialized from localhost; use an SSH tunnel or local browser"))
		return
	}
	var input webAuthRequest
	if err := decodeWebJSON(r, &input); err != nil {
		writeWebError(w, http.StatusBadRequest, err)
		return
	}
	status, err := h.app.InitializeWebAdmin(input.Username, input.Password)
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, app.ErrWebAdminAlreadyInitialized) {
			code = http.StatusConflict
		}
		writeWebError(w, code, err)
		return
	}
	if err := h.startSession(w, r, status.Username); err != nil {
		writeWebError(w, http.StatusInternalServerError, err)
		return
	}
	writeWebJSON(w, http.StatusCreated, webAuthResponse{Initialized: true, Authenticated: true, Username: status.Username})
}

func (h *webManagementHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if !webMutationRequestAllowed(r) {
		writeWebError(w, http.StatusForbidden, errors.New("web mutation request denied"))
		return
	}
	var input webAuthRequest
	if err := decodeWebJSON(r, &input); err != nil {
		writeWebError(w, http.StatusBadRequest, err)
		return
	}
	status, err := h.app.AuthenticateWebAdmin(input.Username, input.Password)
	if err != nil {
		code := http.StatusUnauthorized
		if errors.Is(err, app.ErrWebAdminNotInitialized) {
			code = http.StatusConflict
		}
		writeWebError(w, code, err)
		return
	}
	if err := h.startSession(w, r, status.Username); err != nil {
		writeWebError(w, http.StatusInternalServerError, err)
		return
	}
	writeWebJSON(w, http.StatusOK, webAuthResponse{Initialized: true, Authenticated: true, Username: status.Username})
}

func (h *webManagementHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if !webMutationRequestAllowed(r) {
		writeWebError(w, http.StatusForbidden, errors.New("web mutation request denied"))
		return
	}
	if cookie, err := r.Cookie(webSessionCookieName); err == nil {
		h.sessions.delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     webSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   webRequestSecure(r),
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *webManagementHandler) handleManagementCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if !webMutationRequestAllowed(r) {
		writeWebError(w, http.StatusForbidden, errors.New("web mutation request denied"))
		return
	}
	if _, ok := h.authenticatedUsername(r); !ok {
		writeWebError(w, http.StatusUnauthorized, errors.New("web login required"))
		return
	}
	var call webCallRequest
	if err := decodeWebJSON(r, &call); err != nil {
		writeWebError(w, http.StatusBadRequest, err)
		return
	}
	result, err := h.dispatch(r.Context(), call)
	if err != nil {
		writeWebError(w, http.StatusBadRequest, err)
		return
	}
	writeWebJSON(w, http.StatusOK, map[string]any{"result": result})
}

func (h *webManagementHandler) dispatch(ctx context.Context, call webCallRequest) (any, error) {
	arg := func(index int, target any) error {
		if index >= len(call.Args) {
			return fmt.Errorf("%s argument %d is required", call.Method, index+1)
		}
		if err := json.Unmarshal(call.Args[index], target); err != nil {
			return fmt.Errorf("%s argument %d: %w", call.Method, index+1, err)
		}
		return nil
	}
	var s1, s2, s3 string
	var b1 bool
	switch call.Method {
	case "GetSnapshot":
		return h.management.Snapshot()
	case "UpdateWorktreeSettings":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		return h.management.WorktreeSettingsUpdate(s1, s2)
	case "RefreshHostEnvironment":
		return h.management.HostEnvironmentRefresh()
	case "GetLoggingStatus":
		return h.management.LoggingStatus()
	case "GetGatewayAccessStatus":
		return h.management.GatewayAccessStatus()
	case "GetGatewayDiagnostics":
		return h.management.GatewayDiagnostics()
	case "SetGatewayAllowedHosts":
		var hosts []string
		if err := arg(0, &hosts); err != nil {
			return nil, err
		}
		return h.management.GatewayAllowedHostsSet(hosts)
	case "ConfigureGatewayAdminAPIKey":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.GatewayAdminAPIKeySet(s1)
	case "RotateGatewayAdminAPIKey":
		return h.management.GatewayAdminAPIKeyRotate()
	case "RotateGatewayAgentAPIKey":
		return h.management.GatewayAgentAPIKeyRotate()
	case "ClearGatewayAdminAPIKey":
		return h.management.GatewayAdminAPIKeyClear()
	case "SetGatewayAgentAPIKey":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.GatewayAgentAPIKeySet(s1)
	case "ClearGatewayAgentAPIKey":
		return h.management.GatewayAgentAPIKeyClear()
	case "GetExecAuthorizationStatus":
		return h.management.ExecAuthorizationStatus()
	case "SetExecFullAuthorization":
		if err := arg(0, &b1); err != nil {
			return nil, err
		}
		return h.management.ExecFullAuthorizationSet(b1)
	case "ListSkillSources":
		return h.management.SkillSourceList()
	case "ListSkillAvailability":
		return h.management.SkillAvailabilityList()
	case "AddWorkspace":
		var input webWorkspaceInput
		if err := arg(0, &input); err != nil {
			return nil, err
		}
		return h.management.WorkspaceAdd(input.Path, input.Name)
	case "RenameWorkspace":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		return h.management.WorkspaceRename(s1, s2)
	case "RemoveWorkspace":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.WorkspaceRemove(s1)
	case "DiscoverWorkspace":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		var request model.DiscoveryRequest
		if err := arg(1, &request); err != nil {
			return nil, err
		}
		return h.management.WorkspaceDiscover(s1, request)
	case "CreateEnvironment":
		var input webEnvironmentInput
		if err := arg(0, &input); err != nil {
			return nil, err
		}
		return h.management.EnvironmentCreate(input.WorkspaceID, input.Name, input.Root)
	case "RenameEnvironment":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		return h.management.EnvironmentRename(s1, s2)
	case "RemoveEnvironment":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return nil, h.management.EnvironmentRemove(s1)
	case "InspectEnvironment":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.EnvironmentInspect(s1)
	case "EnvironmentTreeDigest":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		var request model.DiscoveryRequest
		if err := arg(1, &request); err != nil {
			return nil, err
		}
		return h.management.EnvironmentTreeDigest(s1, request)
	case "EnvironmentWorkspaceOptions":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.EnvironmentWorkspaceOptions(s1)
	case "EnvironmentWorkspaceRecommendations":
		return h.management.EnvironmentWorkspaceRecommendations()
	case "SetEnvironmentWorkspace":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		return h.management.EnvironmentWorkspaceSet(s1, s2)
	case "AllowExecutable":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.ExecAllow(s1)
	case "RemoveExecutable":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.ExecRemove(s1)
	case "BlockExecutable":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.ExecBlock(s1)
	case "UnblockExecutable":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.ExecUnblock(s1)
	case "ClearExecDenial":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.ExecDenyClear(s1)
	case "ClearAllExecDenials":
		return nil, h.management.ExecDenyClearAll()
	case "AddMCP":
		var input webMCPInput
		if err := arg(0, &input); err != nil {
			return nil, err
		}
		return h.management.MCPAddConfig(input.Name, catalog.MCPConfig{
			Transport: input.Transport, AuthMode: input.AuthMode, Endpoint: input.Endpoint,
			HeaderRefs: input.HeaderRefs, Executable: input.Executable, Args: input.Args,
			EnvRefs: input.EnvRefs, HealthPolicy: input.HealthPolicy, DefaultInclude: input.DefaultInclude,
		})
	case "UpdateMCP":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		var input webMCPInput
		if err := arg(1, &input); err != nil {
			return nil, err
		}
		return h.management.MCPUpdateConfig(s1, input.Name, catalog.MCPConfig{
			Transport: input.Transport, AuthMode: input.AuthMode, Endpoint: input.Endpoint,
			HeaderRefs: input.HeaderRefs, Executable: input.Executable, Args: input.Args,
			EnvRefs: input.EnvRefs, HealthPolicy: input.HealthPolicy, DefaultInclude: input.DefaultInclude,
		})
	case "RemoveMCP":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return nil, h.management.MCPRemove(s1)
	case "SetMCPDefault":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &b1); err != nil {
			return nil, err
		}
		return h.management.MCPSetDefault(s1, b1)
	case "ProbeMCPHealth":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.MCPProbe(ctx, s1)
	case "PreviewMCPImport":
		var input webMCPImportInput
		if err := arg(0, &input); err != nil {
			return nil, err
		}
		return h.management.MCPImportPreview(app.MCPImportInput{Format: input.Format, Content: input.Content, SelectedNames: input.SelectedNames, ConflictPolicy: input.ConflictPolicy, DefaultInclude: input.DefaultInclude, SourceScope: input.SourceScope})
	case "ApplyMCPImport":
		var input webMCPImportInput
		if err := arg(0, &input); err != nil {
			return nil, err
		}
		return h.management.MCPImportApply(app.MCPImportInput{Format: input.Format, Content: input.Content, SelectedNames: input.SelectedNames, ConflictPolicy: input.ConflictPolicy, DefaultInclude: input.DefaultInclude, SourceScope: input.SourceScope})
	case "AddSkillSource":
		var input webSkillSourceInput
		if err := arg(0, &input); err != nil {
			return nil, err
		}
		return h.management.SkillSourceAdd(input.Root, input.SupportRoots, input.DefaultInclude)
	case "UpdateSkillSource":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		var input webSkillSourceInput
		if err := arg(1, &input); err != nil {
			return nil, err
		}
		return h.management.SkillSourceUpdate(s1, input.Root, input.SupportRoots, input.DefaultInclude)
	case "RefreshSkillSource":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.SkillSourceRefresh(s1)
	case "RemoveSkillSource":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.SkillSourceRemove(s1)
	case "RemoveSkill":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return nil, h.management.SkillRemove(s1)
	case "SetSkillDefault":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &b1); err != nil {
			return nil, err
		}
		return h.management.SkillSetDefault(s1, b1)
	case "SetWorkspaceMCP":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		if err := arg(2, &b1); err != nil {
			return nil, err
		}
		return h.management.WorkspaceMCPSet(s1, s2, b1)
	case "SetWorkspaceSkill":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		if err := arg(2, &b1); err != nil {
			return nil, err
		}
		return h.management.WorkspaceSkillSet(s1, s2, b1)
	case "SetEnvironmentMCP":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		if err := arg(2, &b1); err != nil {
			return nil, err
		}
		return h.management.EnvironmentMCPSet(s1, s2, b1)
	case "SetEnvironmentSkill":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		if err := arg(2, &b1); err != nil {
			return nil, err
		}
		return h.management.EnvironmentSkillSet(s1, s2, b1)
	case "ListEnvironmentSkills":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.EnvironmentSkillList(s1)
	case "ListGlobalMemory":
		return h.management.GlobalMemoryList()
	case "WriteGlobalMemory":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		return nil, h.management.GlobalMemoryWrite(s1, s2)
	case "DeleteGlobalMemory":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return nil, h.management.GlobalMemoryDelete(s1)
	case "ListEnvironmentMemory":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.management.EnvironmentMemoryList(s1)
	case "WriteEnvironmentMemory":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		if err := arg(2, &s3); err != nil {
			return nil, err
		}
		return nil, h.management.EnvironmentMemoryWrite(s1, s2, s3)
	case "DeleteEnvironmentMemory":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		return nil, h.management.EnvironmentMemoryDelete(s1, s2)
	case "AcquireRuntimeWriter":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		return h.app.WriterAcquire(s1, s2)
	case "ReleaseRuntimeWriter":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		return h.app.WriterRelease(s1, s2, false)
	case "ListVerifiers":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		return h.app.VerifierList(s1)
	case "RunVerifier":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		if err := arg(2, &s3); err != nil {
			return nil, err
		}
		return h.app.RunVerifier(ctx, s1, s2, s3, 64*1024)
	case "ListProcesses":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if h.owner == nil {
			return nil, errors.New("Gateway runtime owner is unavailable")
		}
		return h.owner.ListDevProcesses(s1)
	case "GetProcessLogs":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		if h.owner == nil {
			return nil, errors.New("Gateway runtime owner is unavailable")
		}
		return h.owner.DevProcessLogs(s1, s2)
	case "StopProcess":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		if err := arg(2, &s3); err != nil {
			return nil, err
		}
		if h.owner == nil {
			return nil, errors.New("Gateway runtime owner is unavailable")
		}
		return h.owner.StopDevProcess(s1, s2, s3)
	case "ListRuns":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if h.owner == nil {
			return nil, errors.New("Gateway runtime owner is unavailable")
		}
		return h.owner.ListAgentRuns(s1)
	case "CancelRun":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		if err := arg(2, &s3); err != nil {
			return nil, err
		}
		if h.owner == nil {
			return nil, errors.New("Gateway runtime owner is unavailable")
		}
		return h.owner.CancelAgentRun(s1, s2, s3)
	case "GetTemporaryEnvironmentStatus":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if h.owner == nil {
			return nil, errors.New("Gateway runtime owner is unavailable")
		}
		return h.owner.TemporaryEnvironmentStatus(ctx, s1)
	case "PromoteTemporaryEnvironment":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		if h.owner == nil {
			return nil, errors.New("Gateway runtime owner is unavailable")
		}
		return h.owner.PromoteTemporaryEnvironment(ctx, s1, s2)
	case "CleanupTemporaryEnvironment":
		if err := arg(0, &s1); err != nil {
			return nil, err
		}
		if err := arg(1, &s2); err != nil {
			return nil, err
		}
		if err := arg(2, &b1); err != nil {
			return nil, err
		}
		if h.owner == nil {
			return nil, errors.New("Gateway runtime owner is unavailable")
		}
		return h.owner.TemporaryEnvironmentCleanup(ctx, s1, s2, b1)
	case "CleanupExpiredTemporaryEnvironments":
		if err := arg(0, &b1); err != nil {
			return nil, err
		}
		if h.owner == nil {
			return nil, errors.New("Gateway runtime owner is unavailable")
		}
		return h.owner.ExpiredTemporaryEnvironmentsCleanup(ctx, b1)
	default:
		return nil, fmt.Errorf("unsupported web management method %q", call.Method)
	}
}

func (h *webManagementHandler) authenticatedUsername(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(webSessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return "", false
	}
	return h.sessions.username(cookie.Value)
}

func (h *webManagementHandler) startSession(w http.ResponseWriter, r *http.Request, username string) error {
	token, err := h.sessions.create(username, webSessionTTL)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     webSessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   webRequestSecure(r),
		MaxAge:   int(webSessionTTL.Seconds()),
	})
	return nil
}

func (h *webManagementHandler) serveAsset(w http.ResponseWriter, r *http.Request, name string) {
	data, err := fs.ReadFile(h.assets, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(name, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}

func (s *webSessionStore) create(username string, ttl time.Duration) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate web session: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	key := sha256.Sum256([]byte(token))
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	s.sessions[key] = webSession{Username: username, ExpiresAt: now.Add(ttl)}
	return token, nil
}

func (s *webSessionStore) username(token string) (string, bool) {
	key := sha256.Sum256([]byte(token))
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	session, ok := s.sessions[key]
	if !ok {
		return "", false
	}
	return session.Username, true
}

func (s *webSessionStore) delete(token string) {
	key := sha256.Sum256([]byte(token))
	s.mu.Lock()
	delete(s.sessions, key)
	s.mu.Unlock()
}

func (s *webSessionStore) cleanupLocked(now time.Time) {
	for key, session := range s.sessions {
		if !session.ExpiresAt.After(now) {
			delete(s.sessions, key)
		}
	}
}

func webBootstrapRequestAllowed(r *http.Request) bool {
	if strings.TrimSpace(r.Header.Get("Forwarded")) != "" ||
		strings.TrimSpace(r.Header.Get("X-Forwarded-For")) != "" ||
		strings.TrimSpace(r.Header.Get("X-Real-IP")) != "" {
		return false
	}
	remoteHost, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		remoteHost = strings.TrimSpace(r.RemoteAddr)
	}
	remoteIP := net.ParseIP(strings.Trim(remoteHost, "[]"))
	if remoteIP == nil || !remoteIP.IsLoopback() {
		return false
	}
	host := strings.TrimSpace(r.Host)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func webMutationRequestAllowed(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("X-ADM-Web")) == "1"
}

func webRequestSecure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func decodeWebJSON(r *http.Request, target any) error {
	const maxBody = 1 << 20
	limited := io.LimitReader(r.Body, maxBody+1)
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	if decoder.InputOffset() > maxBody {
		return fmt.Errorf("request body exceeds %d bytes", maxBody)
	}
	return nil
}

func writeWebJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeWebError(w http.ResponseWriter, status int, err error) {
	writeWebJSON(w, status, map[string]string{"error": err.Error()})
}
