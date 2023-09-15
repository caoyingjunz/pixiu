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

type IdMeta struct {
	ClusterId int64 `uri:"clusterId" binding:"required"`
}

// CreateCluster godoc
//
//	@Summary      Create a cluster
//	@Description  Create by a json cluster
//	@Tags         Clusters
//	@Accept       json
//	@Produce      json
//	@Param        cluster  body      types.Cluster  true  "Create cluster"
//	@Success      200      {object}  httputils.Response
//	@Failure      400      {object}  httputils.Response
//	@Failure      404      {object}  httputils.Response
//	@Failure      500      {object}  httputils.Response
//	@Router       /pixiu/clusters/ [post]
//	@Security     Bearer
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

// UpdateCluster godoc
//
//	@Summary      Update an cluster
//	@Description  Update by json cluster
//	@Tags         Clusters
//	@Accept       json
//	@Produce      json
//	@Param        clusterId  path      int            true  "Cluster ID"
//	@Param        cluster    body      types.Cluster  true  "Update cluster"
//	@Success      200        {object}  httputils.Response
//	@Failure      400        {object}  httputils.Response
//	@Failure      404        {object}  httputils.Response
//	@Failure      500        {object}  httputils.Response
//	@Router       /pixiu/clusters/{clusterId} [put]
//	@Security     Bearer
func (cr *clusterRouter) updateCluster(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		idMeta IdMeta
		err    error
	)
	if err = c.ShouldBindUri(&idMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	var cluster types.Cluster
	if err = c.ShouldBindJSON(&cluster); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = cr.c.Cluster().Update(c, idMeta.ClusterId, &cluster); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// DeleteCluster godoc
//
//	@Summary      Delete cluster by clusterId
//	@Description  Delete by cloud cluster ID
//	@Tags         Clusters
//	@Accept       json
//	@Produce      json
//	@Param        clusterId  path      int  true  "Cluster ID"
//	@Success      200        {object}  httputils.Response
//	@Failure      400        {object}  httputils.Response
//	@Failure      404        {object}  httputils.Response
//	@Failure      500        {object}  httputils.Response
//	@Router       /pixiu/clusters/{clusterId} [delete]
//	@Security     Bearer
func (cr *clusterRouter) deleteCluster(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		idMeta IdMeta
		err    error
	)
	if err = c.ShouldBindUri(&idMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = cr.c.Cluster().Delete(c, idMeta.ClusterId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// GetCluster godoc
//
//	@Summary      Get Cluster by clusterId
//	@Description  Get by cloud cluster ID
//	@Tags         Clusters
//	@Accept       json
//	@Produce      json
//	@Param        clusterId  path      int  true  "Cluster ID"
//	@Success      200        {object}  httputils.Response{result=types.Cluster}
//	@Failure      400        {object}  httputils.Response
//	@Failure      404        {object}  httputils.Response
//	@Failure      500        {object}  httputils.Response
//	@Router       /pixiu/clusters/{clusterId} [get]
//	@Security     Bearer
func (cr *clusterRouter) getCluster(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		idMeta IdMeta
		err    error
	)
	if err = c.ShouldBindUri(&idMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if r.Result, err = cr.c.Cluster().Get(c, idMeta.ClusterId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// ListClusters godoc
//
//	@Summary      List clusters
//	@Description  List clusters
//	@Tags         Clusters
//	@Accept       json
//	@Produce      json
//	@Success      200  {array}   httputils.Response{result=[]types.Cluster}
//	@Failure      400  {object}  httputils.Response
//	@Failure      404  {object}  httputils.Response
//	@Failure      500  {object}  httputils.Response
//	@Router       /pixiu/clusters [get]
//	@Security     Bearer
func (cr *clusterRouter) listClusters(c *gin.Context) {
	r := httputils.NewResponse()

	var err error
	if r.Result, err = cr.c.Cluster().List(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// PingCluster godoc
//
//	@Summary      Ping cluster
//	@Description  Do ping
//	@Tags         Clusters
//	@Accept       json
//	@Produce      json
//	@Param        clusterId  path      int  true  "Cluster ID"
//	@Success      200        {array}   httputils.Response
//	@Failure      400        {object}  httputils.Response
//	@Failure      404        {object}  httputils.Response
//	@Failure      500        {object}  httputils.Response
//	@Router       /pixiu/clusters/{clusterId}/ping [get]
//	@Security     Bearer
func (cr *clusterRouter) pingCluster(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		cluster types.Cluster
		err     error
	)
	if err = c.ShouldBindJSON(&cluster); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = cr.c.Cluster().Ping(c, cluster.KubeConfig); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
