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

package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/api/server/router/cluster"
	"github.com/caoyingjunz/pixiu/api/server/router/proxy"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	ctrlutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// HTTP method to operation
var operationsMap = map[string]model.Operation{
	http.MethodGet:    model.OpRead,
	http.MethodPost:   model.OpCreate,
	http.MethodPatch:  model.OpUpdate,
	http.MethodPut:    model.OpUpdate,
	http.MethodDelete: model.OpDelete,
}

// Authorization 鉴权
func Authorization(o *options.Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 允许请求直接通过
		if o.ComponentConfig.Default.Mode.InDebug() || alwaysAllowPath.Has(c.Request.URL.Path) || allowCustomRequest(c) {
			return
		}

		user, err := httputils.GetUserFromRequest(c)
		if err != nil {
			httputils.AbortFailedWithCode(c, http.StatusMethodNotAllowed, err)
			return
		}

		switch user.Status {
		case 1:
			// status 为 1，表示用户只读模式, 只读模式只允许查询请求
			if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodOptions {
				httputils.AbortFailedWithCode(c, http.StatusForbidden, fmt.Errorf("无操作权限"))
				return
			}
		case 2:
			// 禁用用户无法进行任何操作
			httputils.AbortFailedWithCode(c, http.StatusForbidden, fmt.Errorf("用户已被禁用"))
			return
		}

		// Proxy path should be skipped now.
		// TODO: get object and ID from proxy path
		if proxy.IsProxyPath(c) || cluster.IsKubeProxyPath(c) || cluster.IsHelmPath(c) {
			return
		}

		obj, id, ok := httputils.GetObjectFromRequest(c)
		if !ok {
			return
		}

		op := operationsMap[c.Request.Method]
		// load policy for consistency
		// ref: https://github.com/casbin/casbin/issues/679#issuecomment-761525328
		if err := o.Enforcer.LoadPolicy(); err != nil {
			httputils.AbortFailedWithCode(c, http.StatusInternalServerError, err)
			return
		}
		ok, err = o.Enforcer.Enforce(user.Name, obj, id, op.String())
		if err != nil {
			httputils.AbortFailedWithCode(c, http.StatusMethodNotAllowed, err)
			return
		}
		if !ok {
			httputils.AbortFailedWithCode(c, http.StatusForbidden, fmt.Errorf("无操作权限"))
		}
		if id != "" {
			return
		}
		// this is a list API
		if err := ctrlutil.SetIdRangeContext(c, o.Enforcer, user, obj); err != nil {
			httputils.AbortFailedWithCode(c, http.StatusInternalServerError, err)
		}
	}
}
