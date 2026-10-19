//go:build windows

package sandbox

import (
	"context"
	"errors"
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

type sandboxPipe struct {
	read  windows.Handle
	write windows.Handle
}

func launchPlatform(ctx context.Context, request LaunchRequest) (LaunchResult, error) {
	sid, err := windows.StringToSid(request.ProfileSID)
	if err != nil {
		return LaunchResult{}, fmt.Errorf("parse AppContainer SID: %w", err)
	}
	var sidPinner runtime.Pinner
	sidPinner.Pin(sid)
	defer sidPinner.Unpin()

	stdinPipe, err := newSandboxPipe()
	if err != nil {
		return LaunchResult{}, fmt.Errorf("create sandbox stdin pipe: %w", err)
	}
	stdoutPipe, err := newSandboxPipe()
	if err != nil {
		stdinPipe.close()
		return LaunchResult{}, fmt.Errorf("create sandbox stdout pipe: %w", err)
	}
	stderrPipe, err := newSandboxPipe()
	if err != nil {
		stdinPipe.close()
		stdoutPipe.close()
		return LaunchResult{}, fmt.Errorf("create sandbox stderr pipe: %w", err)
	}
	pipesOpen := true
	defer func() {
		if pipesOpen {
			stdinPipe.close()
			stdoutPipe.close()
			stderrPipe.close()
		}
	}()

	if err := windows.SetHandleInformation(stdinPipe.write, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
		return LaunchResult{}, fmt.Errorf("make sandbox stdin parent handle non-inheritable: %w", err)
	}
	if err := windows.SetHandleInformation(stdoutPipe.read, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
		return LaunchResult{}, fmt.Errorf("make sandbox stdout parent handle non-inheritable: %w", err)
	}
	if err := windows.SetHandleInformation(stderrPipe.read, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
		return LaunchResult{}, fmt.Errorf("make sandbox stderr parent handle non-inheritable: %w", err)
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

	childHandles := []windows.Handle{stdinPipe.read, stdoutPipe.write, stderrPipe.write}
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
	startup.StartupInfo.StdInput = stdinPipe.read
	startup.StartupInfo.StdOutput = stdoutPipe.write
	startup.StartupInfo.StdErr = stderrPipe.write
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
	result := LaunchResult{Started: true}
	defer windows.CloseHandle(process.Thread)
	defer windows.CloseHandle(process.Process)

	// Parent never writes stdin. Closing its write end makes the sandboxed
	// process observe EOF instead of trying to open the host NUL device.
	_ = windows.CloseHandle(stdinPipe.write)
	stdinPipe.write = 0
	_ = windows.CloseHandle(stdinPipe.read)
	stdinPipe.read = 0

	stdoutFile := os.NewFile(uintptr(stdoutPipe.read), "adm-sandbox-stdout")
	stderrFile := os.NewFile(uintptr(stderrPipe.read), "adm-sandbox-stderr")
	stdoutPipe.read = 0
	stderrPipe.read = 0

	// The child inherited the write ends. The parent must close its copies so
	// readers terminate as soon as the process tree exits.
	_ = windows.CloseHandle(stdoutPipe.write)
	stdoutPipe.write = 0
	_ = windows.CloseHandle(stderrPipe.write)
	stderrPipe.write = 0
	pipesOpen = false

	stdoutWriter := request.Stdout
	if stdoutWriter == nil {
		stdoutWriter = io.Discard
	}
	stderrWriter := request.Stderr
	if stderrWriter == nil {
		stderrWriter = io.Discard
	}

	var copyWG sync.WaitGroup
	copyErrs := make(chan error, 2)
	copyPipe := func(name string, file *os.File, writer io.Writer) {
		defer copyWG.Done()
		defer file.Close()
		if _, copyErr := io.Copy(writer, file); copyErr != nil && !errors.Is(copyErr, os.ErrClosed) {
			copyErrs <- fmt.Errorf("copy sandbox %s: %w", name, copyErr)
		}
	}
	copyWG.Add(2)
	go copyPipe("stdout", stdoutFile, stdoutWriter)
	go copyPipe("stderr", stderrFile, stderrWriter)

	if err := windows.AssignProcessToJobObject(job, process.Process); err != nil {
		_ = windows.TerminateProcess(process.Process, 1)
		copyWG.Wait()
		close(copyErrs)
		return result, fmt.Errorf("assign AppContainer process to job: %w", err)
	}
	if _, err := windows.ResumeThread(process.Thread); err != nil {
		_ = windows.TerminateJobObject(job, 1)
		copyWG.Wait()
		close(copyErrs)
		return result, fmt.Errorf("resume AppContainer process: %w", err)
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
	select {
	case waitErr = <-waitDone:
	case <-ctx.Done():
		_ = windows.TerminateJobObject(job, 1)
		waitErr = <-waitDone
		if waitErr == nil {
			waitErr = ctx.Err()
		}
	}

	copyWG.Wait()
	close(copyErrs)
	for copyErr := range copyErrs {
		if copyErr != nil && waitErr == nil {
			waitErr = copyErr
		}
	}

	if waitErr != nil {
		if errors.Is(waitErr, context.Canceled) || errors.Is(waitErr, context.DeadlineExceeded) {
			return result, waitErr
		}
		return result, fmt.Errorf("wait for AppContainer process: %w", waitErr)
	}

	var exitCode uint32
	if err := windows.GetExitCodeProcess(process.Process, &exitCode); err != nil {
		return result, fmt.Errorf("read AppContainer process exit code: %w", err)
	}
	result.ExitCode = int(exitCode)
	return result, nil
}

func newSandboxPipe() (sandboxPipe, error) {
	sa := windows.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle: 1,
	}
	var pipe sandboxPipe
	if err := windows.CreatePipe(&pipe.read, &pipe.write, &sa, 0); err != nil {
		return sandboxPipe{}, err
	}
	return pipe, nil
}

func (p *sandboxPipe) close() {
	if p == nil {
		return
	}
	if p.read != 0 {
		_ = windows.CloseHandle(p.read)
		p.read = 0
	}
	if p.write != 0 {
		_ = windows.CloseHandle(p.write)
		p.write = 0
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
