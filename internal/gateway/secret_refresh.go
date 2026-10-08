package gateway

import (
	"strings"

	"ai-dev-manager-v2/internal/app"
)

// A secret rotation must not leave a long-lived MCP session authenticated
// using the previous credential. Only affected MCP sessions are dropped.
func dropMCPsUsingSecret(service *app.Service, owner *runtimeOwner, name string) {
	if owner == nil {
		return
	}
	defs, err := service.MCPs.List()
	if err != nil {
		return
	}
	marker := "${secret:" + name + "}"
	for _, def := range defs {
		referenced := false
		for _, v := range def.HeaderRefs {
			if strings.Contains(v, marker) {
				referenced = true
				break
			}
		}
		for _, v := range def.EnvRefs {
			if strings.Contains(v, marker) {
				referenced = true
				break
			}
		}
		if referenced {
			owner.DropMCP(def.ID)
		}
	}
}
