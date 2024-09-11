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

	"github.com/caoyingjunz/pixiu/pkg/db"
	logutil "github.com/caoyingjunz/pixiu/pkg/util/log"
)

type JobContext struct {
	context.Context
	*logutil.Logger
}

func NewJobContext(name string, cfg *logutil.LogOptions) *JobContext {
	jc := &JobContext{
		Context: db.WithDBContext(context.Background()),
		Logger:  logutil.NewLogger(cfg),
	}
	jc.WithLogField("job", name)
	return jc
}

func (c *JobContext) Log(level logutil.LogLevel, err error) {
	c.Logger.Log(c.Context, level, err)
}
