/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package log

import (
	"path/filepath"
	"strings"
)

type Configuration struct {
	LogType  string
	LogFile  string
	LogLevel string

	RotateMaxSize    int
	RotateMaxAge     int
	RotateMaxBackups int
	Compress         bool
}

type LoggerInterface interface {
	Info(args ...interface{})
	Infof(f string, args ...interface{})
	Error(args ...interface{})
	Errorf(f string, args ...interface{})
	Warn(args ...interface{})
	Warnf(f string, args ...interface{})
}

var (
	Logger    LoggerInterface
	AccessLog LoggerInterface
)

func Register(logType, logDir, logLevel string) {
	// 支持 INFO, WARN 和 ERROR，默认为 INFO
	Level := "info"
	if strings.ToLower(logLevel) == "error" {
		Level = "error"
	} else if strings.ToLower(logLevel) == "warn" {
		Level = "warn"
	}

	AccessLog, _ = NewZapLogger(Configuration{
		LogType:          logType,
		LogFile:          filepath.Join(logDir, "access.log"), // 使用文件类型时生效
		LogLevel:         "info",                              // access 的 log 只会有 info
		RotateMaxSize:    500,
		RotateMaxAge:     7,
		RotateMaxBackups: 3,
	})

	Logger, _ = NewZapLogger(Configuration{
		LogType:          logType,
		LogFile:          filepath.Join(logDir, "gopixiu.log"), // 使用文件类型时生效
		LogLevel:         Level,
		RotateMaxSize:    500,
		RotateMaxAge:     7,
		RotateMaxBackups: 3,
	})
}
