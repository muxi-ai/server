//go:build windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"github.com/rs/zerolog"
	"golang.org/x/sys/windows"
)

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procCreateJobObjectW      = kernel32.NewProc("CreateJobObjectW")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	procSetInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	procTerminateJobObject    = kernel32.NewProc("TerminateJobObject")
)

// Job object structures for Windows process management
type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

const (
	jobObjectExtendedLimitInformationClass = 9
	jobObjectLimitKillOnJobClose           = 0x00002000
)

// setupPlatformProcess configures platform-specific process attributes for Windows
func setupPlatformProcess(cmd interface{}) error {
	// Type assertion to *exec.Cmd
	c, ok := cmd.(*exec.Cmd)
	if !ok {
		return fmt.Errorf("invalid command type")
	}

	// Create process in a new process group
	// This allows us to manage the process tree
	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true, // Don't show console window for child processes
	}

	return nil
}

// createJobObject creates a Windows Job Object for process lifecycle management
// Job Objects allow us to manage process trees (similar to Unix process groups)
func createJobObject(pid int) (windows.Handle, error) {
	// Create job object
	jobHandle, _, err := procCreateJobObjectW.Call(0, 0)
	if jobHandle == 0 {
		return 0, fmt.Errorf("CreateJobObject failed: %w", err)
	}

	// Configure job to kill all processes when job is closed
	var info jobObjectExtendedLimitInformation
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose

	ret, _, err := procSetInformationJobObject.Call(
		jobHandle,
		jobObjectExtendedLimitInformationClass,
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	if ret == 0 {
		windows.CloseHandle(windows.Handle(jobHandle))
		return 0, fmt.Errorf("SetInformationJobObject failed: %w", err)
	}

	// Open process handle
	processHandle, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(pid))
	if err != nil {
		windows.CloseHandle(windows.Handle(jobHandle))
		return 0, fmt.Errorf("OpenProcess failed: %w", err)
	}
	defer windows.CloseHandle(processHandle)

	// Assign process to job object
	ret, _, err = procAssignProcessToJobObject.Call(jobHandle, uintptr(processHandle))
	if ret == 0 {
		windows.CloseHandle(windows.Handle(jobHandle))
		return 0, fmt.Errorf("AssignProcessToJobObject failed: %w", err)
	}

	return windows.Handle(jobHandle), nil
}

// Stop stops a running process gracefully on Windows
func Stop(proc *Process, logger *zerolog.Logger) error {
	if proc.cmd == nil || proc.cmd.Process == nil {
		return fmt.Errorf("process not running")
	}

	if logger == nil {
		l := zerolog.Nop()
		logger = &l
	}

	logger.Info().
		Str("id", proc.ID).
		Int("pid", proc.PID).
		Msg("Stopping process")

	proc.SetStatus(StatusStopping)
	proc.SetStopSignal(true)

	// On Windows, try graceful shutdown first by closing stdin
	// This signals the process that it should exit
	if proc.cmd.Process != nil {
		// Send Ctrl+Break signal (graceful termination for console apps)
		// This is the Windows equivalent of SIGTERM
		dll := syscall.MustLoadDLL("kernel32.dll")
		ctrlProc := dll.MustFindProc("GenerateConsoleCtrlEvent")
		
		// CTRL_BREAK_EVENT = 1
		r1, _, err := ctrlProc.Call(uintptr(1), uintptr(proc.cmd.Process.Pid))
		if r1 == 0 {
			logger.Warn().
				Err(err).
				Str("id", proc.ID).
				Msg("Failed to send Ctrl+Break, forcing termination")
			
			// Force kill if graceful termination fails
			if err := proc.cmd.Process.Kill(); err != nil {
				return fmt.Errorf("failed to kill process: %w", err)
			}
		} else {
			// Wait a bit for graceful shutdown
			time.Sleep(2 * time.Second)
			
			// Check if still running
			if IsProcessRunning(proc.PID) {
				logger.Warn().
					Str("id", proc.ID).
					Msg("Process did not stop gracefully, forcing termination")
				if err := proc.cmd.Process.Kill(); err != nil {
					return fmt.Errorf("failed to kill process: %w", err)
				}
			}
		}
	}

	// Wait for process to exit
	if err := proc.cmd.Wait(); err != nil {
		// Exit error is expected
		logger.Debug().
			Err(err).
			Str("id", proc.ID).
			Msg("Process exited")
	}

	proc.SetStatus(StatusStopped)
	proc.PID = 0
	proc.cmd = nil

	// Clean up PID file
	if err := os.Remove(proc.PIDFile); err != nil {
		logger.Debug().
			Err(err).
			Str("id", proc.ID).
			Msg("Failed to remove PID file")
	}

	logger.Info().
		Str("id", proc.ID).
		Msg("✓ Process stopped")

	return nil
}

// IsProcessRunning checks if a process with the given PID is running on Windows
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	// Try to open the process
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	// Check process exit code
	var exitCode uint32
	err = windows.GetExitCodeProcess(handle, &exitCode)
	if err != nil {
		return false
	}

	// STILL_ACTIVE = 259
	return exitCode == 259
}
