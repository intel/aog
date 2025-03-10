// Apache v2 license
// Copyright (C) 2024 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package opac

// opac is a package that provides implementation for concepts and mechanisms of
// OPAC specification. However, it does not provide the actual implementation of
// actually receiving and processing requests, and sending out responses etc.
// The gateway package is responsible to really handling network related.

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/k0kubun/pp/v3"
)

// Hold all global settings and options for OPAC
type Environment struct {
	Verbose         string // debug, info or warn
	RootDir         string // root directory for all assets such as config files
	WorkDir         string // current work directory
	ConfigFile      string // configuration file
	ForceReload     bool   // force reload many configuration files when touched
	APILayerVersion string // version of this opac api layer (gateway etc.)
	SpecVersion     string // version of the opac specification this api layer supports
}

// Convert relative path to absolute path from the passed in base directory
// No change if the passed in path is already an absolute path
func (env Environment) GetAbsolutePath(p string, base string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(base, p))
}

var once sync.Once
var envSingleton *Environment

func GetEnv() *Environment {
	once.Do(func() {
		env := Environment{}
		cwd, err := os.Getwd()
		if err != nil {
			panic("[GetEnv] Failed to get current working directory")
		}
		env.WorkDir = cwd

		execPath, err := os.Executable()
		if err != nil {
			panic("[GetEnv] Failed to get executable path of OPAC")
		}
		execDir := filepath.Dir(execPath)
		execPath = filepath.ToSlash(execPath)
		if strings.Contains(execPath, "/go-build") { // running through 'go run'?
			slog.Debug("[GetEnv] Running through 'go run'")
			_, filename, _, ok := runtime.Caller(0)
			if !ok {
				panic("[GetEnv] No caller information")
			}
			env.RootDir = filepath.Dir(filepath.Dir(filename))
		} else {
			slog.Debug("[GetEnv] Running through the binary generated by 'go build'")
			env.RootDir = execDir
		}

		envSingleton = &env
	})
	return envSingleton
}

// Colorful pretty printers to help quick debug
func PPprint(a ...interface{}) (n int, err error) {
	return pp.Print(a...)
}

func PPprintf(format string, a ...interface{}) (n int, err error) {
	return pp.Printf(format, a...)
}

func PPprintln(a ...interface{}) (n int, err error) {
	return pp.Println(a...)
}

func PPsprint(a ...interface{}) string {
	return pp.Sprint(a...)
}

func PPsprintf(format string, a ...interface{}) string {
	return pp.Sprintf(format, a...)
}

func PPsprintln(a ...interface{}) string {
	return pp.Sprintln(a...)
}
