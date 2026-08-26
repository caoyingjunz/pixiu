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

package cluster

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (cr *clusterRouter) listServicePods(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		optMeta types.PixiuObjectMeta
		err     error
	)
	if err = c.ShouldBindUri(&optMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if optMeta.Name == "" {
		httputils.SetFailed(c, r, errors.NewError(fmt.Errorf("name is required"), http.StatusBadRequest))
		return
	}
	if err = cr.authorizeClusterAccessByName(c, optMeta.Cluster); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	includeNotReady, _ := strconv.ParseBool(c.Query("includeNotReady"))
	if r.Result, err = cr.c.Cluster().ListServicePods(c, optMeta.Cluster, optMeta.Namespace, optMeta.Name, includeNotReady); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
