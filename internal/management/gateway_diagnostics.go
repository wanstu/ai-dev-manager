package management

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/configpath"
	"ai-dev-manager-v2/internal/gatewayservice"
)

type GatewayDiagnosticIssue struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

type GatewayDiagnostics struct {
	ObservedAt      time.Time                `json:"observed_at"`
	ProcessPID      int                      `json:"process_pid"`
	ProcessUser     string                   `json:"process_user,omitempty"`
	GOOS            string                   `json:"goos"`
	GOARCH          string                   `json:"goarch"`
	Executable      string                   `json:"executable,omitempty"`
	StatePath       string                   `json:"state_path,omitempty"`
	StateDir        string                   `json:"state_dir,omitempty"`
	ClientConfigDir string                   `json:"client_config_dir,omitempty"`
	Access          app.GatewayAccessStatus  `json:"access"`
	Readiness       app.GatewayReadiness     `json:"readiness"`
	Service         gatewayservice.Status    `json:"service"`
	ServiceError    string                   `json:"service_error,omitempty"`
	Issues          []GatewayDiagnosticIssue `json:"issues,omitempty"`
}

func (s *Service) GatewayDiagnostics() (GatewayDiagnostics, error) {
	access, err := s.app.GatewayAccessStatus()
	if err != nil {
		return GatewayDiagnostics{}, err
	}
	readiness, err := s.app.GatewayRemoteReadiness()
	if err != nil {
		return GatewayDiagnostics{}, err
	}

	result := GatewayDiagnostics{
		ObservedAt: time.Now().UTC(),
		ProcessPID: os.Getpid(),
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		Access:     access,
		Readiness:  readiness,
	}
	if current, currentErr := user.Current(); currentErr == nil {
		result.ProcessUser = strings.TrimSpace(current.Username)
	}
	if executable, executableErr := os.Executable(); executableErr == nil {
		result.Executable = executable
	}
	if s != nil && s.app != nil && s.app.Store != nil {
		result.StatePath = s.app.Store.Path()
		result.StateDir = filepath.Dir(result.StatePath)
	}
	if dir, dirErr := configpath.Dir(); dirErr == nil {
		result.ClientConfigDir = dir
	}
	service, serviceErr := gatewayservice.Inspect()
	result.Service = service
	if serviceErr != nil {
		result.ServiceError = serviceErr.Error()
	}
	result.Issues = gatewayDiagnosticIssues(result)
	return result, nil
}

func gatewayDiagnosticIssues(result GatewayDiagnostics) []GatewayDiagnosticIssue {
	issues := make([]GatewayDiagnosticIssue, 0)
	if !result.Readiness.Ready {
		missing := strings.Join(result.Readiness.Missing, ", ")
		if missing == "" {
			missing = "unknown configuration"
		}
		issues = append(issues, GatewayDiagnosticIssue{Code: "remote_access_not_ready", Detail: missing})
	}
	service := result.Service
	if !service.Supported || !service.Installed {
		return issues
	}
	if !service.Managed {
		issues = append(issues, GatewayDiagnosticIssue{
			Code:   "service_unmanaged",
			Detail: service.UnitPath,
		})
		return issues
	}
	if service.UnitVersion != gatewayservice.UnitVersion {
		issues = append(issues, GatewayDiagnosticIssue{
			Code:   "service_unit_outdated",
			Detail: fmt.Sprintf("installed=%d; current=%d", service.UnitVersion, gatewayservice.UnitVersion),
		})
	}
	if !service.Enabled {
		issues = append(issues, GatewayDiagnosticIssue{Code: "service_disabled"})
	}
	if !service.Active {
		issues = append(issues, GatewayDiagnosticIssue{Code: "service_inactive"})
	}
	if service.Active && service.PID > 0 && result.ProcessPID > 0 && service.PID != result.ProcessPID {
		issues = append(issues, GatewayDiagnosticIssue{
			Code:   "service_pid_mismatch",
			Detail: fmt.Sprintf("systemd=%d; process=%d", service.PID, result.ProcessPID),
		})
	}
	if strings.TrimSpace(service.User) != "" && strings.TrimSpace(result.ProcessUser) != "" &&
		!strings.EqualFold(strings.TrimSpace(service.User), strings.TrimSpace(result.ProcessUser)) {
		issues = append(issues, GatewayDiagnosticIssue{
			Code:   "service_user_mismatch",
			Detail: "service=" + service.User + "; process=" + result.ProcessUser,
		})
	}
	if strings.TrimSpace(service.StatePath) != "" && strings.TrimSpace(result.StatePath) != "" &&
		filepath.Clean(service.StatePath) != filepath.Clean(result.StatePath) {
		issues = append(issues, GatewayDiagnosticIssue{
			Code:   "service_state_path_mismatch",
			Detail: "service=" + service.StatePath + "; process=" + result.StatePath,
		})
	}
	if strings.TrimSpace(service.Executable) != "" && strings.TrimSpace(result.Executable) != "" &&
		filepath.Clean(service.Executable) != filepath.Clean(result.Executable) {
		issues = append(issues, GatewayDiagnosticIssue{
			Code:   "service_executable_mismatch",
			Detail: "service=" + service.Executable + "; process=" + result.Executable,
		})
	}
	return issues
}
