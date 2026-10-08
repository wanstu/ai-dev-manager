package app

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"ai-dev-manager-v2/internal/model"
)

// GatewayWhitelistEnabled preserves restrictive Host rules stored by older ADM versions.
func GatewayWhitelistEnabled(settings model.GatewayAccessSettings) bool {
	if settings.WhitelistEnabled != nil {
		return *settings.WhitelistEnabled
	}
	return len(settings.AllowedHosts) > 0
}

func normalizeGatewayAllowedClientIPs(input []string) ([]string, error) {
	normalized := make([]string, 0, len(input))
	seen := make(map[string]struct{})
	for _, item := range input {
		raw := strings.TrimSpace(item)
		if raw == "" {
			continue
		}
		var value string
		if prefix, err := netip.ParsePrefix(raw); err == nil && prefix.Addr().Zone() == "" {
			value = prefix.Masked().String()
		} else if addr, err := netip.ParseAddr(raw); err == nil && addr.Zone() == "" {
			value = addr.Unmap().String()
		} else {
			return nil, fmt.Errorf("无效的客户端来源 IP 或 CIDR：%q", raw)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized, nil
}

// SetGatewayAccessPolicy updates both allowlists and their switch atomically.
func (s *Service) SetGatewayAccessPolicy(enabled bool, hosts, clientIPs []string) (GatewayAccessStatus, error) {
	normalizedHosts, err := normalizeGatewayAllowedHosts(hosts)
	if err != nil {
		return GatewayAccessStatus{}, err
	}
	normalizedIPs, err := normalizeGatewayAllowedClientIPs(clientIPs)
	if err != nil {
		return GatewayAccessStatus{}, err
	}
	if enabled && (len(normalizedHosts) == 0 || (len(normalizedHosts) == 1 && normalizedHosts[0] == "*")) && len(normalizedIPs) == 0 {
		return GatewayAccessStatus{}, fmt.Errorf("启用访问白名单前，请至少填写一个域名/目标 IP 或客户端来源 IP")
	}
	if err := s.Store.Update(func(state *model.State) error {
		state.GatewayAccess.AllowedHosts = append([]string(nil), normalizedHosts...)
		state.GatewayAccess.AllowedClientIPs = append([]string(nil), normalizedIPs...)
		state.GatewayAccess.WhitelistEnabled = &enabled
		return nil
	}); err != nil {
		return GatewayAccessStatus{}, err
	}
	return s.GatewayAccessStatus()
}
