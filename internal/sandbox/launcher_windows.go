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

func launchPlatform(ctx context.Context, request LaunchRequest) (LaunchResult, error) {
	sid, err := windows.StringToSid(request.ProfileSID)
	if err != nil {
		return LaunchResult{}, fmt.Errorf("parse AppContainer SID: %w", err)
	}
	var sidPinner runtime.Pinner
	sidPinner.Pin(sid)
	defer sidPinner.Unpin()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		return LaunchResult{}, fmt.Errorf("create sandbox stdin pipe: %w", err)
	}
	defer stdinR.Close()
	// The sandbox is non-interactive. Close the parent writer immediately so
	// reads from stdin observe EOF without opening the AppContainer-inaccessible
	// NUL device.
	_ = stdinW.Close()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return LaunchResult{}, fmt.Errorf("create sandbox stdout pipe: %w", err)
	}
	defer stdoutR.Close()
	defer stdoutW.Close()

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		return LaunchResult{}, fmt.Errorf("create sandbox stderr pipe: %w", err)
	}
	defer stderrR.Close()
	defer stderrW.Close()

	childHandles := []windows.Handle{
		windows.Handle(stdinR.Fd()),
		windows.Handle(stdoutW.Fd()),
		windows.Handle(stderrW.Fd()),
	}
	for _, handle := range childHandles {
		if err := windows.SetHandleInformation(handle, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
			return LaunchResult{}, fmt.Errorf("mark sandbox stdio handle inheritable: %w", err)
		}
	}

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
	if err := attrs.Update(
		windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST,
		unsafe.Pointer(&childHandles[0]),
		uintptr(len(childHandles))*unsafe.Sizeof(childHandles[0]),
	); err != nil {
		return LaunchResult{}, fmt.Errorf("set sandbox inherited handle list: %w", err)
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

	startup := windows.StartupInfoEx{}
	startup.StartupInfo.Cb = uint32(unsafe.Sizeof(startup))
	startup.StartupInfo.Flags |= windows.STARTF_USESTDHANDLES
	startup.StartupInfo.StdInput = childHandles[0]
	startup.StartupInfo.StdOutput = childHandles[1]
	startup.StartupInfo.StdErr = childHandles[2]
	startup.ProcThreadAttributeList = attrs.List()

	process := windows.ProcessInformation{}
	flags := uint32(windows.CREATE_SUSPENDED | windows.CREATE_UNICODE_ENVIRONMENT | windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_NO_WINDOW)
	if err := windows.CreateProcess(
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
	); err != nil {
		return LaunchResult{}, fmt.Errorf("create AppContainer process: %w", err)
	}
	defer windows.CloseHandle(process.Thread)
	defer windows.CloseHandle(process.Process)

	// The child owns inherited copies after CreateProcess. Drop the parent's
	// copies before reading so EOF is observable when the process tree exits.
	_ = stdinR.Close()
	_ = stdoutW.Close()
	_ = stderrW.Close()

	stdoutTarget := request.Stdout
	if stdoutTarget == nil {
		stdoutTarget = io.Discard
	}
	stderrTarget := request.Stderr
	if stderrTarget == nil {
		stderrTarget = io.Discard
	}
	var copyWG sync.WaitGroup
	copyWG.Add(2)
	go func() {
		defer copyWG.Done()
		_, _ = io.Copy(stdoutTarget, stdoutR)
	}()
	go func() {
		defer copyWG.Done()
		_, _ = io.Copy(stderrTarget, stderrR)
	}()

	if err := windows.AssignProcessToJobObject(job, process.Process); err != nil {
		_ = windows.TerminateProcess(process.Process, 1)
		copyWG.Wait()
		return LaunchResult{}, fmt.Errorf("assign AppContainer process to job: %w", err)
	}
	if _, err := windows.ResumeThread(process.Thread); err != nil {
		_ = windows.TerminateJobObject(job, 1)
		copyWG.Wait()
		return LaunchResult{}, fmt.Errorf("resume AppContainer process: %w", err)
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

	select {
	case err := <-waitDone:
		copyWG.Wait()
		if err != nil {
			return LaunchResult{}, fmt.Errorf("wait for AppContainer process: %w", err)
		}
	case <-ctx.Done():
		_ = windows.TerminateJobObject(job, 1)
		<-waitDone
		copyWG.Wait()
		return LaunchResult{}, ctx.Err()
	}

	var exitCode uint32
	if err := windows.GetExitCodeProcess(process.Process, &exitCode); err != nil {
		return LaunchResult{}, fmt.Errorf("read AppContainer process exit code: %w", err)
	}
	return LaunchResult{ExitCode: int(exitCode)}, nil
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
