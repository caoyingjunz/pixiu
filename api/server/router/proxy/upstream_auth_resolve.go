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

package proxy

import (
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	upstreamDatasourceIDHeader       = "X-Pixiu-Datasource-Id"
	upstreamProxyAuthorizationHeader = "X-Pixiu-Proxy-Authorization"
)

func (p *proxyRouter) resolveUpstreamAuth(c *gin.Context, dsIDStr string) string {
	if dsIDStr == "" {
		// New datasource tests run before the record has an ID. In that case
		// use the credential explicitly supplied by the frontend.
		return takeUpstreamProxyAuth(c)
	}
	if auth := takeUpstreamProxyAuth(c); auth != "" {
		return auth
	}

	datasourceID, err := strconv.ParseInt(dsIDStr, 10, 64)
	if err != nil || datasourceID <= 0 {
		return ""
	}
	datasource, err := p.c.Datasource().Get(c, datasourceID)
	if err != nil || datasource == nil {
		return ""
	}

	var username, password string
	switch datasource.Type {
	case model.DatasourceTypeLog, model.DatasourceTypeMiddleware:
		// 中间件（如 Nacos）的鉴权账号复用 log 配置存储
		if datasource.Config.Log == nil {
			return ""
		}
		username = datasource.Config.Log.UserName
		password = datasource.Config.Log.Password
	case model.DatasourceTypeAlert:
		if datasource.Config.Alert == nil {
			return ""
		}
		username = datasource.Config.Alert.UserName
		password = datasource.Config.Alert.Password
	default:
		return ""
	}
	if username == "" && password == "" {
		return ""
	}

	token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return "Basic " + token
}

func takeUpstreamProxyAuth(c *gin.Context) string {
	auth := strings.TrimSpace(c.Request.Header.Get(upstreamProxyAuthorizationHeader))
	c.Request.Header.Del(upstreamProxyAuthorizationHeader)
	c.Request.Header.Del(upstreamDatasourceIDHeader)
	if strings.HasPrefix(strings.ToLower(auth), "basic ") {
		return auth
	}
	return ""
}
