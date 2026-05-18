//go:build windows

package lock

import (
	"os"
	"syscall"
)

// isProcessRunning checks if a process with the given PID is running.
// On Windows, we try to open the process with SYNCHRONIZE permission.
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openProcess := kernel32.NewProc("OpenProcess")
	closeHandle := kernel32.NewProc("CloseHandle")

	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	handle, _, err := openProcess.Call(
		uintptr(PROCESS_QUERY_LIMITED_INFORMATION),
		uintptr(0),
		uintptr(pid),
	)
	if handle == 0 {
		return false
	}
	closeHandle.Call(handle)
	return true
}
