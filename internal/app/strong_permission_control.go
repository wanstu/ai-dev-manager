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
			Message:           "强权限控制已关闭；Agent 使用 v1.4.4 兼容的宽松权限边界。工具仍全部可见，v1.4.4 中原本属于 Admin 的管理工具仍需要 Admin 权限。",
		}
	}
	return StrongPermissionControlStatus{
		Enabled:           true,
		Mode:              "strong",
		Compatibility:     "current",
		DefaultEnabled:    true,
		RequiresReconnect: false,
		Message:           "强权限控制已开启；Agent 工具全部可见，但受限工具会在调用时返回明确权限原因和操作建议。不会自动降级到宽松模式。",
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
