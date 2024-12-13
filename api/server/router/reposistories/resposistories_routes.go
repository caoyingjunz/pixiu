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

package reposistories

import (
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/gin-gonic/gin"
)

// createReposistories create a reposistories
//
// @Summary create a reposistories
// @Description create a reposistories
// @Tags reposistories
// @Accept  json
// @Produce  json
// @Param   body     body     types.RepoForm  true  "create reposistories"
// @Success 200 {object} httputils.Response
// @Router /reposistories [post]
func (re *reposistoriesRouter) createReposistories(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err      error
		formData types.RepoForm
	)
	if err = c.ShouldBindJSON(&formData); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = re.c.Repositories().Create(c, &formData); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// deleteReposistory deletes a repository by its ID
//
// @Summary delete a repository by ID
// @Description deletes a repository from the system using the provided ID
// @Tags reposistories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Success 200 {object} httputils.Response
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories/{id} [delete]
func (re *reposistoriesRouter) deleteReposistory(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err      error
		repoMeta types.RepoId
	)
	if err = c.ShouldBindUri(&repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = re.c.Repositories().Delete(c, repoMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// updateReposistory updates a repository by its ID
//
// @Summary update a repository by ID
// @Description updates a repository from the system using the provided ID
// @Tags reposistories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Param repository body types.RepUpdateForm true "repository information"
// @Success 200 {object} httputils.Response
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories/{id} [put]
func (re *reposistoriesRouter) updateReposistory(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err      error
		repoMeta types.RepoId
		formData types.RepoUpdateForm
	)
	if err = httputils.ShouldBindAny(c, &formData, &repoMeta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = re.c.Repositories().Update(c, repoMeta.Id, &formData); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// getReposistory retrieves a repository by its ID
//
// @Summary get a repository by ID
// @Description retrieves a repository from the system using the provided ID
// @Tags reposistories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Success 200 {object} httputils.Response{result=types.Repo}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories/{id} [get]
func (re *reposistoriesRouter) getReposistory(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err      error
		repoMeta types.RepoId
	)
	if err = c.ShouldBindUri(&repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = re.c.Repositories().Get(c, repoMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// getReposistoryByName retrieves a repository by its name
//
// @Summary get a repository by name
// @Description retrieves a repository from the system using the provided name
// @Tags reposistories
// @Accept json
// @Produce json
// @Param name path string true "Repository Name"
// @Success 200 {object} httputils.Response{result=types.Repo}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories/name/{name} [get]
func (re *reposistoriesRouter) getReposistoryByName(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err      error
		repoMeta types.RepoName
	)
	if err = c.ShouldBindUri(&repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = re.c.Repositories().GetByName(c, repoMeta.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// listReposistories list all reposistories
//
// @Summary list all reposistories
// @Description list all reposistories
// @Tags reposistories
// @Accept  json
// @Produce  json
// @Success 200 {object} httputils.Response{result=[]types.Repo}
// @Router /reposistories [get]
func (re *reposistoriesRouter) listReposistories(c *gin.Context) {
	r := httputils.NewResponse()
	var err error

	if r.Result, err = re.c.Repositories().List(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// getDetail retrieves detailed information about a repository by its ID
//
// @Summary get detailed repository information by ID
// @Description retrieves detailed information about a repository from the system using the provided ID
// @Tags reposistories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Success 200 {object} httputils.Response{result=model.ChartIndex}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories/{id}/detail [get]

func (re *reposistoriesRouter) getRepoCharts(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		repoMeta types.RepoId
	)

	if err = c.ShouldBindUri(&repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = re.c.Repositories().GetRepoChartsById(c, repoMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// GetRepoChartsByURL retrieves detailed information about a repository by its URL
//
// @Summary get detailed repository information by URL
// @Description retrieves detailed information about a repository from the system using the provided URL
// @Tags reposistories
// @Accept json
// @Produce json
// @Param url path string true "Repository URL"
// @Success 200 {object} httputils.Response{result=model.ChartIndex}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories/url/{url}/detail [get]
func (re *reposistoriesRouter) getRepoChartsByURL(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		repoMeta types.RepoURL
	)

	if err = httputils.ShouldBindAny(c, nil, nil, &repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = re.c.Repositories().GetRepoChartsByURL(c, repoMeta.Url); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
