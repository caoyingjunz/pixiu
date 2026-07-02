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
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	tokenutil "github.com/caoyingjunz/pixiu/pkg/util/token"
)

// Authentication 身份认证
func Authentication(o *options.Options) gin.HandlerFunc {
	keyBytes := []byte(o.ComponentConfig.Default.JWTKey)

	return func(c *gin.Context) {
		// debug 模式，全部放通
		if o.ComponentConfig.Default.Mode.InDebug() {
			// Considered all as root user when running in debug mode.
			root, err := o.Factory.User().GetRoot(c)
			if err != nil {
				httputils.AbortFailedWithCode(c, http.StatusInternalServerError, err)
				return
			}
			httputils.SetUserToContext(c, root)
			return
		}

		// 生产模式，api级别校验
		if alwaysAllowPath.Has(c.Request.URL.Path) || allowCustomRequest(c) {
			return
		}

		roleId, err := parseRoleAndValidClaim(c, o, keyBytes)
		if err != nil {
			httputils.AbortFailedWithCode(c, http.StatusUnauthorized, err)
			return
		}
		if err = o.Controller.User().ValidAccess(c, *roleId); err != nil {
			httputils.AbortFailedWithCode(c, http.StatusForbidden, err)
			return
		}
	}
}

func parseRoleAndValidClaim(c *gin.Context, o *options.Options, keyBytes []byte) (*int64, error) {
	isWs := strings.EqualFold(c.GetHeader("Upgrade"), "websocket")
	token, err := tokenutil.ExtractToken(c, isWs)
	if err != nil {
		return nil, err
	}
	claim, err := tokenutil.ParseToken(token, keyBytes)
	if err != nil {
		return nil, err
	}

	ok, err := o.Controller.User().ValidateLoginToken(c, claim.Id, token)
	if err != nil {
		return nil, fmt.Errorf("未登陆或者密码被修改，请重新登陆")
	}
	if !ok {
		return nil, fmt.Errorf("已被他人登陆")
	}
	user, err := o.Factory.User().Get(c, claim.Id)
	if err != nil || user == nil {
		return nil, fmt.Errorf("无法获取用户")
	}
	httputils.SetUserToContext(c, user)

	roleId := int64(user.Role)
	return &roleId, nil
}
