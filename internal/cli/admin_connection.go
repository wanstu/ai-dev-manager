package cli

import (
	"fmt"
	"os"
	"strings"

	"ai-dev-manager-v2/internal/adminmcp"
	"ai-dev-manager-v2/internal/gateway"
)

const admBaseURLEnv = "ADM_V2_URL"

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
	return adminmcp.New(target.AdminMCPURL), nil
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
