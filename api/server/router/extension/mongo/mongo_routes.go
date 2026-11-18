/*
Copyright 2026 The Pixiu Authors.

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

package mongo

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type meta struct {
	DatasourceId int64 `uri:"datasourceId"`
}

type slowLogOptions struct {
	Count int64 `form:"count"`
}

type pingMongoRequest struct {
	DatasourceId int64 `json:"datasource_id"`
	types.MongoSourceConfig
}

// pingMongoAdhoc 临时探测：请求体直接传连接配置，不依赖已保存的数据源
func (mr *mongoRouter) pingMongoAdhoc(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req pingMongoRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mongo().PingAdhoc(c, req.DatasourceId, &req.MongoSourceConfig); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mongoRouter) pingMongo(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mongo().Ping(c, m.DatasourceId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mongoRouter) getMongoInfo(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mongo().Info(c, m.DatasourceId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mongoRouter) getMongoReplicaSet(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mongo().ReplicaSet(c, m.DatasourceId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mongoRouter) listMongoSlowLogs(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m    meta
		opts slowLogOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mongo().SlowLogs(c, m.DatasourceId, opts.Count); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mongoRouter) listMongoParams(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mongo().Params(c, m.DatasourceId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
