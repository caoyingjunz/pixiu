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

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	tokenutil "github.com/caoyingjunz/pixiu/pkg/util/token"
)

// Authentication 身份认证
func Authentication(o *options.Options) gin.HandlerFunc {
	keyBytes := []byte(o.ComponentConfig.Default.JWTKey)

	return func(c *gin.Context) {
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

		if alwaysAllowPath.Has(c.Request.URL.Path) || allowCustomRequest(c) {
			return
		}

		if err := validate(c, o, keyBytes); err != nil {
			httputils.AbortFailedWithCode(c, http.StatusUnauthorized, err)
			return
		}
	}
}

func validate(c *gin.Context, o *options.Options, keyBytes []byte) error {
	token, err := extractToken(c, false)
	if err != nil {
		return err
	}
	claim, err := tokenutil.ParseToken(token, keyBytes)
	if err != nil {
		return err
	}

	existToken, err := o.Controller.User().GetLoginToken(c, claim.Id)
	if err != nil {
		return fmt.Errorf("未登陆或者密码被修改，请重新登陆")
	}
	if token != existToken {
		return fmt.Errorf("已被他人登陆")
	}

	user, err := o.Factory.User().Get(c, claim.Id)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.ErrUnauthorized
	}
	httputils.SetUserToContext(c, user)

	return nil
}

// 从请求头中获取 token
func extractToken(c *gin.Context, ws bool) (string, error) {
	emptyFunc := func(t string) bool { return len(t) == 0 }
	if ws {
		wsToken := c.GetHeader("Sec-WebSocket-Protocol")
		if emptyFunc(wsToken) {
			return "", fmt.Errorf("authorization header is not provided")
		}
		return wsToken, nil
	}

	token := c.GetHeader("Authorization")
	if emptyFunc(token) {
		return "", fmt.Errorf("authorization header is not provided")
	}
	fields := strings.Fields(token)
	if len(fields) != 2 {
		return "", fmt.Errorf("invalid authorization header format")
	}
	if fields[0] != "Bearer" {
		return "", fmt.Errorf("unsupported authorization type")
	}

	return fields[1], nil
}
