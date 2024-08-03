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

package plan

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
)

type taskNodeMeta struct {
	planMeta `json:",inline"`

	TaskId int64 `uri:"taskId" binding:"required"`
}

func (t *planRouter) runTasks(c *gin.Context) {
	r := httputils.NewResponse()

	httputils.SetSuccess(c, r)
}

func (t *planRouter) listTasks(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt   planMeta
		watch WatchMeta
		err   error
	)
	if err = httputils.ShouldBindAny(c, nil, &opt, &watch); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	// 不是长连接请求则直接返回
	if !watch.Watch {
		if r.Result, err = t.c.Plan().ListTasks(c, opt.PlanId); err != nil {
			httputils.SetFailed(c, r, err)
			return
		}
		httputils.SetSuccess(c, r)
		return
	}

	// 长连接请求
	t.c.Plan().WatchTasks(c, opt.PlanId, c.Writer, c.Request)
}

func (t *planRouter) watchTaskLog(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		opt watchTaskLogMeta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &opt, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = t.c.Plan().WatchTaskLog(c, opt.PlanId, opt.TaskId, c.Writer, c.Request); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
}
