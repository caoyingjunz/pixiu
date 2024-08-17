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

	klog "github.com/sirupsen/logrus"

	"github.com/caoyingjunz/pixiu/pkg/db"
)

const (
	SuccessMsg = "SUCCESS"
	FailMsg    = "FAIL"
)

type JobContext struct {
	context.Context
	StartTime time.Time
	LogEntry  *klog.Entry
}

func NewJobContext(name string) *JobContext {
	return &JobContext{
		Context:   db.WithDBContext(context.Background()),
		StartTime: time.Now(),
		LogEntry:  klog.WithField("job", name),
	}
}

func (c *JobContext) WithLogFields(fields map[string]interface{}) {
	c.LogEntry = c.LogEntry.WithFields(fields)
}

func (c *JobContext) Logger(err error) {
	fields := klog.Fields{
		"latency": fmt.Sprintf("%dµs", time.Since(c.StartTime).Microseconds()),
	}
	if sqls := db.GetSQLs(c); len(sqls) > 0 {
		fields["sqls"] = sqls
	}
	if err != nil {
		fields["error"] = err
		c.LogEntry.WithFields(fields).Error(FailMsg)
		return
	}

	c.LogEntry.WithFields(fields).Info(SuccessMsg)
}
