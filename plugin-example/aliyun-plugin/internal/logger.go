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

package internal

import "log"

type printfLogger struct{}

type LogConfig struct {
	LogLevel string
	LogPath  string
}

func (printfLogger) Info(format string, args ...interface{}) {
	log.Printf(format, args...)
}

var LogicLogger printfLogger

func InitLogger(LogConfig) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}
