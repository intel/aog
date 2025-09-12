//go:build !windows

//*****************************************************************************
// Copyright 2024-2025 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//*****************************************************************************

package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/intel/aog/internal/logger"
)

// UnixProcess implements PlatformProcess for Unix-like systems
type UnixProcess struct{}

// newPlatformProcess creates a new platform-specific process implementation
func newPlatformProcess() PlatformProcess {
	return &UnixProcess{}
}

// StartProcess starts a process with given command
func (p *UnixProcess) StartProcess(cmd *exec.Cmd, mode StartMode) (*ProcessHandle, error) {
	switch mode {
	case StartModeForeground:
		return p.startForeground(cmd)
	case StartModeBackground:
		return p.startBackground(cmd)
	default:
		return p.startBackground(cmd)
	}
}

// startForeground starts process in foreground mode
func (p *UnixProcess) startForeground(cmd *exec.Cmd) (*ProcessHandle, error) {
	// Set up output streams
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %v", err)
	}

	return &ProcessHandle{
		Process: cmd,
		PID:     cmd.Process.Pid,
	}, nil
}

// startBackground starts process in background mode using nohup
func (p *UnixProcess) startBackground(cmd *exec.Cmd) (*ProcessHandle, error) {
	// For daemon mode, use nohup to run in background and get PID
	args := []string{"-c", fmt.Sprintf("nohup %s %s >/dev/null 2>&1 & echo $!",
		cmd.Path, strings.Join(cmd.Args[1:], " "))}

	nohupCmd := exec.Command("sh", args...)
	nohupCmd.Dir = cmd.Dir
	nohupCmd.Env = cmd.Env

	output, err := nohupCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to start background process: %v", err)
	}

	// Parse the output PID
	pidStr := strings.TrimSpace(string(output))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse daemon PID: %v", err)
	}

	// Find the actual process
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to find background process: %v", err)
	}

	// Create a dummy cmd for compatibility
	dummyCmd := &exec.Cmd{Process: process}

	return &ProcessHandle{
		Process: dummyCmd,
		PID:     pid,
	}, nil
}

// GracefulShutdown gracefully shuts down a process
func (p *UnixProcess) GracefulShutdown(ctx context.Context, handle *ProcessHandle) error {
	if handle == nil || handle.Process == nil || handle.Process.Process == nil {
		return fmt.Errorf("invalid process handle")
	}

	process := handle.Process.Process

	// Send SIGTERM signal for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		logger.EngineLogger.Warn(fmt.Sprintf("[Process] Failed to send SIGTERM to PID %d: %v", handle.PID, err))
		// If SIGTERM fails, try SIGKILL directly
		return process.Signal(syscall.SIGKILL)
	}

	// Wait for process to exit gracefully with timeout
	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	// Use context timeout or default 30 seconds
	timeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	select {
	case err := <-done:
		if err != nil {
			logger.EngineLogger.Debug(fmt.Sprintf("[Process] Process %d exited with error: %v", handle.PID, err))
		}
		return nil
	case <-time.After(timeout):
		logger.EngineLogger.Warn(fmt.Sprintf("[Process] Graceful shutdown timeout for PID %d, force killing", handle.PID))
		// Force kill if graceful shutdown times out
		return process.Signal(syscall.SIGKILL)
	case <-ctx.Done():
		logger.EngineLogger.Warn(fmt.Sprintf("[Process] Context cancelled, force killing PID %d", handle.PID))
		return process.Signal(syscall.SIGKILL)
	}
}

// KillProcess forcefully kills a process
func (p *UnixProcess) KillProcess(handle *ProcessHandle) error {
	if handle == nil || handle.Process == nil || handle.Process.Process == nil {
		return fmt.Errorf("invalid process handle")
	}

	process := handle.Process.Process

	// Send SIGKILL signal
	if err := process.Signal(syscall.SIGKILL); err != nil {
		// If the process is already dead, that's okay
		if strings.Contains(err.Error(), "process already finished") {
			return nil
		}
		return fmt.Errorf("failed to kill process %d: %v", handle.PID, err)
	}

	// Wait a bit for the process to be killed
	go func() {
		process.Wait()
	}()

	return nil
}

// IsProcessRunning checks if a process is running
func (p *UnixProcess) IsProcessRunning(handle *ProcessHandle) bool {
	if handle == nil || handle.PID <= 0 {
		return false
	}

	// Find process by PID
	process, err := os.FindProcess(handle.PID)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist or we don't have permission
		return false
	}

	return true
}
