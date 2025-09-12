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
	"os/exec"
	"time"
)

// ProcessStatus represents the status of a process
type ProcessStatus int

const (
	ProcessStatusStopped ProcessStatus = iota
	ProcessStatusStarting
	ProcessStatusRunning
	ProcessStatusStopping
	ProcessStatusError
)

func (s ProcessStatus) String() string {
	switch s {
	case ProcessStatusStopped:
		return "stopped"
	case ProcessStatusStarting:
		return "starting"
	case ProcessStatusRunning:
		return "running"
	case ProcessStatusStopping:
		return "stopping"
	case ProcessStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// StartMode defines how the process should be started
type StartMode string

const (
	StartModeForeground StartMode = "foreground" // 前台模式，显示输出
	StartModeBackground StartMode = "background" // 后台模式，静默运行
)

// StartConfig contains essential configuration for starting a process
type StartConfig struct {
	Name        string        // 进程名称
	ExecPath    string        // 可执行文件路径
	Args        []string      // 命令行参数
	Env         []string      // 环境变量
	WorkDir     string        // 工作目录
	Mode        StartMode     // 启动模式
	Timeout     time.Duration // 启动超时
	HealthCheck func() error  // 健康检查函数
}

// ProcessInfo contains information about a process
type ProcessInfo struct {
	PID    int           `json:"pid"`
	Status ProcessStatus `json:"status"`
	Name   string        `json:"name"`
}

// ProcessManager defines the core interface for process management
type ProcessManager interface {
	// Core lifecycle operations
	Start(ctx context.Context, config *StartConfig) error
	Stop(ctx context.Context) error
	Kill() error

	// Status queries
	IsRunning() bool
	Status() ProcessStatus
	PID() int
}

// ProcessInfoProvider provides detailed process information (optional extension)
type ProcessInfoProvider interface {
	Info() ProcessInfo
}

// PlatformProcess defines platform-specific process operations
type PlatformProcess interface {
	// StartProcess starts a process with given command
	StartProcess(cmd *exec.Cmd, mode StartMode) (*ProcessHandle, error)

	// GracefulShutdown gracefully shuts down a process
	GracefulShutdown(ctx context.Context, handle *ProcessHandle) error

	// KillProcess forcefully kills a process
	KillProcess(handle *ProcessHandle) error

	// IsProcessRunning checks if a process is running
	IsProcessRunning(handle *ProcessHandle) bool
}

// ProcessHandle represents a handle to a running process
type ProcessHandle struct {
	Process *exec.Cmd
	PID     int
}
