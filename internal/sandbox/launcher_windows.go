//go:build windows

package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	procThreadAttributeSecurityCapabilities = 0x00020009
	windowsProcessWaitPollInterval          = 25 * time.Millisecond
	windowsJobTerminationGrace              = 2 * time.Second
	windowsProcessTerminationGrace          = 1 * time.Second
	windowsPipeDrainGrace                   = 1500 * time.Millisecond
	windowsPipeCancelGrace                  = 500 * time.Millisecond
)

type sidAndAttributes struct {
	SID        *windows.SID
	Attributes uint32
}

type securityCapabilities struct {
	AppContainerSID *windows.SID
	Capabilities    *sidAndAttributes
	CapabilityCount uint32
	Reserved        uint32
}

const seGroupEnabled = 0x00000004

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

	var capabilityPinner runtime.Pinner
	defer capabilityPinner.Unpin()
	capabilityEntries := make([]sidAndAttributes, 0, len(request.Capabilities))
	for _, name := range request.Capabilities {
		capabilitySID, capabilityErr := windowsCapabilitySID(name)
		if capabilityErr != nil {
			return LaunchResult{}, capabilityErr
		}
		capabilityPinner.Pin(capabilitySID)
		capabilityEntries = append(capabilityEntries, sidAndAttributes{SID: capabilitySID, Attributes: seGroupEnabled})
	}
	if len(capabilityEntries) > 0 {
		capabilityPinner.Pin(&capabilityEntries[0])
	}

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
	if len(capabilityEntries) > 0 {
		capabilities.Capabilities = &capabilityEntries[0]
		capabilities.CapabilityCount = uint32(len(capabilityEntries))
	}
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
	stdoutCopy := startWindowsPipeCopy(pipes.stdoutRead, stdoutCapture)
	pipes.stdoutRead = 0
	stderrCopy := startWindowsPipeCopy(pipes.stderrRead, stderrCapture)
	pipes.stderrRead = 0
	pipeCopies := []*windowsPipeCopy{stdoutCopy, stderrCopy}
	copiesDone := windowsPipeCopiesDone(pipeCopies)
	defer closeWindowsPipeCopies(pipeCopies)

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
	runtime.KeepAlive(capabilityEntries)
	runtime.KeepAlive(capabilities)
	if createErr != nil {
		_ = windows.CloseHandle(pipes.stdoutWrite)
		pipes.stdoutWrite = 0
		_ = windows.CloseHandle(pipes.stderrWrite)
		pipes.stderrWrite = 0
		pipeErr := waitWindowsPipeCopies(copiesDone, pipeCopies)
		if pipeErr != nil {
			return launchCaptureResult(false, -1, stdoutCapture, stderrCapture), fmt.Errorf("create AppContainer process: %w; %v", createErr, pipeErr)
		}
		return launchCaptureResult(false, -1, stdoutCapture, stderrCapture), fmt.Errorf("create AppContainer process: %w", createErr)
	}
	defer windows.CloseHandle(process.Thread)
	defer windows.CloseHandle(process.Process)

	// The child owns the inherited ends after CreateProcess. Closing our copies
	// prevents them from keeping stdout/stderr alive and gives stdin immediate EOF.
	pipes.closeChildEnds()

	if err := windows.AssignProcessToJobObject(job, process.Process); err != nil {
		_ = windows.TerminateProcess(process.Process, 1)
		_ = waitWindowsProcessHandle(process.Process, windowsProcessTerminationGrace)
		pipeErr := waitWindowsPipeCopies(copiesDone, pipeCopies)
		if pipeErr != nil {
			return launchCaptureResult(true, -1, stdoutCapture, stderrCapture), fmt.Errorf("assign AppContainer process to job: %w; %v", err, pipeErr)
		}
		return launchCaptureResult(true, -1, stdoutCapture, stderrCapture), fmt.Errorf("assign AppContainer process to job: %w", err)
	}
	if _, err := windows.ResumeThread(process.Thread); err != nil {
		_ = windows.TerminateJobObject(job, 1)
		if waitErr := waitWindowsProcessHandle(process.Process, windowsJobTerminationGrace); waitErr != nil {
			_ = windows.TerminateProcess(process.Process, 1)
			_ = waitWindowsProcessHandle(process.Process, windowsProcessTerminationGrace)
		}
		pipeErr := waitWindowsPipeCopies(copiesDone, pipeCopies)
		if pipeErr != nil {
			return launchCaptureResult(true, -1, stdoutCapture, stderrCapture), fmt.Errorf("resume AppContainer process: %w; %v", err, pipeErr)
		}
		return launchCaptureResult(true, -1, stdoutCapture, stderrCapture), fmt.Errorf("resume AppContainer process: %w", err)
	}

	contextErr, waitErr := waitWindowsProcessContext(ctx, process.Process, job)

	// A sandbox command owns its process tree. Once the root command exits (or
	// cancellation starts), terminate descendants before draining stdout/stderr.
	descendantErr := windows.TerminateJobObject(job, 0)
	pipeErr := waitWindowsPipeCopies(copiesDone, pipeCopies)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(process.Process, &exitCode); err != nil {
		return launchCaptureResult(true, -1, stdoutCapture, stderrCapture), fmt.Errorf("read AppContainer process exit code: %w", err)
	}
	result := launchCaptureResult(true, int(exitCode), stdoutCapture, stderrCapture)
	if contextErr != nil {
		if waitErr != nil || descendantErr != nil || pipeErr != nil {
			return result, fmt.Errorf("%w; sandbox cleanup: process_wait=%v descendant_kill=%v pipe_drain=%v", contextErr, waitErr, descendantErr, pipeErr)
		}
		return result, contextErr
	}
	if waitErr != nil {
		return result, fmt.Errorf("wait for AppContainer process: %w", waitErr)
	}
	if descendantErr != nil {
		return result, fmt.Errorf("terminate AppContainer descendants: %w", descendantErr)
	}
	if pipeErr != nil {
		return result, pipeErr
	}
	return result, nil
}

