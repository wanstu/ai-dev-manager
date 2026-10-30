//go:build windows

package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"ai-dev-manager-v2/internal/sandbox"

	"golang.org/x/sys/windows"
)

const (
	sandboxACLPipeName        = `\\.\pipe\AI-Dev-Manager-Sandbox-ACL-v7`
	sandboxACLOpPing          = "ping"
	sandboxACLOpPrepare       = "prepare"
	sandboxACLOpCleanup       = "cleanup"
	sandboxACLMaxClients      = 16
	sandboxACLMaxRequestBytes = 64 * 1024
)

var errSandboxACLBrokerUnavailable = errors.New("sandbox ACL broker is unavailable")

type sandboxACLBrokerRequest struct {
	Operation       string `json:"operation"`
	EnvironmentID   string `json:"environment_id,omitempty"`
	ProfileSID      string `json:"profile_sid,omitempty"`
	WorkspaceRoot   string `json:"workspace_root,omitempty"`
	EnvironmentRoot string `json:"environment_root,omitempty"`
}

type sandboxACLBrokerResponse struct {
	Status sandbox.EnvironmentAccessStatus `json:"status,omitempty"`
	Error  string                          `json:"error,omitempty"`
}

func RunSandboxACLBroker() error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return fmt.Errorf("sandbox ACL broker requires an elevated SYSTEM/admin token")
	}
	allowedSID, err := sandboxHostPrepInstalledUserSID()
	if err != nil {
		return fmt.Errorf("resolve sandbox ACL broker user: %w", err)
	}
	sd, err := windows.SecurityDescriptorFromString(
		fmt.Sprintf("D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;%s)", allowedSID),
	)
	if err != nil {
		return fmt.Errorf("build sandbox ACL broker pipe security descriptor: %w", err)
	}
	sa := windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}
	name, err := windows.UTF16PtrFromString(sandboxACLPipeName)
	if err != nil {
		return err
	}

	clientSlots := make(chan struct{}, sandboxACLMaxClients)
	for {
		pipe, err := windows.CreateNamedPipe(
			name,
			windows.PIPE_ACCESS_DUPLEX,
			windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
			windows.PIPE_UNLIMITED_INSTANCES,
			64*1024,
			64*1024,
			5000,
			&sa,
		)
		if err != nil {
			return fmt.Errorf("create sandbox ACL broker pipe: %w", err)
		}
		connectErr := windows.ConnectNamedPipe(pipe, nil)
		if connectErr != nil && !errors.Is(connectErr, windows.ERROR_PIPE_CONNECTED) {
			_ = windows.CloseHandle(pipe)
			return fmt.Errorf("connect sandbox ACL broker pipe: %w", connectErr)
		}
		clientSlots <- struct{}{}
		go func(handle windows.Handle) {
			defer func() { <-clientSlots }()
			_ = serveSandboxACLBrokerClient(handle, allowedSID)
		}(pipe)
	}
}

func serveSandboxACLBrokerClient(pipe windows.Handle, allowedSID string) error {
	file := os.NewFile(uintptr(pipe), "sandbox-acl-pipe")
	if file == nil {
		_ = windows.CloseHandle(pipe)
		return fmt.Errorf("wrap sandbox ACL pipe handle")
	}
	defer file.Close()
	var request sandboxACLBrokerRequest
	if err := json.NewDecoder(io.LimitReader(file, sandboxACLMaxRequestBytes)).Decode(&request); err != nil {
		return writeSandboxACLBrokerResponse(file, sandboxACLBrokerResponse{Error: "decode request: " + err.Error()})
	}
	if request.Operation == sandboxACLOpPing {
		return writeSandboxACLBrokerResponse(file, sandboxACLBrokerResponse{})
	}
	if request.Operation != sandboxACLOpPrepare && request.Operation != sandboxACLOpCleanup {
		return writeSandboxACLBrokerResponse(file, sandboxACLBrokerResponse{Error: "unsupported operation"})
	}
	if err := validateSandboxACLBrokerRequest(pipe, allowedSID, request); err != nil {
		return writeSandboxACLBrokerResponse(file, sandboxACLBrokerResponse{Error: err.Error()})
	}

	var err error
	if request.Operation == sandboxACLOpPrepare {
		err = prepareSandboxACLBrokerAccess(request)
	} else {
		err = cleanupSandboxACLBrokerAccess(request)
	}
	response := sandboxACLBrokerResponse{}
	if err != nil {
		response.Error = err.Error()
	}
	return writeSandboxACLBrokerResponse(file, response)
}

