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
	"context"
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const upstreamDatasourceIDHeader = "X-Pixiu-Datasource-Id"

// resolveServiceProxyUpstreamAuth 解析集群内 service proxy 的上游 Basic 认证。
// K8s apiserver 的 service proxy 会剥离 Authorization，需经 port-forward 注入认证。
// 优先 X-Pixiu-Datasource-Id（已保存数据源），否则读取 X-Pixiu-Proxy-Authorization（创建/测试前临时认证）。
func (p *proxyRouter) resolveServiceProxyUpstreamAuth(c *gin.Context) string {
	if dsID := strings.TrimSpace(c.Request.Header.Get(upstreamDatasourceIDHeader)); dsID != "" {
		if auth := p.resolveUpstreamAuth(c, dsID); auth != "" {
			return auth
		}
	}
	return strings.TrimSpace(c.Request.Header.Get(externalProxyAuthorizationHeaderKey))
}

func (p *proxyRouter) resolveUpstreamAuth(c *gin.Context, dsIDStr string) string {
	if dsIDStr == "" {
		return ""
	}
	c.Request.Header.Del(upstreamDatasourceIDHeader)

	datasourceID, err := strconv.ParseInt(dsIDStr, 10, 64)
	if err != nil || datasourceID <= 0 {
		return ""
	}
	datasource, err := p.c.Datasource().Get(context.TODO(), datasourceID)
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

	token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return "Basic " + token
}
