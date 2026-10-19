//go:build windows

package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const procThreadAttributeSecurityCapabilities = 0x00020009

type securityCapabilities struct {
	AppContainerSID *windows.SID
	Capabilities    *windows.SIDAndAttributes
	CapabilityCount uint32
	Reserved        uint32
}

type windowsChildPipes struct {
	stdinRead   windows.Handle
	stdinWrite  windows.Handle
	stdoutRead  windows.Handle
	stdoutWrite windows.Handle
	stderrRead  windows.Handle
	stderrWrite windows.Handle
}

func launchPlatform(ctx context.Context, request LaunchRequest) (LaunchResult, error) {
	sid, err := windows.StringToSid(request.ProfileSID)
	if err != nil {
		return LaunchResult{}, fmt.Errorf("parse AppContainer SID: %w", err)
	}
	var sidPinner runtime.Pinner
	sidPinner.Pin(sid)
	defer sidPinner.Unpin()

	pipes, err := newWindowsChildPipes()
	if err != nil {
		return LaunchResult{}, err
	}
	defer pipes.closeAll()

	attrs, err := windows.NewProcThreadAttributeList(2)
	if err != nil {
		return LaunchResult{}, fmt.Errorf("create process attribute list: %w", err)
	}
	defer attrs.Delete()

	capabilities := securityCapabilities{AppContainerSID: sid}
	if err := attrs.Update(
		procThreadAttributeSecurityCapabilities,
		unsafe.Pointer(&capabilities),
		unsafe.Sizeof(capabilities),
	); err != nil {
		return LaunchResult{}, fmt.Errorf("set AppContainer process attribute: %w", err)
	}

	handles := []windows.Handle{pipes.stdinRead, pipes.stdoutWrite, pipes.stderrWrite}
	if err := attrs.Update(
		windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST,
		unsafe.Pointer(&handles[0]),
		uintptr(len(handles))*unsafe.Sizeof(handles[0]),
	); err != nil {
		return LaunchResult{}, fmt.Errorf("set sandbox inherited-handle list: %w", err)
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return LaunchResult{}, fmt.Errorf("create sandbox job object: %w", err)
	}
	defer windows.CloseHandle(job)

	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	); err != nil {
		return LaunchResult{}, fmt.Errorf("configure sandbox job object: %w", err)
	}

	executablePtr, err := windows.UTF16PtrFromString(request.Executable)
	if err != nil {
		return LaunchResult{}, fmt.Errorf("encode sandbox executable: %w", err)
	}
	commandLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{request.Executable}, request.Args...)))
	if err != nil {
		return LaunchResult{}, fmt.Errorf("encode sandbox command line: %w", err)
	}
	cwdPtr, err := windows.UTF16PtrFromString(request.Cwd)
	if err != nil {
		return LaunchResult{}, fmt.Errorf("encode sandbox cwd: %w", err)
	}
	environment, err := windowsEnvironmentBlock(request.Environment)
	if err != nil {
		return LaunchResult{}, err
	}

	stdoutCapture := newLaunchCapture(request.MaxOutputBytes, request.Stdout)
	stderrCapture := newLaunchCapture(request.MaxOutputBytes, request.Stderr)
	var copies sync.WaitGroup
	copies.Add(2)
	go copyWindowsPipe(&copies, pipes.stdoutRead, stdoutCapture)
	pipes.stdoutRead = 0
	go copyWindowsPipe(&copies, pipes.stderrRead, stderrCapture)
	pipes.stderrRead = 0

	startup := windows.StartupInfoEx{}
	startup.StartupInfo.Cb = uint32(unsafe.Sizeof(startup))
	startup.StartupInfo.Flags |= windows.STARTF_USESTDHANDLES
	startup.StartupInfo.StdInput = pipes.stdinRead
	startup.StartupInfo.StdOutput = pipes.stdoutWrite
	startup.StartupInfo.StdErr = pipes.stderrWrite
	startup.ProcThreadAttributeList = attrs.List()

	process := windows.ProcessInformation{}
	flags := uint32(windows.CREATE_SUSPENDED | windows.CREATE_UNICODE_ENVIRONMENT | windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_NO_WINDOW)
	createErr := windows.CreateProcess(
		executablePtr,
		commandLine,
		nil,
		nil,
		true,
		flags,
		&environment[0],
		cwdPtr,
		&startup.StartupInfo,
		&process,
	)
	runtime.KeepAlive(handles)
	runtime.KeepAlive(capabilities)
	if createErr != nil {
		_ = windows.CloseHandle(pipes.stdoutWrite)
		pipes.stdoutWrite = 0
		_ = windows.CloseHandle(pipes.stderrWrite)
		pipes.stderrWrite = 0
		copies.Wait()
		return launchCaptureResult(false, -1, stdoutCapture, stderrCapture), fmt.Errorf("create AppContainer process: %w", createErr)
	}
	defer windows.CloseHandle(process.Thread)
	defer windows.CloseHandle(process.Process)

	// The child owns the inherited ends after CreateProcess. Closing our copies
	// prevents them from keeping stdout/stderr alive and gives stdin immediate EOF.
	pipes.closeChildEnds()

	if err := windows.AssignProcessToJobObject(job, process.Process); err != nil {
		_ = windows.TerminateProcess(process.Process, 1)
		copies.Wait()
		return launchCaptureResult(true, -1, stdoutCapture, stderrCapture), fmt.Errorf("assign AppContainer process to job: %w", err)
	}
	if _, err := windows.ResumeThread(process.Thread); err != nil {
		_ = windows.TerminateJobObject(job, 1)
		copies.Wait()
		return launchCaptureResult(true, -1, stdoutCapture, stderrCapture), fmt.Errorf("resume AppContainer process: %w", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		status, waitErr := windows.WaitForSingleObject(process.Process, windows.INFINITE)
		if waitErr != nil {
			waitDone <- waitErr
			return
		}
		if status != windows.WAIT_OBJECT_0 {
			waitDone <- fmt.Errorf("unexpected process wait status 0x%x", status)
			return
		}
		waitDone <- nil
	}()

	var waitErr error
	var contextErr error
	select {
	case waitErr = <-waitDone:
	case <-ctx.Done():
		contextErr = ctx.Err()
		_ = windows.TerminateJobObject(job, 1)
		waitErr = <-waitDone
	}

	// A verifier command owns its process tree. Once the root command exits,
	// terminate any descendants that are still alive before collecting output.
	_ = windows.TerminateJobObject(job, 0)
	copies.Wait()

	var exitCode uint32
	if err := windows.GetExitCodeProcess(process.Process, &exitCode); err != nil {
		return launchCaptureResult(true, -1, stdoutCapture, stderrCapture), fmt.Errorf("read AppContainer process exit code: %w", err)
	}
	result := launchCaptureResult(true, int(exitCode), stdoutCapture, stderrCapture)
	if contextErr != nil {
		return result, contextErr
	}
	if waitErr != nil {
		return result, fmt.Errorf("wait for AppContainer process: %w", waitErr)
	}
	return result, nil
}

