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

package config

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MatusOllah/slogcolor"
	"github.com/fatih/color"
	"github.com/intel/aog/internal/client"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/version"
	"github.com/spf13/pflag"
)

const (
	// Log levels
	LogLevelDebug = "debug"
	LogLevelWarn  = "warn"
	LogLevelError = "error"

	// Default configurations
	DefaultLogLevel = "ERROR"
	DefaultVerbose  = "info"
	DefaultRootDir  = "./"

	// Database types
	DatastoreSQLite = "sqlite"

	// Database file
	DefaultDatabaseFile = "aog.db"

	// Directory names
	LogsDirectory = "logs"

	// File names
	ServerLogFile  = "server.log"
	ConsoleLogFile = "console.log"

	// Time formats
	DefaultTimeFormat = "2006-01-02 15:04:05"

	// Log file expiration in days
	DefaultLogExpireDays = 7

	// Environment variable keys
	EnvAOGHost                = "AOG_HOST"
	EnvModelIdleTimeout       = "AOG_MODEL_IDLE_TIMEOUT"
	EnvModelCleanupInterval   = "AOG_MODEL_CLEANUP_INTERVAL"
	EnvLocalModelQueueSize    = "AOG_LOCAL_MODEL_QUEUE_SIZE"
	EnvLocalModelQueueTimeout = "AOG_LOCAL_MODEL_QUEUE_TIMEOUT"
)

var GlobalEnvironment *AOGEnvironment

type AOGEnvironment struct {
	ApiHost                string        // host
	Datastore              string        // path to the datastore
	DatastoreType          string        // type of the datastore
	Verbose                string        // debug, info or warn
	RootDir                string        // root directory for all assets such as config files
	APIVersion             string        // version of this core app layer (gateway etc.)
	SpecVersion            string        // version of the core specification this app layer supports
	LogDir                 string        // logs dir
	LogHTTP                string        // path to the http log
	LogLevel               string        // log level
	LogFileExpireDays      int           // log file expiration time
	ConsoleLog             string        // aog server console log path
	ModelIdleTimeout       time.Duration // model idle timeout duration
	ModelCleanupInterval   time.Duration // model cleanup check interval
	LocalModelQueueSize    int           // local model queue size
	LocalModelQueueTimeout time.Duration // local model queue timeout
}

var (
	once         sync.Once
	envSingleton *AOGEnvironment
)

type AOGClient struct {
	client.Client
}

func NewAOGClient() *AOGClient {
	return &AOGClient{
		Client: *client.NewClient(Host(), http.DefaultClient),
	}
}

// Host returns the scheme and host. Host can be configured via the AOG_HOST environment variable.
// Default is scheme host and host "127.0.0.1:16688"
func Host() *url.URL {
	defaultPort := constants.DefaultHTTPPort

	s := strings.TrimSpace(Var(EnvAOGHost))
	scheme, hostport, ok := strings.Cut(s, "://")
	switch {
	case !ok:
		scheme, hostport = types.ProtocolHTTP, s
	case scheme == types.ProtocolHTTP:
		defaultPort = constants.DefaultHTTPPort80
	case scheme == types.ProtocolHTTPS:
		defaultPort = constants.DefaultHTTPSPort
	}

	hostport, path, _ := strings.Cut(hostport, "/")
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		host, port = constants.DefaultHost, defaultPort
		if ip := net.ParseIP(strings.Trim(hostport, "[]")); ip != nil {
			host = ip.String()
		} else if hostport != "" {
			host = hostport
		}
	}

	if n, err := strconv.ParseInt(port, 10, 32); err != nil || n > 65535 || n < 0 {
		slog.Warn("invalid port, using default", "port", port, "default", defaultPort)
		port = defaultPort
	}

	return &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(host, port),
		Path:   path,
	}
}

// Var returns an environment variable stripped of leading and trailing quotes or spaces
func Var(key string) string {
	return strings.Trim(strings.TrimSpace(os.Getenv(key)), "\"'")
}

