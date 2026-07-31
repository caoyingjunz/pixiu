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

package log

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
)

var once sync.Once

type LogFormat string

const (
	LogFormatJson LogFormat = "json"
	LogFormatText LogFormat = "text"
)

var ErrInvalidLogFormat = errors.New("invalid log format")

// LogLevel 请求/任务日志级别（与 config.yaml log.level 对应）。
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

type LogOptions struct {
	LogFormat LogFormat `yaml:"format"`
	LogSQL    bool      `yaml:"sql"`
	LogLevel  LogLevel  `yaml:"level"`
	// LogVerbosity is the k8s.io/klog/v2 verbosity level, equivalent to the -v flag.
	// Default is 0. When both are set, an explicitly provided -v flag takes precedence.
	LogVerbosity uint `yaml:"verbosity"`
}

// DefaultLogOptions returns the default configs.
func DefaultLogOptions() *LogOptions {
	return &LogOptions{
		LogFormat:    LogFormatJson,
		LogSQL:       false,
		LogLevel:     InfoLevel,
		LogVerbosity: 0,
	}
}

func (o *LogOptions) Valid() error {
	switch o.LogFormat {
	case LogFormatJson, LogFormatText:
		return nil
	default:
		return ErrInvalidLogFormat
	}
}

// Init 初始化全局日志（仅 klog/v2）。
// Priority for -v: explicitly set CLI flag (cliVerbositySet=true) > config log.verbosity > default 0.
func (o *LogOptions) Init(cliVerbositySet bool) {
	if o == nil {
		return
	}
	once.Do(func() {
		if o.LogLevel == "" {
			o.LogLevel = InfoLevel
		}
		if o.LogFormat == "" {
			o.LogFormat = LogFormatJson
		}
		o.applyKlogVerbosity(cliVerbositySet)

		klog.Infof("logging initialized: backend=klog/v2 format=%s level=%s verbosity=%s sql=%t",
			o.LogFormat,
			o.LogLevel.String(),
			currentKlogVerbosity(),
			o.LogSQL)
	})
}

func (o *LogOptions) applyKlogVerbosity(cliVerbositySet bool) {
	if cliVerbositySet {
		return
	}
	_ = flag.Set("v", strconv.FormatUint(uint64(o.LogVerbosity), 10))
}

func currentKlogVerbosity() string {
	if f := flag.CommandLine.Lookup("v"); f != nil {
		return f.Value.String()
	}
	return "0"
}

const (
	SuccessMsg = "SUCCESS"
	ErrorMsg   = "ERROR"
	FailMsg    = "FAIL"
)

// Logger 基于 klog/v2 的请求/任务结构化日志封装。
type Logger struct {
	startTime time.Time
	logSQL    bool
	format    LogFormat
	minLevel  LogLevel
	fields    map[string]interface{}
}

func NewLogger(cfg *LogOptions) *Logger {
	format := LogFormatJson
	level := InfoLevel
	logSQL := false
	if cfg != nil {
		if cfg.LogFormat != "" {
			format = cfg.LogFormat
		}
		if cfg.LogLevel != "" {
			level = cfg.LogLevel
		}
		logSQL = cfg.LogSQL
	}
	return &Logger{
		startTime: time.Now(),
		logSQL:    logSQL,
		format:    format,
		minLevel:  level,
		fields:    make(map[string]interface{}),
	}
}

func (l *Logger) WithLogField(key string, value interface{}) {
	l.fields[key] = value
}

func (l *Logger) WithLogFields(fields map[string]interface{}) {
	for k, v := range fields {
		l.fields[k] = v
	}
}

func (l *Logger) enabled(level LogLevel) bool {
	switch l.minLevel {
	case ErrorLevel:
		// 仅错误路径会通过 err!=nil 打出；成功路径全部抑制
		return false
	case DebugLevel:
		return true
	default: // info
		return level != DebugLevel
	}
}

func (l *Logger) Log(ctx context.Context, level LogLevel, err error) {
	fields := make(map[string]interface{}, len(l.fields)+3)
	for k, v := range l.fields {
		fields[k] = v
	}
	if l.logSQL {
		if sqls := db.GetSQLs(ctx); len(sqls) > 0 {
			fields["sqls"] = sqls
		}
	}
	fields["latency"] = fmt.Sprintf("%dµs", time.Since(l.startTime).Microseconds())

	if err != nil {
		fields["error"] = err.Error()
		l.emit(FailMsg, "error", fields)
		return
	}
	if !l.enabled(level) {
		return
	}
	lvl := "info"
	if level == DebugLevel {
		lvl = "debug"
	}
	l.emit(SuccessMsg, lvl, fields)
}

func (l *Logger) emit(msg, level string, fields map[string]interface{}) {
	if l.format == LogFormatJson {
		payload := make(map[string]interface{}, len(fields)+3)
		for k, v := range fields {
			payload[k] = v
		}
		payload["msg"] = msg
		payload["level"] = level
		payload["ts"] = time.Now().Format(time.RFC3339Nano)
		b, err := json.Marshal(payload)
		if err != nil {
			klog.Errorf("marshal log payload failed: %v", err)
			return
		}
		line := string(b)
		if level == "error" {
			klog.Error(line)
		} else if level == "debug" {
			klog.V(2).Info(line)
		} else {
			klog.Info(line)
		}
		return
	}

	// text：使用 klog 结构化 InfoS/ErrorS
	kvs := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		if level == "error" && k == "error" {
			continue
		}
		kvs = append(kvs, k, v)
	}
	if level == "error" {
		var logErr error
		if e, ok := fields["error"].(string); ok && e != "" {
			logErr = errors.New(e)
		}
		klog.ErrorS(logErr, msg, kvs...)
		return
	}
	if level == "debug" {
		klog.V(2).InfoS(msg, kvs...)
		return
	}
	klog.InfoS(msg, kvs...)
}
