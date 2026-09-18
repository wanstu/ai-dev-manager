package desktop

import (
	"runtime"

	productversion "ai-dev-manager-v2/internal/version"
)

type AboutInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
}

func (a *Adapter) GetAboutInfo() AboutInfo {
	return AboutInfo{
		Name:      "AI Dev Manager",
		Version:   productversion.Current(),
		GoVersion: runtime.Version(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
	}
}
