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
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func TimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05"))
}

func NewZapLogger(c Configuration) (LoggerInterface, error) {
	var w io.Writer
	// 支持 标准输出，标准错误输出，和指定日志文件
	switch strings.ToLower(c.LogType) {
	case "stderr":
		w = os.Stderr
	case "file":
		w = &lumberjack.Logger{
			Filename:   c.LogFile,
			MaxSize:    c.RotateMaxSize,
			MaxAge:     c.RotateMaxAge,
			MaxBackups: c.RotateMaxBackups,
			Compress:   c.Compress,
		}
	default:
		w = os.Stdout
	}

	cfg := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}
	// 设置日志级别
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(c.LogLevel)); err != nil {
		return nil, err
	}

	core := zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), zapcore.NewMultiWriteSyncer(zapcore.AddSync(w)), level)

	var cores []zapcore.Core
	cores = append(cores, core)
	Tee := zapcore.NewTee(cores...)
	logger := zap.New(Tee, zap.AddCaller(), zap.AddCallerSkip(1))
	return &zapLogger{
		logger:    logger,
		writer:    w,
		verbosity: 0,
	}, nil
}

type zapLogger struct {
	logger    *zap.Logger
	writer    io.Writer
	verbosity int
}

func (l *zapLogger) Info(args ...interface{}) {
	l.logger.Info(fmt.Sprint(args...))
}

func (l *zapLogger) Infof(f string, args ...interface{}) {
	l.logger.Info(fmt.Sprintf(f, args...))
}

func (l *zapLogger) Error(args ...interface{}) {
	l.logger.Error(fmt.Sprint(args...))
}

func (l *zapLogger) Errorf(f string, args ...interface{}) {
	l.logger.Error(fmt.Sprintf(f, args...))
}

func (l *zapLogger) Warn(args ...interface{}) {
	l.logger.Warn(fmt.Sprint(args...))
}

func (l *zapLogger) Warnf(f string, args ...interface{}) {
	l.logger.Warn(fmt.Sprintf(f, args...))
}
