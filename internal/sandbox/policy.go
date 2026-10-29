package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type RuntimeDirs struct {
	Root        string `json:"root"`
	ProfileRoot string `json:"profile_root"`
	TempRoot    string `json:"temp_root"`
	CacheRoot   string `json:"cache_root"`
	BuildRoot   string `json:"build_root"`
}

type Support struct {
	Platform                    string   `json:"platform"`
	Supported                   bool     `json:"supported"`
	EnvironmentSanitization     bool     `json:"environment_sanitization"`
	ProcessTreeBoundary         bool     `json:"process_tree_boundary"`
	DedicatedIdentitySupported  bool     `json:"dedicated_identity_supported"`
	StrongFileBoundarySupported bool     `json:"strong_file_boundary_supported"`
	RequiresAdminSetup          bool     `json:"requires_admin_setup"`
	Notes                       []string `json:"notes,omitempty"`
}

var inheritedEnvironmentKeys = map[string]struct{}{
	"PATH":                   {},
	"PATHEXT":                {},
	"SYSTEMROOT":             {},
	"WINDIR":                 {},
	"COMSPEC":                {},
	"OS":                     {},
	"PROCESSOR_ARCHITECTURE": {},
	"PROCESSOR_IDENTIFIER":   {},
	"PROCESSOR_LEVEL":        {},
	"PROCESSOR_REVISION":     {},
	"NUMBER_OF_PROCESSORS":   {},
	"TERM":                   {},
	"LANG":                   {},
	"LC_ALL":                 {},
	"GOROOT":                 {},
	"GOTOOLCHAIN":            {},
	"CGO_ENABLED":            {},
	"CC":                     {},
	"CXX":                    {},
	"JAVA_HOME":              {},
	"MAVEN_HOME":             {},
	"GRADLE_HOME":            {},
	"DOTNET_ROOT":            {},
	"PKG_CONFIG_PATH":        {},
}

// DefaultRuntimeDirs returns the private directory layout used by one sandbox
// execution identity. Callers create these directories before process start.
func DefaultRuntimeDirs(root string) RuntimeDirs {
	root = filepath.Clean(root)
	return RuntimeDirs{
		Root:        root,
		ProfileRoot: filepath.Join(root, "profile"),
		TempRoot:    filepath.Join(root, "tmp"),
		CacheRoot:   filepath.Join(root, "cache"),
		BuildRoot:   filepath.Join(root, "build"),
	}
}

func (d RuntimeDirs) Validate() error {
	if strings.TrimSpace(d.Root) == "" || !filepath.IsAbs(d.Root) {
		return fmt.Errorf("sandbox runtime root must be absolute")
	}
	for name, value := range map[string]string{
		"profile_root": d.ProfileRoot,
		"temp_root":    d.TempRoot,
		"build_root":   d.BuildRoot,
	} {
		if strings.TrimSpace(value) == "" || !filepath.IsAbs(value) {
			return fmt.Errorf("sandbox %s must be absolute", name)
		}
		if !pathWithin(d.Root, value) {
			return fmt.Errorf("sandbox %s must stay within runtime root", name)
		}
	}
	if strings.TrimSpace(d.CacheRoot) == "" || !filepath.IsAbs(d.CacheRoot) {
		return fmt.Errorf("sandbox cache_root must be absolute")
	}
	if !pathWithin(d.Root, d.CacheRoot) {
		environmentRuntimeRoot := filepath.Dir(d.Root)
		if !pathWithin(environmentRuntimeRoot, d.CacheRoot) || pathWithin(d.Root, d.CacheRoot) {
			return fmt.Errorf("sandbox cache_root must stay within the Environment sandbox root")
		}
	}
	return nil
}

// BuildEnvironment constructs a deliberately small process environment.
// It does not inherit arbitrary user variables, which prevents credentials
// such as cloud tokens, SSH agent sockets, database passwords, and shell
// injection variables from crossing into sandboxed project code.
//
// extra is trusted ADM-owned configuration, not Agent-provided input.
func BuildEnvironment(base []string, dirs RuntimeDirs, extra map[string]string) ([]string, error) {
	if err := dirs.Validate(); err != nil {
		return nil, err
	}

	values := make(map[string]string)
	displayKeys := make(map[string]string)
	for _, item := range base {
		key, value, ok := splitEnvironment(item)
		if !ok {
			continue
		}
		normalized := strings.ToUpper(key)
		if _, allowed := inheritedEnvironmentKeys[normalized]; !allowed {
			continue
		}
		values[normalized] = value
		displayKeys[normalized] = canonicalEnvironmentKey(normalized)
	}

	set := func(key, value string) {
		normalized := strings.ToUpper(strings.TrimSpace(key))
		if normalized == "" {
			return
		}
		values[normalized] = value
		displayKeys[normalized] = canonicalEnvironmentKey(normalized)
	}

	set("USERPROFILE", dirs.ProfileRoot)
	set("HOME", dirs.ProfileRoot)
	set("APPDATA", filepath.Join(dirs.ProfileRoot, "AppData", "Roaming"))
	set("LOCALAPPDATA", filepath.Join(dirs.ProfileRoot, "AppData", "Local"))
	set("TEMP", dirs.TempRoot)
	set("TMP", dirs.TempRoot)

	set("GOCACHE", filepath.Join(dirs.CacheRoot, "go-build"))
	set("GOMODCACHE", filepath.Join(dirs.CacheRoot, "go-mod"))
	set("GOPATH", filepath.Join(dirs.CacheRoot, "go-path"))
	set("NPM_CONFIG_CACHE", filepath.Join(dirs.CacheRoot, "npm"))
	set("BUN_INSTALL_CACHE_DIR", filepath.Join(dirs.CacheRoot, "bun"))
	set("CARGO_HOME", filepath.Join(dirs.CacheRoot, "cargo"))
	set("GRADLE_USER_HOME", filepath.Join(dirs.CacheRoot, "gradle"))
	set("XDG_CACHE_HOME", filepath.Join(dirs.CacheRoot, "xdg"))
	set("XDG_CONFIG_HOME", filepath.Join(dirs.ProfileRoot, ".config"))

	// Git must not load the interactive user's global configuration, helpers,
	// hooks, or credential setup from their real profile.
	set("GIT_CONFIG_NOSYSTEM", "1")
	set("GIT_CONFIG_GLOBAL", os.DevNull)
	set("GIT_TERMINAL_PROMPT", "0")

	for key, value := range extra {
		set(key, value)
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, displayKeys[key]+"="+values[key])
	}
	return out, nil
}

func splitEnvironment(item string) (key, value string, ok bool) {
	index := strings.IndexByte(item, '=')
	if index <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(item[:index]), item[index+1:], true
}

func canonicalEnvironmentKey(key string) string {
	switch strings.ToUpper(key) {
	case "NPM_CONFIG_CACHE":
		return "NPM_CONFIG_CACHE"
	default:
		return strings.ToUpper(key)
	}
}

func pathWithin(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
