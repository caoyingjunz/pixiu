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

package cluster

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type Action struct {
	Act             string `form:"action" binding:"required"`
	ResourceVersion string `form:"resourceVersion" binding:"required"`
}

func (cr *clusterRouter) ReRunJob(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		jobMeta types.PixiuObjectMeta
		action  Action
		err     error
	)
	if err = httputils.ShouldBindAny(c, nil, &jobMeta, &action); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = cr.c.Cluster().ReRunJob(c, jobMeta.Cluster, jobMeta.Namespace, jobMeta.Name, action.ResourceVersion); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
