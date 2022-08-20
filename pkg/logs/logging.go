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

package logs

import "path/filepath"

type Configuration struct {
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
}

var Logger LoggerInterface
var AccessLog LoggerInterface

func Register(logDir string, logLevel string) {
	if len(logLevel) == 0 {
		logLevel = "INFO"
	}

	AccessLog, _ = NewZapLogger(Configuration{
		LogFile:          filepath.Join(logDir, "access.log"),
		LogLevel:         logLevel,
		RotateMaxSize:    500,
		RotateMaxAge:     7,
		RotateMaxBackups: 3,
	})

	Logger, _ = NewZapLogger(Configuration{
		LogFile:          filepath.Join(logDir, "gopixiu.log"),
		LogLevel:         logLevel,
		RotateMaxSize:    500,
		RotateMaxAge:     7,
		RotateMaxBackups: 3,
	})
}