func writeSandboxACLBrokerResponse(file *os.File, response sandboxACLBrokerResponse) error {
	return json.NewEncoder(file).Encode(response)
}

func prepareSandboxACLBrokerAccess(request sandboxACLBrokerRequest) error {
	profileSID := strings.TrimSpace(request.ProfileSID)
	environmentRoot := filepath.Clean(request.EnvironmentRoot)
	if _, err := sandbox.EnsureWindowsAppContainerPath(environmentRoot, profileSID, sandbox.PathAccessReadWrite); err != nil {
		return fmt.Errorf("grant Environment root ACL: %w", err)
	}
	rootGranted, err := sandbox.WindowsAppContainerPathAccessGranted(environmentRoot, profileSID, sandbox.PathAccessReadWrite)
	if err != nil {
		return err
	}
	if !rootGranted {
		return fmt.Errorf("Environment root ACL verification failed")
	}
	return nil
}

func cleanupSandboxACLBrokerAccess(request sandboxACLBrokerRequest) error {
	profileSID := strings.TrimSpace(request.ProfileSID)
	environmentRoot := filepath.Clean(request.EnvironmentRoot)
	rootGranted, err := sandbox.WindowsAppContainerPathAccessGranted(environmentRoot, profileSID, sandbox.PathAccessReadWrite)
	if err != nil {
		return err
	}
	if rootGranted {
		if err := sandbox.RevokeWindowsAppContainerPath(environmentRoot, profileSID); err != nil {
			return fmt.Errorf("revoke Environment root ACL: %w", err)
		}
	}
	return nil
}

func validateSandboxACLBrokerRequest(pipe windows.Handle, allowedSID string, request sandboxACLBrokerRequest) error {
	profileName, err := sandbox.StableAppContainerProfileName(strings.TrimSpace(request.EnvironmentID))
	if err != nil {
		return fmt.Errorf("invalid environment id: %w", err)
	}
	profileSID := strings.TrimSpace(request.ProfileSID)
	parsedSID, err := windows.StringToSid(profileSID)
	if err != nil || parsedSID == nil || !strings.HasPrefix(strings.ToUpper(parsedSID.String()), "S-1-15-2-") {
		if err != nil {
			return fmt.Errorf("invalid AppContainer profile SID: %w", err)
		}
		return fmt.Errorf("invalid AppContainer profile SID %q", profileSID)
	}
	workspaceRoot := filepath.Clean(strings.TrimSpace(request.WorkspaceRoot))
	environmentRoot := filepath.Clean(strings.TrimSpace(request.EnvironmentRoot))
	if !filepath.IsAbs(workspaceRoot) || !filepath.IsAbs(environmentRoot) {
		return fmt.Errorf("sandbox ACL roots must be absolute")
	}
	traversalRoot, err := sandbox.ResolveEnvironmentTraversalRoot(workspaceRoot, environmentRoot)
	if err != nil {
		return err
	}
	token, err := sandboxACLBrokerClientToken(pipe, allowedSID)
	if err != nil {
		return err
	}
	defer token.Close()
	return validateSandboxACLRequestAsClient(token, profileName, profileSID, traversalRoot, environmentRoot)
}

func sandboxACLBrokerClientToken(pipe windows.Handle, allowedSID string) (windows.Token, error) {
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(pipe, &pid); err != nil {
		return 0, fmt.Errorf("resolve sandbox ACL broker client pid: %w", err)
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0, fmt.Errorf("open sandbox ACL broker client process: %w", err)
	}
	defer windows.CloseHandle(process)

	var primary windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE|windows.TOKEN_IMPERSONATE, &primary); err != nil {
		return 0, fmt.Errorf("open sandbox ACL broker client token: %w", err)
	}
	defer primary.Close()
	user, err := primary.GetTokenUser()
	if err != nil {
		return 0, fmt.Errorf("read sandbox ACL broker client user: %w", err)
	}
	clientSID := user.User.Sid.String()
	if !strings.EqualFold(clientSID, strings.TrimSpace(allowedSID)) {
		return 0, fmt.Errorf("sandbox ACL broker client user %s is not the installed ADM user", clientSID)
	}

	var impersonation windows.Token
	if err := windows.DuplicateTokenEx(
		primary,
		windows.TOKEN_QUERY|windows.TOKEN_IMPERSONATE,
		nil,
		windows.SecurityImpersonation,
		windows.TokenImpersonation,
		&impersonation,
	); err != nil {
		return 0, fmt.Errorf("duplicate sandbox ACL broker client token: %w", err)
	}
	return impersonation, nil
}