func newWindowsChildPipes() (*windowsChildPipes, error) {
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), InheritHandle: 1}
	p := &windowsChildPipes{}
	if err := windows.CreatePipe(&p.stdinRead, &p.stdinWrite, &sa, 0); err != nil {
		return nil, fmt.Errorf("create sandbox stdin pipe: %w", err)
	}
	if err := windows.CreatePipe(&p.stdoutRead, &p.stdoutWrite, &sa, 0); err != nil {
		p.closeAll()
		return nil, fmt.Errorf("create sandbox stdout pipe: %w", err)
	}
	if err := windows.CreatePipe(&p.stderrRead, &p.stderrWrite, &sa, 0); err != nil {
		p.closeAll()
		return nil, fmt.Errorf("create sandbox stderr pipe: %w", err)
	}
	for _, handle := range []windows.Handle{p.stdinWrite, p.stdoutRead, p.stderrRead} {
		if err := windows.SetHandleInformation(handle, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
			p.closeAll()
			return nil, fmt.Errorf("make sandbox parent pipe non-inheritable: %w", err)
		}
	}
	return p, nil
}

func (p *windowsChildPipes) closeChildEnds() {
	for _, target := range []*windows.Handle{&p.stdinRead, &p.stdinWrite, &p.stdoutWrite, &p.stderrWrite} {
		if *target != 0 {
			_ = windows.CloseHandle(*target)
			*target = 0
		}
	}
}

func (p *windowsChildPipes) closeAll() {
	for _, target := range []*windows.Handle{
		&p.stdinRead, &p.stdinWrite,
		&p.stdoutRead, &p.stdoutWrite,
		&p.stderrRead, &p.stderrWrite,
	} {
		if *target != 0 {
			_ = windows.CloseHandle(*target)
			*target = 0
		}
	}
}

func copyWindowsPipe(group *sync.WaitGroup, handle windows.Handle, target io.Writer) {
	defer group.Done()
	file := os.NewFile(uintptr(handle), "adm-sandbox-pipe")
	if file == nil {
		_ = windows.CloseHandle(handle)
		return
	}
	defer file.Close()
	_, _ = io.Copy(target, file)
}

func launchCaptureResult(started bool, exitCode int, stdout, stderr *launchCapture) LaunchResult {
	stdoutText, stdoutTruncated := stdout.Snapshot()
	stderrText, stderrTruncated := stderr.Snapshot()
	return LaunchResult{
		Started:         started,
		ExitCode:        exitCode,
		Stdout:          stdoutText,
		Stderr:          stderrText,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: stderrTruncated,
	}
}

func windowsEnvironmentBlock(items []string) ([]uint16, error) {
	block := make([]uint16, 0, 256)
	for _, item := range items {
		if item == "" {
			continue
		}
		if containsNUL(item) {
			return nil, fmt.Errorf("sandbox environment contains NUL")
		}
		block = append(block, utf16.Encode([]rune(item))...)
		block = append(block, 0)
	}
	block = append(block, 0)
	if len(block) == 1 {
		block = append(block, 0)
	}
	return block, nil
}

func containsNUL(value string) bool {
	for _, r := range value {
		if r == 0 {
			return true
		}
	}
	return false
}
