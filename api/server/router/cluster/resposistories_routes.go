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
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/gin-gonic/gin"
)

// createReposistories creates a new repository
//
// @Summary create a new repository
// @Description creates a new repository from the system
// @Tags reposistories
// @Accept json
// @Produce json
// @Param cluster body types.RepoObjectMeta true "Kubernetes cluster name"
// @Param repoForm body types.RepoForm true "repository information"
// @Success 200 {object} httputils.Response
// @Failure 400 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories [post]
func (re *clusterRouter) createReposistories(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err       error
		formData  types.RepoForm
		pixiuMeta types.RepoObjectMeta
	)

	if err = httputils.ShouldBindAny(c, &formData, &pixiuMeta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = re.c.Cluster().Helm(pixiuMeta.Cluster).Repositories().Create(c, &formData); err != nil {
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
func (re *clusterRouter) deleteReposistory(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err      error
		repoMeta types.RepoId
	)
	if err = c.ShouldBindUri(&repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = re.c.Cluster().Helm(repoMeta.Cluster).Repositories().Delete(c, repoMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// updateReposistory updates a repository by its ID
//
// @Summary update a repository by ID
// @Description updates a repository in the system using the provided ID
// @Tags reposistories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Param repoForm body types.RepoUpdateForm true "repository information"
// @Success 200 {object} httputils.Response
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories/{id} [put]

func (re *clusterRouter) updateReposistory(c *gin.Context) {
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
	if err = re.c.Cluster().Helm(repoMeta.Cluster).Repositories().Update(c, repoMeta.Id, &formData); err != nil {
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
func (re *clusterRouter) getReposistory(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err      error
		repoMeta types.RepoId
	)
	if err = c.ShouldBindUri(&repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = re.c.Cluster().Helm(repoMeta.Cluster).Repositories().Get(c, repoMeta.Id); err != nil {
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
// @Param name path string true "Repository name"
// @Success 200 {object} httputils.Response{result=types.Repo}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories/name/{name} [get]
func (re *clusterRouter) getReposistoryByName(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		err      error
		repoMeta types.RepoName
	)
	if err = c.ShouldBindUri(&repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = re.c.Cluster().Helm(repoMeta.Cluster).Repositories().GetByName(c, repoMeta.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// listReposistories lists all repositories in the specified cluster
//
// @Summary list all repositories
// @Description retrieves a list of all repositories from the specified Kubernetes cluster
// @Tags reposistories
// @Accept json
// @Produce json
// @Param cluster query string true "Kubernetes cluster name"
// @Success 200 {object} httputils.Response{result=[]model.Repositories}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories [get]
func (re *clusterRouter) listReposistories(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err       error
		pixiuMeta types.RepoObjectMeta
	)
	if err = c.ShouldBindUri(&pixiuMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if r.Result, err = re.c.Cluster().Helm(pixiuMeta.Cluster).Repositories().List(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// getRepoCharts retrieves charts of a repository by its ID
//
// @Summary get repository charts by ID
// @Description retrieves charts associated with a repository from the system using the provided ID
// @Tags reposistories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Success 200 {object} httputils.Response{result=model.ChartIndex}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories/{id}/charts [get]
func (re *clusterRouter) getRepoCharts(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		repoMeta types.RepoId
	)

	if err = c.ShouldBindUri(&repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = re.c.Cluster().Helm(repoMeta.Cluster).Repositories().GetRepoChartsById(c, repoMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// getRepoChartsByURL retrieves charts of a repository by its URL
//
// @Summary get repository charts by URL
// @Description retrieves charts associated with a repository from the system using the provided URL
// @Tags reposistories
// @Accept json
// @Produce json
// @Param cluster query string true "Kubernetes cluster name"
// @Param url query string true "Repository URL"
// @Success 200 {object} httputils.Response{result=model.ChartIndex}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories/charts [get]
func (re *clusterRouter) getRepoChartsByURL(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err       error
		repoMeta  types.RepoURL
		pixiuMeta types.RepoObjectMeta
	)

	if err = httputils.ShouldBindAny(c, nil, &pixiuMeta, &repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = re.c.Cluster().Helm(pixiuMeta.Cluster).Repositories().GetRepoChartsByURL(c, repoMeta.Url); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// getChartValues retrieves values of a chart in a repository
//
// @Summary get chart values
// @Description retrieves values of a chart in a repository from the system using the provided chart name and version
// @Tags reposistories
// @Accept json
// @Produce json
// @Param cluster query string true "Kubernetes cluster name"
// @Param chart query string true "Chart name"
// @Param version query string true "Chart version"
// @Success 200 {object} httputils.Response{result=model.ChartValues}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /reposistories/values [get]
func (re *clusterRouter) getChartValues(c *gin.Context) {

	r := httputils.NewResponse()
	var (
		err       error
		repoMeta  types.ChartValues
		pixiuMeta types.RepoObjectMeta
	)

	if err = httputils.ShouldBindAny(c, nil, &pixiuMeta, &repoMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = re.c.Cluster().Helm(pixiuMeta.Cluster).Repositories().GetRepoChartValues(c, repoMeta.Chart, repoMeta.Version); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)

}
