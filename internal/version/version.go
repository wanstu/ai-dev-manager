package version

import (
	"runtime/debug"
	"strings"
)

// Version is the single linker-overridable ADM product version.
// Release builds set it with:
//
//	-ldflags "-X ai-dev-manager-v2/internal/version.Version=vX.Y.Z"
//
// Ordinary source builds remain explicitly identified as development builds.
var Version = "dev"

// Current returns the product version embedded into this binary. When a binary
// was installed as a versioned Go module without linker injection, the module
// version is used as a fallback. Local source builds report "dev".
func Current() string {
	if value := strings.TrimSpace(Version); value != "" && value != "dev" {
		return value
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if value := strings.TrimSpace(info.Main.Version); value != "" && value != "(devel)" {
			return value
		}
	}
	return "dev"
}
