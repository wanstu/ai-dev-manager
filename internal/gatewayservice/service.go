package gatewayservice

import (
	"fmt"
	"net"
	"path"
	"strconv"
	"strings"
)

const (
	UnitName    = "adm-gateway.service"
	UnitPath    = "/etc/systemd/system/" + UnitName
	UnitVersion = 1
)

type TargetUser struct {
	Name      string
	Home      string
	UID       int
	GID       int
	StatePath string
}

type InstallOptions struct {
	User       TargetUser
	Listen     string
	Executable string
	Start      bool
}

type Status struct {
	Supported   bool   `json:"supported"`
	Installed   bool   `json:"installed"`
	Active      bool   `json:"active"`
	Enabled     bool   `json:"enabled"`
	PID         int    `json:"pid,omitempty"`
	UnitPath    string `json:"unit_path,omitempty"`
	User        string `json:"user,omitempty"`
	Listen      string `json:"listen,omitempty"`
	Executable  string `json:"executable,omitempty"`
	StatePath   string `json:"state_path,omitempty"`
	Managed     bool   `json:"managed"`
	UnitVersion int    `json:"unit_version,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

func Supported() bool { return platformSupported() }

func CheckManagementPrivileges() error {
	return checkManagementPrivilegesPlatform()
}

func ResolveTargetUser(name string) (TargetUser, error) {
	return resolveTargetUser(name)
}

func FixStateOwnership(target TargetUser) error {
	return fixStateOwnership(target)
}

func Install(options InstallOptions) (Status, error) {
	if err := validateInstallOptions(options); err != nil {
		return Status{}, err
	}
	return installPlatform(options)
}

func Reconfigure(options InstallOptions) (Status, error) {
	if err := validateInstallOptions(options); err != nil {
		return Status{}, err
	}
	return reconfigurePlatform(options)
}

func Start() (Status, error) {
	return startPlatform()
}

func Restart() (Status, error) {
	return restartPlatform()
}

func Stop() (Status, error) {
	return stopPlatform()
}

func SetEnabled(enabled bool) (Status, error) {
	return setEnabledPlatform(enabled)
}

func Uninstall() error {
	return uninstallPlatform()
}

func Inspect() (Status, error) {
	return inspectPlatform()
}

func validateManagedServiceStatus(status Status) error {
	if !status.Installed {
		return fmt.Errorf("ADM Gateway service is not installed")
	}
	if !status.Managed {
		path := strings.TrimSpace(status.UnitPath)
		if path == "" {
			path = UnitPath
		}
		return fmt.Errorf("%s is not managed by ADM; refusing to modify it", path)
	}
	return nil
}

func validateInstallOptions(options InstallOptions) error {
	if strings.TrimSpace(options.User.Name) == "" {
		return fmt.Errorf("gateway service user is required")
	}
	if strings.TrimSpace(options.User.Home) == "" || !path.IsAbs(options.User.Home) {
		return fmt.Errorf("gateway service user home must be an absolute path")
	}
	if strings.TrimSpace(options.Executable) == "" || !path.IsAbs(options.Executable) {
		return fmt.Errorf("gateway service executable must be an absolute path")
	}
	if strings.TrimSpace(options.User.StatePath) == "" || !path.IsAbs(options.User.StatePath) {
		return fmt.Errorf("gateway service state path must be an absolute path")
	}
	if _, _, err := net.SplitHostPort(strings.TrimSpace(options.Listen)); err != nil {
		return fmt.Errorf("invalid gateway service listen %q: %w", options.Listen, err)
	}
	for name, value := range map[string]string{
		"user": options.User.Name, "home": options.User.Home,
		"state_path": options.User.StatePath, "executable": options.Executable, "listen": options.Listen,
	} {
		if strings.ContainsAny(value, "\r\n\x00") {
			return fmt.Errorf("gateway service %s contains an invalid control character", name)
		}
	}
	return nil
}

func RenderSystemdUnit(options InstallOptions) (string, error) {
	if err := validateInstallOptions(options); err != nil {
		return "", err
	}
	return strings.Join([]string{
		"# Managed by AI Dev Manager. Do not edit by hand.",
		"# ADM-Unit-Version: " + strconv.Itoa(UnitVersion),
		"# ADM-User: " + options.User.Name,
		"# ADM-Listen: " + options.Listen,
		"# ADM-Executable: " + options.Executable,
		"# ADM-StatePath: " + options.User.StatePath,
		"[Unit]",
		"Description=AI Dev Manager Gateway",
		"After=network-online.target",
		"Wants=network-online.target",
		"",
		"[Service]",
		"Type=simple",
		"User=" + options.User.Name,
		"Environment=" + systemdQuote("HOME="+options.User.Home),
		"Environment=" + systemdQuote("ADM_V2_HOME="+path.Dir(options.User.StatePath)),
		"WorkingDirectory=" + systemdQuote(options.User.Home),
		"ExecStart=" + systemdQuote(options.Executable) + " gateway start --listen " + systemdQuote(options.Listen),
		"Restart=on-failure",
		"RestartSec=2",
		"KillSignal=SIGTERM",
		"TimeoutStopSec=15",
		"UMask=0077",
		"",
		"[Install]",
		"WantedBy=multi-user.target",
		"",
	}, "\n"), nil
}

func systemdQuote(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "%", "%%")
	value = strings.ReplaceAll(value, "$", "$$")
	return "\"" + value + "\""
}

func parseManagedUnit(data string) Status {
	status := Status{Supported: true, Installed: true, Managed: true, UnitPath: UnitPath}
	for _, line := range strings.Split(data, "\n") {
		switch {
		case strings.HasPrefix(line, "# ADM-Unit-Version: "):
			status.UnitVersion, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "# ADM-Unit-Version: ")))
		case strings.HasPrefix(line, "# ADM-User: "):
			status.User = strings.TrimSpace(strings.TrimPrefix(line, "# ADM-User: "))
		case strings.HasPrefix(line, "# ADM-Listen: "):
			status.Listen = strings.TrimSpace(strings.TrimPrefix(line, "# ADM-Listen: "))
		case strings.HasPrefix(line, "# ADM-Executable: "):
			status.Executable = strings.TrimSpace(strings.TrimPrefix(line, "# ADM-Executable: "))
		case strings.HasPrefix(line, "# ADM-StatePath: "):
			status.StatePath = strings.TrimSpace(strings.TrimPrefix(line, "# ADM-StatePath: "))
		}
	}
	return status
}

func ParseUIDGID(uid, gid string) (int, int, error) {
	u, err := strconv.Atoi(uid)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid uid %q", uid)
	}
	g, err := strconv.Atoi(gid)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid gid %q", gid)
	}
	return u, g, nil
}
