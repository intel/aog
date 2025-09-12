//go:build windows

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
	"time"

	"github.com/intel/aog/internal/logger"
)

// WindowsProcess implements PlatformProcess for Windows
type WindowsProcess struct{}

// newPlatformProcess creates a new platform-specific process implementation
func newPlatformProcess() PlatformProcess {
	return &WindowsProcess{}
}

// StartProcess starts a process with given command
func (p *WindowsProcess) StartProcess(cmd *exec.Cmd, mode StartMode) (*ProcessHandle, error) {
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
func (p *WindowsProcess) startForeground(cmd *exec.Cmd) (*ProcessHandle, error) {
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

// startBackground starts process in background mode
func (p *WindowsProcess) startBackground(cmd *exec.Cmd) (*ProcessHandle, error) {
	// For Windows background mode, we can start the process directly
	// and let it run in the background
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start background process: %v", err)
	}

	return &ProcessHandle{
		Process: cmd,
		PID:     cmd.Process.Pid,
	}, nil
}

// GracefulShutdown gracefully shuts down a process
func (p *WindowsProcess) GracefulShutdown(ctx context.Context, handle *ProcessHandle) error {
	if handle == nil || handle.PID <= 0 {
		return fmt.Errorf("invalid process handle")
	}

	// Try to gracefully terminate using taskkill
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(handle.PID))
	err := cmd.Run()
	if err != nil {
		logger.EngineLogger.Warn(fmt.Sprintf("[Process] Failed to gracefully terminate PID %d: %v", handle.PID, err))
		// Fallback to force kill
		return p.forceKill(handle.PID)
	}

	// Wait for process to exit gracefully with timeout
	timeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	// Check if process has exited
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-ticker.C:
			if !p.isProcessRunningByPID(handle.PID) {
				return nil // Process has exited
			}
		case <-timeoutTimer.C:
			logger.EngineLogger.Warn(fmt.Sprintf("[Process] Graceful shutdown timeout for PID %d, force killing", handle.PID))
			return p.forceKill(handle.PID)
		case <-ctx.Done():
			logger.EngineLogger.Warn(fmt.Sprintf("[Process] Context cancelled, force killing PID %d", handle.PID))
			return p.forceKill(handle.PID)
		}
	}
}

// KillProcess forcefully kills a process
func (p *WindowsProcess) KillProcess(handle *ProcessHandle) error {
	if handle == nil || handle.PID <= 0 {
		return fmt.Errorf("invalid process handle")
	}

	return p.forceKill(handle.PID)
}

// forceKill forcefully kills a process using taskkill /F
func (p *WindowsProcess) forceKill(pid int) error {
	// Use taskkill /F to force terminate the process and all child processes
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	err := cmd.Run()
	if err != nil {
		// Check if the error is because process doesn't exist
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not running") {
			return nil // Process is already dead
		}
		return fmt.Errorf("failed to force kill process %d: %v", pid, err)
	}

	return nil
}

// IsProcessRunning checks if a process is running
func (p *WindowsProcess) IsProcessRunning(handle *ProcessHandle) bool {
	if handle == nil || handle.PID <= 0 {
		return false
	}

	return p.isProcessRunningByPID(handle.PID)
}

// isProcessRunningByPID checks if a process is running by PID
func (p *WindowsProcess) isProcessRunningByPID(pid int) bool {
	// Use tasklist to check if process exists
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid))
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	outputStr := string(output)
	// If the output contains the PID, the process is running
	return strings.Contains(outputStr, strconv.Itoa(pid))
}
