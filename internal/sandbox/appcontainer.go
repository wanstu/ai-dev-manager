package sandbox

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

const appContainerProfilePrefix = "AI.Dev.Manager.Sandbox."

type AppContainerProfile struct {
	Name string `json:"name"`
	SID  string `json:"sid"`
}

func StableAppContainerProfileName(scope string) (string, error) {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return "", fmt.Errorf("sandbox scope is required")
	}
	sum := sha256.Sum256([]byte(scope))
	return appContainerProfilePrefix + fmt.Sprintf("%x", sum[:10]), nil
}
