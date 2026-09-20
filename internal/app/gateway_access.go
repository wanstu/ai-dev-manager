package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"strings"

	"ai-dev-manager-v2/internal/model"
)

const (
	gatewayAPIKeyHashPrefix = "sha256:"
	gatewayAPIKeyBytes      = 32
)

type GatewayAccessStatus struct {
	AllowedHosts          []string `json:"allowed_hosts"`
	AdminAPIKeyConfigured bool     `json:"admin_api_key_configured"`
	AgentAPIKeyConfigured bool     `json:"agent_api_key_configured"`
}

type GatewayReadiness struct {
	HostPolicyConfigured  bool     `json:"host_policy_configured"`
	AdminAPIKeyConfigured bool     `json:"admin_api_key_configured"`
	AgentAPIKeyConfigured bool     `json:"agent_api_key_configured"`
	Ready                 bool     `json:"ready"`
	Missing               []string `json:"missing,omitempty"`
}

type GatewayKeyRotationResult struct {
	Status      GatewayAccessStatus `json:"status"`
	AdminAPIKey string              `json:"admin_api_key,omitempty"`
	AgentAPIKey string              `json:"agent_api_key,omitempty"`
}

type GatewaySetupResult struct {
	Status            GatewayAccessStatus `json:"status"`
	Readiness         GatewayReadiness    `json:"readiness"`
	AdminAPIKey       string              `json:"admin_api_key,omitempty"`
	AgentAPIKey       string              `json:"agent_api_key,omitempty"`
	AdminKeyGenerated bool                `json:"admin_key_generated"`
	AgentKeyGenerated bool                `json:"agent_key_generated"`
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

func (s *Service) GatewayRemoteReadiness() (GatewayReadiness, error) {
	status, err := s.GatewayAccessStatus()
	if err != nil {
		return GatewayReadiness{}, err
	}
	return gatewayReadiness(status), nil
}

func gatewayReadiness(status GatewayAccessStatus) GatewayReadiness {
	result := GatewayReadiness{
		HostPolicyConfigured:  len(status.AllowedHosts) > 0,
		AdminAPIKeyConfigured: status.AdminAPIKeyConfigured,
		AgentAPIKeyConfigured: status.AgentAPIKeyConfigured,
	}
	if !result.HostPolicyConfigured {
		result.Missing = append(result.Missing, "Host policy")
	}
	if !result.AdminAPIKeyConfigured {
		result.Missing = append(result.Missing, "Admin API Key")
	}
	if !result.AgentAPIKeyConfigured {
		result.Missing = append(result.Missing, "Agent API Key")
	}
	result.Ready = len(result.Missing) == 0
	return result
}

func normalizeGatewayAllowedHosts(hosts []string) ([]string, error) {
	normalized := make([]string, 0, len(hosts))
	seen := map[string]struct{}{}
	for _, raw := range hosts {
		host, err := NormalizeGatewayAllowedHost(raw)
		if err != nil {
			return nil, err
		}
		if host == "" {
			continue
		}
		if host == "*" {
			return []string{"*"}, nil
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		normalized = append(normalized, host)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func (s *Service) SetGatewayAllowedHosts(hosts []string) (GatewayAccessStatus, error) {
	normalized, err := normalizeGatewayAllowedHosts(hosts)
	if err != nil {
		return GatewayAccessStatus{}, err
	}
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

func (s *Service) RotateGatewayAdminAPIKey() (GatewayKeyRotationResult, error) {
	key, err := GenerateGatewayAPIKey()
	if err != nil {
		return GatewayKeyRotationResult{}, err
	}
	status, err := s.SetGatewayAdminAPIKey(key)
	if err != nil {
		return GatewayKeyRotationResult{}, err
	}
	return GatewayKeyRotationResult{Status: status, AdminAPIKey: key}, nil
}

func (s *Service) RotateGatewayAgentAPIKey() (GatewayKeyRotationResult, error) {
	key, err := GenerateGatewayAPIKey()
	if err != nil {
		return GatewayKeyRotationResult{}, err
	}
	status, err := s.SetGatewayAgentAPIKey(key)
	if err != nil {
		return GatewayKeyRotationResult{}, err
	}
	return GatewayKeyRotationResult{Status: status, AgentAPIKey: key}, nil
}

func (s *Service) RotateGatewayAPIKeys() (GatewayKeyRotationResult, error) {
	adminKey, err := GenerateGatewayAPIKey()
	if err != nil {
		return GatewayKeyRotationResult{}, err
	}
	agentKey, err := GenerateGatewayAPIKey()
	if err != nil {
		return GatewayKeyRotationResult{}, err
	}
	adminHash := hashGatewayAPIKey(adminKey)
	agentHash := hashGatewayAPIKey(agentKey)
	if err := s.Store.Update(func(state *model.State) error {
		state.GatewayAccess.AdminAPIKeyHash = adminHash
		state.GatewayAccess.AgentAPIKeyHash = agentHash
		return nil
	}); err != nil {
		return GatewayKeyRotationResult{}, err
	}
	status, err := s.GatewayAccessStatus()
	if err != nil {
		return GatewayKeyRotationResult{}, err
	}
	return GatewayKeyRotationResult{Status: status, AdminAPIKey: adminKey, AgentAPIKey: agentKey}, nil
}

func (s *Service) SetupGatewayRemote(hosts []string, rotateKeys bool) (GatewaySetupResult, error) {
	settings, err := s.GatewayAccessConfig()
	if err != nil {
		return GatewaySetupResult{}, err
	}
	if len(hosts) == 0 {
		if len(settings.AllowedHosts) > 0 {
			hosts = append([]string(nil), settings.AllowedHosts...)
		} else {
			hosts = []string{"*"}
		}
	}
	normalized, err := normalizeGatewayAllowedHosts(hosts)
	if err != nil {
		return GatewaySetupResult{}, err
	}
	var adminKey, agentKey string
	generateAdmin := rotateKeys || strings.TrimSpace(settings.AdminAPIKeyHash) == ""
	generateAgent := rotateKeys || strings.TrimSpace(settings.AgentAPIKeyHash) == ""
	if generateAdmin {
		adminKey, err = GenerateGatewayAPIKey()
		if err != nil {
			return GatewaySetupResult{}, err
		}
	}
	if generateAgent {
		agentKey, err = GenerateGatewayAPIKey()
		if err != nil {
			return GatewaySetupResult{}, err
		}
	}
	if err := s.Store.Update(func(state *model.State) error {
		state.GatewayAccess.AllowedHosts = append([]string(nil), normalized...)
		if generateAdmin {
			state.GatewayAccess.AdminAPIKeyHash = hashGatewayAPIKey(adminKey)
		}
		if generateAgent {
			state.GatewayAccess.AgentAPIKeyHash = hashGatewayAPIKey(agentKey)
		}
		return nil
	}); err != nil {
		return GatewaySetupResult{}, err
	}
	status, err := s.GatewayAccessStatus()
	if err != nil {
		return GatewaySetupResult{}, err
	}
	return GatewaySetupResult{
		Status: status, Readiness: gatewayReadiness(status),
		AdminAPIKey: adminKey, AgentAPIKey: agentKey,
		AdminKeyGenerated: generateAdmin, AgentKeyGenerated: generateAgent,
	}, nil
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

// GenerateGatewayAPIKey returns a 256-bit cryptographically secure key encoded
// as lowercase hex. Its format is equivalent to: openssl rand -hex 32.
func GenerateGatewayAPIKey() (string, error) {
	value := make([]byte, gatewayAPIKeyBytes)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate ADM API key: %w", err)
	}
	return hex.EncodeToString(value), nil
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
