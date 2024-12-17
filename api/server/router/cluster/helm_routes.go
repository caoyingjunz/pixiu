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

// GetRelease retrieves a release by its name from the specified namespace and cluster
//
// @Summary get a release by name
// @Description retrieves a release from the specified namespace and cluster using the provided name
// @Tags helm
// @Accept json
// @Produce json
// @Param cluster path string true "Kubernetes cluster name"
// @Param namespace path string true "Kubernetes namespace"
// @Param name path string true "Release name"
// @Success 200 {object} httputils.Response{result=types.Release}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /helm/release/{cluster}/{namespace}/{name} [get]
func (cr *clusterRouter) GetRelease(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		helmMeta types.PixiuObjectMeta
	)
	if err = c.ShouldBindUri(&helmMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if r.Result, err = cr.c.Cluster().Helm(helmMeta.Cluster).Releases(helmMeta.Namespace).GetRelease(c, helmMeta.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// ListReleases retrieves a list of all releases in the specified namespace and cluster
//
// @Summary list all releases
// @Description retrieves a list of all releases from the specified namespace and cluster
// @Tags helm
// @Accept json
// @Produce json
// @Param cluster path string true "Kubernetes cluster name"
// @Param namespace path string true "Kubernetes namespace"
// @Success 200 {object} httputils.Response{result=[]types.Release}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /helm/releases/{cluster}/{namespace} [get]
func (cr *clusterRouter) ListReleases(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		helmMeta types.PixiuObjectMeta
	)
	if err = c.ShouldBindUri(&helmMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if r.Result, err = cr.c.Cluster().Helm(helmMeta.Cluster).Releases(helmMeta.Namespace).ListRelease(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// InstallRelease installs a new release in the specified namespace and cluster
//
// @Summary install a new release
// @Description installs a new release into the specified Kubernetes namespace and cluster
// @Tags helm
// @Accept json
// @Produce json
// @Param cluster path string true "Kubernetes cluster name"
// @Param namespace path string true "Kubernetes namespace"
// @Param body body types.ReleaseForm true "Release information"
// @Success 200 {object} httputils.Response
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /helm/releases/{cluster}/{namespace} [post]
func (cr *clusterRouter) InstallRelease(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		helmMeta   types.PixiuObjectMeta
		releaseOpt types.Release
	)
	if err = httputils.ShouldBindAny(c, &releaseOpt, &helmMeta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if r.Result, err = cr.c.Cluster().Helm(helmMeta.Cluster).Releases(helmMeta.Namespace).InstallRelease(c, &releaseOpt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// UninstallRelease uninstalls a release from the specified namespace and cluster
//
// @Summary uninstall a release
// @Description uninstalls a release from the specified namespace and cluster
// @Tags helm
// @Accept json
// @Produce json
// @Param cluster path string true "Kubernetes cluster name"
// @Param namespace path string true "Kubernetes namespace"
// @Param name path string true "Release name"
// @Success 200 {object} httputils.Response
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /helm/releases/{cluster}/{namespace}/{name} [delete]
func (cr *clusterRouter) UninstallRelease(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		helmMeta types.PixiuObjectMeta
	)
	if err = c.ShouldBindUri(&helmMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if r.Result, err = cr.c.Cluster().Helm(helmMeta.Cluster).Releases(helmMeta.Namespace).UninstallRelease(c, helmMeta.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// UpgradeRelease upgrades a release in the specified namespace and cluster
//
// @Summary upgrade a release
// @Description upgrades a release in the specified namespace and cluster
// @Tags helm
// @Accept json
// @Produce json
// @Param cluster path string true "Kubernetes cluster name"
// @Param namespace path string true "Kubernetes namespace"
// @Param name path string true "Release name"
// @Param body body types.ReleaseForm true "Release information"
// @Success 200 {object} httputils.Response
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /helm/releases/{cluster}/{namespace}/{name} [put]
func (cr *clusterRouter) UpgradeRelease(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		helmMeta   types.PixiuObjectMeta
		releaseOpt types.Release
	)
	if err = httputils.ShouldBindAny(c, &releaseOpt, &helmMeta, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if r.Result, err = cr.c.Cluster().Helm(helmMeta.Cluster).Releases(helmMeta.Namespace).UpgradeRelease(c, &releaseOpt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// GetReleaseHistory retrieves the history of a release in the specified namespace and cluster
//
// @Summary get a release history
// @Description retrieves the history of a release from the specified Kubernetes namespace and cluster
// @Tags helm
// @Accept json
// @Produce json
// @Param cluster path string true "Kubernetes cluster name"
// @Param namespace path string true "Kubernetes namespace"
// @Param name path string true "Release name"
// @Success 200 {object} httputils.Response{result=types.ReleaseHistory}
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /helm/releases/history/{cluster}/{namespace}/{name} [get]
func (cr *clusterRouter) GetReleaseHistory(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		helmMeta types.PixiuObjectMeta
	)
	if err = c.ShouldBindUri(&helmMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if r.Result, err = cr.c.Cluster().Helm(helmMeta.Cluster).Releases(helmMeta.Namespace).GetReleaseHistory(c, helmMeta.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// RollbackRelease rolls back a release in the specified namespace and cluster to a specific revision
//
// @Summary rollback a release
// @Description rolls back a release from the specified Kubernetes namespace and cluster to a specific revision
// @Tags helm
// @Accept json
// @Produce json
// @Param cluster path string true "Kubernetes cluster name"
// @Param namespace path string true "Kubernetes namespace"
// @Param name path string true "Release name"
// @Param version query int true "Release version"
// @Success 200 {object} httputils.Response
// @Failure 400 {object} httputils.Response
// @Failure 404 {object} httputils.Response
// @Failure 500 {object} httputils.Response
// @Router /helm/releases/rollback/{cluster}/{namespace}/{name} [post]
func (cr *clusterRouter) RollbackRelease(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err          error
		helmMeta     types.PixiuObjectMeta
		reverionMeta types.ReleaseHistory
	)
	if err = httputils.ShouldBindAny(c, nil, &helmMeta, &reverionMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err = cr.c.Cluster().Helm(helmMeta.Cluster).Releases(helmMeta.Namespace).RollbackRelease(c, helmMeta.Name, reverionMeta.Version); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
