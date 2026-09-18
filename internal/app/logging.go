package app

import "ai-dev-manager-v2/internal/logging"

func (s *Service) Log(level, event string, fields map[string]string) {
	if s == nil || s.Logger == nil {
		return
	}
	_ = s.Logger.Log(level, event, fields)
}

func (s *Service) LoggingStatus() logging.Status {
	if s == nil || s.Logger == nil {
		return logging.Status{}
	}
	return s.Logger.Status()
}
