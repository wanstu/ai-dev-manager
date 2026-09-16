package gateway

import (
	"strings"

	"ai-dev-manager-v2/internal/app"
)

func appIsExecutableNotAllowedError(_ *app.Service, err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "executable") && strings.Contains(message, "not allowed")
}
