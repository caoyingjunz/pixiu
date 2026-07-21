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
	"errors"
	"flag"
	"fmt"
	"strconv"
	"sync"
	"time"

	klog "github.com/sirupsen/logrus"
	k8slog "k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
)

var once sync.Once

type LogFormat string

const (
	LogFormatJson LogFormat = "json"
	LogFormatText LogFormat = "text"
)

var ErrInvalidLogFormat = errors.New("invalid log format")

type LogLevel = klog.Level

// Providing 3 log levels now.
const (
	ErrorLevel LogLevel = klog.ErrorLevel
	InfoLevel  LogLevel = klog.InfoLevel
	DebugLevel LogLevel = klog.DebugLevel
)

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

// Init initializes application logging (logrus) and k8s klog verbosity once.
// Priority for -v: explicitly set CLI flag (cliVerbositySet=true) > config log.verbosity > default 0.
func (o *LogOptions) Init(cliVerbositySet bool) {
	if o == nil {
		return
	}
	once.Do(func() {
		if o.LogLevel == 0 {
			o.LogLevel = InfoLevel
		}
		klog.SetLevel(o.LogLevel)
		switch o.LogFormat {
		case LogFormatJson:
			klog.SetFormatter(&klog.JSONFormatter{TimestampFormat: time.RFC3339Nano})
		default:
			klog.SetFormatter(&klog.TextFormatter{FullTimestamp: true, TimestampFormat: time.RFC3339Nano})
		}

		o.applyKlogVerbosity(cliVerbositySet)

		k8slog.Infof("logging initialized: format=%s level=%s verbosity=%s sql=%t",
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

type Logger struct {
	startTime time.Time
	logSQL    bool
	logEntry  *klog.Entry
}

func NewLogger(cfg *LogOptions) *Logger {
	return &Logger{
		startTime: time.Now(),
		logSQL:    cfg.LogSQL,
		logEntry:  klog.NewEntry(klog.StandardLogger()),
	}
}

func (l *Logger) WithLogField(key string, value interface{}) {
	l.logEntry = l.logEntry.WithField(key, value)
}

func (l *Logger) WithLogFields(fields map[string]interface{}) {
	l.logEntry = l.logEntry.WithFields(fields)
}

func (l *Logger) Log(ctx context.Context, level LogLevel, err error) {
	fields := make(map[string]interface{})
	if l.logSQL {
		if sqls := db.GetSQLs(ctx); len(sqls) > 0 {
			fields["sqls"] = sqls
		}
	}
	fields["latency"] = fmt.Sprintf("%dµs", time.Since(l.startTime).Microseconds())

	if err != nil {
		fields["error"] = err
		l.logEntry.WithFields(fields).Error(FailMsg)
		return
	}

	switch level {
	case DebugLevel:
		l.logEntry.WithFields(fields).Debug(SuccessMsg)
	case InfoLevel:
		l.logEntry.WithFields(fields).Info(SuccessMsg)
	}
}
