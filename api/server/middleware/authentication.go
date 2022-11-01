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

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

// Authentication 身份认证
func Authentication(c *gin.Context) {
	if AlwaysAllowPath.Has(c.Request.URL.Path) {
		return
	}

	r := httputils.NewResponse()
	token := c.GetHeader("Authorization")
	if len(token) == 0 {
		r.SetCode(http.StatusUnauthorized)
		httputils.SetFailed(c, r, fmt.Errorf("authorization header is not provided"))
		c.Abort()
		return
	}

	fields := strings.Fields(token)
	if len(fields) != 2 {
		r.SetCode(http.StatusUnauthorized)
		httputils.SetFailed(c, r, fmt.Errorf("invalid authorization header format"))
		c.Abort()
		return
	}

	if fields[0] != "Bearer" {
		r.SetCode(http.StatusUnauthorized)
		httputils.SetFailed(c, r, fmt.Errorf("unsupported authorization type"))
		c.Abort()
		return
	}

	accessToken := fields[1]
	jwtKey := pixiu.CoreV1.User().GetJWTKey()
	claims, err := httputils.ParseToken(accessToken, jwtKey)
	if err != nil {
		r.SetCode(http.StatusUnauthorized)
		httputils.SetFailed(c, r, err)
		c.Abort()
		return
	}

	c.Set("userId", claims.Id)
}
