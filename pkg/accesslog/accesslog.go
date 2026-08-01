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

package accesslog

import (
	"encoding/json"
	"time"

	"k8s.io/klog/v2"
)

type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

// Level 控制成功访问日志是否输出（失败始终输出）。
// 与 klog -v / log.verbosity 无关。
type Level string

const (
	LevelError Level = "error"
	LevelInfo  Level = "info"
	LevelDebug Level = "debug"
)

// SuccessTier 表示成功访问事件的级别档位。
// info：HTTP 请求、常规定时任务；debug：高频同步类任务。
type SuccessTier string

const (
	TierInfo  SuccessTier = "info"
	TierDebug SuccessTier = "debug"
)

type Options struct {
	Format Format
	Level  Level
	SQL    bool
}

func DefaultOptions() Options {
	return Options{
		Format: FormatJSON,
		Level:  LevelInfo,
	}
}

// AllowSuccess 按 log.level 过滤成功访问日志：
// error 抑制全部成功；info 仅 info 档；debug 允许 info 与 debug 档。
func (o Options) AllowSuccess(tier SuccessTier) bool {
	level := o.Level
	if level == "" {
		level = LevelInfo
	}
	switch level {
	case LevelError:
		return false
	case LevelDebug:
		return true
	default: // info
		return tier != TierDebug
	}
}

// Emit 输出一条访问日志（HTTP 等）。非 error 使用 klog.Info / InfoS。
func Emit(opts Options, msg, level string, fields map[string]interface{}, err error) {
	emit(opts, 0, msg, level, fields, err)
}

// EmitV 输出访问日志；非 error 时走 klog.V(v)（定时任务成功日志使用）。
func EmitV(opts Options, v klog.Level, msg, level string, fields map[string]interface{}, err error) {
	emit(opts, v, msg, level, fields, err)
}

func emit(opts Options, v klog.Level, msg, level string, fields map[string]interface{}, err error) {
	format := opts.Format
	if format == "" {
		format = FormatJSON
	}

	if format == FormatText {
		kvs := make([]interface{}, 0, len(fields)*2)
		for k, val := range fields {
			if level == "error" && k == "error" {
				continue
			}
			kvs = append(kvs, k, val)
		}
		if level == "error" {
			klog.ErrorS(err, msg, kvs...)
			return
		}
		if v > 0 {
			klog.V(v).InfoS(msg, kvs...)
			return
		}
		klog.InfoS(msg, kvs...)
		return
	}

	payload := make(map[string]interface{}, len(fields)+3)
	for k, val := range fields {
		payload[k] = val
	}
	payload["msg"] = msg
	payload["level"] = level
	payload["ts"] = time.Now().Format(time.RFC3339Nano)
	b, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		klog.Errorf("marshal access log failed: %v", marshalErr)
		return
	}
	line := string(b)
	if level == "error" {
		klog.Error(line)
		return
	}
	if v > 0 {
		klog.V(v).Info(line)
		return
	}
	klog.Info(line)
}
