//go:build !windows

package lock

import (
	"os"
	"syscall"
)

// isProcessRunning checks if a process with the given PID is running.
// On Unix, we use Signal(0) which performs error checking without sending a signal.
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal(0) checks if process exists without sending a signal
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
