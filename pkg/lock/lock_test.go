package lock

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestAcquire_Success(t *testing.T) {
	// Ensure no stale lock
	lockPath := getLockPath()
	os.Remove(lockPath)

	holdingPID, err := Acquire()
	if err != nil {
		t.Fatalf("Acquire() failed: %v", err)
	}
	if holdingPID != 0 {
		t.Errorf("holdingPID = %d, want 0", holdingPID)
	}

	// Clean up
	Release()
}

func TestAcquire_AlreadyHeld(t *testing.T) {
	// Clean up first
	lockPath := getLockPath()
	os.Remove(lockPath)

	// First acquire should succeed
	_, err := Acquire()
	if err != nil {
		t.Fatalf("First Acquire() failed: %v", err)
	}

	// Second acquire should fail with current process PID
	holdingPID, err := Acquire()
	if err == nil {
		t.Fatal("Second Acquire() should have failed")
	}
	if holdingPID != os.Getpid() {
		t.Errorf("holdingPID = %d, want %d (current process)", holdingPID, os.Getpid())
	}

	// Clean up
	Release()
}

func TestRelease_Success(t *testing.T) {
	// Clean up first
	lockPath := getLockPath()
	os.Remove(lockPath)

	// Acquire then release
	_, err := Acquire()
	if err != nil {
		t.Fatalf("Acquire() failed: %v", err)
	}

	err = Release()
	if err != nil {
		t.Fatalf("Release() failed: %v", err)
	}

	// Lock file should be gone
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("Lock file should have been removed")
	}
}

func TestIsHeld_NotHeld(t *testing.T) {
	// Clean up first
	lockPath := getLockPath()
	os.Remove(lockPath)

	held, pid, err := IsHeld()
	if err != nil {
		t.Fatalf("IsHeld() failed: %v", err)
	}
	if held {
		t.Error("IsHeld() = true, want false")
	}
	if pid != 0 {
		t.Errorf("pid = %d, want 0", pid)
	}
}

func TestIsHeld_Held(t *testing.T) {
	// Clean up first
	lockPath := getLockPath()
	os.Remove(lockPath)

	// Acquire lock
	_, err := Acquire()
	if err != nil {
		t.Fatalf("Acquire() failed: %v", err)
	}

	// Check if held
	held, pid, err := IsHeld()
	if err != nil {
		t.Fatalf("IsHeld() failed: %v", err)
	}
	if !held {
		t.Error("IsHeld() = false, want true")
	}
	if pid != os.Getpid() {
		t.Errorf("pid = %d, want %d", pid, os.Getpid())
	}

	// Clean up
	Release()
}

func TestAcquire_StaleLock(t *testing.T) {
	// Clean up first
	lockPath := getLockPath()
	os.Remove(lockPath)

	// Create a stale lock with a non-existent PID
	staleLock := LockFile{
		PID:       99999, // Very unlikely to exist
		StartTime: time.Now(),
	}
	data, _ := json.Marshal(staleLock)
	os.WriteFile(lockPath, data, 0644)

	// Acquire should succeed (stale lock removed)
	holdingPID, err := Acquire()
	if err != nil {
		t.Fatalf("Acquire() failed with stale lock: %v", err)
	}
	if holdingPID != 0 {
		t.Errorf("holdingPID = %d, want 0", holdingPID)
	}

	// Verify new lock is ours
	held, pid, _ := IsHeld()
	if !held || pid != os.Getpid() {
		t.Error("Lock should be held by current process after acquiring")
	}

	// Clean up
	Release()
}
