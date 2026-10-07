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
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/pkg/datasource/query"
	"github.com/caoyingjunz/pixiu/pkg/types"
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
	auth, err := p.c.Datasource().GetUpstreamAuth(c, datasourceID)
	if err != nil {
		return ""
	}
	return auth
}

// injectDatasourceLoginCredentials adds stored credentials to form-based login requests.
func (p *proxyRouter) injectDatasourceLoginCredentials(c *gin.Context, matchesTarget func(*types.Datasource) bool) {
	if matchesTarget == nil || c.Request.Method != http.MethodPost || !isDatasourceLoginPath(c.Request.URL.Path) {
		return
	}
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		return
	}
	datasourceID, err := strconv.ParseInt(strings.TrimSpace(c.Request.Header.Get(upstreamDatasourceIDHeader)), 10, 64)
	if err != nil || datasourceID <= 0 {
		return
	}
	datasource, err := p.c.Datasource().Get(c, datasourceID)
	if err != nil || datasource == nil || datasourceURL(datasource) == "" || !matchesTarget(datasource) {
		return
	}
	username, password, err := p.c.Datasource().GetUpstreamCredentials(c, datasourceID)
	if err != nil || username == "" || c.Request.Body == nil {
		return
	}
	body := c.Request.Body
	data, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		return
	}
	values, err := url.ParseQuery(string(data))
	if err != nil {
		return
	}
	values.Set("username", username)
	values.Set("password", password)
	encoded := values.Encode()
	c.Request.Body = io.NopCloser(strings.NewReader(encoded))
	c.Request.ContentLength = int64(len(encoded))
	c.Request.Header.Set("Content-Length", strconv.Itoa(len(encoded)))
}

func isDatasourceLoginPath(path string) bool {
	path = strings.TrimRight(path, "/")
	return strings.HasSuffix(path, "/login")
}

func matchesExternalDatasourceTarget(datasource *types.Datasource, target *url.URL) bool {
	if datasource == nil || !datasource.External || target == nil {
		return false
	}
	configured, err := url.Parse(datasourceURL(datasource))
	if err != nil || configured.Scheme == "" || configured.Hostname() == "" {
		return false
	}
	if !strings.EqualFold(configured.Scheme, target.Scheme) || !strings.EqualFold(configured.Hostname(), target.Hostname()) {
		return false
	}
	if configured.Port() == target.Port() {
		return true
	}
	// Allow a same-host console endpoint on the conventional 8080 port.
	return target.Port() == "8080"
}

func matchesInClusterDatasourceTarget(datasource *types.Datasource, targetPath string) bool {
	if datasource == nil || datasource.External {
		return false
	}
	endpoint, err := query.ParseInClusterEndpoint(datasourceURL(datasource))
	if err != nil {
		return false
	}
	expectedPrefix := fmt.Sprintf("/api/v1/namespaces/%s/services/%s:%d/proxy%s", endpoint.Namespace, endpoint.ServiceName, endpoint.Port, endpoint.BasePath)
	return targetPath == expectedPrefix || strings.HasPrefix(targetPath, expectedPrefix+"/")
}

func datasourceURL(datasource *types.Datasource) string {
	if datasource == nil {
		return ""
	}
	if datasource.Config.Log != nil {
		return strings.TrimSpace(datasource.Config.Log.URL)
	}
	if datasource.Config.Alert != nil {
		return strings.TrimSpace(datasource.Config.Alert.URL)
	}
	return ""
}
