//go:build windows

package runtime

import (
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsManagedProcess struct {
	mu       sync.Mutex
	job      windows.Handle
	canceled bool
}

func startManagedProcess(cmd *exec.Cmd) (ManagedProcess, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create process job: %w", err)
	}
	managed := &windowsManagedProcess{job: job}
	success := false
	defer func() {
		if !success {
			managed.Close()
		}
	}()
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return nil, fmt.Errorf("configure process job: %w", err)
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// os/exec closes the primary thread handle. Start suspended and recover
	// that thread via Toolhelp before any application code can create children.
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	if cmd.Cancel != nil {
		cmd.Cancel = func() error {
			managed.mu.Lock()
			defer managed.mu.Unlock()
			managed.canceled = true
			if managed.job == 0 {
				return nil
			}
			return windows.TerminateJobObject(managed.job, 1)
		}
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	fail := func(err error) (ManagedProcess, error) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return fail(fmt.Errorf("open suspended process: %w", err))
	}
	defer windows.CloseHandle(handle)
	// Serialize assignment with cancellation. If cancellation occurred while
	// the job was empty, cmd.Wait will still be bounded by the normal cancel
	// path; the caller's context is checked by the launch wrapper.
	managed.mu.Lock()
	if managed.canceled {
		err = fmt.Errorf("launch canceled")
	} else {
		err = windows.AssignProcessToJobObject(job, handle)
	}
	managed.mu.Unlock()
	if err != nil {
		return fail(fmt.Errorf("assign suspended process to ADM job: %w", err))
	}
	if err := resumeManagedThread(uint32(cmd.Process.Pid)); err != nil {
		return fail(err)
	}
	success = true
	return managed, nil
}

func resumeManagedThread(pid uint32) error {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snap)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	var tid uint32
	for err = windows.Thread32First(snap, &entry); err == nil; err = windows.Thread32Next(snap, &entry) {
		if entry.OwnerProcessID == pid {
			if tid != 0 {
				return fmt.Errorf("unexpected multiple suspended threads")
			}
			tid = entry.ThreadID
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
	}
	if tid == 0 {
		return fmt.Errorf("suspended process thread unavailable")
	}
	thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, tid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(thread)
	_, err = windows.ResumeThread(thread)
	return err
}

func (m *windowsManagedProcess) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.job != 0 {
		_ = windows.CloseHandle(m.job)
		m.job = 0
	}
}

type jobAccounting struct {
	UserTime            int64
	KernelTime          int64
	PeriodUserTime      int64
	PeriodKernelTime    int64
	PageFaults          uint32
	TotalProcesses      uint32
	ActiveProcesses     uint32
	TerminatedProcesses uint32
}
type processMemoryCounters struct {
	Size              uint32
	PageFaults        uint32
	PeakWorkingSet    uintptr
	WorkingSet        uintptr
	QuotaPeakPaged    uintptr
	QuotaPaged        uintptr
	QuotaPeakNonPaged uintptr
	QuotaNonPaged     uintptr
	Pagefile          uintptr
	PeakPagefile      uintptr
}

var isProcessInJob = windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")
var getProcessMemoryInfo = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

func (m *windowsManagedProcess) Sample() (ProcessResources, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.job == 0 {
		return ProcessResources{}, fmt.Errorf("process job closed")
	}
	var account jobAccounting
	if err := windows.QueryInformationJobObject(m.job, 1, uintptr(unsafe.Pointer(&account)), uint32(unsafe.Sizeof(account)), nil); err != nil {
		return ProcessResources{}, err
	}
	out := ProcessResources{SampledAt: time.Now().UTC(), CPUTimeMS: uint64((account.UserTime + account.KernelTime) / 10000), ProcessCount: int(account.ActiveProcesses), Scope: "job", Complete: true}
	// JOBOBJECT_BASIC_PROCESS_ID_LIST has two DWORDs followed by ULONG_PTRs.
	// Bound memory usage; if the job is larger, mark the sample unavailable.
	var buffer struct {
		Assigned uint32
		Count    uint32
		PIDs     [1024]uintptr
	}
	if err := windows.QueryInformationJobObject(m.job, 3, uintptr(unsafe.Pointer(&buffer)), uint32(unsafe.Sizeof(buffer)), nil); err != nil {
		return ProcessResources{}, err
	}
	count := buffer.Count
	if count > uint32(len(buffer.PIDs)) {
		return ProcessResources{}, fmt.Errorf("job process list exceeds observation budget")
	}
	if int(count) != out.ProcessCount {
		out.Complete = false
	}
	for i := uint32(0); i < count; i++ {
		pid := buffer.PIDs[i]
		handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(pid))
		if err != nil {
			out.Complete = false
			continue
		}
		var member uint32
		ok, _, _ := isProcessInJob.Call(uintptr(handle), uintptr(m.job), uintptr(unsafe.Pointer(&member)))
		if ok == 0 || member == 0 {
			windows.CloseHandle(handle)
			out.Complete = false
			continue
		}
		counters := processMemoryCounters{Size: uint32(unsafe.Sizeof(processMemoryCounters{}))}
		ok, _, _ = getProcessMemoryInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&counters)), uintptr(counters.Size))
		windows.CloseHandle(handle)
		if ok == 0 {
			out.Complete = false
			continue
		}
		out.MemoryBytes += uint64(counters.WorkingSet)
	}
	return out, nil
}
