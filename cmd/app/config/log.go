/*
Copyright 2024 The Pixiu Authors.

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

package config

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/accesslog"
)

var logInitOnce sync.Once

type LogFormat string

const (
	LogFormatJson LogFormat = "json"
	LogFormatText LogFormat = "text"
)

var ErrInvalidLogFormat = errors.New("invalid log format")

// LogLevel 请求/任务访问日志级别（与 config.yaml log.level 对应）。
type LogLevel string

const (
	ErrorLevel LogLevel = "error"
	InfoLevel  LogLevel = "info"
	DebugLevel LogLevel = "debug"
)

func (l LogLevel) String() string {
	if l == "" {
		return string(InfoLevel)
	}
	return string(l)
}

func parseLogLevel(s string) (LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", string(InfoLevel):
		return InfoLevel, nil
	case string(ErrorLevel):
		return ErrorLevel, nil
	case string(DebugLevel):
		return DebugLevel, nil
	default:
		return "", fmt.Errorf("invalid log level %q", s)
	}
}

// UnmarshalYAML 兼容配置中的 level: info|error|debug
func (l *LogLevel) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	parsed, err := parseLogLevel(s)
	if err != nil {
		return err
	}
	*l = parsed
	return nil
}

// LogOptions 日志配置：verbosity 作用于全局 klog；format/level/sql 作用于请求与定时任务访问日志。
type LogOptions struct {
	LogFormat    LogFormat `yaml:"format"`
	LogSQL       bool      `yaml:"sql"`
	LogLevel     LogLevel  `yaml:"level"`
	LogVerbosity uint      `yaml:"verbosity"`
}

func DefaultLogOptions() LogOptions {
	return LogOptions{
		LogFormat:    LogFormatJson,
		LogSQL:       false,
		LogLevel:     InfoLevel,
		LogVerbosity: 0,
	}
}

func (o *LogOptions) Valid() error {
	switch o.LogFormat {
	case "", LogFormatJson, LogFormatText:
		return nil
	default:
		return ErrInvalidLogFormat
	}
}

// AccessOptions 返回请求/任务访问日志选项（与 klog verbosity 无关）。
func (o *LogOptions) AccessOptions() accesslog.Options {
	if o == nil {
		return accesslog.DefaultOptions()
	}
	format := accesslog.FormatJSON
	if o.LogFormat == LogFormatText {
		format = accesslog.FormatText
	}
	level := accesslog.LevelInfo
	switch o.LogLevel {
	case ErrorLevel:
		level = accesslog.LevelError
	case DebugLevel:
		level = accesslog.LevelDebug
	}
	return accesslog.Options{
		Format: format,
		Level:  level,
		SQL:    o.LogSQL,
	}
}

// Init 应用 log.verbosity 到 klog -v。
// Priority: explicitly set CLI -v > config log.verbosity > default 0.
func (o *LogOptions) Init(cliVerbositySet bool) {
	if o == nil {
		return
	}
	logInitOnce.Do(func() {
		if o.LogLevel == "" {
			o.LogLevel = InfoLevel
		}
		if o.LogFormat == "" {
			o.LogFormat = LogFormatJson
		}
		if !cliVerbositySet {
			_ = flag.Set("v", strconv.FormatUint(uint64(o.LogVerbosity), 10))
		}
		verbosity := "0"
		if f := flag.CommandLine.Lookup("v"); f != nil {
			verbosity = f.Value.String()
		}
		klog.Infof("logging initialized: backend=klog/v2 format=%s level=%s verbosity=%s sql=%t",
			o.LogFormat, o.LogLevel.String(), verbosity, o.LogSQL)
	})
}
