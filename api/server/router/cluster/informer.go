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

type ResourceMeta struct {
	Cluster   string `uri:"cluster" binding:"required"`
	Resource  string `uri:"resource" binding:"required"`
	Namespace string `uri:"namespace"`
	Name      string `uri:"name"`
}

func (cr *clusterRouter) getIndexerResource(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		resourceMeta ResourceMeta
		err          error
	)
	if err = httputils.ShouldBindAny(c, nil, &resourceMeta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = cr.c.Cluster().GetIndexerResource(c, resourceMeta.Cluster, resourceMeta.Resource, resourceMeta.Namespace, resourceMeta.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) listIndexerResources(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		resourceMeta ResourceMeta
		listOption   types.ListOptions // 分页设置
		err          error
	)
	if err = httputils.ShouldBindAny(c, nil, &resourceMeta, &listOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = cr.c.Cluster().ListIndexerResources(c, resourceMeta.Cluster, resourceMeta.Resource, resourceMeta.Namespace, listOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
