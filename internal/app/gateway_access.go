package app

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"strings"

	"ai-dev-manager-v2/internal/model"
)

const gatewayAPIKeyHashPrefix = "sha256:"

type GatewayAccessStatus struct {
	AllowedHosts          []string `json:"allowed_hosts"`
	AdminAPIKeyConfigured bool     `json:"admin_api_key_configured"`
	AgentAPIKeyConfigured bool     `json:"agent_api_key_configured"`
}

func (s *Service) GatewayAccessConfig() (model.GatewayAccessSettings, error) {
	state, err := s.Store.Load()
	if err != nil {
		return model.GatewayAccessSettings{}, err
	}
	settings := state.GatewayAccess
	settings.AllowedHosts = append([]string(nil), settings.AllowedHosts...)
	return settings, nil
}

func (s *Service) GatewayAccessStatus() (GatewayAccessStatus, error) {
	settings, err := s.GatewayAccessConfig()
	if err != nil {
		return GatewayAccessStatus{}, err
	}
	return gatewayAccessStatus(settings), nil
}

func (s *Service) SetGatewayAllowedHosts(hosts []string) (GatewayAccessStatus, error) {
	normalized := make([]string, 0, len(hosts))
	seen := map[string]struct{}{}
	for _, raw := range hosts {
		host, err := NormalizeGatewayAllowedHost(raw)
		if err != nil {
			return GatewayAccessStatus{}, err
		}
		if host == "" {
			continue
		}
		if host == "*" {
			normalized = []string{"*"}
			break
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		normalized = append(normalized, host)
	}
	sort.Strings(normalized)
	if err := s.Store.Update(func(state *model.State) error {
		state.GatewayAccess.AllowedHosts = append([]string(nil), normalized...)
		return nil
	}); err != nil {
		return GatewayAccessStatus{}, err
	}
	return s.GatewayAccessStatus()
}

func (s *Service) SetGatewayAdminAPIKey(secret string) (GatewayAccessStatus, error) {
	hash, err := validateAndHashGatewayAPIKey(secret)
	if err != nil {
		return GatewayAccessStatus{}, err
	}
	if err := s.Store.Update(func(state *model.State) error {
		if strings.TrimSpace(state.GatewayAccess.AgentAPIKeyHash) != "" && state.GatewayAccess.AgentAPIKeyHash == hash {
			return fmt.Errorf("Admin API key must be different from Agent API key")
		}
		state.GatewayAccess.AdminAPIKeyHash = hash
		return nil
	}); err != nil {
		return GatewayAccessStatus{}, err
	}
	return s.GatewayAccessStatus()
}

func (s *Service) SetGatewayAgentAPIKey(secret string) (GatewayAccessStatus, error) {
	hash, err := validateAndHashGatewayAPIKey(secret)
	if err != nil {
		return GatewayAccessStatus{}, err
	}
	if err := s.Store.Update(func(state *model.State) error {
		if strings.TrimSpace(state.GatewayAccess.AdminAPIKeyHash) != "" && state.GatewayAccess.AdminAPIKeyHash == hash {
			return fmt.Errorf("Agent API key must be different from Admin API key")
		}
		state.GatewayAccess.AgentAPIKeyHash = hash
		return nil
	}); err != nil {
		return GatewayAccessStatus{}, err
	}
	return s.GatewayAccessStatus()
}

func (s *Service) ClearGatewayAdminAPIKey() (GatewayAccessStatus, error) {
	if err := s.Store.Update(func(state *model.State) error {
		state.GatewayAccess.AdminAPIKeyHash = ""
		return nil
	}); err != nil {
		return GatewayAccessStatus{}, err
	}
	return s.GatewayAccessStatus()
}

func (s *Service) ClearGatewayAgentAPIKey() (GatewayAccessStatus, error) {
	if err := s.Store.Update(func(state *model.State) error {
		state.GatewayAccess.AgentAPIKeyHash = ""
		return nil
	}); err != nil {
		return GatewayAccessStatus{}, err
	}
	return s.GatewayAccessStatus()
}

func (s *Service) VerifyGatewayAdminAPIKey(secret string) (bool, error) {
	settings, err := s.GatewayAccessConfig()
	if err != nil {
		return false, err
	}
	return verifyGatewayAPIKeyHash(settings.AdminAPIKeyHash, secret), nil
}

func (s *Service) VerifyGatewayAgentAPIKey(secret string) (bool, error) {
	settings, err := s.GatewayAccessConfig()
	if err != nil {
		return false, err
	}
	return verifyGatewayAPIKeyHash(settings.AgentAPIKeyHash, secret), nil
}

func validateAndHashGatewayAPIKey(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 16 {
		return "", fmt.Errorf("ADM API key must be at least 16 characters")
	}
	return hashGatewayAPIKey(secret), nil
}

func verifyGatewayAPIKeyHash(expectedHash, secret string) bool {
	expectedHash = strings.TrimSpace(expectedHash)
	if expectedHash == "" {
		return false
	}
	provided := hashGatewayAPIKey(strings.TrimSpace(secret))
	if len(provided) != len(expectedHash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expectedHash)) == 1
}

func hashGatewayAPIKey(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return gatewayAPIKeyHashPrefix + hex.EncodeToString(sum[:])
}

func gatewayAccessStatus(settings model.GatewayAccessSettings) GatewayAccessStatus {
	return GatewayAccessStatus{
		AllowedHosts:          append([]string(nil), settings.AllowedHosts...),
		AdminAPIKeyConfigured: strings.TrimSpace(settings.AdminAPIKeyHash) != "",
		AgentAPIKeyConfigured: strings.TrimSpace(settings.AgentAPIKeyHash) != "",
	}
}

func NormalizeGatewayAllowedHost(raw string) (string, error) {
	host := strings.TrimSpace(strings.ToLower(raw))
	if host == "" {
		return "", nil
	}
	if host == "*" {
		return "*", nil
	}
	if strings.Contains(host, "://") || strings.ContainsAny(host, "/\\?#@") {
		return "", fmt.Errorf("invalid gateway allowed host %q; enter only an IP or DNS name", raw)
	}
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(strings.TrimSuffix(host, "."), "[]")
	if host == "" {
		return "", fmt.Errorf("invalid gateway allowed host %q", raw)
	}
	if net.ParseIP(host) != nil {
		return host, nil
	}
	if len(host) > 253 {
		return "", fmt.Errorf("invalid gateway allowed host %q", raw)
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid gateway allowed host %q", raw)
		}
		for _, ch := range label {
			if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
				continue
			}
			return "", fmt.Errorf("invalid gateway allowed host %q", raw)
		}
	}
	return host, nil
}
