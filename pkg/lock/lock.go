// Package lock provides process-level locking for nim to prevent concurrent
// invocations. Only one nim process can hold the lock at a time.
package lock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LockFile represents the structure stored in the lock file.
type LockFile struct {
	PID       int       `json:"pid"`
	StartTime time.Time `json:"start_time"`
}

var (
	// currentLockPath is the path to the lock file for this process
	currentLockPath string
)

// Acquire attempts to acquire the process lock.
// Returns the PID of the holding process and an error if lock is held.
func Acquire() (holdingPID int, err error) {
	lockPath := getLockPath()
	currentLockPath = lockPath

	// Try to read existing lock file
	existing, err := readLockFile(lockPath)
	if err == nil && existing != nil {
		// Lock file exists, check if process is still running
		if isProcessRunning(existing.PID) {
			return existing.PID, fmt.Errorf("lock held by process %d", existing.PID)
		}
		// Process not running, remove stale lock
		os.Remove(lockPath)
	}

	// Create new lock file
	lock := LockFile{
		PID:       os.Getpid(),
		StartTime: time.Now(),
	}

	data, err := json.Marshal(lock)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal lock file: %w", err)
	}

	// Write atomically using temp file + rename
	tmpPath := lockPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return 0, fmt.Errorf("failed to write lock file: %w", err)
	}

	if err := os.Rename(tmpPath, lockPath); err != nil {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("failed to acquire lock: %w", err)
	}

	return 0, nil
}

// Release releases the process lock.
func Release() error {
	if currentLockPath == "" {
		return nil // No lock acquired
	}

	// Verify we still own the lock
	existing, err := readLockFile(currentLockPath)
	if err != nil {
		return nil // Lock file doesn't exist or can't be read
	}

	if existing.PID != os.Getpid() {
		return fmt.Errorf("lock is owned by different process (PID %d)", existing.PID)
	}

	return os.Remove(currentLockPath)
}

// IsHeld checks if the lock is currently held by a running process.
// Returns true and the PID if held, false and 0 if not held.
func IsHeld() (bool, int, error) {
	lockPath := getLockPath()
	existing, err := readLockFile(lockPath)
	if err != nil {
		return false, 0, nil // Lock file doesn't exist
	}

	if isProcessRunning(existing.PID) {
		return true, existing.PID, nil
	}

	// Stale lock, clean it up
	os.Remove(lockPath)
	return false, 0, nil
}

// getLockPath returns the path to the lock file.
// Uses /tmp/nim-<username>.lock for Unix-like systems.
func getLockPath() string {
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	if username == "" {
		username = "unknown"
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("nim-%s.lock", username))
}

// readLockFile reads and parses the lock file.
func readLockFile(path string) (*LockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lock LockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	return &lock, nil
}
