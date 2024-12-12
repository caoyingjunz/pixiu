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

	if r.Result, err = cr.c.Cluster().Helm(helmMeta.Cluster).Releases(helmMeta.Namespace).ListRelease(); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
func (cr *clusterRouter) ListRepositories(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err      error
		helmMeta types.HelmObjectMeta
	)
	if err = c.ShouldBindUri(&helmMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if r.Result, err = cr.c.Cluster().Helm(helmMeta.Cluster).Repositories().List(); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
