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
	"github.com/caoyingjunz/pixiu/pkg/controller/helm"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (hr *helmRouter) authorizeRelease(c *gin.Context, cluster, namespace string) (helm.ReleaseInterface, error) {
	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		return nil, err
	}
	if _, err = hr.c.Cluster().AuthorizeClusterAccessByName(c, user, cluster); err != nil {
		return nil, err
	}
	return hr.c.Helm().Release(cluster, namespace), nil
}

func (hr *helmRouter) GetRelease(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		helmMeta types.PixiuObjectMeta
	)
	if err = c.ShouldBindUri(&helmMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	releaseAPI, err := hr.authorizeRelease(c, helmMeta.Cluster, helmMeta.Namespace)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = releaseAPI.Get(c, helmMeta.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) ListReleases(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		helmMeta types.PixiuObjectMeta
	)
	if err = c.ShouldBindUri(&helmMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	releaseAPI, err := hr.authorizeRelease(c, helmMeta.Cluster, helmMeta.Namespace)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = releaseAPI.List(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) InstallRelease(c *gin.Context) {
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

	releaseAPI, err := hr.authorizeRelease(c, helmMeta.Cluster, helmMeta.Namespace)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = releaseAPI.Install(c, &releaseOpt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) UninstallRelease(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		helmMeta types.PixiuObjectMeta
	)
	if err = c.ShouldBindUri(&helmMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	releaseAPI, err := hr.authorizeRelease(c, helmMeta.Cluster, helmMeta.Namespace)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = releaseAPI.Uninstall(c, helmMeta.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) UpgradeRelease(c *gin.Context) {
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

	releaseAPI, err := hr.authorizeRelease(c, helmMeta.Cluster, helmMeta.Namespace)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = releaseAPI.Upgrade(c, &releaseOpt); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) GetReleaseHistory(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		helmMeta types.PixiuObjectMeta
	)
	if err = c.ShouldBindUri(&helmMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	releaseAPI, err := hr.authorizeRelease(c, helmMeta.Cluster, helmMeta.Namespace)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = releaseAPI.History(c, helmMeta.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (hr *helmRouter) RollbackRelease(c *gin.Context) {
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

	releaseAPI, err := hr.authorizeRelease(c, helmMeta.Cluster, helmMeta.Namespace)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = releaseAPI.Rollback(c, helmMeta.Name, reverionMeta.Version); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