func validateSandboxACLRequestAsClient(token windows.Token, profileName, profileSID, traversalRoot, environmentRoot string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.SetThreadToken(nil, token); err != nil {
		return fmt.Errorf("impersonate sandbox ACL broker client: %w", err)
	}
	defer windows.RevertToSelf()

	expectedProfile, err := sandbox.DeriveWindowsAppContainerProfile(profileName)
	if err != nil {
		return fmt.Errorf("derive Environment AppContainer SID as broker client: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(expectedProfile.SID), strings.TrimSpace(profileSID)) {
		return fmt.Errorf("AppContainer profile SID does not match Environment %s", profileName)
	}

	if err := probeSandboxACLClientEnvironmentRoot(environmentRoot); err != nil {
		return fmt.Errorf("client lacks Environment root modify access required by sandbox ACL grant: %w", err)
	}
	for _, path := range sandboxACLTraversalPaths(traversalRoot, environmentRoot) {
		if err := probeSandboxACLClientTraversalPath(path); err != nil {
			return fmt.Errorf("client lacks traversal access on %s: %w", path, err)
		}
	}
	return nil
}

func probeSandboxACLClientEnvironmentRoot(path string) error {
	root := filepath.Clean(path)
	temp, err := os.CreateTemp(root, ".adm-sandbox-acl-probe-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err := temp.Write([]byte("adm-sandbox-acl-probe")); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Remove(name)
}

func probeSandboxACLClientTraversalPath(path string) error {
	ptr, err := windows.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(
		ptr,
		windows.GENERIC_READ|windows.GENERIC_EXECUTE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return err
	}
	return windows.CloseHandle(handle)
}

func sandboxACLTraversalPaths(traversalRoot, environmentRoot string) []string {
	traversalRoot = filepath.Clean(traversalRoot)
	environmentRoot = filepath.Clean(environmentRoot)
	if strings.EqualFold(traversalRoot, environmentRoot) {
		return nil
	}
	current := filepath.Dir(environmentRoot)
	var reversed []string
	for {
		reversed = append(reversed, current)
		if strings.EqualFold(current, traversalRoot) {
			break
		}
		next := filepath.Dir(current)
		if next == current {
			return nil
		}
		current = next
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed
}

func requestSandboxACLBroker(ctx context.Context, request sandboxACLBrokerRequest) (sandbox.EnvironmentAccessStatus, error) {
	deadline := time.Now().Add(3 * time.Second)
	for {
		name, err := windows.UTF16PtrFromString(sandboxACLPipeName)
		if err != nil {
			return sandbox.EnvironmentAccessStatus{}, err
		}
		handle, err := windows.CreateFile(
			name,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0,
			nil,
			windows.OPEN_EXISTING,
			0,
			0,
		)
		if err == nil {
			file := os.NewFile(uintptr(handle), "sandbox-acl-client")
			if file == nil {
				_ = windows.CloseHandle(handle)
				return sandbox.EnvironmentAccessStatus{}, fmt.Errorf("wrap sandbox ACL broker client handle")
			}
			defer file.Close()
			if err := json.NewEncoder(file).Encode(request); err != nil {
				return sandbox.EnvironmentAccessStatus{}, err
			}
			var response sandboxACLBrokerResponse
			if err := json.NewDecoder(file).Decode(&response); err != nil {
				return sandbox.EnvironmentAccessStatus{}, err
			}
			if strings.TrimSpace(response.Error) != "" {
				return response.Status, fmt.Errorf("%s", response.Error)
			}
			return response.Status, nil
		}
		if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) && !errors.Is(err, windows.ERROR_PIPE_BUSY) {
			return sandbox.EnvironmentAccessStatus{}, fmt.Errorf("connect sandbox ACL broker: %w", err)
		}
		if time.Now().After(deadline) {
			return sandbox.EnvironmentAccessStatus{}, errSandboxACLBrokerUnavailable
		}
		select {
		case <-ctx.Done():
			return sandbox.EnvironmentAccessStatus{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func sandboxACLBrokerReady() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := requestSandboxACLBroker(ctx, sandboxACLBrokerRequest{Operation: sandboxACLOpPing})
	return err == nil
}
