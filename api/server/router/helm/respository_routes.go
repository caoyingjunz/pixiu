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

// createRepository creates a new repository in the specified cluster
//
// @Summary create a repository
// @Description creates a new repository in the specified Kubernetes cluster
// @Tags repositories
// @Accept json
// @Produce json
// @Param cluster query string true "Kubernetes cluster name"
// @Param body body types.RepoForm true "Repository information"
// @Success 200 {object} httputils.Response
// @Failure 400 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /repositories [post]
func (hr *helmRouter) createRepository(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err error
		req types.CreateRepository
	)
	if err = httputils.ShouldBindAny(c, &req, nil, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = hr.c.Helm().Repository().Create(c, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// deleteRepository deletes a repository by its ID
//
// @Summary delete a repository by ID
// @Description deletes a repository from the system using the provided ID
// @Tags repositories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Success 200 {object} httputils.Response
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /repositories/{id} [delete]
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

// updateRepository updates a repository by its ID
//
// @Summary update a repository by ID
// @Description updates a repository in the system using the provided ID and update information
// @Tags repositories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Param body body types.RepoUpdateForm true "Repository update information"
// @Success 200 {object} httputils.Response
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
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

// getRepository retrieves a repository by its ID
//
// @Summary get a repository by ID
// @Description retrieves a repository from the system using the provided ID
// @Tags repositories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Success 200 {object} httputils.Response{result=types.Repository}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /repositories/{id} [get]
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

// listRepositories retrieves a list of all repositories
//
// @Summary list repositories
// @Description retrieves a list of all repositories in the system
// @Tags repositories
// @Accept json
// @Produce json
// @Success 200 {object} httputils.Response{result=[]types.Repository}
// @Failure 400 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /repositories [get]
func (hr *helmRouter) listRepositories(c *gin.Context) {
	r := httputils.NewResponse()
	var err error

	if r.Result, err = hr.c.Helm().Repository().List(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// getRepoCharts retrieves charts of a repository by its ID
//
// @Summary get repository charts by ID
// @Description retrieves charts associated with a repository from the system using the provided ID
// @Tags repositories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Success 200 {object} httputils.Response{result=model.ChartIndex}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /repositories/{id}/charts [get]
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

// getRepoChartsByURL retrieves charts of a repository by its URL
//
// @Summary get repository charts by URL
// @Description retrieves charts associated with a repository from the system using the provided URL
// @Tags repositories
// @Accept json
// @Produce json
// @Param url query string true "Repository URL"
// @Success 200 {object} httputils.Response{result=model.ChartIndex}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /repositories/charts [get]
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

// getChartValues retrieves the values of a specific chart version
//
// @Summary get chart values
// @Description retrieves values for a specific chart version using the provided chart name and version
// @Tags charts
// @Accept json
// @Produce json
// @Param chart query string true "Chart name"
// @Param version query string true "Chart version"
// @Success 200 {object} httputils.Response{result=types.ChartValues}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /repositories/chartvalues [get]
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
