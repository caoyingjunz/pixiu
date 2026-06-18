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

package datasource

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type meta struct {
	ClusterName  string `uri:"clusterName" binding:"required"`
	DatasourceId int64  `uri:"datasourceId"`
}

type getDefaultDatasourceQuery struct {
	Type model.DatasourceType `form:"type" binding:"required,oneof=0 1"`
}

func (r *router) createDatasource(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		m   meta
		req types.CreateClusterDatasourceRequest
		err error
	)
	if err = c.ShouldBindUri(&m); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Datasource().Create(c, m.ClusterName, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) listDatasources(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = c.ShouldBindUri(&m); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Datasource().List(c, m.ClusterName); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getDefaultDatasource(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		m   meta
		q   getDefaultDatasourceQuery
		err error
	)
	if err = c.ShouldBindUri(&m); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = c.ShouldBindQuery(&q); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Datasource().GetDefault(c, m.ClusterName, q.Type); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) setDefaultDatasource(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = c.ShouldBindUri(&m); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Datasource().SetDefault(c, m.ClusterName, m.DatasourceId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getDatasource(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = c.ShouldBindUri(&m); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Datasource().Get(c, m.ClusterName, m.DatasourceId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) updateDatasource(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		m   meta
		req types.UpdateClusterDatasourceRequest
		err error
	)
	if err = c.ShouldBindUri(&m); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Datasource().Update(c, m.ClusterName, m.DatasourceId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteDatasource(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = c.ShouldBindUri(&m); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Datasource().Delete(c, m.ClusterName, m.DatasourceId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}
