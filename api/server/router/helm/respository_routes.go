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

package helm

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (hr *helmRouter) createRepository(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err error
		req types.CreateRepository
	)
	if err = httputils.BindCreateRequest(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = hr.c.Helm().Repository().Create(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) deleteRepository(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err      error
		repoMeta types.RepoId
	)
	if err = c.ShouldBindUri(&repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = hr.c.Helm().Repository().Delete(c, repoMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) updateRepository(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err      error
		repoMeta types.RepoId
		formData types.UpdateRepository
	)
	if err = httputils.ShouldBindAny(c, &formData, &repoMeta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = hr.c.Helm().Repository().Update(c, repoMeta.Id, &formData); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) getRepository(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err      error
		repoMeta types.RepoId
	)
	if err = c.ShouldBindUri(&repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = hr.c.Helm().Repository().Get(c, repoMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) listRepositories(c *gin.Context) {
	r := httputils.NewResponse()
	var err error

	if r.Result, err = hr.c.Helm().Repository().List(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) getRepoCharts(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		repoMeta types.RepoId
	)

	if err = c.ShouldBindUri(&repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = hr.c.Helm().Repository().GetChartsById(c, repoMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) getRepoChartsByURL(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		repoMeta types.RepoURL
	)

	if err = httputils.ShouldBindAny(c, nil, nil, &repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = hr.c.Helm().Repository().GetChartsByURL(c, repoMeta.Url); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) getChartValues(c *gin.Context) {

	r := httputils.NewResponse()
	var (
		err      error
		repoMeta types.ChartValues
	)

	if err = httputils.ShouldBindAny(c, nil, nil, &repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = hr.c.Helm().Repository().GetChartValues(c, repoMeta.Chart, repoMeta.Version); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)

}
