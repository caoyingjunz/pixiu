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

package cluster

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (cr *clusterRouter) createCluster(c *gin.Context) {
	r := httputils.NewResponse()

	var cluster types.Cluster
	if err := c.ShouldBindJSON(&cluster); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := cr.c.Cluster().Create(c, &cluster); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) updateCluster(c *gin.Context) {
	r := httputils.NewResponse()
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) deleteCluster(c *gin.Context) {
	r := httputils.NewResponse()
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) getCluster(c *gin.Context) {
	r := httputils.NewResponse()
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) listClusters(c *gin.Context) {
	r := httputils.NewResponse()
	httputils.SetSuccess(c, r)
}

func (cr *clusterRouter) pingCluster(c *gin.Context) {
	r := httputils.NewResponse()
	httputils.SetSuccess(c, r)
}
