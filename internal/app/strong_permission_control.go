package app

import "ai-dev-manager-v2/internal/model"

type StrongPermissionControlStatus struct {
	Enabled           bool   `json:"enabled"`
	Mode              string `json:"mode"`
	Compatibility     string `json:"compatibility"`
	DefaultEnabled    bool   `json:"default_enabled"`
	RequiresReconnect bool   `json:"requires_reconnect"`
	Message           string `json:"message"`
}

func strongPermissionControlStatus(disabled bool) StrongPermissionControlStatus {
	if disabled {
		return StrongPermissionControlStatus{
			Enabled:           false,
			Mode:              "v1.4.4_compatible",
			Compatibility:     "v1.4.4",
			DefaultEnabled:    true,
			RequiresReconnect: false,
			Message:           "强权限控制已关闭；Agent 可以使用更多宿主机能力，开发操作更灵活但风险更高。工具仍全部可见，Admin、密钥和 Secret 等管理权限仍保持隔离。",
		}
	}
	return StrongPermissionControlStatus{
		Enabled:           true,
		Mode:              "strong",
		Compatibility:     "current",
		DefaultEnabled:    true,
		RequiresReconnect: false,
		Message:           "强权限控制已开启；Agent 工具全部可见，受限操作会被拦截并返回明确原因和操作建议。建议保持开启。",
	}
}

func (s *Service) StrongPermissionControlStatus() (StrongPermissionControlStatus, error) {
	state, err := s.Store.Load()
	if err != nil {
		return StrongPermissionControlStatus{}, err
	}
	return strongPermissionControlStatus(state.StrongPermissionControlDisabled), nil
}

func (s *Service) StrongPermissionControlEnabled() (bool, error) {
	status, err := s.StrongPermissionControlStatus()
	if err != nil {
		return true, err
	}
	return status.Enabled, nil
}

func (s *Service) SetStrongPermissionControl(enabled bool) (StrongPermissionControlStatus, error) {
	if err := s.Store.Update(func(state *model.State) error {
		state.StrongPermissionControlDisabled = !enabled
		return nil
	}); err != nil {
		return StrongPermissionControlStatus{}, err
	}
	return strongPermissionControlStatus(!enabled), nil
}
