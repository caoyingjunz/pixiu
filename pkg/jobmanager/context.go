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
	"fmt"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/accesslog"
	"github.com/caoyingjunz/pixiu/pkg/db"
)

// AccessLogLevel is an alias for accesslog.SuccessTier.
type AccessLogLevel = accesslog.SuccessTier

const (
	AccessLogInfo  = accesslog.TierInfo
	AccessLogDebug = accesslog.TierDebug
)

type JobContext struct {
	context.Context
	opts      accesslog.Options
	startTime time.Time
	fields    map[string]interface{}
}

func NewJobContext(name string, opts *accesslog.Options) *JobContext {
	o := accesslog.DefaultOptions()
	if opts != nil {
		o = *opts
	}
	return &JobContext{
		Context:   db.WithDBContext(context.Background()),
		opts:      o,
		startTime: time.Now(),
		fields:    map[string]interface{}{"job": name},
	}
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
		accesslog.Emit(c.opts, "FAIL", "error", fields, err)
		return
	}
	if !c.opts.AllowSuccess(level) {
		return
	}
	lvl := string(accesslog.TierInfo)
	if level == accesslog.TierDebug {
		lvl = string(accesslog.TierDebug)
	}
	// 定时任务成功访问日志统一走 V(2)，避免默认级别刷屏
	accesslog.EmitV(c.opts, 2, "SUCCESS", lvl, fields, nil)
}
