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

package jobmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
)

// AccessLogLevel controls whether a successful job run is recorded.
type AccessLogLevel string

const (
	AccessLogInfo  AccessLogLevel = "info"
	AccessLogDebug AccessLogLevel = "debug"
)

// AccessLogOptions mirrors config log.format / level / sql for job access logs.
type AccessLogOptions struct {
	Format string // json | text
	Level  string // error | info | debug
	SQL    bool
}

type JobContext struct {
	context.Context
	opts      *AccessLogOptions
	startTime time.Time
	fields    map[string]interface{}
}

func NewJobContext(name string, opts *AccessLogOptions) *JobContext {
	if opts == nil {
		opts = &AccessLogOptions{Format: "json", Level: "info"}
	}
	jc := &JobContext{
		Context:   db.WithDBContext(context.Background()),
		opts:      opts,
		startTime: time.Now(),
		fields:    map[string]interface{}{"job": name},
	}
	return jc
}

func (c *JobContext) WithLogFields(fields map[string]interface{}) {
	for k, v := range fields {
		c.fields[k] = v
	}
}

func (c *JobContext) Log(level AccessLogLevel, err error) {
	fields := make(map[string]interface{}, len(c.fields)+2)
	for k, v := range c.fields {
		fields[k] = v
	}
	if c.opts.SQL {
		if sqls := db.GetSQLs(c); len(sqls) > 0 {
			fields["sqls"] = sqls
		}
	}
	fields["latency"] = fmt.Sprintf("%dµs", time.Since(c.startTime).Microseconds())

	if err != nil {
		fields["error"] = err.Error()
		c.emit("FAIL", "error", fields, err)
		return
	}
	if !c.enabled(level) {
		return
	}
	lvl := "info"
	if level == AccessLogDebug {
		lvl = "debug"
	}
	c.emit("SUCCESS", lvl, fields, nil)
}

func (c *JobContext) enabled(level AccessLogLevel) bool {
	switch c.opts.Level {
	case "error":
		return false
	case "debug":
		return true
	default: // info
		return level != AccessLogDebug
	}
}

func (c *JobContext) emit(msg, level string, fields map[string]interface{}, err error) {
	if c.opts.Format == "text" {
		kvs := make([]interface{}, 0, len(fields)*2)
		for k, v := range fields {
			if level == "error" && k == "error" {
				continue
			}
			kvs = append(kvs, k, v)
		}
		if level == "error" {
			klog.ErrorS(err, msg, kvs...)
			return
		}
		if level == "debug" {
			klog.V(2).InfoS(msg, kvs...)
			return
		}
		klog.InfoS(msg, kvs...)
		return
	}

	payload := make(map[string]interface{}, len(fields)+3)
	for k, v := range fields {
		payload[k] = v
	}
	payload["msg"] = msg
	payload["level"] = level
	payload["ts"] = time.Now().Format(time.RFC3339Nano)
	b, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		klog.Errorf("marshal job access log failed: %v", marshalErr)
		return
	}
	line := string(b)
	switch level {
	case "error":
		klog.Error(line)
	case "debug":
		klog.V(2).Info(line)
	default:
		klog.Info(line)
	}
}
