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
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// ProcessManager simplified process manager
type ProcessManager struct {
	cmd       *exec.Cmd
	execPath  string
	host      string
	modelsDir string
}

// NewProcessManager creates process manager
func NewProcessManager(execPath, host, modelsDir string) *ProcessManager {
	return &ProcessManager{
		execPath:  execPath,
		host:      host,
		modelsDir: modelsDir,
	}
}

// Start starts ollama process
func (pm *ProcessManager) Start(mode string) error {
	// Check if service is already running
	if pm.HealthCheck() == nil {
		return nil // Already running
	}

	// Create start command
	pm.cmd = exec.Command(pm.execPath, "serve")

	// Set environment variables
	pm.cmd.Env = append(os.Environ(),
		"OLLAMA_HOST="+pm.host,
		"OLLAMA_MODELS="+pm.modelsDir,
	)

	// Platform-specific setup (implemented in process_unix.go / process_windows.go)
	pm.setupProcAttr()

	// daemon mode: run in background, no output
	if mode == "daemon" {
		pm.cmd.Stdout = nil
		pm.cmd.Stderr = nil
	}

	// Start process
	if err := pm.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ollama: %w", err)
	}

	// Wait for service to be ready (up to 30 seconds)
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		if pm.HealthCheck() == nil {
			return nil
		}
	}

	return fmt.Errorf("ollama service not ready after 30 seconds")
}

// Stop stops ollama process
func (pm *ProcessManager) Stop() error {
	if pm.cmd == nil || pm.cmd.Process == nil {
		return nil
	}

	// Platform-specific stop logic (implemented in process_unix.go / process_windows.go)
	return pm.stopProcess()
}

// HealthCheck performs health check
func (pm *ProcessManager) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "http://"+pm.host, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	return fmt.Errorf("unhealthy status: %d", resp.StatusCode)
}