func NewAOGEnvironment() *AOGEnvironment {
	once.Do(func() {
		env := AOGEnvironment{
			ApiHost:                constants.DefaultHost + ":" + constants.DefaultHTTPPort,
			Datastore:              DefaultDatabaseFile,
			DatastoreType:          DatastoreSQLite,
			LogDir:                 LogsDirectory,
			LogHTTP:                ServerLogFile,
			LogLevel:               DefaultLogLevel,
			LogFileExpireDays:      DefaultLogExpireDays,
			Verbose:                DefaultVerbose,
			RootDir:                DefaultRootDir,
			APIVersion:             version.AOGVersion,
			SpecVersion:            version.AOGVersion,
			ConsoleLog:             ConsoleLogFile,
			ModelIdleTimeout:       5 * time.Minute,  // Default 5 minutes
			ModelCleanupInterval:   1 * time.Minute,  // Default 1 minute
			LocalModelQueueSize:    10,               // Default queue size 10
			LocalModelQueueTimeout: 30 * time.Second, // Default queue timeout 30 seconds
		}

		// Read model management configuration from environment variables
		if idleTimeoutStr := Var(EnvModelIdleTimeout); idleTimeoutStr != "" {
			if duration, err := time.ParseDuration(idleTimeoutStr); err == nil {
				env.ModelIdleTimeout = duration
			} else {
				slog.Warn("Invalid model idle timeout, using default", "value", idleTimeoutStr, "default", env.ModelIdleTimeout)
			}
		}

		if cleanupIntervalStr := Var(EnvModelCleanupInterval); cleanupIntervalStr != "" {
			if duration, err := time.ParseDuration(cleanupIntervalStr); err == nil {
				env.ModelCleanupInterval = duration
			} else {
				slog.Warn("Invalid model cleanup interval, using default", "value", cleanupIntervalStr, "default", env.ModelCleanupInterval)
			}
		}

		// Read local model queue configuration from environment variables
		if queueSizeStr := Var(EnvLocalModelQueueSize); queueSizeStr != "" {
			if size, err := strconv.Atoi(queueSizeStr); err == nil && size > 0 {
				env.LocalModelQueueSize = size
			} else {
				slog.Warn("Invalid local model queue size, using default", "value", queueSizeStr, "default", env.LocalModelQueueSize)
			}
		}

		if queueTimeoutStr := Var(EnvLocalModelQueueTimeout); queueTimeoutStr != "" {
			if duration, err := time.ParseDuration(queueTimeoutStr); err == nil {
				env.LocalModelQueueTimeout = duration
			} else {
				slog.Warn("Invalid local model queue timeout, using default", "value", queueTimeoutStr, "default", env.LocalModelQueueTimeout)
			}
		}

		// Read model management configuration from environment variables
		if idleTimeoutStr := Var(EnvModelIdleTimeout); idleTimeoutStr != "" {
			if duration, err := time.ParseDuration(idleTimeoutStr); err == nil {
				env.ModelIdleTimeout = duration
			} else {
				slog.Warn("Invalid model idle timeout, using default", "value", idleTimeoutStr, "default", env.ModelIdleTimeout)
			}
		}

		if cleanupIntervalStr := Var(EnvModelCleanupInterval); cleanupIntervalStr != "" {
			if duration, err := time.ParseDuration(cleanupIntervalStr); err == nil {
				env.ModelCleanupInterval = duration
			} else {
				slog.Warn("Invalid model cleanup interval, using default", "value", cleanupIntervalStr, "default", env.ModelCleanupInterval)
			}
		}

		// Read local model queue configuration from environment variables
		if queueSizeStr := Var(EnvLocalModelQueueSize); queueSizeStr != "" {
			if size, err := strconv.Atoi(queueSizeStr); err == nil && size > 0 {
				env.LocalModelQueueSize = size
			} else {
				slog.Warn("Invalid local model queue size, using default", "value", queueSizeStr, "default", env.LocalModelQueueSize)
			}
		}

		if queueTimeoutStr := Var(EnvLocalModelQueueTimeout); queueTimeoutStr != "" {
			if duration, err := time.ParseDuration(queueTimeoutStr); err == nil {
				env.LocalModelQueueTimeout = duration
			} else {
				slog.Warn("Invalid local model queue timeout, using default", "value", queueTimeoutStr, "default", env.LocalModelQueueTimeout)
			}
		}

		var err error
		env.RootDir, err = utils.GetAOGDataDir()
		if err != nil {
			panic("[Init Env] get user dir failed: " + err.Error())
		}
		env.Datastore = filepath.Join(env.RootDir, env.Datastore)
		env.LogDir = filepath.Join(env.RootDir, env.LogDir)
		env.LogHTTP = filepath.Join(env.LogDir, env.LogHTTP)
		env.ConsoleLog = filepath.Join(env.LogDir, env.ConsoleLog)
		if err := os.MkdirAll(env.LogDir, 0o750); err != nil {
			panic("[Init Env] create logs path : " + err.Error())
		}

		envSingleton = &env
	})
	return envSingleton
}

// FlagSets Define a struct to hold the flag sets and their order
type FlagSets struct {
	Order    []string
	FlagSets map[string]*pflag.FlagSet
}

// NewFlagSets Initialize the FlagSets struct
func NewFlagSets() *FlagSets {
	return &FlagSets{
		Order:    []string{},
		FlagSets: make(map[string]*pflag.FlagSet),
	}
}

// AddFlagSet Add a flag set to the struct and maintain the order
func (fs *FlagSets) AddFlagSet(name string, flagSet *pflag.FlagSet) {
	if _, exists := fs.FlagSets[name]; !exists {
		fs.Order = append(fs.Order, name)
	}
	fs.FlagSets[name] = flagSet
}

// GetFlagSet Get the flag set by name, creating it if it doesn't exist
func (fs *FlagSets) GetFlagSet(name string) *pflag.FlagSet {
	if _, exists := fs.FlagSets[name]; !exists {
		fs.FlagSets[name] = pflag.NewFlagSet(name, pflag.ExitOnError)
		fs.Order = append(fs.Order, name)
	}
	return fs.FlagSets[name]
}

// Flags returns the flag sets for the AOGEnvironment.
func (s *AOGEnvironment) Flags() *FlagSets {
	fss := NewFlagSets()
	fs := fss.GetFlagSet("generic")
	fs.StringVar(&s.ApiHost, "app-host", s.ApiHost, "API host")
	fs.StringVar(&s.Verbose, "verbose", s.Verbose, "Log verbosity level")
	return fss
}

func (s *AOGEnvironment) SetSlogColor() {
	opts := slogcolor.DefaultOptions
	if s.Verbose == LogLevelDebug {
		opts.Level = slog.LevelDebug
	} else if s.Verbose == LogLevelWarn {
		opts.Level = slog.LevelWarn
	} else {
		opts.Level = slog.LevelInfo
	}
	opts.SrcFileMode = slogcolor.Nop
	opts.MsgColor = color.New(color.FgHiYellow)

	slog.SetDefault(slog.New(slogcolor.NewHandler(os.Stderr, opts)))
	_, _ = color.New(color.FgHiCyan).Println(">>>>>> AOG Open Gateway Starting : " + time.Now().Format(DefaultTimeFormat) + "\n\n")
	defer func() {
		_, _ = color.New(color.FgHiGreen).Println("\n\n<<<<<< AOG Open Gateway Stopped : " + time.Now().Format(DefaultTimeFormat))
	}()
}
