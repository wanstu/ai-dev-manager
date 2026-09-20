package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"ai-dev-manager-v2/internal/adminmcp"
	"ai-dev-manager-v2/internal/gateway"
)

const admBaseURLEnv = "ADM_V2_URL"
const admAdminAPIKeyEnv = "ADM_ADMIN_API_KEY"
const admLegacyAdminAPIKeyEnv = "ADM_V2_ADMIN_API_KEY"

func defaultADMBaseURL() string {
	baseURL, err := gateway.HTTPBaseURL(gateway.DefaultHTTPListen)
	if err != nil {
		return "http://127.0.0.1:43137"
	}
	return baseURL
}

func parseADMTarget(args []string) (string, []string, error) {
	baseURL := strings.TrimSpace(os.Getenv(admBaseURLEnv))
	if baseURL == "" {
		baseURL = defaultADMBaseURL()
	}
	if len(args) == 0 {
		return baseURL, args, nil
	}

	filtered := make([]string, 0, len(args))
	seen := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--adm-url":
			if seen {
				return "", nil, fmt.Errorf("--adm-url may only be provided once")
			}
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
				return "", nil, fmt.Errorf("--adm-url requires an ADM base URL")
			}
			baseURL = strings.TrimSpace(args[i+1])
			seen = true
			i++
		case strings.HasPrefix(arg, "--adm-url="):
			if seen {
				return "", nil, fmt.Errorf("--adm-url may only be provided once")
			}
			baseURL = strings.TrimSpace(strings.TrimPrefix(arg, "--adm-url="))
			if baseURL == "" {
				return "", nil, fmt.Errorf("--adm-url requires an ADM base URL")
			}
			seen = true
		default:
			filtered = append(filtered, arg)
		}
	}
	return baseURL, filtered, nil
}

func localGatewayListenFromBaseURL(raw string) (string, error) {
	return gateway.LocalHTTPListenFromBaseURL(raw)
}

func newCLIAdminClient(baseURL string) (*adminmcp.Client, error) {
	target, err := gateway.ResolveHTTPTarget(baseURL)
	if err != nil {
		return nil, err
	}
	client := adminmcp.NewWithAPIKey(target.AdminMCPURL, clientAdminAPIKey())
	return client.WithBeforeCall(func(context.Context) error {
		status, err := gateway.InspectHTTPBaseURL(target.BaseURL)
		if err != nil {
			return nil
		}
		if status.State == gateway.HTTPStateIncompatible && status.RecognizedADMGateway {
			return fmt.Errorf("ADM Gateway %s is incompatible and cannot be managed: %s", status.BaseURL, status.Detail)
		}
		return nil
	}), nil
}

func normalizeCLIADMBaseURL(raw string) (string, error) {
	target, err := gateway.ResolveHTTPTarget(raw)
	if err != nil {
		return "", err
	}
	return target.BaseURL, nil
}

func withCLIAdmin(baseURL string, fn func(cliManagementBackend) error) error {
	client, err := newCLIAdminClient(baseURL)
	if err != nil {
		return err
	}
	return fn(client)
}

func clientAdminAPIKey() string {
	if value := strings.TrimSpace(os.Getenv(admAdminAPIKeyEnv)); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv(admLegacyAdminAPIKeyEnv))
}