func waitWindowsProcessContext(ctx context.Context, process, job windows.Handle) (contextErr error, waitErr error) {
	for {
		status, err := windows.WaitForSingleObject(process, uint32(windowsProcessWaitPollInterval/time.Millisecond))
		if err != nil {
			return nil, err
		}
		switch status {
		case windows.WAIT_OBJECT_0:
			return nil, nil
		case uint32(windows.WAIT_TIMEOUT):
		default:
			return nil, fmt.Errorf("unexpected process wait status 0x%x", status)
		}

		select {
		case <-ctx.Done():
			contextErr = ctx.Err()
			jobKillErr := windows.TerminateJobObject(job, 1)
			jobWaitErr := waitWindowsProcessHandle(process, windowsJobTerminationGrace)
			if jobWaitErr == nil {
				if jobKillErr != nil {
					return contextErr, fmt.Errorf("terminate AppContainer job after cancellation: %w", jobKillErr)
				}
				return contextErr, nil
			}

			processKillErr := windows.TerminateProcess(process, 1)
			processWaitErr := waitWindowsProcessHandle(process, windowsProcessTerminationGrace)
			if processWaitErr != nil {
				return contextErr, fmt.Errorf(
					"AppContainer process did not stop after cancellation: job_kill=%v job_wait=%v process_kill=%v process_wait=%v",
					jobKillErr, jobWaitErr, processKillErr, processWaitErr,
				)
			}
			if processKillErr != nil {
				return contextErr, fmt.Errorf("force terminate AppContainer process after cancellation: %w", processKillErr)
			}
			return contextErr, nil
		default:
		}
	}
}

func waitWindowsProcessHandle(process windows.Handle, timeout time.Duration) error {
	milliseconds := uint32(timeout / time.Millisecond)
	if milliseconds == 0 {
		milliseconds = 1
	}
	status, err := windows.WaitForSingleObject(process, milliseconds)
	if err != nil {
		return err
	}
	switch status {
	case windows.WAIT_OBJECT_0:
		return nil
	case uint32(windows.WAIT_TIMEOUT):
		return fmt.Errorf("process did not stop within %s", timeout)
	default:
		return fmt.Errorf("unexpected process wait status 0x%x", status)
	}
}

func waitWindowsPipeCopies(done <-chan struct{}, copies []*windowsPipeCopy) error {
	return waitWindowsPipeCopiesWithGrace(done, copies, windowsPipeDrainGrace, windowsPipeCancelGrace)
}

func waitWindowsPipeCopiesWithGrace(done <-chan struct{}, copies []*windowsPipeCopy, drainGrace, cancelGrace time.Duration) error {
	if waitWindowsDone(done, drainGrace) {
		return nil
	}
	for _, copy := range copies {
		if copy != nil {
			// A child or descendant can keep an inherited stdout/stderr handle
			// alive after the root process exits. Cancel the parent's pending
			// pipe reads so output collection cannot stall Run completion.
			copy.cancel()
		}
	}
	if waitWindowsDone(done, cancelGrace) {
		return nil
	}
	return fmt.Errorf("sandbox stdout/stderr did not drain within %s after process exit/cancellation", drainGrace+cancelGrace)
}

func waitWindowsDone(done <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

func windowsCapabilitySID(name string) (*windows.SID, error) {
	switch name {
	case CapabilityInternetClient:
		// SECURITY_CAPABILITY_INTERNET_CLIENT = S-1-15-3-1.
		return windows.StringToSid("S-1-15-3-1")
	default:
		return nil, fmt.Errorf("unsupported Windows AppContainer capability %q", name)
	}
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

type windowsPipeCopy struct {
	file *os.File
	done chan struct{}
}

func startWindowsPipeCopy(handle windows.Handle, target io.Writer) *windowsPipeCopy {
	copy := &windowsPipeCopy{
		file: os.NewFile(uintptr(handle), "adm-sandbox-pipe"),
		done: make(chan struct{}),
	}
	if copy.file == nil {
		_ = windows.CloseHandle(handle)
		close(copy.done)
		return copy
	}
	go func() {
		defer close(copy.done)
		_, _ = io.Copy(target, copy.file)
	}()
	return copy
}

func windowsPipeCopiesDone(copies []*windowsPipeCopy) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, copy := range copies {
			if copy != nil {
				<-copy.done
			}
		}
	}()
	return done
}

func closeWindowsPipeCopies(copies []*windowsPipeCopy) {
	for _, copy := range copies {
		if copy != nil && copy.file != nil {
			_ = copy.file.Close()
		}
	}
}

func (c *windowsPipeCopy) cancel() {
	if c == nil || c.file == nil {
		return
	}
	select {
	case <-c.done:
		return
	default:
		_ = windows.CancelIoEx(windows.Handle(c.file.Fd()), nil)
	}
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
