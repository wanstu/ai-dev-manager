//go:build windows

package sandbox

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	userPrivUser      = 1
	ufScript          = 0x0001
	ufNormalAccount   = 0x0200
	ufDontExpirePasswd = 0x10000

	nerrSuccess       = 0
	errorAccessDenied = 5
	nerrUserExists    = 2224
	nerrUserNotFound  = 2221
)

var (
	netapi32      = windows.NewLazySystemDLL("netapi32.dll")
	procNetUserAdd = netapi32.NewProc("NetUserAdd")
	procNetUserDel = netapi32.NewProc("NetUserDel")
)

type userInfo1 struct {
	Name        *uint16
	Password    *uint16
	PasswordAge uint32
	Priv        uint32
	HomeDir     *uint16
	Comment     *uint16
	Flags       uint32
	ScriptPath  *uint16
}

type AccountError struct {
	Operation string
	Code      uint32
}

func (e *AccountError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s failed with Windows status %d", e.Operation, e.Code)
}

func (e *AccountError) Is(target error) bool {
	other, ok := target.(*AccountError)
	return ok && e.Code == other.Code
}

var (
	ErrSandboxAdminRequired = &AccountError{Code: errorAccessDenied}
	ErrSandboxAccountExists = &AccountError{Code: nerrUserExists}
	ErrSandboxAccountMissing = &AccountError{Code: nerrUserNotFound}
)

func CurrentWindowsUserSID() (string, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return "", fmt.Errorf("open current process token: %w", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("read current token user: %w", err)
	}
	if user == nil || user.User.Sid == nil {
		return "", fmt.Errorf("current token user SID is unavailable")
	}
	return user.User.Sid.String(), nil
}

func WindowsSandboxAccountName() (string, error) {
	sid, err := CurrentWindowsUserSID()
	if err != nil {
		return "", err
	}
	return StableWindowsSandboxUsername(sid)
}

func WindowsProcessElevated() bool {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

func LookupWindowsSandboxAccount(username string) (LocalAccount, error) {
	username = stringsTrimSpace(username)
	if username == "" {
		return LocalAccount{}, fmt.Errorf("sandbox username is required")
	}
	sid, _, accountType, err := windows.LookupSID("", username)
	if err != nil {
		if errors.Is(err, syscall.ERROR_NONE_MAPPED) {
			return LocalAccount{}, &AccountError{Operation: "LookupSID", Code: nerrUserNotFound}
		}
		return LocalAccount{}, fmt.Errorf("lookup sandbox account %q: %w", username, err)
	}
	if accountType != windows.SidTypeUser {
		return LocalAccount{}, fmt.Errorf("sandbox identity %q is not a user account", username)
	}
	return LocalAccount{Username: username, SID: sid.String()}, nil
}

func ProvisionWindowsSandboxAccount(username, password string) (LocalAccount, error) {
	username = stringsTrimSpace(username)
	if username == "" {
		return LocalAccount{}, fmt.Errorf("sandbox username is required")
	}
	if err := ValidateSandboxPassword(password); err != nil {
		return LocalAccount{}, err
	}
	namePtr, err := windows.UTF16PtrFromString(username)
	if err != nil {
		return LocalAccount{}, err
	}
	passwordPtr, err := windows.UTF16PtrFromString(password)
	if err != nil {
		return LocalAccount{}, err
	}
	commentPtr, err := windows.UTF16PtrFromString("AI Dev Manager sandbox account")
	if err != nil {
		return LocalAccount{}, err
	}
	info := userInfo1{
		Name:     namePtr,
		Password: passwordPtr,
		Priv:     userPrivUser,
		Comment:  commentPtr,
		Flags:    ufScript | ufNormalAccount | ufDontExpirePasswd,
	}
	var parameterError uint32
	status, _, callErr := procNetUserAdd.Call(
		0,
		1,
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Pointer(&parameterError)),
	)
	if uint32(status) != nerrSuccess {
		code := uint32(status)
		if code == 0 && callErr != nil && callErr != syscall.Errno(0) {
			return LocalAccount{}, fmt.Errorf("NetUserAdd: %w", callErr)
		}
		return LocalAccount{}, &AccountError{Operation: "NetUserAdd", Code: code}
	}
	account, err := LookupWindowsSandboxAccount(username)
	if err != nil {
		_ = DeleteWindowsSandboxAccount(username)
		return LocalAccount{}, err
	}
	return account, nil
}

func DeleteWindowsSandboxAccount(username string) error {
	username = stringsTrimSpace(username)
	if username == "" {
		return fmt.Errorf("sandbox username is required")
	}
	namePtr, err := windows.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	status, _, callErr := procNetUserDel.Call(0, uintptr(unsafe.Pointer(namePtr)))
	if uint32(status) == nerrSuccess || uint32(status) == nerrUserNotFound {
		return nil
	}
	if uint32(status) == 0 && callErr != nil && callErr != syscall.Errno(0) {
		return fmt.Errorf("NetUserDel: %w", callErr)
	}
	return &AccountError{Operation: "NetUserDel", Code: uint32(status)}
}

func stringsTrimSpace(value string) string {
	start, end := 0, len(value)
	for start < end {
		switch value[start] {
		case ' ', '\t', '\r', '\n':
			start++
		default:
			goto trimEnd
		}
	}
trimEnd:
	for end > start {
		switch value[end-1] {
		case ' ', '\t', '\r', '\n':
			end--
		default:
			return value[start:end]
		}
	}
	return value[start:end]
}
