//go:build unix || darwin || linux
// +build unix darwin linux

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

package utils

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// setupProcAttr sets Unix platform-specific process attributes
func (pm *ProcessManager) setupProcAttr() {
	// Unix system: create new process group to ensure proper cleanup
	pm.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// stopProcess Unix platform process stop logic
func (pm *ProcessManager) stopProcess() error {
	// If there's a process started via Start(), stop it first
	if pm.cmd != nil && pm.cmd.Process != nil {
		return pm.stopManagedProcess()
	}

	// Otherwise find and stop all ollama serve processes
	return pm.stopOllamaByName()
}

// stopManagedProcess stops process started by current manager
func (pm *ProcessManager) stopManagedProcess() error {
	// Try graceful shutdown (SIGTERM)
	if err := pm.cmd.Process.Signal(syscall.SIGTERM); err == nil {
		// Wait up to 5 seconds
		done := make(chan error, 1)
		go func() {
			done <- pm.cmd.Wait()
		}()

		select {
		case <-done:
			return nil
		case <-time.After(5 * time.Second):
			// Timeout, proceed to force kill
		}
	}

	// Force kill
	if err := pm.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	pm.cmd.Wait()
	return nil
}

// stopOllamaByName finds and stops all ollama serve processes by process name
func (pm *ProcessManager) stopOllamaByName() error {
	// Use pgrep to find all ollama processes
	out, err := exec.Command("pgrep", "-f", "ollama serve").Output()
	if err != nil {
		// pgrep returns 1 if no processes found, this is normal
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil // No running ollama processes
		}
		return fmt.Errorf("failed to find ollama processes: %w", err)
	}

	// Parse PID list
	pids := string(out)
	if len(pids) == 0 {
		return nil // No processes
	}

	// Send SIGTERM signal to all ollama processes
	for _, pidStr := range strings.Split(strings.TrimSpace(pids), "\n") {
		if pidStr == "" {
			continue
		}

		// Use kill command to send SIGTERM
		if err := exec.Command("kill", "-TERM", pidStr).Run(); err != nil {
			// Process may have already exited, continue with other processes
			continue
		}
	}

	// Wait up to 5 seconds for graceful exit
	time.Sleep(5 * time.Second)

	// Check if there are still running processes, if yes then force kill
	out, err = exec.Command("pgrep", "-f", "ollama serve").Output()
	if err == nil && len(out) > 0 {
		// Still have running processes, force kill
		for _, pidStr := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if pidStr == "" {
				continue
			}
			exec.Command("kill", "-9", pidStr).Run()
		}
	}

	return nil
}
