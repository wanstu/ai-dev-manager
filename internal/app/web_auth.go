package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/model"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrWebAdminAlreadyInitialized = errors.New("web admin is already initialized")
	ErrWebAdminNotInitialized     = errors.New("web admin is not initialized")
	ErrWebInvalidCredentials      = errors.New("invalid web admin credentials")
)

type WebAdminStatus struct {
	Initialized bool   `json:"initialized"`
	Username    string `json:"username,omitempty"`
}

func (s *Service) WebAdminStatus() (WebAdminStatus, error) {
	state, err := s.Store.Load()
	if err != nil {
		return WebAdminStatus{}, err
	}
	if state.WebAccess.Admin == nil {
		return WebAdminStatus{}, nil
	}
	return WebAdminStatus{Initialized: true, Username: state.WebAccess.Admin.Username}, nil
}

func (s *Service) InitializeWebAdmin(username, password string) (WebAdminStatus, error) {
	username = strings.TrimSpace(username)
	if err := validateWebAdminCredentials(username, password); err != nil {
		return WebAdminStatus{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return WebAdminStatus{}, fmt.Errorf("hash web admin password: %w", err)
	}
	now := time.Now().UTC()
	err = s.Store.Update(func(state *model.State) error {
		if state.WebAccess.Admin != nil {
			return ErrWebAdminAlreadyInitialized
		}
		state.WebAccess.Admin = &model.WebAdminAccount{
			Username:     username,
			PasswordHash: string(hash),
			CreatedAt:    now,
		}
		return nil
	})
	if err != nil {
		return WebAdminStatus{}, err
	}
	return WebAdminStatus{Initialized: true, Username: username}, nil
}

func (s *Service) AuthenticateWebAdmin(username, password string) (WebAdminStatus, error) {
	username = strings.TrimSpace(username)
	state, err := s.Store.Load()
	if err != nil {
		return WebAdminStatus{}, err
	}
	admin := state.WebAccess.Admin
	if admin == nil {
		return WebAdminStatus{}, ErrWebAdminNotInitialized
	}
	if !strings.EqualFold(admin.Username, username) ||
		bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return WebAdminStatus{}, ErrWebInvalidCredentials
	}
	return WebAdminStatus{Initialized: true, Username: admin.Username}, nil
}

func validateWebAdminCredentials(username, password string) error {
	if len(username) < 3 || len(username) > 64 {
		return fmt.Errorf("username must contain 3 to 64 characters")
	}
	if strings.ContainsAny(username, "\r\n\t") {
		return fmt.Errorf("username contains invalid whitespace")
	}
	if len(password) < 8 {
		return fmt.Errorf("password must contain at least 8 characters")
	}
	if len(password) > 1024 {
		return fmt.Errorf("password is too long")
	}
	return nil
}
